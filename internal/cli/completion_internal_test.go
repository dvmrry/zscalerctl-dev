package cli

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestCompletionScriptsExposeSameGeneratedSurface(t *testing.T) {
	t.Parallel()

	wantTokens := completionSurfaceTokensForTest()
	for _, shell := range completionShells {
		shell := shell
		t.Run(shell, func(t *testing.T) {
			t.Parallel()

			script, err := completionScript(shell)
			if err != nil {
				t.Fatalf("completionScript(%q) error = %v, want nil", shell, err)
			}

			var missing []string
			for _, token := range wantTokens {
				if !completionScriptContainsToken(script, token) {
					missing = append(missing, token)
				}
			}
			if len(missing) > 0 {
				t.Errorf("completionScript(%q) missing %d source-of-truth token(s): %s", shell, len(missing), strings.Join(missing, ", "))
			}
		})
	}
}

func completionSurfaceTokensForTest() []string {
	seen := map[string]struct{}{}
	add := func(values ...string) {
		for _, value := range values {
			if strings.TrimSpace(value) == "" {
				continue
			}
			seen[value] = struct{}{}
		}
	}

	add(completionFlags...)
	add(completionDumpFlags...)
	add(completionFormats...)
	add(completionRedaction...)
	add(completionColors...)
	add(completionShells...)
	add(completionProductValues()...)
	add(completionCommandNames()...)
	add(operationNames()...)
	add(allResourceNames()...)
	add(dumpResourceNames()...)

	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func completionScriptContainsToken(script, token string) bool {
	if tokenBoundaryRE(token).MatchString(script) {
		return true
	}
	if strings.HasPrefix(token, "--") {
		// fish spells long flags as "-l name" instead of "--name".
		name := regexp.QuoteMeta(strings.TrimPrefix(token, "--"))
		return regexp.MustCompile(`(^|\s)-l\s+` + name + `(\s|$)`).MatchString(script)
	}
	return false
}

func tokenBoundaryRE(token string) *regexp.Regexp {
	// Treat resource-name characters as part of tokens so "locations" does not
	// pass only because "zia/locations" is present.
	boundary := `A-Za-z0-9_/\-,`
	return regexp.MustCompile(`(^|[^` + boundary + `])` + regexp.QuoteMeta(token) + `($|[^` + boundary + `])`)
}
