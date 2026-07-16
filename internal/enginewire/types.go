package enginewire

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Protocol and schema constants pin the immutable bootstrap and v1 contract.
const (
	Protocol = "zscalerctl.engine.stdio"

	BootstrapSchemaID     = "urn:zscalerctl:engine-stdio:bootstrap:1"
	BootstrapSchemaSHA256 = "e559b5568159568978b8b6fdaef9a75cd4022d2cefd1895338e890b7633720cf"
	V1SchemaID            = "urn:zscalerctl:engine-stdio:protocol:1"
	V1SchemaSHA256        = "6cba5a8170e538bd6eacde38c84526873f691421d6dc5f57cacfbd5f9438c522"
	V1Version             = "1"

	BootstrapFrameBytes = 64 << 10
	BootstrapJSONDepth  = 8
	V1FrameBytes        = 1 << 20
	V1JSONDepth         = 64
	AggregateItemBytes  = 64 << 20
	FragmentChunkBytes  = 512 << 10
	MaxSafeInteger      = 1<<53 - 1

	MaxURLCount              = 1024
	MaxReadFieldCount        = 1024
	MaxReadFilterCount       = 1024
	MaxProductSelectorCount  = 16
	MaxResourceSelectorCount = 4096
	MaxPathBytes             = 32768
	MaxControlStringBytes    = 8192
)

// Capability identifies one closed engine operation family.
type Capability string

const (
	CapabilityEngineManifest Capability = "engine.manifest"
	CapabilityCatalogSchema  Capability = "catalog.schema"
	CapabilityStatusInspect  Capability = "status.inspect"
	CapabilityZIAURLLookup   Capability = "zia.url_lookup"
	CapabilityResourcesRead  Capability = "resources.read"
	CapabilityDumpWrite      Capability = "dump.write"
	CapabilityDiffCompare    Capability = "diff.compare"
)

// Operation identifies one action within a capability.
type Operation string

const (
	OperationManifest     Operation = "manifest"
	OperationList         Operation = "list"
	OperationGet          Operation = "get"
	OperationShow         Operation = "show"
	OperationDoctor       Operation = "doctor"
	OperationAuthStatus   Operation = "auth_status"
	OperationConfigStatus Operation = "config_status"
	OperationLookup       Operation = "lookup"
	OperationDump         Operation = "dump"
	OperationDiff         Operation = "diff"
)

// Product identifies one supported Zscaler product namespace.
type Product string

const (
	ProductZIA       Product = "zia"
	ProductZPA       Product = "zpa"
	ProductZTW       Product = "ztw"
	ProductZCC       Product = "zcc"
	ProductZIdentity Product = "zidentity"
)

// Redaction identifies one supported output-scrubbing mode.
type Redaction string

const (
	RedactionStandard Redaction = "standard"
	RedactionShare    Redaction = "share"
	RedactionParanoid Redaction = "paranoid"
)

// ItemKind discriminates one semantic item payload.
type ItemKind string

const (
	ItemCatalogResource   ItemKind = "catalog_resource"
	ItemURLClassification ItemKind = "url_classification"
	ItemProjectedRecord   ItemKind = "projected_record"
	ItemDiffResource      ItemKind = "diff_resource"
	ItemDiffAdded         ItemKind = "diff_added"
	ItemDiffRemoved       ItemKind = "diff_removed"
	ItemDiffFieldChange   ItemKind = "diff_field_change"
)

// SafeInteger is an exact nonnegative integer within JavaScript's safe range.
type SafeInteger uint64

// Uint64 returns the validated integer value.
func (n SafeInteger) Uint64() uint64 { return uint64(n) }

func (n SafeInteger) MarshalJSON() ([]byte, error) {
	if uint64(n) > MaxSafeInteger {
		return nil, fmt.Errorf("%w: integer %d exceeds JavaScript safe range", ErrInvalidFrame, n)
	}
	return []byte(strconv.FormatUint(uint64(n), 10)), nil
}

func (n *SafeInteger) UnmarshalJSON(data []byte) error {
	if n == nil {
		return fmt.Errorf("%w: nil safe-integer destination", ErrInvalidFrame)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("%w: structural integer: %v", ErrInvalidFrame, err)
	}
	number, ok := value.(json.Number)
	if !ok {
		return fmt.Errorf("%w: structural integer is not a JSON number", ErrInvalidFrame)
	}
	parsed, err := parseSafeIntegerLexeme(number.String())
	if err != nil {
		return err
	}
	*n = SafeInteger(parsed)
	return nil
}

func parseSafeIntegerLexeme(lexeme string) (uint64, error) {
	if !validJSONNumberLexeme(lexeme) {
		return 0, fmt.Errorf("%w: structural integer has invalid JSON number syntax", ErrInvalidFrame)
	}
	negative := strings.HasPrefix(lexeme, "-")
	if negative {
		lexeme = lexeme[1:]
	}
	exponent := 0
	if index := strings.IndexAny(lexeme, "eE"); index >= 0 {
		exponent = parseBoundedExponent(lexeme[index+1:])
		lexeme = lexeme[:index]
	}
	fractionDigits := 0
	if index := strings.IndexByte(lexeme, '.'); index >= 0 {
		fractionDigits = len(lexeme) - index - 1
		lexeme = lexeme[:index] + lexeme[index+1:]
	}
	if strings.Trim(lexeme, "0") == "" {
		return 0, nil
	}
	if negative {
		return 0, fmt.Errorf("%w: structural number is negative", ErrInvalidFrame)
	}
	shift := exponent - fractionDigits
	if shift < 0 {
		trim := -shift
		if trim > len(lexeme) || strings.Trim(lexeme[len(lexeme)-trim:], "0") != "" {
			return 0, fmt.Errorf("%w: structural number is not integral", ErrInvalidFrame)
		}
		lexeme = lexeme[:len(lexeme)-trim]
	} else if shift > 0 {
		if shift > 16 || len(strings.TrimLeft(lexeme, "0"))+shift > 16 {
			return 0, fmt.Errorf("%w: structural integer exceeds JavaScript safe range", ErrInvalidFrame)
		}
		lexeme += strings.Repeat("0", shift)
	}
	lexeme = strings.TrimLeft(lexeme, "0")
	if lexeme == "" {
		return 0, nil
	}
	if len(lexeme) > 16 {
		return 0, fmt.Errorf("%w: structural integer exceeds JavaScript safe range", ErrInvalidFrame)
	}
	value, err := strconv.ParseUint(lexeme, 10, 64)
	if err != nil || value > MaxSafeInteger {
		return 0, fmt.Errorf("%w: structural integer exceeds JavaScript safe range", ErrInvalidFrame)
	}
	return value, nil
}

func parseBoundedExponent(lexeme string) int {
	sign := 1
	if strings.HasPrefix(lexeme, "+") {
		lexeme = lexeme[1:]
	} else if strings.HasPrefix(lexeme, "-") {
		sign = -1
		lexeme = lexeme[1:]
	}
	value := 0
	for _, digit := range []byte(lexeme) {
		if value > 1000 {
			return sign * 1001
		}
		value = value*10 + int(digit-'0')
	}
	return sign * value
}

func validatePositive(name string, value SafeInteger) error {
	if value == 0 || uint64(value) > MaxSafeInteger {
		return fmt.Errorf("%w: %s must be in 1..%d", ErrInvalidFrame, name, MaxSafeInteger)
	}
	return nil
}

func validateSafe(name string, value SafeInteger) error {
	if uint64(value) > MaxSafeInteger {
		return fmt.Errorf("%w: %s must be in 0..%d", ErrInvalidFrame, name, MaxSafeInteger)
	}
	return nil
}

func validateCapabilityOperation(capability Capability, operation Operation) error {
	valid := false
	switch capability {
	case CapabilityEngineManifest:
		valid = operation == OperationManifest
	case CapabilityCatalogSchema:
		valid = operation == OperationList
	case CapabilityStatusInspect:
		valid = operation == OperationDoctor || operation == OperationAuthStatus || operation == OperationConfigStatus
	case CapabilityZIAURLLookup:
		valid = operation == OperationLookup
	case CapabilityResourcesRead:
		valid = operation == OperationList || operation == OperationGet || operation == OperationShow
	case CapabilityDumpWrite:
		valid = operation == OperationDump
	case CapabilityDiffCompare:
		valid = operation == OperationDiff
	}
	if !valid {
		return fmt.Errorf("%w: invalid capability/operation pair %q/%q", ErrInvalidFrame, capability, operation)
	}
	return nil
}

func validateProduct(name string, product Product) error {
	switch product {
	case ProductZIA, ProductZPA, ProductZTW, ProductZCC, ProductZIdentity:
		return nil
	default:
		return fmt.Errorf("%w: %s is not a known product", ErrInvalidFrame, name)
	}
}

func validateRedaction(name string, redaction Redaction) error {
	switch redaction {
	case RedactionStandard, RedactionShare, RedactionParanoid:
		return nil
	default:
		return fmt.Errorf("%w: %s is not a known redaction mode", ErrInvalidFrame, name)
	}
}

func validateStructuralString(name, value string, minimumRunes, maximumRunes, maximumBytes int) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("%w: %s is not valid UTF-8", ErrInvalidFrame, name)
	}
	runes := utf8.RuneCountInString(value)
	if runes < minimumRunes || (maximumRunes > 0 && runes > maximumRunes) {
		return fmt.Errorf("%w: %s has invalid character length", ErrInvalidFrame, name)
	}
	if maximumBytes > 0 && len(value) > maximumBytes {
		return fmt.Errorf("%w: %s exceeds byte limit", ErrInvalidFrame, name)
	}
	for _, r := range value {
		if r <= 0x1f || (r >= 0x7f && r <= 0x9f) || unicode.Is(unicode.Cf, r) {
			return fmt.Errorf("%w: %s contains a forbidden control or format rune", ErrInvalidFrame, name)
		}
	}
	return nil
}

func validateControlString(name, value string) error {
	return validateStructuralString(name, value, 0, MaxControlStringBytes, MaxControlStringBytes)
}

func validateResourceName(name, value string) error {
	return validateStructuralString(name, value, 1, 128, MaxControlStringBytes)
}

func validateFieldName(name, value string) error {
	return validateStructuralString(name, value, 1, 256, MaxControlStringBytes)
}

func validatePath(name, value string) error {
	return validateStructuralString(name, value, 1, MaxPathBytes, MaxPathBytes)
}

func validateVersionToken(value string) error {
	if len(value) < 1 || len(value) > 32 {
		return fmt.Errorf("%w: version token length", ErrInvalidFrame)
	}
	for i, b := range []byte(value) {
		valid := b >= 'A' && b <= 'Z' || b >= 'a' && b <= 'z' || b >= '0' && b <= '9'
		if i > 0 {
			valid = valid || b == '.' || b == '_' || b == '-'
		}
		if !valid {
			return fmt.Errorf("%w: invalid version token", ErrInvalidFrame)
		}
	}
	return nil
}

func validateUniqueStrings(name string, values []string, maximum int, validate func(string, string) error) error {
	if values == nil {
		return fmt.Errorf("%w: %s must be an array", ErrInvalidFrame, name)
	}
	if len(values) > maximum {
		return fmt.Errorf("%w: %s has too many entries", ErrInvalidFrame, name)
	}
	seen := make(map[string]struct{}, len(values))
	for i, value := range values {
		if err := validate(fmt.Sprintf("%s[%d]", name, i), value); err != nil {
			return err
		}
		if _, ok := seen[value]; ok {
			return fmt.Errorf("%w: %s contains a duplicate", ErrInvalidFrame, name)
		}
		seen[value] = struct{}{}
	}
	return nil
}
