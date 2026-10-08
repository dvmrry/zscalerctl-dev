package redact

import (
	"encoding/base64"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Pasted credentials that the assignment and generic label rules cannot bound:
// passwords containing punctuation, CLI and PowerShell credential arguments,
// Markdown table cells and other separators, values on the line after their
// label, values placed before their label, percent-encoded query signatures,
// HTTP Basic credentials, and JSON name/value credential pairs. Every rule
// needs both a credential label (or command syntax that only carries a
// secret) and a value that passes a shape check, and the scanner only adds
// redactions to what the earlier rules produced.

type pasteSpan struct {
	start int
	end   int
}

// maxPasteValueLen caps how far a value is followed, keeping each label's
// inspection bounded and the whole scan linear.
const maxPasteValueLen = 256

var pasteCredentialNeedles = []string{
	"pass", "pwd", "kennwort", "contrase", "key", "token", "secret", "creden",
	"sig", "auth", "basic", "api", "jeton", "clave", "schl", "/user:", "-p",
	"curl",
}

var pasteCredentialUnicodeNeedles = []string{"パスワード", "トークン", "キー"}

// pasteCredentialMayMatch is a necessary condition for every paste rule: each
// one requires one of these label words or command fragments.
func pasteCredentialMayMatch(in string) bool {
	view := prefilterText{text: in}
	if view.containsAnyFold(pasteCredentialNeedles) {
		return true
	}
	if view.isASCII() {
		return false
	}
	for _, needle := range pasteCredentialUnicodeNeedles {
		if strings.Contains(in, needle) {
			return true
		}
	}
	return false
}

func scanPasteCredentials(in string) (string, int) {
	if !pasteCredentialMayMatch(in) {
		return in, 0
	}
	var spans []pasteSpan
	collectPastePasswordLabels(in, &spans)
	collectPasteKeyLabels(in, &spans)
	collectPasteNextLineValues(in, &spans)
	collectPasteYAMLNameValues(in, &spans)
	collectPasteCommandCredentials(in, &spans)
	collectPasteEncodedQueryValues(in, &spans)
	collectPasteBasicCredentials(in, &spans)
	collectPasteValuesBeforeLabels(in, &spans)
	return applyPasteSpans(in, spans)
}

func applyPasteSpans(in string, spans []pasteSpan) (string, int) {
	if len(spans) == 0 {
		return in, 0
	}
	sort.Slice(spans, func(i, j int) bool {
		if spans[i].start != spans[j].start {
			return spans[i].start < spans[j].start
		}
		return spans[i].end > spans[j].end
	})
	var b strings.Builder
	last, count := 0, 0
	for _, span := range spans {
		if span.start < last || span.end <= span.start || span.end > len(in) ||
			strings.Contains(in[span.start:span.end], "<REDACTED") {
			continue
		}
		b.WriteString(in[last:span.start])
		b.WriteString(markerSecret)
		last = span.end
		count++
	}
	if count == 0 {
		return in, 0
	}
	b.WriteString(in[last:])
	return b.String(), count
}

// Password-class labels, including common localized forms and the "PASS"
// suffix of snake_case setting names ("DB_PASS", "smtp_pass"); see
// pastePasswordLabelAt.
var pastePasswordLabelRE = regexp.MustCompile(`(?i)password|passwd|passphrase|passwort|kennwort|pwd|mot de passe|contraseña|パスワード|pass`)

// pastePasswordLabelAt reports whether a pastePasswordLabelRE match is a
// label: a whole word (or CamelCase suffix), and a bare "pass" only as the
// last part of a snake_case name, so prose ("mountain pass", "first pass")
// is not a label.
func pastePasswordLabelAt(in string, start, end int) bool {
	if !pasteLabelBoundary(in, start, end) {
		return false
	}
	return !strings.EqualFold(in[start:end], "pass") || (start > 0 && in[start-1] == '_')
}

// Key-class labels the generic label rule does not name (localized labels,
// "auth", "SecretID"), plus the generic words it does, for separators it
// does not accept (arrows, dashes, full-width colon, table cells, <code>).
var pasteKeyLabelRE = regexp.MustCompile(`(?i)api[ -]?schlüssel|zugangsschlüssel|clé (?:api|d'accès)|clave de api|jeton(?: d'accès)?|credencial|credentials?|secret[ _-]?id|secret|token|key|auth|apiキー|トークン`)

// Words before a "key" label that name a database, PKI or tagging concept
// rather than a credential ("Primary key → <uuid>", "Tag key | Owner").
var pasteNonCredentialKeyQualifiers = map[string]bool{
	"primary": true, "foreign": true, "partition": true, "row": true,
	"public": true, "signing": true, "encryption": true, "cache": true,
	"tag": true, "sort": true, "object": true, "record": true,
	"idempotency": true, "resource": true, "asset": true, "host": true,
	"registry": true, "lookup": true, "hash": true, "composite": true,
	"unique": true, "index": true, "routing": true, "shard": true,
	"surrogate": true, "natural": true, "candidate": true, "dedup": true,
}

// Words that mark prose about password handling rather than a password
// ("Password policy is ...", "Password reset guide: ...").
var pastePasswordPolicyTerms = []string{
	"policy", "guidance", "guide", "hash", "algorithm", "baseline", "reset",
	"history", "complexity", "expir", "management", "manager", "protocol",
	"standard", "reference", "schedule", "source", "document", "minimum",
	"maximum", "audit", "team", "rollout", "rotation", "task", "export",
	"breach", "specification", "format", "profile", "lockout", "interval",
	"configuration", "project", "file", "length", "requirement", "vault",
	"stored", "change", "incident", "request", "ticket", "output", "runbook",
}

type pasteSeparatorKind int

const (
	pasteSeparatorNone pasteSeparatorKind = iota
	pasteSeparatorDirect
	pasteSeparatorWhitespace
	pasteSeparatorTable
	pasteSeparatorNextLine
)

func collectPastePasswordLabels(in string, spans *[]pasteSpan) {
	lineEnd := -1 // cached: labels arrive in order, so each line is measured once
	for _, label := range pastePasswordLabelRE.FindAllStringIndex(in, -1) {
		if !pastePasswordLabelAt(in, label[0], label[1]) {
			continue
		}
		if label[1] > lineEnd {
			lineEnd = pasteLineEnd(in, label[1])
		}
		start, kind := pasteSeparator(in, label[0], label[1], lineEnd)
		if kind == pasteSeparatorNextLine {
			continue // handled by collectPasteNextLineValues
		}
		if kind != pasteSeparatorNone {
			if kind == pasteSeparatorDirect && strings.EqualFold(in[label[0]:label[1]], "passphrase") {
				if words := pastePassphraseWordsRE.FindStringIndex(in[start:lineEnd]); words != nil {
					*spans = append(*spans, pasteSpan{start, start + words[1]})
					continue
				}
			}
			valueStart, valueEnd, quoted := pastePasswordValueSpan(in, start, lineEnd)
			if valueEnd > valueStart {
				value := in[valueStart:valueEnd]
				ok := pastePasswordValue(value, false)
				// After plain whitespace the next word is usually prose
				// ("Password policy**: ..."): require a quoted value, password
				// punctuation, or an all-digit trial password.
				if kind == pasteSeparatorWhitespace && !quoted &&
					(!pasteStrongPassword(value) && !isAllDigits(value) || pastePasswordPolicyContext(value)) {
					ok = false
				}
				if ok && pasteValueIsMetadata(in, label[0], label[1], valueStart, valueEnd, true) {
					continue
				}
				if ok {
					*spans = append(*spans, pasteSpan{valueStart, valueEnd})
					continue
				}
			}
		}
		// Natural phrasing ("password for the trial account is X"): a value
		// with password punctuation or key shape, never in policy prose.
		phrase := genericCredentialPhraseRE.FindStringIndex(in[label[1]:lineEnd])
		if phrase == nil || pastePasswordPolicyContext(in[label[1]:label[1]+phrase[1]]) {
			continue
		}
		valueStart, valueEnd, _ := pastePasswordValueSpan(in, label[1]+phrase[1], lineEnd)
		if valueEnd > valueStart && pastePasswordValue(in[valueStart:valueEnd], true) &&
			!pasteValueIsMetadata(in, label[0], label[1], valueStart, valueEnd, true) {
			*spans = append(*spans, pasteSpan{valueStart, valueEnd})
		}
	}
}

var pastePassphraseWordsRE = regexp.MustCompile(`^(?:[A-Z][a-z]+[ \t]+){2,7}[A-Z][a-z]+[!?#$%&*0-9]+`)

func collectPasteKeyLabels(in string, spans *[]pasteSpan) {
	lineEnd := -1 // cached: labels arrive in order, so each line is measured once
	for _, label := range pasteKeyLabelRE.FindAllStringIndex(in, -1) {
		if !pasteLabelBoundary(in, label[0], label[1]) || pasteNonCredentialKeyLabel(in, label[0], label[1]) ||
			jsonPairKeyIdentifierAt(in, label[0], label[1]) {
			continue
		}
		if label[1] > lineEnd {
			lineEnd = pasteLineEnd(in, label[1])
		}
		start, kind := pasteSeparator(in, label[0], label[1], lineEnd)
		if kind == pasteSeparatorDirect || kind == pasteSeparatorTable {
			if valueStart, valueEnd := pasteKeyValueSpan(in, start, lineEnd); valueEnd > valueStart &&
				credentialShapedValue(in[valueStart:valueEnd], false) {
				if !pasteValueIsMetadata(in, label[0], label[1], valueStart, valueEnd, false) {
					*spans = append(*spans, pasteSpan{valueStart, valueEnd})
				}
				continue
			}
		}
		if kind == pasteSeparatorNextLine {
			continue
		}
		// Natural phrasing for labels the generic rule does not know
		// ("Credencial del portal: X").
		phrase := genericCredentialPhraseRE.FindStringIndex(in[label[1]:lineEnd])
		if phrase == nil {
			continue
		}
		if valueStart, valueEnd := pasteKeyValueSpan(in, label[1]+phrase[1], lineEnd); valueEnd > valueStart &&
			credentialShapedValue(in[valueStart:valueEnd], false) &&
			!pasteValueIsMetadata(in, label[0], label[1], valueStart, valueEnd, false) {
			*spans = append(*spans, pasteSpan{valueStart, valueEnd})
		}
	}
}

// pasteLabelBoundary rejects a label word inside a longer word ("monkey",
// "keyboard", "password-file", "passwords"), except the end of a CamelCase
// identifier ("NewPassword", "serviceKey"). Japanese labels need no boundary.
func pasteLabelBoundary(in string, start, end int) bool {
	first, _ := utf8.DecodeRuneInString(in[start:])
	if first < utf8.RuneSelf && start > 0 {
		r, _ := utf8.DecodeLastRuneInString(in[:start])
		if (unicode.IsLetter(r) || unicode.IsDigit(r)) && !camelCaseLabelStart(in, start) {
			return false
		}
	}
	if end < len(in) {
		r, _ := utf8.DecodeRuneInString(in[end:])
		if r == '-' {
			return strings.HasPrefix(in[end:], "->")
		}
		last, _ := utf8.DecodeLastRuneInString(in[:end])
		if r == '_' || (last < utf8.RuneSelf && (unicode.IsLetter(r) || unicode.IsDigit(r))) {
			return false
		}
	}
	return true
}

// pasteNonCredentialKeyLabel reports a "key" label whose modifier names a data
// or metadata key: the generic label rule's nonCredentialKeyModifiers
// ("HotKey", "Tag key") or the paste rule's own qualifiers ("Encryption key").
func pasteNonCredentialKeyLabel(in string, start, end int) bool {
	if !strings.EqualFold(in[start:end], "key") {
		return false
	}
	if nonCredentialKeyModifiers[labelModifierBefore(in, start)] {
		return true
	}
	before := strings.TrimRight(in[:start], " \t_-*`\"'")
	i := len(before)
	for i > 0 && isASCIILetter(rune(before[i-1])) {
		i--
	}
	modifier := strings.ToLower(before[i:])
	return pasteNonCredentialKeyQualifiers[modifier] || nonCredentialKeyModifiers[modifier]
}

// pasteValueIsMetadata applies the generic label rule's metadata predicates
// to a value found after a label: words between them that name metadata
// ("Key fingerprint is", "Function key binding:", "Password hash ..."), with
// an immediate noun required for password or credential-prefixed labels, or a
// value that is a public identifier, digest or revision named by its context.
func pasteValueIsMetadata(in string, labelStart, labelEnd, valueStart, valueEnd int, password bool) bool {
	if valueStart > labelEnd && phraseNamesMetadata(in[labelEnd:valueStart],
		password || credentialKeyModifiers[labelModifierBefore(in, labelStart)]) {
		return true
	}
	return isBarePublicIdentifier(in[valueStart:valueEnd]) || isContextualPublicValue(in, valueStart, valueEnd)
}

func pasteLineEnd(in string, from int) int {
	if newline := strings.IndexByte(in[from:], '\n'); newline >= 0 {
		end := from + newline
		if end > from && in[end-1] == '\r' {
			end--
		}
		return end
	}
	return len(in)
}

func skipPasteBlanks(in string, i, end int) int {
	for i < end && (in[i] == ' ' || in[i] == '\t') {
		i++
	}
	return i
}

var pasteSeparators = []string{"->", "→", "—", "–", "：", ":", "="}
var pasteCueWords = []string{"is", "was", "ist", "est", "es"}

// pasteSeparator classifies what follows a label: a ":"/"="/arrow/dash or
// "is"/"ist"/"est" cue (direct), a Markdown table cell boundary (table),
// nothing but the end of the line (next line), or plain whitespace.
func pasteSeparator(in string, labelStart, labelEnd, lineEnd int) (int, pasteSeparatorKind) {
	i := labelEnd
	for n := 0; n < 3 && i < lineEnd && strings.IndexByte("*_\"'`", in[i]) >= 0; n++ {
		i++
	}
	afterWrappers := i
	if j := skipPasteBlanks(in, i, lineEnd); j < lineEnd && in[j] == '(' {
		if closing := strings.IndexByte(in[j:pasteBound(j, lineEnd, 49)], ')'); closing > 0 {
			i = j + closing + 1
		}
	}
	j := skipPasteBlanks(in, i, lineEnd)
	if j == lineEnd {
		return lineEnd, pasteSeparatorNextLine
	}
	if in[j] == '|' {
		if !pasteTableLabelCell(in, labelStart) {
			return 0, pasteSeparatorNone
		}
		return skipPasteBlanks(in, j+1, lineEnd), pasteSeparatorTable
	}
	for _, separator := range pasteSeparators {
		if strings.HasPrefix(in[j:lineEnd], separator) {
			k := skipPasteBlanks(in, j+len(separator), lineEnd)
			if k == lineEnd || pasteYAMLBlockIndicator(in[k:lineEnd]) {
				if separator == ":" || separator == "：" {
					return lineEnd, pasteSeparatorNextLine
				}
				return 0, pasteSeparatorNone
			}
			return k, pasteSeparatorDirect
		}
	}
	if j == afterWrappers {
		return 0, pasteSeparatorNone // a word follows the label directly
	}
	lower := strings.ToLower(in[j:pasteBound(j, lineEnd, 4)])
	for _, cue := range pasteCueWords {
		if strings.HasPrefix(lower, cue) && len(lower) > len(cue) && (lower[len(cue)] == ' ' || lower[len(cue)] == '\t') {
			return skipPasteBlanks(in, j+len(cue), lineEnd), pasteSeparatorDirect
		}
	}
	return j, pasteSeparatorWhitespace
}

func pasteYAMLBlockIndicator(rest string) bool {
	switch strings.TrimSpace(rest) {
	case "|", "|-", "|+", ">", ">-", ">+":
		return true
	}
	return false
}

// pasteTableLabelCell reports whether the label starts a short Markdown table
// cell: only pipes, emphasis and at most three words precede it on its line.
func pasteTableLabelCell(in string, labelStart int) bool {
	window := labelStart - 64
	if window < 0 {
		window = 0
	}
	cell := in[window:labelStart]
	if newline := strings.LastIndexByte(cell, '\n'); newline >= 0 {
		cell = cell[newline+1:]
	} else if pipe := strings.LastIndexByte(cell, '|'); pipe < 0 && window > 0 {
		return false // a long cell is prose, not a label
	}
	if pipe := strings.LastIndexByte(cell, '|'); pipe >= 0 {
		cell = cell[pipe+1:]
	}
	words := 0
	for _, word := range strings.Fields(cell) {
		word = strings.Trim(word, "*_`")
		if word == "" {
			continue
		}
		if !isLettersOnly(word) {
			return false
		}
		words++
	}
	return words <= 3
}

// pastePasswordValueSpan bounds a password by its quotes or HTML code
// wrapper, or else by whitespace and the ";", ",", "&", "|" and "<"
// delimiters of connection strings, query strings and tables, so punctuation
// inside the password stays inside the value.
func pastePasswordValueSpan(in string, start, lineEnd int) (int, int, bool) {
	if start, end, ok := pasteWrappedValue(in, start, lineEnd); ok {
		return start, end, true
	}
	for n := 0; n < 2 && start < lineEnd && (in[start] == '*' || in[start] == '_'); n++ {
		start++
	}
	end := start
	for end < lineEnd && end-start < maxPasteValueLen && strings.IndexByte(" \t;,&|<\"'`", in[end]) < 0 {
		end++
	}
	for end > start && strings.IndexByte(".)*_", in[end-1]) >= 0 {
		end--
	}
	return start, end, false
}

// pasteWrappedValue returns the content of a quoted, backticked, <code> or
// <kbd> value that closes on the same line.
func pasteWrappedValue(in string, start, lineEnd int) (int, int, bool) {
	if start >= lineEnd {
		return 0, 0, false
	}
	rest := in[start:lineEnd]
	for _, tag := range []string{"code", "kbd"} {
		if len(rest) > len(tag)+2 && strings.EqualFold(rest[:len(tag)+2], "<"+tag+">") {
			open := start + len(tag) + 2
			limit := pasteBound(open, lineEnd, maxPasteValueLen+len(tag)+3)
			if closing := strings.Index(strings.ToLower(in[open:limit]), "</"+tag+">"); closing > 0 {
				return open, open + closing, true
			}
			return 0, 0, false
		}
	}
	switch quote := in[start]; quote {
	case '"', '\'', '`':
		if closing := strings.IndexByte(in[start+1:pasteBound(start+1, lineEnd, maxPasteValueLen+1)], quote); closing > 0 {
			return start + 1, start + 1 + closing, true
		}
	}
	return 0, 0, false
}

// pasteBack returns end-n floored at zero.
func pasteBack(end, n int) int {
	if end > n {
		return end - n
	}
	return 0
}

// pasteBound returns start+n capped at end, so per-label lookahead stays
// bounded on long lines.
func pasteBound(start, end, n int) int {
	if end-start > n {
		return start + n
	}
	return end
}

func pasteKeyValueSpan(in string, start, lineEnd int) (int, int) {
	if start, end, ok := pasteWrappedValue(in, start, lineEnd); ok {
		return start, end
	}
	for n := 0; n < 3 && start < lineEnd && strings.IndexByte("*_<(", in[start]) >= 0; n++ {
		start++
	}
	end := start
	for end < lineEnd && end-start < maxPasteValueLen && isCredentialValueByte(in[end]) {
		end++
	}
	for end > start && in[end-1] == '.' {
		end--
	}
	return start, end
}

const pasteStrongPunctuation = "!@#$%^&*?~"

// pasteStrongPassword reports a value of eight or more characters that mixes
// letters with password punctuation other than a lone "@" (an account or
// email address).
func pasteStrongPassword(value string) bool {
	if len(value) < 8 || !hasLetter(value) || strings.ContainsAny(value, " \t") {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] != '@' && strings.IndexByte(pasteStrongPunctuation, value[i]) >= 0 {
			return true
		}
	}
	return false
}

var pasteEmailLikeRE = regexp.MustCompile(`^[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)+$`)

// pasteReferenceValue reports a value that points at a secret rather than
// containing one: variables, templates, paths (Unix, UNC or Windows drive),
// flags, masks and markers.
func pasteReferenceValue(value string) bool {
	if value == "" || strings.Contains(value, "REDACTED") {
		return true
	}
	if strings.IndexByte(`$%{<[(/\~.-*#`, value[0]) >= 0 || isWindowsDrivePath(value) {
		return true
	}
	for _, fragment := range []string{"${", "{{", "$(", "://"} {
		if strings.Contains(value, fragment) {
			return true
		}
	}
	return fewDistinctRunes(value, 2) || pasteEmailLikeRE.MatchString(value)
}

// fewDistinctRunes reports whether value uses at most limit distinct runes
// ("********", "xxxxxxxx", "00000000").
func fewDistinctRunes(value string, limit int) bool {
	seen := make([]rune, 0, limit+1)
	for _, r := range value {
		known := false
		for _, s := range seen {
			if s == r {
				known = true
				break
			}
		}
		if !known {
			if len(seen) == limit {
				return false
			}
			seen = append(seen, r)
		}
	}
	return true
}

// pastePasswordValue judges a value bounded after a password label. A value
// placed directly after the label may also be an all-digit trial password or
// a word joined to a number; a value found through natural phrasing needs
// password punctuation or the key shape the generic label rule uses for
// phrased values.
func pastePasswordValue(value string, phrased bool) bool {
	value = strings.TrimSpace(value)
	if len(value) < 6 || len(value) > maxPasteValueLen || pasteReferenceValue(value) ||
		strings.HasSuffix(value, ":") || strings.Contains(value, "**") || strings.Contains(value, "__") {
		return false // a label or Markdown emphasis, not a password
	}
	if pasteStrongPassword(value) {
		return true
	}
	if phrased {
		return credentialShapedValue(value, false)
	}
	if isAllDigits(value) {
		return len(value) <= 16
	}
	return credentialShapedValue(value, true)
}

func pastePasswordPolicyContext(bridge string) bool {
	lower := strings.ToLower(bridge)
	for _, term := range pastePasswordPolicyTerms {
		if strings.Contains(lower, term) {
			return true
		}
	}
	return false
}

// pasteLiteralSecret judges a literal in a position that only holds a
// password (cmdkey /pass:, PSCredential, ConvertTo-SecureString).
func pasteLiteralSecret(value string) bool {
	return len(value) >= 4 && len(value) <= maxPasteValueLen && !pasteReferenceValue(value) &&
		strings.IndexByte(`\*`, value[0]) < 0
}

// collectPasteNextLineValues handles a short label line ending in ":" (or a
// bare label, optionally with a parenthetical) followed by the value alone
// on the next content line, after blank lines, "#" comments or a code fence
// opener, or in a YAML block scalar.
func collectPasteNextLineValues(in string, spans *[]pasteSpan) {
	for lineStart := 0; lineStart < len(in); {
		lineEnd := pasteLineEnd(in, lineStart)
		next := strings.IndexByte(in[lineStart:], '\n')
		if next < 0 {
			return
		}
		next += lineStart + 1
		line := in[lineStart:lineEnd]
		password, ok := pasteLabelLine(line)
		valueStart, valueEnd := 0, 0
		if ok {
			valueStart, valueEnd, ok = pasteNextLineValue(in, next)
		} else if label := strings.TrimSuffix(line, markerSecret); len(label) < len(line) &&
			strings.HasSuffix(strings.TrimRight(label, " \t"), ":") {
			// "token: >-" whose block indicator the assignment rule already
			// replaced: only a deeper-indented next line continues the value.
			if password, ok = pasteLabelLine(label); ok {
				valueStart, valueEnd, ok = pasteIndentedContinuation(in, line, next)
			}
		}
		if ok {
			value := in[valueStart:valueEnd]
			if (password && pastePasswordValue(value, false)) || (!password && credentialShapedValue(value, false)) {
				*spans = append(*spans, pasteSpan{valueStart, valueEnd})
			}
		}
		lineStart = next
	}
}

func pasteIndentedContinuation(in, labelLine string, start int) (int, int, bool) {
	lineEnd := pasteLineEnd(in, start)
	valueStart := skipPasteBlanks(in, start, lineEnd)
	labelIndent := len(labelLine) - len(strings.TrimLeft(labelLine, " \t"))
	valueEnd := lineEnd
	for valueEnd > valueStart && (in[valueEnd-1] == ' ' || in[valueEnd-1] == '\t') {
		valueEnd--
	}
	if valueStart-start <= labelIndent || valueEnd <= valueStart || valueEnd-valueStart > maxPasteValueLen ||
		strings.ContainsAny(in[valueStart:valueEnd], " \t") {
		return 0, 0, false
	}
	return valueStart, valueEnd, true
}

var pasteLabelLineRE = regexp.MustCompile(`^[\s#>*_|-]*((?:[\p{L}\p{N}'-]+[ \t]+){0,7}[\p{L}\p{N}'-]+)(?:[ \t]*\([^()\n]{1,40}\))?[*_\x60]*[ \t]*(:|：)?[ \t]*(?:[|>][-+]?)?[ \t]*$`)

// pasteLabelLine reports whether a line is only a short credential label.
// Without a colon, the line must end in the label word itself.
func pasteLabelLine(line string) (password bool, ok bool) {
	if len(line) > 120 {
		return false, false
	}
	match := pasteLabelLineRE.FindStringSubmatchIndex(line)
	if match == nil {
		return false, false
	}
	words := line[match[2]:match[3]]
	colon := match[4] >= 0
	if pastePasswordPolicyContext(words) {
		return false, false
	}
	for _, re := range []*regexp.Regexp{pastePasswordLabelRE, pasteKeyLabelRE} {
		for _, label := range re.FindAllStringIndex(words, -1) {
			if !pasteLabelBoundary(words, label[0], label[1]) || pasteNonCredentialKeyLabel(words, label[0], label[1]) ||
				(re == pastePasswordLabelRE && !pastePasswordLabelAt(words, label[0], label[1])) {
				continue
			}
			if !colon && label[1] != len(words) {
				continue
			}
			password := re == pastePasswordLabelRE
			// "Key fingerprint:" or "Password hash (SHA-256):" introduces metadata.
			if phraseNamesMetadata(line[match[2]+label[1]:],
				password || credentialKeyModifiers[labelModifierBefore(words, label[0])]) {
				return false, false
			}
			return password, true
		}
	}
	return false, false
}

func pasteNextLineValue(in string, start int) (int, int, bool) {
	for skipped := 0; start < len(in) && skipped <= 3; skipped++ {
		lineEnd := pasteLineEnd(in, start)
		line := strings.TrimSpace(in[start:lineEnd])
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "```") ||
			(line == markerSecret && pasteFencedValueFollows(in, lineEnd)) {
			next := strings.IndexByte(in[start:], '\n')
			if next < 0 {
				return 0, 0, false
			}
			start += next + 1
			continue
		}
		valueStart := skipPasteBlanks(in, start, lineEnd)
		if in[valueStart] == '>' {
			valueStart = skipPasteBlanks(in, valueStart+1, lineEnd)
		}
		valueEnd := lineEnd
		for valueEnd > valueStart && (in[valueEnd-1] == ' ' || in[valueEnd-1] == '\t') {
			valueEnd--
		}
		if valueEnd-valueStart >= 2 && strings.IndexByte("`'\"", in[valueStart]) >= 0 && in[valueEnd-1] == in[valueStart] {
			valueStart, valueEnd = valueStart+1, valueEnd-1
		}
		if valueEnd <= valueStart || valueEnd-valueStart > maxPasteValueLen ||
			strings.ContainsAny(in[valueStart:valueEnd], " \t") {
			return 0, 0, false
		}
		return valueStart, valueEnd, true
	}
	return 0, 0, false
}

// pasteFencedValueFollows reports whether the line after lineEnd is followed
// by a closing code fence. The assignment rules treat "Token:\n```sh" as a
// label and value and redact the fence opener, leaving a marker line before
// the fenced value.
func pasteFencedValueFollows(in string, lineEnd int) bool {
	valueLine := strings.IndexByte(in[lineEnd:], '\n')
	if valueLine < 0 {
		return false
	}
	valueStart := lineEnd + valueLine + 1
	closing := strings.IndexByte(in[valueStart:], '\n')
	if closing < 0 {
		return false
	}
	closingStart := valueStart + closing + 1
	return strings.HasPrefix(strings.TrimSpace(in[closingStart:pasteLineEnd(in, closingStart)]), "```")
}

// pasteCredentialNameRE matches environment-variable and setting names that
// hold a credential ("API_KEY", "MYSQL_PWD", "DB_PASS", "client-secret",
// "password").
var pasteCredentialNameRE = regexp.MustCompile(`(?i)^(?:(?:[a-z0-9]+_)+pass|(?:[a-z0-9]+[_.-])*(?:password|passwd|pwd|passphrase|secret|token|api[_-]?key|apikey|access[_-]?key|secret[_-]?key|private[_-]?key|credentials?|client[_-]?secret|auth[_-]?token))$`)

func pastePasswordName(name string) bool {
	return pastePasswordLabelRE.MatchString(name) && !strings.Contains(strings.ToLower(name), "key")
}

// collectPasteYAMLNameValues handles Kubernetes-style env lists:
// "- name: API_KEY" followed within three lines by "value: X".
func collectPasteYAMLNameValues(in string, spans *[]pasteSpan) {
	for lineStart := 0; lineStart < len(in); {
		lineEnd := pasteLineEnd(in, lineStart)
		next := strings.IndexByte(in[lineStart:], '\n')
		if next < 0 {
			return
		}
		next += lineStart + 1
		line := strings.TrimSpace(in[lineStart:lineEnd])
		line = strings.TrimSpace(strings.TrimPrefix(line, "-"))
		if strings.HasPrefix(line, "name:") {
			name := strings.Trim(strings.TrimSpace(line[len("name:"):]), `"'`)
			if pasteCredentialNameRE.MatchString(name) {
				pasteYAMLValue(in, next, pastePasswordName(name), spans)
			}
		}
		lineStart = next
	}
}

func pasteYAMLValue(in string, start int, password bool, spans *[]pasteSpan) {
	for n := 0; n < 3 && start < len(in); n++ {
		lineEnd := pasteLineEnd(in, start)
		line := in[start:lineEnd]
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") {
			return
		}
		if strings.HasPrefix(trimmed, "value:") {
			valueStart := skipPasteBlanks(in, start+strings.Index(line, "value:")+len("value:"), lineEnd)
			var valueEnd int
			if wrappedStart, wrappedEnd, ok := pasteWrappedValue(in, valueStart, lineEnd); ok {
				valueStart, valueEnd = wrappedStart, wrappedEnd
			} else {
				valueEnd = lineEnd
				for valueEnd > valueStart && (in[valueEnd-1] == ' ' || in[valueEnd-1] == '\t') {
					valueEnd--
				}
			}
			if valueEnd > valueStart && pasteNameValueSecret(in[valueStart:valueEnd], password) {
				*spans = append(*spans, pasteSpan{valueStart, valueEnd})
			}
			return
		}
		next := strings.IndexByte(in[start:], '\n')
		if next < 0 {
			return
		}
		start += next + 1
	}
}

func pasteNameValueSecret(value string, password bool) bool {
	if password {
		return pastePasswordValue(value, false)
	}
	return credentialShapedValue(value, false)
}

const pasteQuotedArg = `'[^'\n]{1,256}'|"[^"\n]{1,256}"`

// pasteCommandGap spans the rest of one command: it stops at a pipe, ";",
// "&&" or a background "&", but not at "&" inside a URL.
const pasteCommandGap = `(?:[^\n|;&]|&[^\n|;&\s]){0,200}?`

var (
	// mysql -p'X' (lowercase; -P is the port) and sqlcmd -P 'X' (uppercase;
	// -p prints statistics): quoted only, and only within a command that takes
	// its password that way ("git log -p" is a patch flag).
	pasteShortPasswordOptionRE = regexp.MustCompile(`\b(?:(?i:mysql|mysqldump|mysqladmin|mariadb|mariadb-dump)\b[^\n|;&]{0,200}?[ \t]-p|(?i:sqlcmd|bcp|osql)\b[^\n|;&]{0,200}?[ \t]-P)[ \t]*(` + pasteQuotedArg + `)`)
	// sshpass -p X, only among sshpass's own options (-f file, -d fd,
	// -P prompt, -e, -v): the wrapped command's "-p" is its port.
	pasteSSHPassRE    = regexp.MustCompile(`\b(?i:sshpass)(?:[ \t]+(?:-[fdP][ \t]*(?:` + pasteQuotedArg + `|[^\s'"]+)|-[evhV]))*[ \t]+-p[ \t]*(` + pasteQuotedArg + `|[^\s'"]{1,256})`)
	pasteCurlUserRE   = regexp.MustCompile(`(?i)\bcurl\b` + pasteCommandGap + `[ \t](?:-u|--user)(?:=|[ \t]+)(` + pasteQuotedArg + `|[^\s'"]{1,256})`)
	pasteCmdkeyPassRE = regexp.MustCompile(`(?i)(?:^|[ \t])/pass:(` + pasteQuotedArg + `|[^\s'"]{1,256})`)
	pasteNetUseRE     = regexp.MustCompile(`(?i)\bnet[ \t]+use\b` + pasteCommandGap + `[ \t](` + pasteQuotedArg + `|[^\s'"/\\*][^\s'"]{0,255})[ \t]+/user:`)
	// ConvertTo-SecureString 'X' -AsPlainText; the flag alone also marks the
	// literal, since "$password = ConvertTo-SecureString" is itself redacted
	// by the password assignment rule before this scanner runs.
	pasteSecureStringRE     = regexp.MustCompile(`(?i)(?:\bConvertTo-SecureString[ \t]+(?:-String[ \t]+)?(` + pasteQuotedArg + `)|[ \t](` + pasteQuotedArg + `)[ \t]+-AsPlainText\b)`)
	pasteCredentialObjectRE = regexp.MustCompile(`(?i)\b(?:PSCredential|NetworkCredential)[ \t]*\([ \t]*(?:'[^'\n]*'|"[^"\n]*"|\$[A-Za-z_][A-Za-z0-9_:]*)[ \t]*,[ \t]*(` + pasteQuotedArg + `)`)
)

func collectPasteCommandCredentials(in string, spans *[]pasteSpan) {
	appendPasteArgument(in, pasteShortPasswordOptionRE, spans, func(value string) bool {
		return pastePasswordValue(value, false)
	})
	for _, re := range []*regexp.Regexp{pasteSSHPassRE, pasteCmdkeyPassRE, pasteNetUseRE, pasteCredentialObjectRE} {
		appendPasteArgument(in, re, spans, pasteLiteralSecret)
	}
	for _, match := range pasteSecureStringRE.FindAllStringSubmatchIndex(in, -1) {
		if match[4] < 0 || pasteSecureStringLiteral(in, match[4], match[1]) {
			appendPasteMatch(in, match, spans, pasteLiteralSecret)
		}
	}
	// curl -u user:password: only the part after the first colon.
	for _, match := range pasteCurlUserRE.FindAllStringSubmatchIndex(in, -1) {
		start, end := unquotePasteArgument(in, match[2], match[3])
		if colon := strings.IndexByte(in[start:end], ':'); colon > 0 && pasteLiteralSecret(in[start+colon+1:end]) {
			*spans = append(*spans, pasteSpan{start + colon + 1, end})
		}
	}
}

// pasteSecureStringLiteral reports whether the quoted argument at start,
// before the -AsPlainText flag ending at end, is ConvertTo-SecureString's
// plaintext: it follows the command or its -String parameter. A literal after
// another command or parameter names something else ("Get-Secret 'name'
// -AsPlainText", "Get-Secret -Name 'name' -AsPlainText"). The password
// assignment rule may already have replaced the command name ("$password =
// <marker> 'x' -AsPlainText"); the marker then counts only before "-Force",
// which ConvertTo-SecureString takes and Get-Secret does not.
func pasteSecureStringLiteral(in string, start, end int) bool {
	fields := strings.Fields(in[:start])
	if len(fields) == 0 {
		return false
	}
	last := strings.TrimLeft(fields[len(fields)-1], "(")
	if strings.EqualFold(last, "-String") || strings.EqualFold(last, "ConvertTo-SecureString") {
		return true
	}
	after := strings.Fields(in[end:])
	return strings.HasSuffix(last, ">") && strings.Contains(last, "<REDACTED:") &&
		len(after) > 0 && strings.EqualFold(strings.TrimRight(after[0], ");"), "-Force")
}

func appendPasteArgument(in string, re *regexp.Regexp, spans *[]pasteSpan, secret func(string) bool) {
	for _, match := range re.FindAllStringSubmatchIndex(in, -1) {
		appendPasteMatch(in, match, spans, secret)
	}
}

// appendPasteMatch records the first matched argument group when secret
// accepts its unquoted value.
func appendPasteMatch(in string, match []int, spans *[]pasteSpan, secret func(string) bool) {
	for group := 2; group+1 < len(match); group += 2 {
		if match[group] < 0 {
			continue
		}
		start, end := unquotePasteArgument(in, match[group], match[group+1])
		if end > start && secret(in[start:end]) {
			*spans = append(*spans, pasteSpan{start, end})
		}
		return
	}
}

func unquotePasteArgument(in string, start, end int) (int, int) {
	if end-start >= 2 && (in[start] == '\'' || in[start] == '"') && in[end-1] == in[start] {
		return start + 1, end - 1
	}
	return start, end
}

// Percent-encoded credential query values (SAS "sig=", "key=",
// "credential="): the escapes split the value for the label rules, so decode
// it for inspection and redact the complete encoded value.
var pasteEncodedQueryRE = regexp.MustCompile(`(?i)(?:^|[?&;\s'"(])(?:sig|signature|key|api[_-]?key|credential|token|secret|password|passwd|pwd)=([^&\s"'<>#]*%[0-9A-Fa-f]{2}[^&\s"'<>#]*)`)

func collectPasteEncodedQueryValues(in string, spans *[]pasteSpan) {
	for _, match := range pasteEncodedQueryRE.FindAllStringSubmatchIndex(in, -1) {
		start, end := match[2], match[3]
		for end > start && in[end-1] == '.' {
			end--
		}
		decoded, err := url.QueryUnescape(in[start:end])
		if err != nil {
			continue
		}
		if credentialShapedValue(decoded, false) || pasteStrongPassword(decoded) {
			*spans = append(*spans, pasteSpan{start, end})
		}
	}
}

// HTTP Basic credentials: base64 after "Basic", "BASIC_AUTH=" or an "auth:"
// key that decodes to printable "user:password".
var pasteBasicCredentialRE = regexp.MustCompile(`(?i)(?:\bbasic[ \t]+|(?:^|[^A-Za-z0-9])basic[_-]?auth["']?[ \t]*[:=][ \t]*["']?|(?:^|[^A-Za-z0-9_])auth["']?[ \t]*[:=][ \t]*["']?)([A-Za-z0-9+/]{8,}={0,2})`)

func collectPasteBasicCredentials(in string, spans *[]pasteSpan) {
	for _, match := range pasteBasicCredentialRE.FindAllStringSubmatchIndex(in, -1) {
		start, end := match[2], match[3]
		if end < len(in) && (isASCIIAlnum(in[end]) || strings.IndexByte("+/=_-", in[end]) >= 0) {
			continue
		}
		if decodesToBasicCredential(in[start:end]) {
			*spans = append(*spans, pasteSpan{start, end})
		}
	}
	// BASIC_AUTH=user:password written out before encoding.
	for _, match := range pastePlainBasicCredentialRE.FindAllStringSubmatchIndex(in, -1) {
		if pasteLiteralSecret(in[match[2]:match[3]]) {
			*spans = append(*spans, pasteSpan{match[2], match[3]})
		}
	}
}

// The name may carry a prefix ("SNOW_BASIC_AUTH").
var pastePlainBasicCredentialRE = regexp.MustCompile(`(?i)(?:^|[^A-Za-z0-9])basic[_-]?auth["']?[ \t]*[:=][ \t]*["']?[^\s:"'@/]{1,64}:([^\s"']{4,256})`)

func decodesToBasicCredential(value string) bool {
	encoding := base64.StdEncoding
	if !strings.HasSuffix(value, "=") && len(value)%4 != 0 {
		encoding = base64.RawStdEncoding
	}
	decoded, err := encoding.DecodeString(value)
	if err != nil {
		return false
	}
	colon := -1
	for i, ch := range decoded {
		if ch < 0x20 || ch > 0x7e {
			return false
		}
		if ch == ':' && colon < 0 {
			colon = i
		}
	}
	return colon > 0 && colon < len(decoded)-1 && !strings.ContainsRune(string(decoded[:colon]), ' ')
}

// A value placed before its label: "<uuid> is the current token",
// "<uuid> (API key)", "`<uuid>` — key copied from the portal",
// "Use <uuid> as the key for ...".
// The cue is found first and the value is then read backwards from it, which
// keeps the scan cheap on long text without cues.
var pasteValueBeforeLabelCueRE = regexp.MustCompile(`(?i)[\x60'"]?(?:[ \t]+(?:is|was)[ \t]+the[ \t]+(?:(?:current|new|trial|test|poc|temporary|api|access|vendor|partner|shared)[ \t]+){0,2}|[ \t]*\([ \t]*(?:the[ \t]+)?(?:(?:api|poc|trial|access)[ \t]+)?|[ \t]+[—–][ \t]+(?:the[ \t]+)?(?:(?:api|poc|trial|access)[ \t]+)?|[ \t]+as[ \t]+the[ \t]+(?:(?:current|new|trial|test|poc|temporary|api|access)[ \t]+){0,2})(key|token|password|passphrase|credential|secret)s?\b`)

const pasteValueBeforeLabelMaxLen = 128

func isPasteBeforeLabelValueByte(ch byte) bool {
	return isASCIIAlnum(ch) || strings.IndexByte("!@#$%^&*._~+/=-", ch) >= 0
}

var pasteValueBeforeLabelNonCredentialWords = map[string]bool{
	"id": true, "ids": true, "identifier": true, "fingerprint": true,
	"thumbprint": true, "name": true, "length": true, "rotation": true,
	"reference": true, "field": true, "column": true, "type": true,
	"size": true, "format": true, "version": true, "vault": true,
	"store": true, "manager": true, "policy": true, "owner": true,
	"holder": true,
}

func collectPasteValuesBeforeLabels(in string, spans *[]pasteSpan) {
	for _, match := range pasteValueBeforeLabelCueRE.FindAllStringSubmatchIndex(in, -1) {
		end := match[0]
		start := end
		for start > 0 && end-start < pasteValueBeforeLabelMaxLen && isPasteBeforeLabelValueByte(in[start-1]) {
			start--
		}
		for start < end && !isASCIIAlnum(in[start]) {
			start++
		}
		if end-start < 8 || (start > 0 && isPasteBeforeLabelValueByte(in[start-1])) {
			continue
		}
		if quote := in[match[0]]; strings.IndexByte("`'\"", quote) >= 0 && (start == 0 || in[start-1] != quote) {
			continue
		}
		cue := strings.ToLower(strings.TrimLeft(in[match[0]:match[2]], "`'\" \t"))
		if strings.HasPrefix(cue, "as") {
			before := strings.ToLower(strings.TrimRight(in[pasteBack(start, 16):start], "`'\" \t"))
			if !strings.HasSuffix(before, "use") || (len(before) > 3 && isASCIILetter(rune(before[len(before)-4]))) {
				continue
			}
		}
		next := skipPasteBlanks(in, match[1], len(in))
		word := next
		for word < len(in) && word-next < 16 && (isASCIILetter(rune(in[word])) || in[word] == ')') {
			word++
		}
		if pasteValueBeforeLabelNonCredentialWords[strings.Trim(strings.ToLower(in[next:word]), ")")] {
			continue
		}
		for end > start && strings.IndexByte(".,", in[end-1]) >= 0 {
			end--
		}
		value := in[start:end]
		password := strings.HasPrefix(strings.ToLower(in[match[2]:match[3]]), "pass")
		if (password && pastePasswordValue(value, true)) || (!password && credentialShapedValue(value, false)) {
			*spans = append(*spans, pasteSpan{start, end})
		}
	}
}

// JSON credential context. Keys that name a credential without a suffix the
// assignment rules know ("credentials", "consumerKey", "SecretID") redact a
// credential-shaped string value; {"name"|"key"|"label": <credential name>,
// "value": X} pairs redact X; arrays under "token"/"tokens" redact
// credential-shaped strings. A "primaryKey"/"secondaryKey" value is ambiguous
// (Azure access keys versus database identifiers): it is redacted when it is
// credential-shaped and either not a public identifier (UUID, hex, ULID,
// cloud resource ID), or another credential key sits in the same object.
var jsonCredentialContextKeyRE = regexp.MustCompile(`(?i)^(?:credentials?|consumer[_-]?key|account[_-]?key|service[_-]?key|access[_-]?key|poc[_-]?key|key[_-]?value|secret[_-]?id|basic[_-]?auth)$`)
var jsonPrimaryKeyRE = regexp.MustCompile(`(?i)^(?:primary|secondary)[_-]?key$`)
var jsonCredentialArrayKeyRE = regexp.MustCompile(`(?i)^(?:tokens?|api[_-]?keys|credentials|secrets)$`)

type pasteJSONObject struct {
	label, value     jsonStringToken
	hasLabel         bool
	hasValue         bool
	credentialCue    bool
	context, primary []jsonStringToken
}

func (o *pasteJSONObject) note(key string, token jsonStringToken) {
	switch strings.ToLower(key) {
	case "name", "key", "label":
		if !o.hasLabel {
			o.label, o.hasLabel = token, true
		}
		return
	case "value":
		o.value, o.hasValue = token, true
		return
	}
	switch {
	case jsonPrimaryKeyRE.MatchString(key):
		o.primary = append(o.primary, token)
	case jsonCredentialContextKeyRE.MatchString(key):
		o.context = append(o.context, token)
		o.credentialCue = true
	case jsonSecretAssignmentKeyRE.MatchString(key):
		o.credentialCue = true
	}
}

func (p *jsonDocumentParser) markPasteCredentialObject(o *pasteJSONObject) {
	for _, token := range o.context {
		// Long high-entropy values are left to the rendered-string entropy
		// rule, which already redacts them under any key.
		if credentialShapedValue(token.value, false) && !entropyRuleRedacts(token.value) {
			p.addPasteCredentialValue(token)
		}
	}
	if o.hasLabel && o.hasValue && pasteCredentialNameRE.MatchString(o.label.value) &&
		pasteNameValueSecret(o.value.value, pastePasswordName(o.label.value)) {
		p.addPasteCredentialValue(o.value)
	}
	for _, token := range o.primary {
		identifier := isPublicIdentifierValue(token.value) || isHex(token.value)
		if credentialShapedValue(token.value, false) && (o.credentialCue || !identifier) {
			p.addPasteCredentialValue(token)
		}
	}
}

// entropyRuleRedacts reports whether the rendered-string entropy rule would
// redact value as a whole token in standard mode.
func entropyRuleRedacts(value string) bool {
	return highEntropyFreeTextTokenRE.FindString(value) == value && !canonicalUUIDRE.MatchString(value) &&
		!compactUUIDRE.MatchString(value) && !publicHexFingerprintRE.MatchString(value) &&
		looksLikeHighEntropySecret(value)
}

// markPasteCredentialArray inspects the direct string elements of an array
// value under a credential key; first is the index in p.strings where the
// array's tokens begin.
func (p *jsonDocumentParser) markPasteCredentialArray(key string, first, arrayStart, arrayEnd int) {
	if !jsonCredentialArrayKeyRE.MatchString(key) {
		return
	}
	direct := directArrayStringStarts(p.in, arrayStart, arrayEnd)
	for _, token := range p.strings[first:] {
		if !direct[token.start] {
			continue // nested inside an element object or array: not the credential list
		}
		before := strings.TrimRight(p.in[:token.start], " \t\r\n")
		after := strings.TrimLeft(p.in[token.end:], " \t\r\n")
		if before == "" || after == "" || strings.IndexByte("[,", before[len(before)-1]) < 0 ||
			strings.IndexByte(",]", after[0]) < 0 {
			continue
		}
		if credentialShapedValue(token.value, false) {
			p.addPasteCredentialValue(token)
		}
	}
}

// directArrayStringStarts returns the opening-quote offsets of the strings that
// are direct elements of the JSON array in in[start:end] (in[start] == '['), in
// one pass that skips string contents.
func directArrayStringStarts(in string, start, end int) map[int]bool {
	starts := make(map[int]bool)
	depth := 0
	for i := start; i < end; i++ {
		switch in[i] {
		case '[', '{':
			depth++
		case ']', '}':
			depth--
		case '"':
			if depth == 1 {
				starts[i] = true
			}
			for i++; i < end && in[i] != '"'; i++ {
				if in[i] == '\\' {
					i++
				}
			}
		}
	}
	return starts
}

// addPasteCredentialValue marks a string token; the keys these rules inspect
// are disjoint from classifyJSONKey's, so only an immediate repeat can occur.
func (p *jsonDocumentParser) addPasteCredentialValue(token jsonStringToken) {
	if n := len(p.sensitiveValues); n > 0 && p.sensitiveValues[n-1].start == token.start {
		return
	}
	p.sensitiveValues = append(p.sensitiveValues, jsonSensitiveValue{
		start:    token.start,
		end:      token.end,
		ruleName: "paste_credential",
		marker:   markerSecret,
		priority: 3,
	})
}
