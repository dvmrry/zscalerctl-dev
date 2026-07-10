package redact

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Mode string

const (
	ModeStandard Mode = "standard"
	ModeShare    Mode = "share"
	ModeParanoid Mode = "paranoid"
)

func ParseMode(value string) (Mode, error) {
	switch Mode(strings.ToLower(strings.TrimSpace(value))) {
	case ModeStandard:
		return ModeStandard, nil
	case ModeShare:
		return ModeShare, nil
	case ModeParanoid:
		return ModeParanoid, nil
	default:
		return "", fmt.Errorf("unsupported redaction mode; supported: standard, share, paranoid")
	}
}

func EffectiveMode(mode Mode) Mode {
	if mode == "" {
		return ModeStandard
	}
	return mode
}

type Redactor struct {
	mode Mode
}

type Report struct {
	Counts map[string]int `json:"counts,omitempty"`
}

func (r Report) Empty() bool {
	return len(r.Counts) == 0
}

func New(mode Mode) Redactor {
	return Redactor{mode: EffectiveMode(mode)}
}

func (r Redactor) Mode() Mode {
	return r.mode
}

func (r Redactor) Bytes(in []byte) []byte {
	out, _ := r.ScanString(string(in))
	return []byte(out)
}

func (r Redactor) String(in string) string {
	out, _ := r.ScanString(in)
	return out
}

func (r Redactor) ScanString(in string) (string, Report) {
	view := prefilterText{text: in}
	if containsFold("authorization").match(&view) {
		// The plain-text Authorization rule intentionally consumes the rest of a
		// header value. Keep that regex inside decoded JSON string tokens so it
		// cannot consume quotes, object members, or NDJSON record boundaries.
		if json.Valid([]byte(in)) {
			return r.scanJSON(in)
		}
		if out, report, ok := r.scanNDJSON(in); ok {
			return out, report
		}
	}
	return r.scanPlainString(in)
}

func (r Redactor) scanPlainString(in string) (string, Report) {
	out := in
	var report Report
	out, report = scanRules(out, report, baseRules)
	if r.mode == ModeShare || r.mode == ModeParanoid {
		out, report = scanRules(out, report, shareRules)
	}
	return out, report
}

func (r Redactor) scanJSON(in string) (string, Report) {
	return scanJSONDocument(in, func(value string) (string, Report) {
		out := value
		var report Report
		out, report = scanRules(out, report, jsonBaseRules)
		if r.mode == ModeShare || r.mode == ModeParanoid {
			out, report = scanRules(out, report, shareRules)
		}
		return out, report
	})
}

func (r Redactor) scanNDJSON(in string) (string, Report, bool) {
	return scanNDJSONDocuments(in, r.scanJSON)
}

// scanNDJSONDocuments recognizes a stream only when it contains at least two
// complete JSON documents, one per line. This keeps ordinary multi-line text on
// the existing plain-text path while protecting the CLI's buffered NDJSON pass.
func scanNDJSONDocuments(in string, scanDocument jsonDocumentScanner) (string, Report, bool) {
	var out strings.Builder
	out.Grow(len(in))
	var report Report
	documents := 0

	for remaining := in; remaining != ""; {
		line := remaining
		remaining = ""
		if newline := strings.IndexByte(line, '\n'); newline >= 0 {
			line, remaining = line[:newline+1], line[newline+1:]
		}

		body, ending := line, ""
		if strings.HasSuffix(body, "\n") {
			body, ending = strings.TrimSuffix(body, "\n"), "\n"
			if strings.HasSuffix(body, "\r") {
				body, ending = strings.TrimSuffix(body, "\r"), "\r\n"
			}
		}
		if strings.TrimSpace(body) == "" {
			out.WriteString(line)
			continue
		}
		if !json.Valid([]byte(body)) {
			return "", Report{}, false
		}
		redacted, lineReport := scanDocument(body)
		report = mergeReports(report, lineReport)
		out.WriteString(redacted)
		out.WriteString(ending)
		documents++
	}

	if documents < 2 {
		return "", Report{}, false
	}
	return out.String(), report, true
}

func scanRules(out string, report Report, rules []rule) (string, Report) {
	view := prefilterText{text: out}
	for _, rule := range rules {
		out, report = scanRule(out, report, rule, &view)
	}
	return out, report
}

func scanRule(out string, report Report, rule rule, view *prefilterText) (string, Report) {
	if !rule.prefilter.match(view) {
		return out, report
	}
	count := len(rule.re.FindAllStringIndex(out, -1))
	if count == 0 {
		return out, report
	}
	if report.Counts == nil {
		report.Counts = make(map[string]int)
	}
	report.Counts[rule.name] += count
	out = rule.re.ReplaceAllString(out, rule.replacement)
	*view = prefilterText{text: out}
	return out, report
}

func jsonStringEnd(in string, start int) int {
	for i := start + 1; i < len(in); i++ {
		switch in[i] {
		case '\\':
			i++
		case '"':
			return i + 1
		}
	}
	return -1
}

type jsonDocumentScanner func(string) (string, Report)
type jsonStringScanner func(string) (string, Report)

type jsonStringToken struct {
	start int
	end   int
	value string
}

type jsonSensitiveValue struct {
	start    int
	end      int
	ruleName string
	marker   string
}

type jsonReplacement struct {
	start    int
	end      int
	value    string
	priority int
}

type jsonDocumentParser struct {
	in              string
	pos             int
	strings         []jsonStringToken
	sensitiveValues []jsonSensitiveValue
}

// scanJSONDocument rewrites decoded string tokens and sensitive scalar values
// while retaining the original document's whitespace, key order, and container
// structure. The caller has already established that in is valid JSON; parser
// or rewrite invariant failures return a valid, fail-closed marker document.
func scanJSONDocument(in string, scanString jsonStringScanner) (string, Report) {
	parser := jsonDocumentParser{in: in}
	if !parser.parseDocument() {
		return encodeJSONString(markerSecret), Report{}
	}

	replacements := make([]jsonReplacement, 0, len(parser.strings)+len(parser.sensitiveValues))
	var report Report
	for _, token := range parser.strings {
		value, tokenReport := scanString(token.value)
		report = mergeReports(report, tokenReport)
		if value != token.value {
			replacements = append(replacements, jsonReplacement{
				start: token.start,
				end:   token.end,
				value: encodeJSONString(value),
			})
		}
	}
	for _, sensitive := range parser.sensitiveValues {
		report = addReportCount(report, sensitive.ruleName, 1)
		replacements = append(replacements, jsonReplacement{
			start:    sensitive.start,
			end:      sensitive.end,
			value:    encodeJSONString(sensitive.marker),
			priority: 1,
		})
	}

	sort.Slice(replacements, func(i, j int) bool {
		left, right := replacements[i], replacements[j]
		if left.start != right.start {
			return left.start < right.start
		}
		if left.end != right.end {
			return left.end > right.end
		}
		return left.priority > right.priority
	})

	var out strings.Builder
	out.Grow(len(in))
	last := 0
	for _, replacement := range replacements {
		if replacement.start < last {
			continue
		}
		out.WriteString(in[last:replacement.start])
		out.WriteString(replacement.value)
		last = replacement.end
	}
	out.WriteString(in[last:])
	result := out.String()
	if !json.Valid([]byte(result)) {
		return encodeJSONString(markerSecret), report
	}
	return result, report
}

func mergeReports(dst, src Report) Report {
	for name, count := range src.Counts {
		dst = addReportCount(dst, name, count)
	}
	return dst
}

func addReportCount(report Report, name string, count int) Report {
	if count == 0 {
		return report
	}
	if report.Counts == nil {
		report.Counts = make(map[string]int)
	}
	report.Counts[name] += count
	return report
}

func (p *jsonDocumentParser) parseDocument() bool {
	if _, _, ok := p.parseValue(); !ok {
		return false
	}
	p.skipSpace()
	return p.pos == len(p.in)
}

func (p *jsonDocumentParser) parseValue() (int, int, bool) {
	p.skipSpace()
	start := p.pos
	if start >= len(p.in) {
		return 0, 0, false
	}

	var ok bool
	switch p.in[p.pos] {
	case '"':
		_, ok = p.parseString()
	case '{':
		ok = p.parseObject()
	case '[':
		ok = p.parseArray()
	default:
		for p.pos < len(p.in) && !isJSONValueDelimiter(p.in[p.pos]) {
			p.pos++
		}
		ok = p.pos > start
	}
	return start, p.pos, ok
}

func (p *jsonDocumentParser) parseObject() bool {
	p.pos++
	p.skipSpace()
	if p.consume('}') {
		return true
	}

	for {
		key, ok := p.parseString()
		if !ok {
			return false
		}
		p.skipSpace()
		if !p.consume(':') {
			return false
		}
		valueStart, valueEnd, ok := p.parseValue()
		if !ok {
			return false
		}
		// Match the existing assignment-rule surface: scalar values are replaced,
		// while structured values continue to be scanned recursively by token.
		structuredValue := p.in[valueStart] == '{' || p.in[valueStart] == '['
		if ruleName, marker, sensitive := classifyJSONKey(key.value); sensitive && !structuredValue {
			p.sensitiveValues = append(p.sensitiveValues, jsonSensitiveValue{
				start:    valueStart,
				end:      valueEnd,
				ruleName: ruleName,
				marker:   marker,
			})
		}

		p.skipSpace()
		if p.consume('}') {
			return true
		}
		if !p.consume(',') {
			return false
		}
		p.skipSpace()
	}
}

func (p *jsonDocumentParser) parseArray() bool {
	p.pos++
	p.skipSpace()
	if p.consume(']') {
		return true
	}
	for {
		if _, _, ok := p.parseValue(); !ok {
			return false
		}
		p.skipSpace()
		if p.consume(']') {
			return true
		}
		if !p.consume(',') {
			return false
		}
		p.skipSpace()
	}
}

func (p *jsonDocumentParser) parseString() (jsonStringToken, bool) {
	if p.pos >= len(p.in) || p.in[p.pos] != '"' {
		return jsonStringToken{}, false
	}
	start := p.pos
	end := jsonStringEnd(p.in, start)
	if end < 0 {
		return jsonStringToken{}, false
	}
	var value string
	if err := json.Unmarshal([]byte(p.in[start:end]), &value); err != nil {
		return jsonStringToken{}, false
	}
	token := jsonStringToken{start: start, end: end, value: value}
	p.strings = append(p.strings, token)
	p.pos = end
	return token, true
}

func (p *jsonDocumentParser) skipSpace() {
	for p.pos < len(p.in) {
		switch p.in[p.pos] {
		case ' ', '\t', '\r', '\n':
			p.pos++
		default:
			return
		}
	}
}

func (p *jsonDocumentParser) consume(want byte) bool {
	if p.pos >= len(p.in) || p.in[p.pos] != want {
		return false
	}
	p.pos++
	return true
}

func isJSONValueDelimiter(ch byte) bool {
	switch ch {
	case ' ', '\t', '\r', '\n', ',', ']', '}':
		return true
	default:
		return false
	}
}

func encodeJSONString(value string) string {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value) // bytes.Buffer cannot return a write error.
	return strings.TrimSuffix(out.String(), "\n")
}

// ScanRenderedString applies the standard scanners plus a conservative
// high-entropy token check for strings that are about to be rendered.
func (r Redactor) ScanRenderedString(in string) (string, Report) {
	return r.scanStringWithEntropy(in, highEntropyStructured)
}

// ScanFreeText applies rendered-string scanning to administrator-controlled
// text fields. Kept as a named API because free-text fields remain the highest
// risk place for accidental bare credential paste.
func (r Redactor) ScanFreeText(in string) (string, Report) {
	return r.scanStringWithEntropy(in, highEntropyFreeText)
}

type highEntropyContext int

const (
	highEntropyFreeText highEntropyContext = iota
	highEntropyStructured
)

func (r Redactor) scanStringWithEntropy(in string, context highEntropyContext) (string, Report) {
	out, report := r.ScanString(in)
	matches := highEntropyFreeTextTokenRE.FindAllStringIndex(out, -1)
	if len(matches) == 0 {
		return out, report
	}

	var b strings.Builder
	last := 0
	count := 0
	for _, match := range matches {
		if !shouldRedactHighEntropyToken(out, match[0], match[1], context, r.mode) {
			continue
		}
		b.WriteString(out[last:match[0]])
		b.WriteString(markerSecret)
		last = match[1]
		count++
	}
	if count == 0 {
		return out, report
	}
	b.WriteString(out[last:])
	if report.Counts == nil {
		report.Counts = make(map[string]int)
	}
	report.Counts["high_entropy_rendered_token"] += count
	return b.String(), report
}

type rule struct {
	name        string
	re          *regexp.Regexp
	replacement string
	// prefilter is a cheap necessary-condition gate. It must return false only
	// when the regex cannot match; the regex remains the authority.
	prefilter rulePrefilter
}

type rulePrefilter struct {
	kind     prefilterKind
	needle   string
	needles  []string
	children []rulePrefilter
}

type prefilterKind uint8

const (
	prefilterNone prefilterKind = iota
	prefilterContains
	prefilterContainsFold
	prefilterContainsAnyFold
	prefilterAll
)

func (p rulePrefilter) match(text *prefilterText) bool {
	switch p.kind {
	case prefilterNone:
		return true
	case prefilterContains:
		return strings.Contains(text.text, p.needle)
	case prefilterContainsFold:
		return text.containsFold(p.needle)
	case prefilterContainsAnyFold:
		return text.containsAnyFold(p.needles)
	case prefilterAll:
		for _, child := range p.children {
			if !child.match(text) {
				return false
			}
		}
		return true
	default:
		return true
	}
}

const prefilterLowercaseThreshold = 1024

type prefilterText struct {
	text       string
	ascii      bool
	asciiValid bool
	lower      string
	lowerValid bool
}

func (p *prefilterText) isASCII() bool {
	if !p.asciiValid {
		p.ascii = isASCII(p.text)
		p.asciiValid = true
	}
	return p.ascii
}

func (p *prefilterText) lowerText() string {
	if !p.lowerValid {
		p.lower = strings.ToLower(p.text)
		p.lowerValid = true
	}
	return p.lower
}

func (p *prefilterText) containsFold(needle string) bool {
	if !p.isASCII() {
		return containsFoldUnicode(p.text, needle)
	}
	if len(p.text) >= prefilterLowercaseThreshold {
		return strings.Contains(p.lowerText(), needle)
	}
	return containsFoldASCII(p.text, needle)
}

func (p *prefilterText) containsAnyFold(needles []string) bool {
	if !p.isASCII() {
		for _, needle := range needles {
			if containsFoldUnicode(p.text, needle) {
				return true
			}
		}
		return false
	}
	if len(p.text) >= prefilterLowercaseThreshold {
		lower := p.lowerText()
		for _, needle := range needles {
			if strings.Contains(lower, needle) {
				return true
			}
		}
		return false
	}
	for _, needle := range needles {
		if containsFoldASCII(p.text, needle) {
			return true
		}
	}
	return false
}

const (
	markerSecret          = `<REDACTED:SECRET>`
	markerPrivateKey      = `<REDACTED:PRIVATE_KEY>`
	markerJWT             = `<REDACTED:JWT>`
	markerProvisioningKey = `<REDACTED:PROVISIONING_KEY>`

	provisioningAssignmentKeys = `provision(?:ing)?[_ -]?key|provision[_ -]?token|enrollment[_ -]?token|oauth[_ -]?2[_ -]?enrollment[_ -]?token`
	privateKeyAssignmentKeys   = `ssh[_-]?private[_-]?key|private[_-]?key|certBlob|zrsaencryptedprivatekey|zrsaencryptedsessionkey`
	secretAssignmentKeys       = `authorization|cookie|set[_-]?cookie|session(?:[_-]?id)?|client[_-]?secret|secret|secret[_-]?key|key[_-]?secret|api[_-]?key|api[_-]?token|sandbox[_-]?api[_-]?token|auth[_-]?token|authentication[_-]?token|hec[_-]?token|password|vnc[_-]?password|ssh[_-]?passphrase|ssh[_-]?private[_-]?key[_-]?passphrase|passphrase|psk|pre[_ -]?shared[_ -]?key|shared[_ -]?secret|refresh[_-]?token|access[_-]?token|bearer[_-]?token|jwt[_-]?token|jwt|token|otp|one[_-]?time[_-]?password|temporary[_-]?password`
	secretPhraseKeys           = `client[_ -]?secret|secret[_ -]?key|key[_ -]?secret|api[_ -]?key|api[_ -]?token|sandbox[_ -]?api[_ -]?token|bearer[_ -]?token|refresh[_ -]?token|access[_ -]?token|jwt[_ -]?token|auth[_ -]?token|hec[_ -]?token|psk|pre[_ -]?shared[_ -]?key|shared[_ -]?secret|provision(?:ing)?[_ -]?key|provision[_ -]?token|enrollment[_ -]?token|passphrase|private[_ -]?key|device[_ -]?token|one[_ -]?time[_ -]?token|one[_ -]?time[_ -]?password|temporary[_ -]?password|otp` // #nosec G101 -- redaction keyword patterns (field-name matchers), not a secret
)

var authorizationHeaderRE = regexp.MustCompile(`(?i)(authorization\s*[:=]\s*)\S.*`)
var jsonAuthorizationHeaderRE = regexp.MustCompile(`(?is)(authorization\s*[:=]\s*)\S.*`)

var baseRules = buildBaseRules()
var jsonBaseRules = buildJSONBaseRules()

var jsonProvisioningAssignmentKeyRE = regexp.MustCompile(`(?i)^(?:` + provisioningAssignmentKeys + `)$`)
var jsonPrivateKeyAssignmentKeyRE = regexp.MustCompile(`(?i)^(?:` + privateKeyAssignmentKeys + `)$`)
var jsonSecretAssignmentKeyRE = regexp.MustCompile(`(?i)^(?:` + secretAssignmentKeys + `)$`)

var highEntropyFreeTextTokenRE = regexp.MustCompile(`\b[A-Za-z0-9][A-Za-z0-9._~+/=-]{31,}\b`)
var canonicalUUIDRE = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var compactUUIDRE = regexp.MustCompile(`(?i)^[0-9a-f]{32}$`)
var publicHexFingerprintRE = regexp.MustCompile(`(?i)^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
var gitSHARE = regexp.MustCompile(`(?i)^[0-9a-f]{40}$`)
var gitSHAContextRE = regexp.MustCompile(`(?i)(?:\b(?:git|commit|sha|revision|rev)\b[\s:=#-]*)$`)

func buildJSONBaseRules() []rule {
	rules := append([]rule(nil), baseRules...)
	for i := range rules {
		if rules[i].name == "authorization_header" {
			rules[i].re = jsonAuthorizationHeaderRE
			break
		}
	}
	return rules
}

func classifyJSONKey(key string) (string, string, bool) {
	switch {
	case jsonProvisioningAssignmentKeyRE.MatchString(key):
		return "provisioning_key_assignment", markerProvisioningKey, true
	case jsonPrivateKeyAssignmentKeyRE.MatchString(key):
		return "private_key_assignment", markerPrivateKey, true
	case jsonSecretAssignmentKeyRE.MatchString(key):
		return "secret_assignment", markerSecret, true
	default:
		return "", "", false
	}
}

func buildBaseRules() []rule {
	rules := []rule{
		{
			name:        "private_key_block",
			re:          regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`),
			replacement: markerPrivateKey,
			prefilter:   containsFold("private key"),
		},
		{
			name:        "jwt",
			re:          regexp.MustCompile(`eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}`),
			replacement: markerJWT,
			prefilter:   contains("eyJ"),
		},
		{
			name:        "zscaler_provisioning_key",
			re:          regexp.MustCompile(`\b[0-9]+\|[A-Za-z0-9.-]+\|[A-Za-z0-9+/=_-]{16,}(?:[A-Za-z0-9+/=_ -]{8,})?`),
			replacement: markerProvisioningKey,
			prefilter:   contains("|"),
		},
		{
			// An Authorization header value is entirely credential material, so
			// redact all of it (to end of line) regardless of scheme — Bearer,
			// Basic, Token, ApiKey, NTLM, Digest (multi-param), AWS4-HMAC-SHA256,
			// etc. Matching only one scheme/token left non-Bearer/Basic
			// credentials, and Digest's later params, in the clear.
			name:        "authorization_header",
			re:          authorizationHeaderRE,
			replacement: `${1}` + markerSecret,
			prefilter:   containsFold("authorization"),
		},
		{
			// The password runs to the LAST '@' before the host, so a password
			// containing '@' (e.g. admin:P@ssw0rd@host) is fully redacted. The
			// char class excludes '/' and whitespace, keeping the match inside a
			// single URL's userinfo.
			name:        "credential_url",
			re:          regexp.MustCompile(`(?i)([a-z][a-z0-9+.-]*://)[^/\s:@]+:[^/\s]+@`),
			replacement: `${1}` + markerSecret + `@`,
			prefilter:   all(contains("://"), contains("@")),
		},
	}
	rules = append(rules, assignmentRules("provisioning_key_assignment", provisioningAssignmentKeys, markerProvisioningKey)...)
	rules = append(rules, assignmentRules("private_key_assignment", privateKeyAssignmentKeys, markerPrivateKey)...)
	rules = append(rules, assignmentRules("secret_assignment", secretAssignmentKeys, markerSecret)...)
	rules = append(rules, rule{
		name:        "secret_phrase",
		re:          regexp.MustCompile(`(?i)\b(?:` + secretPhraseKeys + `)\s+([A-Za-z0-9._~+/=|:-]{8,})\b`),
		replacement: markerSecret,
		prefilter:   prefilterForAssignmentKeys(secretPhraseKeys),
	})
	return rules
}

func assignmentRules(name, keys, marker string) []rule {
	key := `["']?(?:` + keys + `)["']?\s*[:=]\s*`
	prefilter := prefilterForAssignmentKeys(keys)
	return []rule{
		{
			name:        name,
			re:          regexp.MustCompile(`(?i)(` + key + `)"(?:\\.|[^"\\])*"`),
			replacement: `${1}"` + marker + `"`,
			prefilter:   prefilter,
		},
		{
			name:        name,
			re:          regexp.MustCompile(`(?i)(` + key + `)'(?:\\.|[^'\\])*'`),
			replacement: `${1}'` + marker + `'`,
			prefilter:   prefilter,
		},
		{
			name:        name,
			re:          regexp.MustCompile(`(?i)(["']?(?:` + keys + `)["']?\s*:\s*)(-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?|true|false|null)(\s*[,}\]])`),
			replacement: `${1}"` + marker + `"${3}`,
			prefilter:   prefilter,
		},
		{
			name:        name,
			re:          regexp.MustCompile(`(?i)(` + key + `)[^<"'\s,}\]\{\[]+`),
			replacement: `${1}` + marker,
			prefilter:   prefilter,
		},
	}
}

var shareRules = []rule{
	{
		name:        "email",
		re:          regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`),
		replacement: `<REDACTED:EMAIL>`,
		prefilter:   contains("@"),
	},
	{
		name:        "ipv4",
		re:          regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),
		replacement: `<REDACTED:IP>`,
		prefilter:   contains("."),
	},
}

func prefilterForAssignmentKeys(keys string) rulePrefilter {
	switch keys {
	case provisioningAssignmentKeys:
		return containsAnyFold("provision", "enrollment", "oauth")
	case privateKeyAssignmentKeys:
		return containsAnyFold("private", "certblob", "zrsa")
	default:
		return containsAnyFold(
			"authorization",
			"cookie",
			"session",
			"secret",
			"key",
			"api",
			"auth",
			"token",
			"password",
			"passphrase",
			"psk",
			"shared",
			"bearer",
			"jwt",
			"otp",
			"hec",
			"provision",
			"enrollment",
			"private",
			"device",
		)
	}
}

func contains(needle string) rulePrefilter {
	return rulePrefilter{kind: prefilterContains, needle: needle}
}

func containsFold(needle string) rulePrefilter {
	return rulePrefilter{kind: prefilterContainsFold, needle: needle}
}

func containsAnyFold(needles ...string) rulePrefilter {
	return rulePrefilter{kind: prefilterContainsAnyFold, needles: needles}
}

func all(filters ...rulePrefilter) rulePrefilter {
	return rulePrefilter{kind: prefilterAll, children: filters}
}

func containsFoldASCII(text, needle string) bool {
	if needle == "" {
		return true
	}
	if len(needle) > len(text) {
		return false
	}
	first := lowerASCII(needle[0])
	for i := 0; i <= len(text)-len(needle); i++ {
		if lowerASCII(text[i]) != first {
			continue
		}
		if equalFoldASCIIAt(text, needle, i) {
			return true
		}
	}
	return false
}

func containsFoldUnicode(text, needle string) bool {
	if needle == "" {
		return true
	}
	for i := 0; i < len(text); {
		if hasFoldedNeedleAt(text, needle, i) {
			return true
		}
		_, size := utf8.DecodeRuneInString(text[i:])
		i += size
	}
	return false
}

func hasFoldedNeedleAt(text, needle string, offset int) bool {
	for i := 0; i < len(needle); i++ {
		if offset >= len(text) {
			return false
		}
		r, size := utf8.DecodeRuneInString(text[offset:])
		if !foldsToASCII(r, needle[i]) {
			return false
		}
		offset += size
	}
	return true
}

func foldsToASCII(r rune, needle byte) bool {
	target := rune(lowerASCII(needle))
	for folded := r; ; {
		if folded == target {
			return true
		}
		folded = unicode.SimpleFold(folded)
		if folded == r {
			return false
		}
	}
}

func equalFoldASCIIAt(text, needle string, offset int) bool {
	for i := 0; i < len(needle); i++ {
		if lowerASCII(text[offset+i]) != lowerASCII(needle[i]) {
			return false
		}
	}
	return true
}

func lowerASCII(ch byte) byte {
	if ch >= 'A' && ch <= 'Z' {
		return ch + ('a' - 'A')
	}
	return ch
}

func isASCII(text string) bool {
	for i := 0; i < len(text); i++ {
		if text[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

func shouldRedactHighEntropyToken(text string, start, end int, context highEntropyContext, mode Mode) bool {
	token := text[start:end]
	if canonicalUUIDRE.MatchString(token) {
		return false
	}
	if context == highEntropyStructured {
		if compactUUIDRE.MatchString(token) || publicHexFingerprintRE.MatchString(token) {
			return mode != ModeStandard
		}
	}
	if gitSHARE.MatchString(token) && hasGitSHAContext(text, start) {
		return false
	}
	return looksLikeHighEntropySecret(token)
}

func hasGitSHAContext(text string, start int) bool {
	contextStart := start - 32
	if contextStart < 0 {
		contextStart = 0
	}
	return gitSHAContextRE.MatchString(text[contextStart:start])
}

func looksLikeHighEntropySecret(token string) bool {
	if len(token) < 32 {
		return false
	}

	var lower, upper, digit, other bool
	for _, ch := range token {
		switch {
		case ch >= 'a' && ch <= 'z':
			lower = true
		case ch >= 'A' && ch <= 'Z':
			upper = true
		case ch >= '0' && ch <= '9':
			digit = true
		default:
			other = true
		}
	}
	if !(digit && (lower || upper || other)) {
		return false
	}

	return shannonEntropy(token) >= 3.5
}

func shannonEntropy(value string) float64 {
	var counts [256]int
	for i := 0; i < len(value); i++ {
		counts[value[i]]++
	}

	total := float64(len(value))
	entropy := 0.0
	for _, count := range counts {
		if count == 0 {
			continue
		}
		p := float64(count) / total
		entropy -= p * math.Log2(p)
	}
	return entropy
}
