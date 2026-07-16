package enginewire

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"
)

func validateJSONObject(data []byte, maximumDepth int) error {
	return validateJSON(data, maximumDepth, true)
}

func validateJSONValue(data []byte, maximumDepth int) error {
	return validateJSON(data, maximumDepth, false)
}

func validateJSON(data []byte, maximumDepth int, requireObject bool) error {
	if !utf8.Valid(data) {
		return ErrInvalidUTF8
	}
	parser := strictJSONParser{data: data, maximumDepth: maximumDepth}
	parser.skipWhitespace()
	if parser.position >= len(parser.data) {
		return fmt.Errorf("%w: empty JSON value", ErrInvalidJSON)
	}
	if requireObject && parser.data[parser.position] != '{' {
		return fmt.Errorf("%w: top-level value must be an object", ErrInvalidJSON)
	}
	if err := parser.parseValue(1); err != nil {
		return err
	}
	parser.skipWhitespace()
	if parser.position != len(parser.data) {
		return parser.fail(ErrInvalidJSON, "trailing data")
	}
	return nil
}

type strictJSONParser struct {
	data         []byte
	position     int
	maximumDepth int
}

func (p *strictJSONParser) parseValue(depth int) error {
	if p.position >= len(p.data) {
		return p.fail(ErrInvalidJSON, "unexpected end of input")
	}
	switch p.data[p.position] {
	case '{':
		return p.parseObject(depth)
	case '[':
		return p.parseArray(depth)
	case '"':
		_, err := p.parseString()
		return err
	case 't':
		return p.parseLiteral("true")
	case 'f':
		return p.parseLiteral("false")
	case 'n':
		return p.parseLiteral("null")
	default:
		return p.parseNumber()
	}
}

func (p *strictJSONParser) parseObject(depth int) error {
	if depth > p.maximumDepth {
		return p.fail(ErrJSONDepth, "object nesting")
	}
	p.position++
	p.skipWhitespace()
	if p.consume('}') {
		return nil
	}
	seen := map[string]struct{}{}
	for {
		if p.position >= len(p.data) || p.data[p.position] != '"' {
			return p.fail(ErrInvalidJSON, "object key must be a string")
		}
		key, err := p.parseString()
		if err != nil {
			return err
		}
		if _, duplicate := seen[key]; duplicate {
			return p.fail(ErrDuplicateKey, "decoded object key")
		}
		seen[key] = struct{}{}
		p.skipWhitespace()
		if !p.consume(':') {
			return p.fail(ErrInvalidJSON, "missing object colon")
		}
		p.skipWhitespace()
		if err := p.parseValue(depth + 1); err != nil {
			return err
		}
		p.skipWhitespace()
		if p.consume('}') {
			return nil
		}
		if !p.consume(',') {
			return p.fail(ErrInvalidJSON, "missing object comma")
		}
		p.skipWhitespace()
	}
}

func (p *strictJSONParser) parseArray(depth int) error {
	if depth > p.maximumDepth {
		return p.fail(ErrJSONDepth, "array nesting")
	}
	p.position++
	p.skipWhitespace()
	if p.consume(']') {
		return nil
	}
	for {
		if err := p.parseValue(depth + 1); err != nil {
			return err
		}
		p.skipWhitespace()
		if p.consume(']') {
			return nil
		}
		if !p.consume(',') {
			return p.fail(ErrInvalidJSON, "missing array comma")
		}
		p.skipWhitespace()
	}
}

func (p *strictJSONParser) parseString() (string, error) {
	start := p.position
	p.position++
	for p.position < len(p.data) {
		value := p.data[p.position]
		switch {
		case value == '"':
			p.position++
			var decoded string
			if err := json.Unmarshal(p.data[start:p.position], &decoded); err != nil {
				return "", p.fail(ErrInvalidJSON, "invalid JSON string")
			}
			return decoded, nil
		case value == '\\':
			p.position++
			if p.position >= len(p.data) {
				return "", p.fail(ErrInvalidJSON, "unterminated string escape")
			}
			escape := p.data[p.position]
			p.position++
			switch escape {
			case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
				continue
			case 'u':
				code, err := p.parseHexCodeUnit()
				if err != nil {
					return "", err
				}
				switch {
				case code >= 0xd800 && code <= 0xdbff:
					if p.position+2 > len(p.data) || p.data[p.position] != '\\' || p.data[p.position+1] != 'u' {
						return "", p.fail(ErrInvalidJSON, "unpaired high surrogate")
					}
					p.position += 2
					low, lowErr := p.parseHexCodeUnit()
					if lowErr != nil {
						return "", lowErr
					}
					if low < 0xdc00 || low > 0xdfff {
						return "", p.fail(ErrInvalidJSON, "unpaired high surrogate")
					}
				case code >= 0xdc00 && code <= 0xdfff:
					return "", p.fail(ErrInvalidJSON, "unpaired low surrogate")
				}
			default:
				return "", p.fail(ErrInvalidJSON, "unsupported string escape")
			}
		case value < 0x20:
			return "", p.fail(ErrInvalidJSON, "unescaped string control")
		case value < utf8.RuneSelf:
			p.position++
		default:
			_, size := utf8.DecodeRune(p.data[p.position:])
			p.position += size
		}
	}
	return "", p.fail(ErrInvalidJSON, "unterminated string")
}

func (p *strictJSONParser) parseHexCodeUnit() (uint16, error) {
	if p.position+4 > len(p.data) {
		return 0, p.fail(ErrInvalidJSON, "short Unicode escape")
	}
	var value uint16
	for range 4 {
		value <<= 4
		b := p.data[p.position]
		p.position++
		switch {
		case b >= '0' && b <= '9':
			value |= uint16(b - '0')
		case b >= 'a' && b <= 'f':
			value |= uint16(b-'a') + 10
		case b >= 'A' && b <= 'F':
			value |= uint16(b-'A') + 10
		default:
			return 0, p.fail(ErrInvalidJSON, "invalid Unicode escape")
		}
	}
	return value, nil
}

func (p *strictJSONParser) parseLiteral(literal string) error {
	if len(p.data)-p.position < len(literal) || string(p.data[p.position:p.position+len(literal)]) != literal {
		return p.fail(ErrInvalidJSON, "invalid literal")
	}
	p.position += len(literal)
	return nil
}

func (p *strictJSONParser) parseNumber() error {
	start := p.position
	if p.consume('-') && p.position >= len(p.data) {
		return p.fail(ErrInvalidJSON, "incomplete number")
	}
	if p.consume('0') {
		if p.position < len(p.data) && p.data[p.position] >= '0' && p.data[p.position] <= '9' {
			return p.fail(ErrInvalidJSON, "number has a leading zero")
		}
	} else {
		if p.position >= len(p.data) || p.data[p.position] < '1' || p.data[p.position] > '9' {
			return p.fail(ErrInvalidJSON, "invalid number integer part")
		}
		for p.position < len(p.data) && p.data[p.position] >= '0' && p.data[p.position] <= '9' {
			p.position++
		}
	}
	if p.consume('.') {
		if p.position >= len(p.data) || p.data[p.position] < '0' || p.data[p.position] > '9' {
			return p.fail(ErrInvalidJSON, "invalid number fraction")
		}
		for p.position < len(p.data) && p.data[p.position] >= '0' && p.data[p.position] <= '9' {
			p.position++
		}
	}
	if p.position < len(p.data) && (p.data[p.position] == 'e' || p.data[p.position] == 'E') {
		p.position++
		if p.position < len(p.data) && (p.data[p.position] == '+' || p.data[p.position] == '-') {
			p.position++
		}
		if p.position >= len(p.data) || p.data[p.position] < '0' || p.data[p.position] > '9' {
			return p.fail(ErrInvalidJSON, "invalid number exponent")
		}
		for p.position < len(p.data) && p.data[p.position] >= '0' && p.data[p.position] <= '9' {
			p.position++
		}
	}
	if p.position == start {
		return p.fail(ErrInvalidJSON, "invalid value")
	}
	return nil
}

func (p *strictJSONParser) skipWhitespace() {
	for p.position < len(p.data) && (p.data[p.position] == ' ' || p.data[p.position] == '\t') {
		p.position++
	}
}

func (p *strictJSONParser) consume(value byte) bool {
	if p.position < len(p.data) && p.data[p.position] == value {
		p.position++
		return true
	}
	return false
}

func (p *strictJSONParser) fail(kind error, detail string) error {
	return fmt.Errorf("%w at byte %d: %s", kind, p.position, detail)
}
