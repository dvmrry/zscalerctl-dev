package redact

import (
	"fmt"
	"math"
	"regexp"
	"strings"
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
		return "", fmt.Errorf("unsupported redaction mode %q", value)
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
	out := string(in)
	report := Report{Counts: make(map[string]int)}
	for _, rule := range baseRules {
		if count := len(rule.re.FindAllStringIndex(out, -1)); count > 0 {
			report.Counts[rule.name] += count
		}
		out = rule.re.ReplaceAllString(out, rule.replacement)
	}
	if r.mode == ModeShare || r.mode == ModeParanoid {
		for _, rule := range shareRules {
			if count := len(rule.re.FindAllStringIndex(out, -1)); count > 0 {
				report.Counts[rule.name] += count
			}
			out = rule.re.ReplaceAllString(out, rule.replacement)
		}
	}
	if len(report.Counts) == 0 {
		report.Counts = nil
	}
	return out, report
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

var baseRules = buildBaseRules()

var highEntropyFreeTextTokenRE = regexp.MustCompile(`\b[A-Za-z0-9][A-Za-z0-9._~+/=-]{31,}\b`)
var canonicalUUIDRE = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var compactUUIDRE = regexp.MustCompile(`(?i)^[0-9a-f]{32}$`)
var publicHexFingerprintRE = regexp.MustCompile(`(?i)^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
var gitSHARE = regexp.MustCompile(`(?i)^[0-9a-f]{40}$`)
var gitSHAContextRE = regexp.MustCompile(`(?i)(?:\b(?:git|commit|sha|revision|rev)\b[\s:=#-]*)$`)

func buildBaseRules() []rule {
	rules := []rule{
		{
			name:        "private_key_block",
			re:          regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`),
			replacement: markerPrivateKey,
		},
		{
			name:        "jwt",
			re:          regexp.MustCompile(`eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}`),
			replacement: markerJWT,
		},
		{
			name:        "zscaler_provisioning_key",
			re:          regexp.MustCompile(`\b[0-9]+\|[A-Za-z0-9.-]+\|[A-Za-z0-9+/=_-]{16,}(?:[A-Za-z0-9+/=_ -]{8,})?`),
			replacement: markerProvisioningKey,
		},
		{
			// An Authorization header value is entirely credential material, so
			// redact all of it (to end of line) regardless of scheme — Bearer,
			// Basic, Token, ApiKey, NTLM, Digest (multi-param), AWS4-HMAC-SHA256,
			// etc. Matching only one scheme/token left non-Bearer/Basic
			// credentials, and Digest's later params, in the clear.
			name:        "authorization_header",
			re:          regexp.MustCompile(`(?i)(authorization\s*[:=]\s*)\S.*`),
			replacement: `${1}` + markerSecret,
		},
		{
			// The password runs to the LAST '@' before the host, so a password
			// containing '@' (e.g. admin:P@ssw0rd@host) is fully redacted. The
			// char class excludes '/' and whitespace, keeping the match inside a
			// single URL's userinfo.
			name:        "credential_url",
			re:          regexp.MustCompile(`(?i)([a-z][a-z0-9+.-]*://)[^/\s:@]+:[^/\s]+@`),
			replacement: `${1}` + markerSecret + `@`,
		},
	}
	rules = append(rules, assignmentRules("provisioning_key_assignment", provisioningAssignmentKeys, markerProvisioningKey)...)
	rules = append(rules, assignmentRules("private_key_assignment", privateKeyAssignmentKeys, markerPrivateKey)...)
	rules = append(rules, assignmentRules("secret_assignment", secretAssignmentKeys, markerSecret)...)
	rules = append(rules, rule{
		name:        "secret_phrase",
		re:          regexp.MustCompile(`(?i)\b(?:` + secretPhraseKeys + `)\s+([A-Za-z0-9._~+/=|:-]{8,})\b`),
		replacement: markerSecret,
	})
	return rules
}

func assignmentRules(name, keys, marker string) []rule {
	key := `["']?(?:` + keys + `)["']?\s*[:=]\s*`
	return []rule{
		{
			name:        name,
			re:          regexp.MustCompile(`(?i)(` + key + `)"(?:\\.|[^"\\])*"`),
			replacement: `${1}"` + marker + `"`,
		},
		{
			name:        name,
			re:          regexp.MustCompile(`(?i)(` + key + `)'(?:\\.|[^'\\])*'`),
			replacement: `${1}'` + marker + `'`,
		},
		{
			name:        name,
			re:          regexp.MustCompile(`(?i)(["']?(?:` + keys + `)["']?\s*:\s*)(-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?|true|false|null)(\s*[,}\]])`),
			replacement: `${1}"` + marker + `"${3}`,
		},
		{
			name:        name,
			re:          regexp.MustCompile(`(?i)(` + key + `)[^<"'\s,}\]\{\[]+`),
			replacement: `${1}` + marker,
		},
	}
}

var shareRules = []rule{
	{
		name:        "email",
		re:          regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`),
		replacement: `<REDACTED:EMAIL>`,
	},
	{
		name:        "ipv4",
		re:          regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),
		replacement: `<REDACTED:IP>`,
	},
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
