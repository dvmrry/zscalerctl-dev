package redact

import (
	"regexp"
	"regexp/syntax"
	"testing"
)

// The whole-input fast path (structuredScanMayMatch) returns the input
// unscanned unless some rule prefilter admits it. The structured scanner also
// redacts by JSON key or label context, so every key those classifiers accept
// must contain a needle some prefilter admits; today that holds only because
// the needle lists overlap. This expands each classifier's alternatives into
// example keys and requires the fast path to admit a document using them, so a
// new key family without a needle fails here instead of leaking.
func TestFastPathAdmitsEveryJSONKeyClassifier(t *testing.T) {
	t.Parallel()
	keyed := map[string]*regexp.Regexp{
		"jsonProvisioningAssignmentKeyRE": jsonProvisioningAssignmentKeyRE,
		"jsonPrivateKeyAssignmentKeyRE":   jsonPrivateKeyAssignmentKeyRE,
		"jsonSecretAssignmentKeyRE":       jsonSecretAssignmentKeyRE,
		"genericCredentialJSONKeyRE":      genericCredentialJSONKeyRE,
		"jsonCredentialContextKeyRE":      jsonCredentialContextKeyRE,
		"jsonPrimaryKeyRE":                jsonPrimaryKeyRE,
		"jsonCredentialArrayKeyRE":        jsonCredentialArrayKeyRE,
	}
	for _, mode := range []Mode{ModeStandard, ModeShare, ModeParanoid} {
		redactor := New(mode)
		for name, re := range keyed {
			for _, key := range regexExamples(t, re) {
				if !re.MatchString(key) {
					t.Fatalf("%s: generated example %q does not match its own regex", name, key)
				}
				if doc := `{"` + key + `":"x"}`; !redactor.structuredScanMayMatch(doc) {
					t.Errorf("%s (%s): fast path skips %s", name, mode, doc)
				}
			}
		}
		for _, label := range regexExamples(t, pasteCredentialNameRE) {
			if doc := `{"name":"` + label + `","value":"x"}`; !redactor.structuredScanMayMatch(doc) {
				t.Errorf("pasteCredentialNameRE (%s): fast path skips %s", mode, doc)
			}
		}
	}
}

// regexExamples returns one matching string per alternative of re: optional
// and repeated parts take their shortest form, a class its first character.
func regexExamples(t *testing.T, re *regexp.Regexp) []string {
	t.Helper()
	parsed, err := syntax.Parse(re.String(), syntax.Perl)
	if err != nil {
		t.Fatalf("syntax.Parse(%s) error = %v", re, err)
	}
	var expand func(*syntax.Regexp) []string
	expand = func(node *syntax.Regexp) []string {
		switch node.Op {
		case syntax.OpLiteral:
			return []string{string(node.Rune)}
		case syntax.OpCharClass:
			return []string{string(node.Rune[0])}
		case syntax.OpAnyChar, syntax.OpAnyCharNotNL:
			return []string{"a"}
		case syntax.OpCapture:
			return expand(node.Sub[0])
		case syntax.OpQuest, syntax.OpStar:
			return []string{""}
		case syntax.OpPlus:
			return expand(node.Sub[0])
		case syntax.OpRepeat:
			out := []string{""}
			for range node.Min {
				out = concat(out, expand(node.Sub[0]))
			}
			return out
		case syntax.OpConcat:
			out := []string{""}
			for _, sub := range node.Sub {
				out = concat(out, expand(sub))
			}
			return out
		case syntax.OpAlternate:
			var out []string
			for _, sub := range node.Sub {
				out = append(out, expand(sub)...)
			}
			return out
		case syntax.OpBeginLine, syntax.OpEndLine, syntax.OpBeginText, syntax.OpEndText,
			syntax.OpWordBoundary, syntax.OpNoWordBoundary, syntax.OpEmptyMatch:
			return []string{""}
		}
		t.Fatalf("regexExamples(%s): unsupported op %v", re, node.Op)
		return nil
	}
	examples := expand(parsed)
	if len(examples) == 0 {
		t.Fatalf("regexExamples(%s): no examples", re)
	}
	return examples
}

func concat(prefixes, suffixes []string) []string {
	out := make([]string, 0, len(prefixes)*len(suffixes))
	for _, prefix := range prefixes {
		for _, suffix := range suffixes {
			out = append(out, prefix+suffix)
		}
	}
	return out
}
