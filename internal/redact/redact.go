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
	if !r.structuredScanMayMatch(in) {
		return in, Report{}
	}
	if out, report, ok := scanStructuredDocuments(in, r.scanJSONString, true); ok {
		return out, report
	}
	return r.scanPlainString(in)
}

func (r Redactor) structuredScanMayMatch(in string) bool {
	// A JSON escape can hide every literal character a rule prefilter needs;
	// decode escaped documents before deciding that no rule can match.
	if strings.Contains(in, `\`) {
		return true
	}
	// Pasted-credential rules include non-ASCII labels that the ASCII-folding
	// rule prefilters cannot express.
	if pasteCredentialMayMatch(in) {
		return true
	}
	view := prefilterText{text: in}
	for _, candidate := range baseRules {
		if candidate.prefilter.match(&view) {
			return true
		}
	}
	if r.mode == ModeShare || r.mode == ModeParanoid {
		for _, candidate := range shareRules {
			if candidate.prefilter.match(&view) {
				return true
			}
		}
	}
	return false
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

func (r Redactor) scanJSONString(value string) (string, Report) {
	// A decoded string that is itself a JSON document (pasted JSON in a
	// description) is scanned as a document, exactly as ScanString scans it
	// when projection sees it on its own, so rendering does not re-redact what
	// projection preserved. Each nested level is strictly shorter.
	if nestedJSONDocumentCandidate(value) {
		if out, report, ok := scanStructuredDocuments(value, r.scanJSONString, true); ok {
			return out, report
		}
	}
	out, report := scanRules(value, Report{}, baseRules)
	if r.mode == ModeShare || r.mode == ModeParanoid {
		out, report = scanRules(out, report, shareRules)
	}
	return out, report
}

// nestedJSONDocumentCandidate reports whether a decoded string starts like a
// JSON object or array, the only documents that carry key/value structure.
func nestedJSONDocumentCandidate(value string) bool {
	trimmed := strings.TrimLeft(value, " \t\r\n")
	return trimmed != "" && (trimmed[0] == '{' || trimmed[0] == '[')
}

// scanStructuredDocuments keeps plaintext regexes inside decoded JSON string
// tokens, so a match cannot consume JSON syntax or a neighboring NDJSON record.
// Trying every valid JSON document also covers escaped keywords that are only
// recognizable after decoding.
func scanStructuredDocuments(in string, scanString jsonStringScanner, classifySensitive bool) (string, Report, bool) {
	scanDocument := func(document string) (string, Report) {
		return scanJSONDocument(document, scanString, classifySensitive)
	}
	if json.Valid([]byte(in)) {
		out, report := scanDocument(in)
		return out, report, true
	}
	return scanNDJSONDocuments(in, scanDocument)
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
	if rule.custom != nil {
		return scanCustomRule(out, report, rule, view)
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

// scanCredentialURLs redacts URL userinfo that carries a user:password colon.
// When the matched userinfo contains one of this package's markers, a colon
// inside the marker does not count: share-mode masking can leave a marker in
// userinfo ("user-<REDACTED:IP>@host"), which would otherwise match the
// credential URL pattern on a later pass.
func scanCredentialURLs(in string) (string, int) {
	matches := credentialURLRE.FindAllStringSubmatchIndex(in, -1)
	if len(matches) == 0 {
		return in, 0
	}

	var out strings.Builder
	last, count := 0, 0
	for _, match := range matches {
		userInfo := strings.TrimSuffix(in[match[3]:match[1]], "@")
		if strings.Contains(userInfo, "<REDACTED:") && !markedUserInfoHasPassword(userInfo) {
			continue
		}

		out.WriteString(in[last:match[2]])
		out.WriteString(in[match[2]:match[3]])
		out.WriteString(markerSecret)
		out.WriteByte('@')
		last = match[1]
		count++
	}
	if count == 0 {
		return in, 0
	}

	out.WriteString(in[last:])
	return out.String(), count
}

// scanCustomRule applies a rule implemented as a scanner function rather than
// a single regex replacement.
func scanCustomRule(out string, report Report, rule rule, view *prefilterText) (string, Report) {
	scanned, count := rule.custom(out)
	if count == 0 {
		return out, report
	}
	report = addReportCount(report, rule.name, count)
	*view = prefilterText{text: scanned}
	return scanned, report
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
	// memberKey is the object member name when the token is a member value.
	memberKey string
}

// publicMemberValue reports whether a member value is a public identifier
// named by its member key ({"_id": <ObjectId>}, {"sha256": <digest>},
// {"commit": <revision>}), the JSON form of the context isContextualPublicValue
// reads in prose. Only values below the 32-character long-entropy floor are
// considered, so the exemption reaches the branch's short free-text rule but
// never changes main's long-entropy decision.
func publicMemberValue(token jsonStringToken) bool {
	if token.memberKey == "" || len(token.memberKey) > publicValueContextWindow || len(token.value) >= 32 {
		return false
	}
	context := token.memberKey + ": "
	text := context + token.value
	return isContextualPublicValue(text, len(context), len(text))
}

type jsonSensitiveValue struct {
	start    int
	end      int
	ruleName string
	marker   string
	priority int
}

type jsonSensitiveClassification struct {
	ruleName string
	marker   string
	priority int
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
func scanJSONDocument(in string, scanString jsonStringScanner, classifySensitive bool) (string, Report) {
	parser := jsonDocumentParser{in: in}
	if !parser.parseDocument() {
		return encodeJSONString(markerSecret), Report{}
	}

	replacements := make([]jsonReplacement, 0, len(parser.strings)+len(parser.sensitiveValues))
	var report Report
	for _, token := range parser.strings {
		if publicMemberValue(token) {
			continue
		}
		value, tokenReport := scanString(token.value)
		report = mergeReports(report, tokenReport)
		if value != token.value || containsRedactionMarker(value) {
			replacements = append(replacements, jsonReplacement{
				start: token.start,
				end:   token.end,
				value: encodeJSONString(value),
			})
		}
	}
	if classifySensitive {
		for _, sensitive := range parser.sensitiveValues {
			if sensitive.ruleName == "" {
				continue // withdrawn: the key of a key/value pair object
			}
			report = addReportCount(report, sensitive.ruleName, 1)
			replacements = append(replacements, jsonReplacement{
				start:    sensitive.start,
				end:      sensitive.end,
				value:    encodeJSONString(sensitive.marker),
				priority: sensitive.priority,
			})
		}
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

func containsRedactionMarker(value string) bool {
	return strings.Contains(value, markerSecret) ||
		strings.Contains(value, markerPrivateKey) ||
		strings.Contains(value, markerJWT) ||
		strings.Contains(value, markerProvisioningKey) ||
		strings.Contains(value, markerEmail) ||
		strings.Contains(value, markerIP)
}

// markedUserInfoHasPassword fails closed: any colon outside the package's
// markers counts as a password separator, even one that might be a host port
// in a match that ran into a query ("user-<REDACTED:IP>@host:443?to=a@b").
// Such ambiguous URLs are over-redacted, never leaked; only a colon that
// exists solely inside a marker is exempt.
func markedUserInfoHasPassword(userInfo string) bool {
	return strings.Contains(redactionMarkerRE.ReplaceAllString(userInfo, ""), ":")
}

// redactionMarkerRE matches only markers this package emits, so arbitrary
// marker-shaped text in userinfo is still judged as credentials.
var redactionMarkerRE = regexp.MustCompile(`<REDACTED:(?:SECRET|PRIVATE_KEY|JWT|PROVISIONING_KEY|EMAIL|IP)>`)

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

	pairKeyEntry, hasValueMember := -1, false
	var object pasteJSONObject
	for {
		key, ok := p.parseString()
		if !ok {
			return false
		}
		p.skipSpace()
		if !p.consume(':') {
			return false
		}
		firstValueString := len(p.strings)
		valueStart, valueEnd, ok := p.parseValue()
		if !ok {
			return false
		}
		if p.in[valueStart] == '[' {
			p.markPasteCredentialArray(key.value, firstValueString, valueStart, valueEnd)
		}
		if p.in[valueStart] == '"' {
			p.strings[len(p.strings)-1].memberKey = key.value
		}
		// Match the existing assignment-rule surface: scalar values are replaced,
		// while structured values continue to be scanned recursively by token.
		structuredValue := p.in[valueStart] == '{' || p.in[valueStart] == '['
		if !structuredValue {
			classifications, count := classifyJSONKey(key.value)
			for _, classification := range classifications[:count] {
				p.sensitiveValues = append(p.sensitiveValues, jsonSensitiveValue{
					start:    valueStart,
					end:      valueEnd,
					ruleName: classification.ruleName,
					marker:   classification.marker,
					priority: classification.priority,
				})
			}
			if p.in[valueStart] == '"' {
				object.note(key.value, p.strings[len(p.strings)-1])
			}
			// A key named only "key" (or "pwd"/"bearer") is ambiguous: tag
			// objects use {"key": "Environment"}. Replace its string value only
			// when the value itself is credential-shaped.
			if p.in[valueStart] == '"' && genericCredentialJSONKeyRE.MatchString(key.value) {
				var decoded string
				passwordKey := !strings.EqualFold(key.value, "key") && !strings.EqualFold(key.value, "bearer")
				if err := json.Unmarshal([]byte(p.in[valueStart:valueEnd]), &decoded); err == nil &&
					credentialShapedValue(decoded, passwordKey) &&
					!(strings.EqualFold(key.value, "pwd") && isReadableWorkingDirectory(decoded)) {
					if strings.EqualFold(key.value, "key") && isPublicIdentifierValue(decoded) {
						pairKeyEntry = len(p.sensitiveValues)
					}
					p.sensitiveValues = append(p.sensitiveValues, jsonSensitiveValue{
						start:    valueStart,
						end:      valueEnd,
						ruleName: "generic_credential_label",
						marker:   markerSecret,
						priority: 3,
					})
				}
			}
		}
		// {"Key": ..., "Value": ...} with any casing, and a scalar, object or
		// array value.
		if strings.EqualFold(key.value, "value") {
			hasValueMember = true
		}

		p.skipSpace()
		if p.consume('}') {
			// In a {"key": K, "value": V} pair the key is a name; a UUID,
			// digest or cloud resource ID there is a public identifier.
			if pairKeyEntry >= 0 && hasValueMember {
				p.sensitiveValues[pairKeyEntry].ruleName = ""
			}
			p.markPasteCredentialObject(&object)
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

var embeddedJSONStringRE = regexp.MustCompile(`"(?:\\.|[^"\\])*"`)

// withGenericLabels runs the generic credential label scanner before scan, for
// text that only appears after decoding and so was not seen by base rules.
func withGenericLabels(scan jsonStringScanner) jsonStringScanner {
	return func(value string) (string, Report) {
		labeled, count := scanGenericCredentialLabels(value)
		report := addReportCount(Report{}, "generic_credential_label", count)
		out, scanReport := scan(labeled)
		return out, mergeReports(report, scanReport)
	}
}

// withEmbeddedJSONStrings extends a decoded-string scanner to also decode
// quoted JSON literals embedded in that string (a JSON document whose string
// value contains an escaped literal in prose).
func withEmbeddedJSONStrings(scan jsonStringScanner) jsonStringScanner {
	return embeddedJSONStringScanner(scan, 0)
}

// maxEmbeddedJSONDepth bounds how many levels of escaped literals inside
// escaped literals are decoded. Each level is strictly shorter than the last.
const maxEmbeddedJSONDepth = 4

func embeddedJSONStringScanner(scan jsonStringScanner, depth int) jsonStringScanner {
	return func(value string) (string, Report) {
		out, report := value, Report{}
		if depth < maxEmbeddedJSONDepth && strings.Contains(value, `\`) {
			out, report = scanEmbeddedJSONStrings(value, withGenericLabels(embeddedJSONStringScanner(scan, depth+1)))
		}
		scanned, scanReport := scan(out)
		return scanned, mergeReports(report, scanReport)
	}
}

// scanEmbeddedJSONStrings decodes each quoted JSON string literal that
// contains an escape, scans the decoded text, and re-encodes the literal when
// the scan redacted something. Text outside literals is left to the caller.
func scanEmbeddedJSONStrings(in string, scan jsonStringScanner) (string, Report) {
	var report Report
	out := embeddedJSONStringRE.ReplaceAllStringFunc(in, func(literal string) string {
		if !strings.Contains(literal, `\`) {
			return literal
		}
		var decoded string
		if json.Unmarshal([]byte(literal), &decoded) != nil {
			return literal
		}
		scanned, literalReport := scan(decoded)
		if literalReport.Empty() {
			return literal
		}
		report = mergeReports(report, literalReport)
		return encodeJSONString(scanned)
	})
	return out, report
}

// keyMaterialEntropy is the Shannon entropy (bits per byte) at or above which
// a long mixed letter/digit segment reads as key material rather than a word.
const keyMaterialEntropy = 3.5

// wordWithNumber reports whether a segment is words joined to numbers
// ("production2024", "NetworkSwitch2024", "Switch01Port48"): letters and
// digits alternate at most four times, and the letters read as words. Mixed-case
// letters must parse as CamelCase or acronym words; single-case letters must
// contain a word-like share of vowels. Random key material rarely has so few
// letter/digit switches, and random mixed-case letters almost never parse as
// CamelCase because isolated lowercase letters are common in them.
func wordWithNumber(segment string) bool {
	switches := 0
	for i := 1; i < len(segment); i++ {
		if isDigitByte(segment[i]) != isDigitByte(segment[i-1]) {
			switches++
		}
	}
	if switches <= 2 {
		return readsAsWords(segment) || singleCaseWordAndNumber(segment)
	}
	// Three or four switches ("Switch01Port48", "Switch01Eth0",
	// "Catalyst01Gi0", "CiscoNexus9K01") only when every letter run has two or
	// more letters (a word, acronym or interface abbreviation), apart from at
	// most one single uppercase model letter, and the segment reads as words.
	if switches > 4 {
		return false
	}
	// One single uppercase letter is allowed as a model letter directly after a
	// digit and before a digit or the end ("Nexus9K01", "Nexus9K").
	singleLetterRuns := 0
	for i := 0; i < len(segment); {
		if !isASCIILetter(rune(segment[i])) {
			i++
			continue
		}
		j := i
		for j < len(segment) && isASCIILetter(rune(segment[j])) {
			j++
		}
		if j-i == 1 {
			modelLetter := segment[i] >= 'A' && segment[i] <= 'Z' && i > 0 && isDigitByte(segment[i-1]) &&
				(j == len(segment) || isDigitByte(segment[j]))
			if !modelLetter {
				return false
			}
			singleLetterRuns++
		}
		i = j
	}
	return singleLetterRuns <= 1 && readsAsWords(segment)
}

// capitalizedWordAt reports whether run has one capital followed by at least
// two lowercase letters at index i.
func capitalizedWordAt(run string, i int) bool {
	if i+2 >= len(run) || run[i] < 'A' || run[i] > 'Z' {
		return false
	}
	return run[i+1] >= 'a' && run[i+1] <= 'z' && run[i+2] >= 'a' && run[i+2] <= 'z'
}

// commonShortCamelWords are two-letter English words that appear as their own
// Capitalized word in policy and setting names ("SignIn", "LogOnAsService",
// "GoToMeeting", "MicrosoftEntraId").
var commonShortCamelWords = map[string]bool{
	"As": true, "At": true, "Be": true, "By": true, "Do": true, "Go": true,
	"Id": true, "In": true, "Is": true, "It": true, "Me": true, "No": true,
	"Of": true, "On": true, "Or": true, "To": true, "Up": true,
	// Name prefixes and short product words ("McAfee", "RaspberryPi").
	"Mc": true, "Pi": true,
}

// mixedCaseSpellings are well-known product and protocol spellings that do not
// split into CamelCase words ("VoIPQualityOfService", "WiFiEnterprise",
// "mDNSResponder"). They are listed explicitly: accepting any lowercase letter
// before an acronym would also accept common random mixed-case fragments.
var mixedCaseSpellings = []string{
	"VoIP", "WiFi", "IoT", "iOS", "iPadOS", "macOS",
	"mDNS", "mTLS", "eBPF", "uRPF", "vNIC", "vCPU", "iSCSI", "FSx",
}

func mixedCaseSpellingAt(run string, i int) int {
	for _, spelling := range mixedCaseSpellings {
		if strings.HasPrefix(run[i:], spelling) {
			return len(spelling)
		}
	}
	return 0
}

// camelCaseWords reports whether a run of ASCII letters parses as words:
// a leading lowercase word or one-letter prefix ("vSwitch"),
// Capitalized words of three or more letters, common two-letter words
// ("SignIn", "LogOn"), a few well-known mixed spellings ("VoIP", "WiFi"), and
// acronyms of two or more capitals (alone, before a Capitalized word as in
// "HTTPServer", before a two-letter word as in "MFAAtSignIn", or with one
// trailing lowercase letter at the end of the run as in "IPv").
func camelCaseWords(run string) bool {
	previousPiece := 0
	for i := 0; i < len(run); {
		if n := mixedCaseSpellingAt(run, i); n > 0 {
			i += n
			previousPiece = n
			continue
		}
		uppers := 0
		for i+uppers < len(run) && run[i+uppers] >= 'A' && run[i+uppers] <= 'Z' {
			uppers++
		}
		lowers := 0
		for j := i + uppers; j+lowers < len(run) && run[j+lowers] >= 'a' && run[j+lowers] <= 'z'; {
			lowers++
		}
		atEnd := i+uppers+lowers == len(run)
		switch {
		case uppers == 0 && lowers >= 2 && i == 0:
		case uppers == 0 && lowers == 1 && i == 0 && capitalizedWordAt(run, 1): // vSwitch, iPhone
		case uppers == 1 && lowers >= 2:
		case uppers == 1 && lowers == 1 && commonShortCamelWords[run[i:i+2]]: // SignIn
		case uppers >= 2 && (lowers == 0 || lowers >= 2):
		case uppers >= 2 && lowers == 1 && atEnd: // IPv, IPv6
		case uppers == 1 && lowers == 0 && atEnd && previousPiece >= 3 && (run[i] == 'V' || run[i] == 'R'):
			// Version or release suffix before digits after a full word:
			// GatewayV2, ServersR2, LabV3. Other letters (model numbers such as
			// C9300L24T4G) are not accepted: they cost labeled-key catch rate.
			// One- and two-letter fragments stay rejected,
			// since random keys produce them often.
		case uppers >= 3 && lowers == 1 && commonShortCamelWords[run[i+uppers-1:i+uppers+1]]: // MFAAt
		default:
			return false
		}
		previousPiece = uppers + lowers
		i += uppers + lowers
	}
	return true
}

func isDigitByte(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

// alphanumericProductNames are product names spelled with digits that appear
// as their own CamelCase word in policy names ("AllowO365SignInForMFA",
// "M365AppsUpdate", "AwsS3Backup", "EC2InstanceConnect").
var alphanumericProductNames = []string{"O365", "M365", "S3", "EC2"}

// maskAlphanumericProductNames replaces each alphanumericProductNames word
// with "-" so the letters around it are judged as their own words. A name
// counts only at CamelCase boundaries: after the start, a separator or a
// lowercase letter, and before the end, a separator or a capital.
func maskAlphanumericProductNames(token string) string {
	var masked []byte
	for i := 0; i < len(token); i++ {
		if i > 0 && (isDigitByte(token[i-1]) || token[i-1] >= 'A' && token[i-1] <= 'Z') {
			continue
		}
		for _, name := range alphanumericProductNames {
			end := i + len(name)
			if end > len(token) || token[i:end] != name ||
				end < len(token) && (isDigitByte(token[end]) || token[end] >= 'a' && token[end] <= 'z') {
				continue
			}
			if masked == nil {
				masked = []byte(token)
			}
			for j := i; j < end; j++ {
				masked[j] = '-'
			}
			break
		}
	}
	if masked == nil {
		return token
	}
	return string(masked)
}

// readsAsWords reports whether a token is built from words rather than random
// characters ("ZscalerClientConnectorRollout2024", "uswest2production01primary",
// "Projects/2024/NetworkSwitch2024"). At least 60% of its letters must sit in
// letter runs of three or more. If the token mixes upper and lower case, every
// such run must parse as CamelCase or acronym words; random mixed-case letters
// almost never do, because isolated lowercase letters are common in them. A
// single-case token must instead have a word-like share of vowels (30%;
// random letters average about 19%). Digit-spelled product names ("O365")
// are masked first.
func readsAsWords(token string) bool {
	token = maskAlphanumericProductNames(token)
	var upper, lower, vowels, inRuns int
	var runs []string
	for _, run := range strings.FieldsFunc(token, func(r rune) bool { return !isASCIILetter(r) }) {
		if len(run) >= 3 {
			inRuns += len(run)
			runs = append(runs, run)
		}
	}
	for i := 0; i < len(token); i++ {
		switch ch := token[i]; {
		case ch >= 'A' && ch <= 'Z':
			upper++
		case ch >= 'a' && ch <= 'z':
			lower++
		default:
			continue
		}
		switch lowerASCII(token[i]) {
		case 'a', 'e', 'i', 'o', 'u':
			vowels++
		}
	}
	letters := upper + lower
	if letters < 4 || inRuns*10 < letters*6 {
		return false
	}
	if upper == 0 || lower == 0 {
		return vowels*10 >= letters*3
	}
	for _, run := range runs {
		if !camelCaseWords(run) {
			return false
		}
	}
	return true
}

// singleCaseWordAndNumber reports whether a segment is one single-case run of
// four or more letters joined to one run of digits ("crowdstrikefalcon2024",
// "POSTGRESQL16"). Compound product names often have too few vowels for
// readsAsWords, but random key material essentially never keeps all of its
// letters and all of its digits in two contiguous runs.
func singleCaseWordAndNumber(segment string) bool {
	letters, digits, switches := 0, 0, 0
	for i := 0; i < len(segment); i++ {
		switch ch := segment[i]; {
		case isDigitByte(ch):
			digits++
		case isASCIILetter(rune(ch)):
			letters++
		default:
			return false
		}
		if i > 0 && isDigitByte(segment[i]) != isDigitByte(segment[i-1]) {
			switches++
		}
	}
	return switches == 1 && letters >= 4 && digits > 0 && !hasMixedCaseLetters(segment)
}

// isReadablePath reports whether a "/"-separated value is a path of words
// rather than Base64 key material ("Network/PointToSite",
// "projects/2024/networkplanning", "/var/lib/postgresql16",
// "CORP/NYC/IDF01/SW02/PORT48"). A single-case segment must be short, a
// number, or read as words; a mixed-case segment must be CamelCase words with
// at most a trailing number in each "."/"-"/"_" piece. Random Base64 split on
// "/" leaves long mixed-case fragments that fail, or contains "+".
func isReadablePath(value string) bool {
	path := strings.TrimPrefix(value, "/")
	if !strings.Contains(path, "/") && path == value {
		return false
	}
	if path == "" {
		return false
	}
	for _, segment := range strings.Split(path, "/") {
		if !readablePathSegment(segment) {
			return false
		}
	}
	return true
}

const readablePathSegmentMax = 16

func readablePathSegment(segment string) bool {
	if segment == "" {
		return false
	}
	// Administrative shares end in "$" ("C$", "IT$").
	if len(segment) > 1 && segment[len(segment)-1] == '$' {
		segment = segment[:len(segment)-1]
	}
	// "Program Files (x86)": each space-separated word, without surrounding
	// parentheses, must read on its own.
	if strings.Contains(segment, " ") {
		for _, word := range strings.Split(segment, " ") {
			if !readablePathSegment(strings.TrimSuffix(strings.TrimPrefix(word, "("), ")")) {
				return false
			}
		}
		return true
	}
	for i := 0; i < len(segment); i++ {
		if !isASCIIAlnum(segment[i]) && strings.IndexByte("._-", segment[i]) < 0 {
			return false
		}
	}
	if !hasMixedCaseLetters(segment) {
		return len(segment) <= readablePathSegmentMax || singleCaseWordAndNumber(segment) || readsAsWords(segment)
	}
	for _, piece := range strings.FieldsFunc(segment, func(r rune) bool { return r == '.' || r == '-' || r == '_' }) {
		if !isAllDigits(piece) && !camelCaseWordsAndNumber(piece) {
			return false
		}
	}
	return true
}

// camelCaseWordsAndNumber reports whether piece is a run of three or more
// letters that parses as CamelCase words, optionally followed by digits
// ("PointToSite", "NetworkSwitch2024").
func camelCaseWordsAndNumber(piece string) bool {
	letters := 0
	for letters < len(piece) && isASCIILetter(rune(piece[letters])) {
		letters++
	}
	return letters >= 3 && (letters == len(piece) || isAllDigits(piece[letters:])) &&
		camelCaseWords(piece[:letters])
}

// isReadableWorkingDirectory reports whether a pwd or cwd value is an
// absolute filesystem path such as "/opt/enterprise2024", "C:\Python312",
// "C:\Users\Admin\Projects", "\\fileserver\share\reports" or
// "\\?\C:\Temp\run2024" rather than a password. Every segment must be
// readable, except that the last segment of a path three or more deep may be
// a short file name ("C:\Windows\Temp\MSI8f3a2.LOG").
func isReadableWorkingDirectory(value string) bool {
	value = strings.TrimPrefix(value, `\\?\`)
	switch {
	case strings.HasPrefix(value, `\\`):
		value = value[2:]
	case isWindowsDrivePath(value):
		value = value[3:]
	case strings.HasPrefix(value, "/"):
		value = value[1:]
	default:
		return false
	}
	value = strings.TrimSuffix(strings.ReplaceAll(value, `\`, "/"), "/")
	if value == "" {
		return false
	}
	segments := strings.Split(value, "/")
	for i, segment := range segments {
		if !readablePathSegment(segment) &&
			(i != len(segments)-1 || len(segments) < 3 || !shortFileName(segment)) {
			return false
		}
	}
	return true
}

// shortFileName reports whether segment is a short ASCII file name.
func shortFileName(segment string) bool {
	if len(segment) > readablePathSegmentMax {
		return false
	}
	for i := 0; i < len(segment); i++ {
		if !isASCIIAlnum(segment[i]) && strings.IndexByte("._-", segment[i]) < 0 {
			return false
		}
	}
	return true
}

// isWindowsDrivePath reports whether value starts with a drive root ("C:\",
// "C:/").
func isWindowsDrivePath(value string) bool {
	return len(value) > 3 && isASCIILetter(rune(value[0])) && value[1] == ':' && (value[2] == '\\' || value[2] == '/')
}

// readableWorkingDirectoryAt reports whether the text at start is a readable
// working directory up to the next space, quote or delimiter, or up to the next
// quote or delimiter for paths with spaces ("C:\Program Files (x86)\Edge").
// JSON-escaped backslashes ("C:\\Users") are read as single backslashes.
func readableWorkingDirectoryAt(in string, start int) bool {
	for _, stops := range []string{" \t\r\n\"'`,;)]}", "\t\r\n\"'`,;]}"} {
		end := start
		for end < len(in) && end-start < 256 && strings.IndexByte(stops, in[end]) < 0 {
			end++
		}
		value := strings.TrimRight(in[start:end], ".")
		if isReadableWorkingDirectory(value) || isReadableWorkingDirectory(collapseEscapedBackslashes(value)) {
			return true
		}
	}
	return false
}

// collapseEscapedBackslashes reads a JSON-escaped path, possibly escaped more
// than once ("C:\\\\Users\\\\Admin\\"): trailing backslashes (an escaped
// closing quote) are dropped and each run of backslashes becomes one, except
// that a leading run stays a UNC prefix ("\\\\server" reads "\\server").
func collapseEscapedBackslashes(value string) string {
	value = strings.TrimRight(value, `\`)
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] != '\x5c' {
			b.WriteByte(value[i])
			continue
		}
		run := i
		for i+1 < len(value) && value[i+1] == '\x5c' {
			i++
		}
		if run == 0 && i > 0 {
			b.WriteString(`\\`)
		} else {
			b.WriteByte('\x5c')
		}
	}
	return b.String()
}

// assignedTokenValue splits a "name=value" token whose name is a word label
// ("PartitionKey=...", "pwd=/srv/x") and returns the value. Base64 only has
// "=" as trailing padding, so an inner "=" after letters is an assignment.
func assignedTokenValue(token string) (string, bool) {
	eq := strings.IndexByte(token, '=')
	if eq <= 0 || eq == len(token)-1 || token[eq+1] == '=' {
		return "", false
	}
	for i := 0; i < eq; i++ {
		if !isASCIILetter(rune(token[i])) && token[i] != '_' && token[i] != '-' {
			return "", false
		}
	}
	return token[eq+1:], true
}

func isASCIILetter(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

type highEntropyContext int

const (
	highEntropyFreeText highEntropyContext = iota
	highEntropyStructured
)

func (r Redactor) scanStringWithEntropy(in string, context highEntropyContext) (string, Report) {
	out, report := r.ScanString(in)
	// JSON escapes can split one decoded token into raw fragments too short for
	// the entropy regex. An escape therefore requires token-aware inspection
	// even when the serialized bytes have no contiguous candidate.
	if !strings.Contains(out, `\`) && !highEntropyFreeTextTokenRE.MatchString(out) &&
		(context != highEntropyFreeText || !shortFreeTextTokenRE.MatchString(out)) {
		return out, report
	}
	entropyScanner := func(value string) (string, Report) {
		return r.scanEntropy(value, Report{}, context)
	}
	scanner := withEmbeddedJSONStrings(entropyScanner)
	if structured, entropyReport, ok := scanStructuredDocuments(out, scanner, false); ok {
		return structured, mergeReports(report, entropyReport)
	}
	scanned, scanReport := scanner(out)
	return scanned, mergeReports(report, scanReport)
}

func (r Redactor) scanEntropy(out string, report Report, context highEntropyContext) (string, Report) {
	out, report = r.scanLongEntropy(out, report, context)
	if context == highEntropyFreeText {
		out, report = scanShortFreeTextTokens(out, report, r.mode)
	}
	return out, report
}

// Free text is where admins paste PoC keys, and many API keys are 20-31
// characters, below the general 32-character floor. In free text only, a
// single unseparated alphanumeric run of 24-31 characters that mixes upper,
// lower and digits and has high entropy is treated as key material. The
// stricter character and entropy conditions keep ticket numbers, circuit IDs
// and CamelCase product names intact.
const (
	shortFreeTextTokenEntropy = 3.7
)

var shortFreeTextTokenRE = regexp.MustCompile(`\b[A-Za-z0-9]{24,31}\b`)

func scanShortFreeTextTokens(out string, report Report, mode Mode) (string, Report) {
	matches := shortFreeTextTokenRE.FindAllStringIndex(out, -1)
	if len(matches) == 0 {
		return out, report
	}
	var b strings.Builder
	last := 0
	count := 0
	for _, match := range matches {
		token := out[match[0]:match[1]]
		if !hasDigit(token) || !hasLetter(token) || readsAsWords(token) || singleCaseWordAndNumber(token) ||
			readsAsNameWithNumbers(token) || dottedNameSegment(out, match[0], match[1]) ||
			shannonEntropy(token) < shortFreeTextTokenEntropy {
			continue
		}
		if mode == ModeStandard && (isBarePublicIdentifier(token) || isContextualPublicValue(out, match[0], match[1])) {
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
	return b.String(), addReportCount(report, "high_entropy_short_free_text_token", count)
}

// readsAsNameWithNumbers reports whether a short token is a name built from
// words, acronyms and at most two numbers ("EnableQoSForMicrosoftTeams2026",
// "nyc01zscalerconnectorprod02", "Enable8021XOnBranchPorts2026"): digits in
// at most two runs, at least half the letters in lowercase runs of two or more
// that contain a vowel, and at least 20% vowels in those runs. Random keys
// scatter their digits and rarely form pronounceable lowercase runs.
func readsAsNameWithNumbers(token string) bool {
	return nameWithNumbers(token, 2)
}

// nameWithNumbers is readsAsNameWithNumbers with at most maxDigitRuns runs of
// digits.
func nameWithNumbers(token string, maxDigitRuns int) bool {
	digitRuns, letters, wordLetters, wordVowels := 0, 0, 0, 0
	for i := 0; i < len(token); {
		ch := token[i]
		switch {
		case ch >= '0' && ch <= '9':
			for i < len(token) && token[i] >= '0' && token[i] <= '9' {
				i++
			}
			digitRuns++
		case ch >= 'a' && ch <= 'z':
			start, vowels := i, 0
			for i < len(token) && token[i] >= 'a' && token[i] <= 'z' {
				if strings.IndexByte("aeiouy", token[i]) >= 0 {
					vowels++
				}
				i++
			}
			letters += i - start
			if i-start >= 2 && vowels > 0 {
				wordLetters += i - start
				wordVowels += vowels
			}
		default:
			letters++
			i++
		}
	}
	return digitRuns <= maxDigitRuns && letters > 0 && wordLetters*2 >= letters && wordVowels*5 >= wordLetters
}

// dottedNameSegment reports whether in[start:end] sits between dots, as a
// segment of a file or host name ("app.652f8a1b9c7d4e30a56b2f90.js").
func dottedNameSegment(in string, start, end int) bool {
	return start > 0 && end < len(in) && in[start-1] == '.' && in[end] == '.'
}

func (r Redactor) scanLongEntropy(out string, report Report, context highEntropyContext) (string, Report) {
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
	// custom, when set, replaces re/replacement: it returns the scanned text
	// and the number of redactions.
	custom func(string) (string, int)
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
	markerEmail           = `<REDACTED:EMAIL>`
	markerIP              = `<REDACTED:IP>`

	provisioningAssignmentKeys = `provision(?:ing)?[_ -]?key|provision[_ -]?token|enrollment[_ -]?token|oauth[_ -]?2[_ -]?enrollment[_ -]?token`
	privateKeyAssignmentKeys   = `ssh[_-]?private[_-]?key|private[_-]?key|certBlob|zrsaencryptedprivatekey|zrsaencryptedsessionkey`
	secretAssignmentKeys       = `authorization|cookie|set[_-]?cookie|session(?:[_-]?id)?|client[_-]?secret|secret|secret[_-]?key|key[_-]?secret|api[_-]?key|api[_-]?token|sandbox[_-]?api[_-]?token|auth[_-]?token|authentication[_-]?token|hec[_-]?token|password|vnc[_-]?password|ssh[_-]?passphrase|ssh[_-]?private[_-]?key[_-]?passphrase|passphrase|psk|pre[_ -]?shared[_ -]?key|shared[_ -]?secret|refresh[_-]?token|access[_-]?token|bearer[_-]?token|jwt[_-]?token|jwt|token|otp|one[_-]?time[_-]?password|temporary[_-]?password`
	secretPhraseKeys           = `client[_ -]?secret|secret[_ -]?key|key[_ -]?secret|api[_ -]?key|api[_ -]?token|sandbox[_ -]?api[_ -]?token|bearer[_ -]?token|refresh[_ -]?token|access[_ -]?token|jwt[_ -]?token|auth[_ -]?token|hec[_ -]?token|psk|pre[_ -]?shared[_ -]?key|shared[_ -]?secret|provision(?:ing)?[_ -]?key|provision[_ -]?token|enrollment[_ -]?token|passphrase|private[_ -]?key|device[_ -]?token|one[_ -]?time[_ -]?token|one[_ -]?time[_ -]?password|temporary[_ -]?password|otp` // #nosec G101 -- redaction keyword patterns (field-name matchers), not a secret
)

var authorizationHeaderRE = regexp.MustCompile(`(?i)(authorization\s*[:=]\s*)\S.*`)

var credentialURLRE = regexp.MustCompile(`(?i)([a-z][a-z0-9+.-]*://)[^/\s:@]+:[^/\s]+@`)

var baseRules = buildBaseRules()

// The plaintext assignment rules can begin within a longer key, but only when
// the sensitive fragment reaches the key's closing quote. Preserve that suffix
// behavior when classifying decoded JSON keys.
var jsonProvisioningAssignmentKeyRE = regexp.MustCompile(`(?i)(?:` + provisioningAssignmentKeys + `)$`)
var jsonPrivateKeyAssignmentKeyRE = regexp.MustCompile(`(?i)(?:` + privateKeyAssignmentKeys + `)$`)
var jsonSecretAssignmentKeyRE = regexp.MustCompile(`(?i)(?:` + secretAssignmentKeys + `)$`)
var genericCredentialJSONKeyRE = regexp.MustCompile(`(?i)^(?:key|pwd|passwd|bearer)$`)

var highEntropyFreeTextTokenRE = regexp.MustCompile(`\b[A-Za-z0-9][A-Za-z0-9._~+/=-]{31,}\b`)
var canonicalUUIDRE = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var compactUUIDRE = regexp.MustCompile(`(?i)^[0-9a-f]{32}$`)
var publicHexFingerprintRE = regexp.MustCompile(`(?i)^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
var gitSHARE = regexp.MustCompile(`(?i)^[0-9a-f]{40}$`)
var gitSHAContextRE = regexp.MustCompile(`(?i)(?:\b(?:git|commit|sha|revision|rev)\b(?:\s+(?:is|was))?[\s:=#-]*)$`)

// Public identifier shapes. Cloud resource IDs and ULIDs (whose leading
// timestamp character pair is "01" for current dates) are recognizable on
// their own. Abbreviated git revisions, MongoDB ObjectIds, KSUIDs and digests
// look like key material and count only right after their naming context.
var cloudResourceIDRE = regexp.MustCompile(`^(?:i|sg|subnet|vpc|vpce|ami|vol|snap|eni|rtb|tgw|igw|nat|acl)-(?:[0-9a-f]{8}|[0-9a-f]{17})$`)
var azureResourceIDRE = regexp.MustCompile(`(?i)^/subscriptions/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}(?:/|$)`)
var ulidRE = regexp.MustCompile(`^01[0-9A-HJKMNP-TV-Z]{24}$`)
var gitRevisionRE = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
var objectIDRE = regexp.MustCompile(`^[0-9a-f]{24}$`)
var objectIDContextRE = regexp.MustCompile(`(?i)(?:\b_id|\bobject\s?id|\boid)["']?\s*[:=(]?\s*["']?$`)
var ksuidRE = regexp.MustCompile(`^[0-9A-Za-z]{27}$`)
var ksuidContextRE = regexp.MustCompile(`(?i)\bksuid\s*[:=]?\s*$`)

// digestContextRE requires a digest noun. Algorithm names alone (MD5, SHA256)
// are not digest context: they describe HMAC keys as often as checksums
// ("HMAC key (SHA256): <hex>"), so such values keep main's entropy decision.
var digestContextRE = regexp.MustCompile(`(?i)\b(?:fingerprint|thumbprint|checksum|digest)\b[^\n]{0,40}$`)

// credentialAfterDigestRE matches a credential word between a digest noun and
// the value ("checksum of the signing key: <hex>"): the value is the
// credential, not a public digest. A credential word before the digest noun
// ("Key fingerprint is <hex>") keeps the exemption. The credential word may
// end a compound label ("SecretKey", "signing_key"), so only its end is
// anchored; a coincidental match only redacts more.
var credentialAfterDigestRE = regexp.MustCompile(`(?i)\b(?:fingerprint|thumbprint|checksum|digest)\b[^\n]*(?:keys?|secrets?|tokens?|passwords?|passwd|pwd|credentials?|bearer)\b[^\n]{0,24}$`)

const publicValueContextWindow = 48

// isBarePublicIdentifier reports whether value is a public identifier that is
// recognizable without context: a cloud resource ID or a ULID.
func isBarePublicIdentifier(value string) bool {
	return cloudResourceIDRE.MatchString(value) || ulidRE.MatchString(value)
}

// isPublicIdentifierValue reports whether value is a UUID, a hex digest of a
// fingerprint length, or a bare public identifier.
func isPublicIdentifierValue(value string) bool {
	return canonicalUUIDRE.MatchString(value) || compactUUIDRE.MatchString(value) ||
		publicHexFingerprintRE.MatchString(value) || isBarePublicIdentifier(value)
}

// isContextualPublicValue reports whether text[start:end] is a public
// identifier named by the words right before it: an abbreviated git revision
// after commit/revision/SHA, an ObjectId after _id/ObjectId, a KSUID after
// KSUID, or a digest-length hex or Base64 value after
// fingerprint/thumbprint/checksum/digest/MD5/SHA. The context window is
// bounded, so the check is constant work per token.
func isContextualPublicValue(text string, start, end int) bool {
	for end < len(text) && end-start < 128 && text[end] == '=' {
		end++ // Base64 padding is outside the token's word boundary
	}
	token := text[start:end]
	contextStart := start - publicValueContextWindow
	if contextStart < 0 {
		contextStart = 0
	}
	context := text[contextStart:start]
	switch {
	case gitRevisionRE.MatchString(token) && gitSHAContextRE.MatchString(context):
		return true
	case objectIDRE.MatchString(token) && objectIDContextRE.MatchString(context):
		return true
	case ksuidRE.MatchString(token) && ksuidContextRE.MatchString(context):
		return true
	case digestShaped(token) && digestContextRE.MatchString(context) && !credentialAfterDigestRE.MatchString(context):
		return true
	}
	return false
}

// digestShaped reports whether value has the exact length of an MD5, SHA-1 or
// SHA-2 digest in hex, or in Base64 with or without its padding.
func digestShaped(value string) bool {
	if isHex(value) {
		switch len(value) {
		case 32, 40, 56, 64, 96, 128:
			return true
		}
		return false
	}
	body := strings.TrimRight(value, "=")
	if !isBase64Text(body) || len(value)-len(body) > 2 {
		return false
	}
	switch len(body) {
	case 22, 27, 38, 43, 64, 86:
		return len(value) == len(body) || len(value)%4 == 0
	}
	return false
}

func classifyJSONKey(key string) ([3]jsonSensitiveClassification, int) {
	var classifications [3]jsonSensitiveClassification
	count := 0
	if jsonProvisioningAssignmentKeyRE.MatchString(key) {
		classifications[count] = jsonSensitiveClassification{
			ruleName: "provisioning_key_assignment",
			marker:   markerProvisioningKey,
			priority: 1,
		}
		count++
	}
	if jsonPrivateKeyAssignmentKeyRE.MatchString(key) {
		classifications[count] = jsonSensitiveClassification{
			ruleName: "private_key_assignment",
			marker:   markerPrivateKey,
			priority: 2,
		}
		count++
	}
	if jsonSecretAssignmentKeyRE.MatchString(key) {
		classifications[count] = jsonSensitiveClassification{
			ruleName: "secret_assignment",
			marker:   markerSecret,
			priority: 3,
		}
		count++
	}
	return classifications, count
}

func buildBaseRules() []rule {
	rules := []rule{
		{
			name:        "private_key_block",
			re:          regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY(?: BLOCK)?-----.*?-----END [A-Z ]*PRIVATE KEY(?: BLOCK)?-----`),
			replacement: markerPrivateKey,
			prefilter:   containsFold("private key"),
		},
		// A private key pasted without its END line (truncated, or cut off by a
		// field limit): redact from the BEGIN line through the armour body that
		// follows, which is Base64, line breaks (raw or JSON-escaped) and PEM or
		// PGP headers ("Proc-Type: 4,ENCRYPTED", "Version: ..."). It stops at the
		// first character armour cannot contain, so later prose ending in
		// punctuation survives.
		{
			name:        "private_key_block",
			re:          regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY(?: BLOCK)?-----(?:[A-Za-z0-9+/=:,. \t\r\n-]|\\[nrt])*`),
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
			name:      "credential_url",
			prefilter: all(contains("://"), contains("@")),
			custom:    scanCredentialURLs,
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
	// Admins paste PoC keys behind generic labels ("POC key: ...", "token ...",
	// "Bearer ...") that the compound-key rules above do not name. Only the
	// value is replaced, and only when it is credential-shaped, so prose such
	// as "key: production" or "token bucket rate 100" survives.
	// Well-known vendor token formats are recognized by prefix at any length
	// above the format's minimum, which catches keys below the general entropy
	// floor with very few false positives.
	rules = append(rules, rule{
		name:      "known_token_prefix",
		custom:    scanKnownTokenPrefixes,
		prefilter: containsAnyFold("gh", "github_pat_", "glpat", "glrt", "hvs.", "xox", "_live_", "_test_", "aiza", "sg.", "akia", "asia", "npm_", "shp", "sk"),
	})
	rules = append(rules, rule{
		name:      "generic_credential_label",
		prefilter: containsAnyFold(genericCredentialLabels...),
		custom:    scanGenericCredentialLabels,
	})
	// Paste formats the label rules above cannot bound: punctuation passwords,
	// CLI and PowerShell arguments, table and next-line values, encoded query
	// signatures and Basic credentials. It runs last so it only adds redactions,
	// and gates itself with pasteCredentialMayMatch, whose non-ASCII labels a
	// rulePrefilter cannot express.
	rules = append(rules, rule{
		name:   "paste_credential",
		custom: scanPasteCredentials,
	})
	return rules
}

// knownTokenPrefixRE matches common vendor token formats: GitHub, GitLab
// (personal and runner tokens), HashiCorp Vault service tokens, Slack, Stripe,
// Google API keys, SendGrid, AWS access key IDs, npm, Shopify and Twilio API
// key SIDs. An AWS access key ID must end at a token boundary, so
// "ASIAREGIONALHEADQUARTERS" is a name, not a key ID.
var knownTokenPrefixRE = regexp.MustCompile(`\b(?:gh[pousr]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,}|gl(?:pat|rt)-[A-Za-z0-9_-]{20,}|hvs\.[A-Za-z0-9_-]{24,}|xox[abposr]-[A-Za-z0-9-]{10,}|(?:sk|rk)_(?:live|test)_[A-Za-z0-9]{16,}|AIza[0-9A-Za-z_-]{30,}|SG\.[A-Za-z0-9_-]{16,}\.[A-Za-z0-9_-]{16,}|(?:AKIA|ASIA)[0-9A-Z]{16}\b|npm_[A-Za-z0-9]{30,}|shp(?:at|ca|pa|ss)_[a-fA-F0-9]{32}|SK[0-9a-f]{32})`)

// scanKnownTokenPrefixes redacts knownTokenPrefixRE matches, except
// documentation names that only share a vendor prefix (a "glpat-" prefix
// followed by runbook words such as token-rotation-runbook, or
// "github_pat_rotation_and_revocation").
func scanKnownTokenPrefixes(in string) (string, int) {
	matches := knownTokenPrefixRE.FindAllStringIndex(in, -1)
	var b strings.Builder
	last, count := 0, 0
	for _, match := range matches {
		if knownTokenSuffixIsWords(in[match[0]:match[1]]) {
			continue
		}
		b.WriteString(in[last:match[0]])
		b.WriteString(markerSecret)
		last = match[1]
		count++
	}
	if count == 0 {
		return in, 0
	}
	b.WriteString(in[last:])
	return b.String(), count
}

// knownTokenSuffixIsWords reports whether the part after a separator-style
// vendor prefix (github_pat_, glpat-, xox?-) is two or more single-case,
// letters-only words joined by "-" or "_". Real tokens of these formats mix
// case or contain digits.
func knownTokenSuffixIsWords(token string) bool {
	var suffix string
	switch {
	case strings.HasPrefix(token, "github_pat_"):
		suffix = token[len("github_pat_"):]
	case strings.HasPrefix(token, "glpat-"), strings.HasPrefix(token, "glrt-"):
		suffix = token[strings.IndexByte(token, '-')+1:]
	case strings.HasPrefix(token, "xox") && len(token) > 5 && token[4] == '-':
		suffix = token[5:]
	default:
		return false
	}
	segments := strings.FieldsFunc(suffix, func(r rune) bool { return r == '-' || r == '_' })
	if len(segments) < 2 || hasMixedCaseLetters(suffix) {
		return false
	}
	for _, segment := range segments {
		if !isLettersOnly(segment) {
			return false
		}
	}
	return readsAsWords(suffix)
}

var genericCredentialLabels = []string{"key", "token", "secret", "passw", "pwd", "bearer", "sig", "credential"}

// genericCredentialLabelWordRE finds candidate label words; leftmost-first
// alternation prefers "password" over "passwd" and "pwd".
var genericCredentialLabelWordRE = regexp.MustCompile(`(?i)credentials?|password|passwd|bearer|secret|token|key|pwd|sig`)

// genericCredentialPhraseRE matches, anchored right after a label word, natural
// phrasing that puts a few words between the label and its value ("key for the
// vendor portal is X", "token was X", "token for the test index: X"): up to six
// filler words on the same line, then a ":"/"=" or "is"/"was"/"are"/"were" cue,
// then any opening wrappers.
var genericCredentialPhraseRE = regexp.MustCompile(`(?i)^(?:[^\S\n]+[^\s:=]{1,30}){0,6}?(?:[^\S\n]*[:=]|[^\S\n]+(?:is|was|are|were)[^\S\n]+)[^\S\n]*(?:[*"'(\[<{\x60\x{2018}\x{2019}\x{201A}\x{201B}\x{201C}\x{201D}\x{201E}\x{201F}\x{00AB}\x{00BB}\x{2039}\x{203A}]|[^\S\n])*`)

// genericCredentialSeparatorRE matches, anchored right after a label word, an
// optional closing quote, a ":"/"=" or whitespace separator, and any opening
// wrappers or further whitespace before the value. Quotes may be escaped with
// backslashes (`key: \"...\"` inside a JSON string); a backslash not followed by
// a quote is left to the value (UNC paths). It accepts Unicode space
// separators (no-break space) and format characters (zero-width space), which
// RE2's ASCII \s does not match.
var genericCredentialSeparatorRE = regexp.MustCompile(`^[*_]{0,3}(?:\x5c*["'\x60\x{2018}\x{2019}\x{201A}\x{201B}\x{201C}\x{201D}\x{201E}\x{201F}\x{00AB}\x{00BB}\x{2039}\x{203A}])?[*_]{0,3}(?:[\s\p{Zs}\p{Cf}]*[:=][\s\p{Zs}\p{Cf}]*|[\s\p{Zs}\p{Cf}]+)(?:\x5c+["'\x60\x{2018}\x{2019}\x{201A}\x{201B}\x{201C}\x{201D}\x{201E}\x{201F}\x{00AB}\x{00BB}\x{2039}\x{203A}]|[*"'(\[<{\x60\x{2018}\x{2019}\x{201A}\x{201B}\x{201C}\x{201D}\x{201E}\x{201F}\x{00AB}\x{00BB}\x{2039}\x{203A}]|[\s\p{Zs}\p{Cf}])*`)

type genericCredentialLabel struct {
	start      int  // label word start
	wordEnd    int  // first byte after the label word
	valueStart int  // first byte after the separator and wrappers
	password   bool // a password or explicit credential label: word-and-number values count
}

// scanGenericCredentialLabels redacts credential-shaped values that follow a
// generic credential label. It first collects every valid label: a label word
// not preceded by a letter or digit ("monkey" and "apikey" are not labels;
// "poc_key" and "x-api-key" are), followed by a separator, with a non-empty
// value. A value runs up to the next valid label, so nested labels
// ("key: Bearer <key>", "key: key=<uuid>") are each examined as their own
// label. If that shorter value is rejected and a later label sits inside the
// same unbroken run of value characters, the whole run is judged once, so a key
// that happens to contain "_key=" is still caught. Each run is judged whole at
// most once, which keeps the scan linear. A quoted value that is a JSON string
// literal with escapes is judged on its decoded content, including any labels
// the decoding reveals.
func scanGenericCredentialLabels(in string) (string, int) {
	var labels []genericCredentialLabel
	for _, word := range genericCredentialLabelWordRE.FindAllStringIndex(in, -1) {
		if word[0] > 0 && isASCIIAlnum(in[word[0]-1]) && !camelCaseLabelStart(in, word[0]) {
			continue
		}
		password := false
		labelWord := strings.ToLower(in[word[0]:word[1]])
		switch labelWord {
		case "password", "passwd", "pwd":
			password = true
		}
		modifier := labelModifierBefore(in, word[0])
		if labelWord == "key" && nonCredentialKeyModifiers[modifier] {
			continue // "Primary key", "PartitionKey", "Tag key": not a credential
		}
		// "API key:", "Access token:", "Client secret:" name the credential
		// explicitly, so a word-built value after them is still judged as a
		// credential, as after a password label.
		wordValues := password || credentialKeyModifiers[modifier]
		if labelWord == "key" && jsonPairKeyIdentifierAt(in, word[0], word[1]) {
			continue // {"key": <public identifier>, "value": ...} inside prose
		}
		valueStart := -1
		if separator := genericCredentialSeparatorRE.FindStringIndex(in[word[1]:]); separator != nil {
			if start := word[1] + separator[1]; startsCredentialValue(in, start) {
				valueStart = start
			}
		}
		// Natural phrasing: when the word right after the label is not
		// credential-shaped, use the value after a nearby ":"/"is"/"was" cue.
		// The looser password rule applies only to a value directly after
		// the label ("Password reset ticket: INC0012345" is a reference).
		phrased := false
		if phrase := genericCredentialPhraseRE.FindStringIndex(in[word[1]:]); phrase != nil {
			if start := word[1] + phrase[1]; start != valueStart && startsCredentialValue(in, start) &&
				(valueStart < 0 || !credentialShapedValue(credentialValuePrefix(in, valueStart), wordValues)) {
				valueStart = start
				phrased = true
			}
		}
		if valueStart < 0 {
			continue // no value: not a label, and must not cut a preceding value
		}
		// "Token ID: ...", "Password hash algorithm is ...": the value
		// describes the credential, it is not the credential.
		// Password labels and credential-prefixed labels must be followed
		// directly by the noun ("Password hash algorithm", "API token ID");
		// other labels may name it later ("Key rotation implementation commit:").
		if phrased && phraseNamesMetadata(in[word[1]:valueStart], wordValues) {
			continue
		}
		if labelWord == "key" && settingKeyModifiers[modifier] &&
			settingKeyNameValue(credentialValuePrefix(in, valueStart)) {
			continue // "Configuration key: EnableQoSForMicrosoftTeams"
		}
		if phrased && phraseNamesIdentifiedObject(in[word[1]:valueStart]) &&
			isPublicIdentifierValue(credentialValuePrefix(in, valueStart)) {
			continue
		}
		// "pwd: /opt/app" is a working directory.
		if labelWord == "pwd" && readableWorkingDirectoryAt(in, valueStart) {
			continue
		}
		labels = append(labels, genericCredentialLabel{word[0], word[1], valueStart, wordValues && !phrased})
	}
	if len(labels) == 0 {
		return in, 0
	}

	var b strings.Builder
	last, count := 0, 0
	runStart, runEnd, runJudged := -1, -1, false
	for i, label := range labels {
		if label.start < last {
			continue // inside a value already replaced
		}
		limit := len(in)
		if i+1 < len(labels) {
			limit = labels[i+1].start
		}
		start, end := label.valueStart, label.valueStart

		// A quoted JSON literal with escapes anywhere in the separator.
		if quote := strings.LastIndexByte(in[label.wordEnd:label.valueStart], '"'); quote >= 0 {
			literalStart := label.wordEnd + quote
			if literalEnd := jsonStringEnd(in, literalStart); literalEnd > 0 && strings.Contains(in[literalStart:literalEnd], "\x5c") {
				var decoded string
				if json.Unmarshal([]byte(in[literalStart:literalEnd]), &decoded) == nil {
					if credentialShapedValue(strings.TrimSpace(decoded), label.password) {
						start, end = literalStart+1, literalEnd-1
					} else if _, nested := scanGenericCredentialLabels(decoded); nested > 0 {
						start, end = literalStart+1, literalEnd-1
					}
				}
			}
		}

		if end == start {
			for end < limit && isCredentialValueByte(in[end]) {
				end++
			}
			for end > start && in[end-1] == '.' {
				end--
			}
			if end == start || !credentialShapedValue(in[start:end], label.password) {
				end = start
				// Judge the whole unbroken run once when a later label cut it.
				if start >= runEnd {
					runStart, runEnd, runJudged = start, start, false
					for runEnd < len(in) && isCredentialValueByte(in[runEnd]) {
						runEnd++
					}
				}
				if !runJudged && runStart == start && runEnd > limit {
					runJudged = true
					whole := runEnd
					for whole > start && in[whole-1] == '.' {
						whole--
					}
					if credentialShapedValue(in[start:whole], label.password) {
						end = whole
					}
				}
				if end == start {
					continue
				}
			}
		}
		b.WriteString(in[last:start])
		b.WriteString(markerSecret)
		last = end
		count++
	}
	if count == 0 {
		return in, 0
	}
	b.WriteString(in[last:])
	return b.String(), count
}

// Wrapper punctuation around a value ("<key>", "(key)", typographic quotes) is
// not part of it. Trailing sentence periods are not part of it either.
const (
	credentialValueOpeners = "*_\"'([<{`‘’‚‛“”„‟«»‹›"
	credentialValueClosers = "*_.\"')]>}`‘’‚‛“”„‟«»‹›"
)

// camelCaseLabelStart reports whether a label word at i ends a CamelCase or
// camelCase identifier ("AccountKey=", "authToken:"): it starts with a capital
// that follows a lowercase letter or digit.
// nonCredentialKeyModifiers name keys that are data or metadata identifiers,
// not credentials: database and table keys, tag keys, cache and idempotency
// keys, hot keys, registry keys and public keys. "Primary" is deliberately
// absent: Azure names subscription and storage access keys "Primary key".
var nonCredentialKeyModifiers = map[string]bool{
	"dedup": true, "natural": true, "candidate": true,
	"foreign": true, "partition": true, "row": true, "sort": true,
	"range": true, "composite": true, "surrogate": true, "unique": true, "lookup": true,
	"tag": true, "object": true, "cache": true, "idempotency": true, "hot": true,
	"registry": true, "record": true, "asset": true, "public": true,
}

// credentialKeyModifiers name keys that are credentials; only an immediate
// metadata noun ("API key name is ...") withdraws the label.
var credentialKeyModifiers = map[string]bool{
	"api": true, "access": true, "secret": true, "account": true, "subscription": true,
	"client": true, "private": true, "shared": true, "poc": true, "integration": true,
	"license": true, "master": true, "root": true,
}

// labelMetadataWords are nouns that, between a credential label and its
// value, make the value metadata about the credential rather than the
// credential itself.
var labelMetadataWords = map[string]bool{
	"fingerprint": true, "thumbprint": true, "checksum": true, "digest": true,
	// Algorithm names (MD5, SHA256) are deliberately absent: "HMAC key
	// (SHA256): <key>" names the key's algorithm, not metadata.
	"hash":       true,
	"identifier": true, "identifiers": true, "id": true, "ids": true, "name": true,
	"names": true, "alias": true, "aliases": true, "owner": true, "owners": true,
	"label": true, "labels": true, "length": true,
	"size": true, "type": true, "algorithm": true, "format": true, "version": true,
	"revision": true, "commit": true, "metadata": true, "reference": true,
	"binding": true, "policy": true, "expiry": true, "expiration": true,
	"serial": true, "etag": true, "ulid": true, "objectid": true, "uuid": true,
	"guid": true,
	// Words that describe a password or key rather than give it ("Password
	// (status):", "Password (last changed):", "Passphrase (error message):").
	"status": true, "changed": true, "last": true, "baseline": true, "procedure": true,
	"runbook": true, "message": true, "schedule": true, "expires": true, "expired": true,
	"standard": true, "requirement": true, "requirements": true, "rule": true, "rules": true,
	"guidance": true, "date": true, "updated": true, "created": true,
}

// settingKeyModifiers name a setting, record or route rather than a
// credential ("Configuration key: EnableQoSForMicrosoftTeams", "Routing key:
// HQ01FW02WAN1"). Unlike nonCredentialKeyModifiers they exempt only a
// name-shaped value: a random value after them is still a key. Function and
// policy keys are absent: Azure Function keys and B2C policy keys are secrets.
var settingKeyModifiers = map[string]bool{
	"business": true, "routing": true, "deduplication": true, "configuration": true,
	"setting": true, "settings": true, "image": true, "shortcut": true,
	"inventory": true, "circuit": true, "route": true, "location": true, "rack": true,
}

// settingKeyNameValue reports whether a value after a settingKeyModifiers label
// reads as a setting or record name: not key-shaped, a public identifier,
// built from words with at most four numbers ("WindowsServer2022Datacenter21H2"),
// a short single-case code, or a code of short segments ("NYC01/IDF02/SW03",
// "nyc1-sw02-eth3-dmz4").
func settingKeyNameValue(value string) bool {
	return !credentialShapedValue(value, false) || isPublicIdentifierValue(value) ||
		nameWithNumbers(value, 4) || len(value) <= 16 && !hasMixedCaseLetters(value) ||
		shortSegmentCode(value)
}

// shortSegmentCode reports a value split by "/", "-", "_" or "." into
// segments of at most eight characters.
func shortSegmentCode(value string) bool {
	segments := strings.FieldsFunc(value, func(r rune) bool { return strings.ContainsRune("/-_.", r) })
	if len(segments) < 2 {
		return false
	}
	for _, segment := range segments {
		if len(segment) > 8 {
			return false
		}
	}
	return true
}

// identifiedObjectNouns name an object whose identifier follows a credential
// label phrase: "Token scope: <uuid>", "Key Vault tenant: <uuid>", "Password
// reset event: <uuid>". They exempt only a public-identifier-shaped value.
var identifiedObjectNouns = map[string]bool{
	"scope": true, "scopes": true, "claim": true, "claims": true, "audience": true,
	"aud": true, "tid": true, "oid": true, "jti": true, "tenant": true, "subscription": true,
	"group": true, "groups": true, "event": true, "events": true, "request": true,
	"transaction": true, "workflow": true, "job": true, "location": true, "storage": true,
	"application": true, "app": true, "device": true, "user": true, "account": true,
	"certificate": true, "session": true, "correlation": true, "trace": true, "vault": true,
}

// phraseNamesIdentifiedObject reports whether a label phrase is a compound
// noun (no function words) ending in an identifiedObjectNouns noun.
func phraseNamesIdentifiedObject(phrase string) bool {
	words := strings.FieldsFunc(phrase, func(r rune) bool { return !isASCIILetter(r) && !unicode.IsDigit(r) })
	for len(words) > 0 && phraseCueWords[strings.ToLower(words[len(words)-1])] {
		words = words[:len(words)-1]
	}
	if len(words) == 0 {
		return false
	}
	for _, word := range words {
		if phraseFunctionWords[strings.ToLower(word)] {
			return false
		}
	}
	return identifiedObjectNouns[strings.ToLower(words[len(words)-1])]
}

// phraseNamesMetadata reports whether the words of a label phrase (the text
// between a label and its phrased value, at most six short words) include a
// labelMetadataWords noun, or, with firstOnly, start with one or form a
// compound noun ending in one: "API key object ID is ..." and "Password
// reset correlation ID = ..." name an identifier, while "API key for the
// name is ..." and "API key for the ID service is ..." still name the key.
func phraseNamesMetadata(phrase string, firstOnly bool) bool {
	words := strings.FieldsFunc(phrase, func(r rune) bool { return !isASCIILetter(r) && !unicode.IsDigit(r) })
	for len(words) > 0 && phraseCueWords[strings.ToLower(words[len(words)-1])] {
		words = words[:len(words)-1]
	}
	if len(words) == 0 {
		return false
	}
	if firstOnly {
		if labelMetadataWords[strings.ToLower(words[0])] {
			return true
		}
		for _, word := range words {
			if phraseFunctionWords[strings.ToLower(word)] {
				return false
			}
		}
		return labelMetadataWords[strings.ToLower(words[len(words)-1])]
	}
	for _, word := range words {
		if labelMetadataWords[strings.ToLower(word)] {
			return true
		}
	}
	return false
}

// phraseCueWords end a label phrase before its value ("ID is ...").
var phraseCueWords = map[string]bool{"is": true, "was": true, "are": true, "were": true, "ist": true, "est": true, "es": true}

// phraseFunctionWords mark a label phrase that is not a compound noun: the
// credential is for, of or with something else ("API key for the name").
var phraseFunctionWords = map[string]bool{
	"for": true, "of": true, "to": true, "on": true, "in": true, "at": true, "with": true,
	"from": true, "by": true, "via": true, "the": true, "a": true, "an": true, "and": true,
	"or": true, "that": true, "this": true, "our": true, "your": true, "my": true,
}

// labelModifierBefore returns the lowercased word directly before a label
// word at i: joined CamelCase ("PartitionKey") or separated by a short run of
// spaces, tabs, "_", "-" and Markdown emphasis or code wrappers ("Primary
// key", "Primary  key", "tag_key", "Primary **key**", "**Primary** key").
func labelModifierBefore(in string, i int) string {
	for n := 0; n < labelModifierGapMax && i > 0; n++ {
		if strings.IndexByte(" \t_-*`", in[i-1]) >= 0 {
			i--
		} else if i > 1 && in[i-2:i] == "\u00a0" {
			i -= 2 // no-break space
		} else {
			break
		}
	}
	end := i
	for i > 0 && isASCIILetter(rune(in[i-1])) {
		i--
	}
	return strings.ToLower(in[i:end])
}

// labelModifierGapMax bounds the separator run labelModifierBefore skips.
const labelModifierGapMax = 16

// jsonPairKeyIdentifierAt reports whether the label word in[start:end] is a
// JSON member name "key" (any case) whose string value is a public identifier
// and whose object also has a "value" member, in either order: the same
// {"key": K, "value": V} pair that parseObject exempts, found in text that
// is not itself a JSON document ("CMDB entry: {...}"). Lookbehind and
// lookahead are bounded, so the check is constant work per label.
func jsonPairKeyIdentifierAt(in string, start, end int) bool {
	// A pair whose quotes are escaped ("{\"key\":\"...\"}" inside a JSON
	// string): judge an unescaped copy of the surrounding window.
	if start >= 2 && in[start-1] == '"' && in[start-2] == '\x5c' {
		lo, hi := pasteBack(start, jsonPairWindow), pasteBound(end, len(in), jsonPairWindow)
		windowStart := len(unescapeJSONQuotes(in[lo:start]))
		return jsonPairKeyIdentifierAt(unescapeJSONQuotes(in[lo:hi]), windowStart, windowStart+end-start)
	}
	if !strings.EqualFold(in[start:end], "key") || start == 0 || in[start-1] != '"' ||
		end >= len(in) || in[end] != '"' {
		return false
	}
	i := skipJSONSpace(in, end+1)
	if i >= len(in) || in[i] != ':' {
		return false
	}
	i = skipJSONSpace(in, i+1)
	if i >= len(in) || in[i] != '"' {
		return false
	}
	valueStart := i + 1
	valueEnd := valueStart
	for valueEnd < len(in) && valueEnd-valueStart < 128 && in[valueEnd] != '"' && in[valueEnd] != '\\' {
		valueEnd++
	}
	if valueEnd >= len(in) || in[valueEnd] != '"' || !isPublicIdentifierValue(in[valueStart:valueEnd]) {
		return false
	}
	if jsonMemberAtDepthZero(in, valueEnd+1, pasteBound(valueEnd+1, len(in), jsonPairWindow), "value") {
		return true
	}
	objectStart := jsonObjectStartBefore(in, start-1)
	return objectStart >= 0 && jsonMemberAtDepthZero(in, objectStart+1, start-1, "value")
}

// unescapeJSONQuotes drops each run of backslashes directly before a double
// quote, turning escaped JSON ("{\\\"key\\\":1}") back into its structure.
func unescapeJSONQuotes(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\x5c' {
			j := i
			for j < len(s) && s[j] == '\x5c' {
				j++
			}
			if j < len(s) && s[j] == '"' {
				i = j - 1
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// jsonPairWindow bounds how far jsonPairKeyIdentifierAt looks for the
// companion member.
const jsonPairWindow = 512

func skipJSONSpace(in string, i int) int {
	for i < len(in) && (in[i] == ' ' || in[i] == '\t' || in[i] == '\r' || in[i] == '\n') {
		i++
	}
	return i
}

// jsonObjectStartBefore returns the index of the "{" that opens the object
// containing position i, scanning back at most jsonPairWindow bytes, or -1.
// Braces inside strings are not distinguished; the window keeps a mistake
// local to one exemption decision.
func jsonObjectStartBefore(in string, i int) int {
	depth := 0
	for j := i - 1; j >= 0 && i-j <= jsonPairWindow; j-- {
		switch in[j] {
		case '}', ']':
			depth++
		case '{', '[':
			if depth == 0 {
				if in[j] == '{' {
					return j
				}
				return -1
			}
			depth--
		}
	}
	return -1
}

// jsonMemberAtDepthZero reports whether in[from:to] contains, at the
// object's own nesting level, a member named name (any case): a string equal
// to name followed by ":". It stops at the object's closing brace.
func jsonMemberAtDepthZero(in string, from, to int, name string) bool {
	depth := 0
	for i := from; i < to; i++ {
		switch in[i] {
		case '{', '[':
			depth++
		case '}', ']':
			if depth == 0 {
				return false
			}
			depth--
		case '"':
			end := jsonStringEnd(in[:to], i)
			if end < 0 {
				return false
			}
			if depth == 0 && strings.EqualFold(in[i+1:end-1], name) {
				if next := skipJSONSpace(in, end); next < len(in) && in[next] == ':' {
					return true
				}
			}
			i = end - 1
		}
	}
	return false
}

func camelCaseLabelStart(in string, i int) bool {
	return i > 0 && in[i] >= 'A' && in[i] <= 'Z' &&
		(in[i-1] >= 'a' && in[i-1] <= 'z' || isDigitByte(in[i-1]))
}

func startsCredentialValue(in string, start int) bool {
	return start < len(in) && (isCredentialValueByte(in[start]) || in[start] == '\x5c')
}

// credentialValuePrefix returns the run of value bytes at start, capped so a
// discovery-time shape check stays linear on long runs.
func credentialValuePrefix(in string, start int) string {
	end := start
	for end < len(in) && end-start < 256 && isCredentialValueByte(in[end]) {
		end++
	}
	return in[start:end]
}

func isCredentialValueByte(ch byte) bool {
	return isASCIIAlnum(ch) || strings.IndexByte("._~+/=-", ch) >= 0
}

func isASCIIAlnum(ch byte) bool {
	return ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9'
}

// credentialShapedValue reports whether a value that follows a generic
// credential label looks like key material rather than prose, a slug or a
// path. After a password label (password, passwd, pwd) a word joined to a
// number is still a credential: that is the shape of a typical weak password
// ("Abc123def456"); after key/token/secret/bearer it is treated as a name
// ("Switch01Port48").
func credentialShapedValue(value string, passwordLabel bool) bool {
	for {
		trimmed := strings.TrimSpace(value)
		trimmed = strings.TrimLeft(trimmed, credentialValueOpeners)
		trimmed = strings.TrimRight(trimmed, credentialValueClosers)
		if trimmed == value {
			break
		}
		value = trimmed
	}
	if referenceIDRE.MatchString(value) {
		return false // ticket and change references: INC0012345, RITM0012345678, OPS-1234
	}
	if cloudResourceIDRE.MatchString(value) || azureResourceIDRE.MatchString(value) {
		return false // public cloud resource IDs: i-0d12e34f56a78b90c, /subscriptions/<uuid>/...
	}
	if canonicalUUIDRE.MatchString(value) {
		return true
	}
	if len(value) >= 32 && isHex(value) {
		return true
	}
	// A preceding credential label is strong evidence, so a single
	// letter-and-digit segment is enough here; no word or entropy check.
	for _, segment := range splitKeySegments(value) {
		if len(segment) >= 12 && hasDigit(segment) && hasLetter(segment) &&
			(passwordLabel || !wordWithNumber(segment)) {
			return true
		}
		if passwordLabel && len(segment) >= 8 && hasDigit(segment) && hasLetter(segment) {
			return true
		}
	}
	// A letters-only key (about 3% of random 20-character base62 keys have no
	// digit) counts when it is long, mixes upper and lower case, does not read
	// as words, and has high entropy. Base64 padding is ignored. Single-case
	// letters-only values are left alone: they are usually tag-like words
	// ("monthlypatchwindows").
	if letters := trimBase64Padding(value); len(letters) >= 16 && isLettersOnly(letters) &&
		hasMixedCaseLetters(letters) && !readsAsWords(letters) &&
		shannonEntropy(letters) >= keyMaterialEntropy {
		return true
	}
	// Standard and URL-safe Base64 key material contains "+", "/", "-" or "_",
	// which split it into short segments that often lack a digit. Judge the
	// whole value (padding ignored) when it is mixed-case, does not read as
	// words, and has high entropy. Mixed case keeps single-case slugs such as
	// "rule-2024-q3-block-gambling" out.
	if b := trimBase64Padding(value); len(b) >= 16 && isBase64Text(b) && hasMixedCaseLetters(b) &&
		!readsAsWords(b) && mostlyNonWordSegments(b) && shannonEntropy(b) >= keyMaterialEntropy {
		return true
	}
	// base64url key material contains "-" and "_", which split it into short
	// segments; judge the whole value when it does not read as words.
	return len(value) >= 16 && looksLikeSplitKeyMaterial(value)
}

// mostlyNonWordSegments reports whether at least 40% of a value's characters
// sit in segments that are neither words nor plain numbers. Split random
// Base64 is mostly such fragments; a path like "Projects/2024/iOS" is mostly
// words and a year (18% non-word).
func mostlyNonWordSegments(value string) bool {
	nonWord := 0
	for _, segment := range splitKeySegments(value) {
		if !isAllDigits(segment) && !readsAsWords(segment) {
			nonWord += len(segment)
		}
	}
	return nonWord*10 >= len(value)*4
}

func isAllDigits(value string) bool {
	for i := 0; i < len(value); i++ {
		if !isDigitByte(value[i]) {
			return false
		}
	}
	return value != ""
}

// referenceIDRE matches ticket, change and issue references: a short
// letter prefix, an optional separator, then only digits.
var referenceIDRE = regexp.MustCompile(`^[A-Za-z]{1,8}[-_]?[0-9]{3,}$`)

func isBase64Text(value string) bool {
	for i := 0; i < len(value); i++ {
		if !isASCIIAlnum(value[i]) && strings.IndexByte("+/_-", value[i]) < 0 {
			return false
		}
	}
	return value != ""
}

// trimBase64Padding removes up to two trailing "=" padding characters.
func trimBase64Padding(value string) string {
	for i := 0; i < 2 && strings.HasSuffix(value, "="); i++ {
		value = value[:len(value)-1]
	}
	return value
}

func hasMixedCaseLetters(value string) bool {
	return strings.IndexFunc(value, func(r rune) bool { return r >= 'a' && r <= 'z' }) >= 0 &&
		strings.IndexFunc(value, func(r rune) bool { return r >= 'A' && r <= 'Z' }) >= 0
}

func isLettersOnly(value string) bool {
	for i := 0; i < len(value); i++ {
		if !isASCIILetter(rune(value[i])) {
			return false
		}
	}
	return value != ""
}

// looksLikeSplitKeyMaterial reports whether a token whose separators break it
// into short segments is still key material: most of it sits in segments that
// mix letters and digits (pure-hex segments, typical generated suffixes, do
// not count), it does not read as words, and it has high entropy. Slugs such
// as "rule-2024-q3-block-gambling" keep letters and digits in separate
// segments and fail the first condition.
func looksLikeSplitKeyMaterial(token string) bool {
	mixed := 0
	for _, segment := range splitKeySegments(token) {
		if hasDigit(segment) && hasLetter(segment) && !isHex(segment) && !wordWithNumber(segment) {
			mixed += len(segment)
		}
	}
	return mixed*2 >= len(token) && !readsAsWords(token) &&
		shannonEntropy(token) >= keyMaterialEntropy
}

// splitKeySegments splits on name, path and base64 separators: path-like values
// such as "Projects/2024/Q3-planning" must not count as one letter-and-digit
// segment, and random base64 stays letter/digit-mixed when split on "/", "+"
// or "=".
func splitKeySegments(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool {
		return strings.ContainsRune("-_./+=:~", r) || unicode.IsSpace(r)
	})
}

func isHex(value string) bool {
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if !(ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'f' || ch >= 'A' && ch <= 'F') {
			return false
		}
	}
	return value != ""
}

func hasDigit(value string) bool {
	return strings.IndexFunc(value, func(r rune) bool { return r >= '0' && r <= '9' }) >= 0
}

func hasLetter(value string) bool {
	return strings.IndexFunc(value, func(r rune) bool { return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' }) >= 0
}

func assignmentRules(name, keys, marker string) []rule {
	key := `["']?(?:` + keys + `)["']?\s*[:=]\s*`
	prefilter := prefilterForAssignmentKeys(keys)
	return []rule{
		// Quotes escaped with backslashes: a JSON document pasted into a
		// JSON string ("password: \"...\"").
		{
			name:        name,
			re:          regexp.MustCompile(`(?i)(` + key + `)(\\+)"(?:[^"\\\n]|\\+[^"\\\n])*\\+"`),
			replacement: `${1}${2}"` + marker + `${2}"`,
			prefilter:   prefilter,
		},
		{
			name:        name,
			re:          regexp.MustCompile(`(?i)(` + key + `)(\\+)'(?:[^'\\\n]|\\+[^'\\\n])*\\+'`),
			replacement: `${1}${2}'` + marker + `${2}'`,
			prefilter:   prefilter,
		},
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
		// Markdown emphasis between the label and a value ("secret:** `KEY`")
		// would otherwise be taken as the whole unquoted value below, leaving
		// the key behind.
		{
			name:        name,
			re:          regexp.MustCompile(`(?i)(` + key + `)[*_]{1,3}\s*\x60?[^<"'\s,}\]\{\[\x60]+\x60?`),
			replacement: `${1}` + marker,
			prefilter:   prefilter,
		},
		{
			name: name,
			// A leading backslash escapes a quote handled above.
			re:          regexp.MustCompile(`(?i)(` + key + `)[^<"'\s,}\]\{\[\\][^<"'\s,}\]\{\[]*`),
			replacement: `${1}` + marker,
			prefilter:   prefilter,
		},
	}
}

var shareRules = []rule{
	{
		name:        "email",
		re:          regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`),
		replacement: markerEmail,
		prefilter:   contains("@"),
	},
	{
		name:        "ipv4",
		re:          regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),
		replacement: markerIP,
		prefilter:   contains("."),
	},
	{
		// Email and IP masking can turn URL userinfo into a credential shape
		// ("alice@example.com:pw@host" -> "<REDACTED:EMAIL>:pw@host"), so the
		// credential URL rule runs again after them; one pass is then already
		// what a rescan would produce.
		name:      "credential_url",
		prefilter: all(contains("://"), contains("@")),
		custom:    scanCredentialURLs,
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

// shouldRedactHighEntropyToken keeps main's decision for tokens of 32 or more
// characters, adding only recognized public identifiers in standard mode. It
// deliberately has no word, path or label exemption: a word-built name of 32
// or more characters cannot be told apart from a word-built password here.
func shouldRedactHighEntropyToken(text string, start, end int, context highEntropyContext, mode Mode) bool {
	token := text[start:end]
	if canonicalUUIDRE.MatchString(token) {
		return false
	}
	if mode == ModeStandard && publicIdentifierToken(text, start, end) {
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

// publicIdentifierToken reports whether a long token is a recognized public
// identifier: a cloud resource ID or ULID, a digest, revision, ObjectId or
// KSUID named by its context, or "Name=<public identifier>" with an
// identifier name ("PartitionKey=<uuid>"). Readable paths and words are
// not exempt on the long-entropy path.
func publicIdentifierToken(text string, start, end int) bool {
	token := text[start:end]
	return isBarePublicIdentifier(token) || isContextualPublicValue(text, start, end) ||
		(!strings.Contains(token, "/") && publicAssignedValue(token))
}

// publicAssignedValue reports whether token is "Name=value" with an
// identifier name and public identifier value, or a non-credential name and
// readable path. An unknown name does not establish that an identifier-shaped
// value is public.
func publicAssignedValue(token string) bool {
	eq := strings.IndexByte(token, '=')
	value, ok := assignedTokenValue(token)
	if !ok {
		return false
	}
	name := strings.ToLower(token[:eq])
	if isReadableWorkingDirectory(value) && (name == "pwd" || name == "cwd") {
		return true
	}
	if !isPublicIdentifierValue(value) && !isReadablePath(value) {
		return false
	}
	for _, label := range genericCredentialLabels {
		if strings.Contains(name, label) {
			modifier := strings.TrimRight(strings.TrimSuffix(name, "key"), "_-")
			return strings.HasSuffix(name, "key") && (nonCredentialKeyModifiers[modifier] || settingKeyModifiers[modifier])
		}
	}
	return isReadablePath(value) || publicIdentifierAssignmentName(token[:eq])
}

// publicIdentifierAssignmentName requires an identifier word at the end of
// the name, alone or after a separator or CamelCase boundary ("LocationId").
// A coincidental suffix inside a word ("liquid") is not identifier context.
func publicIdentifierAssignmentName(name string) bool {
	lower := strings.ToLower(name)
	for _, suffix := range []string{
		"id", "identifier", "uuid", "guid", "ref", "reference", "ulid", "ksuid",
		"fingerprint", "thumbprint", "checksum", "digest", "commit", "revision", "rev",
	} {
		if !strings.HasSuffix(lower, suffix) {
			continue
		}
		start := len(name) - len(suffix)
		if start == 0 || name[start-1] == '_' || name[start-1] == '-' || camelCaseLabelStart(name, start) {
			return true
		}
	}
	return false
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
