package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// agentDocExemptFlags lists global flags deliberately NOT surfaced in the agent
// docs (AGENTS.md + the skill). Each is human-, cosmetic-, or diagnostic-only and
// irrelevant to an agent driving the tool with --format json, and carries the
// reason it is exempt. Adding a flag to completionFlags then forces a choice —
// document it for agents, or exempt it here with a reason — so the agent surface
// can never silently drift behind the flag set. This is the agent-facing twin of
// TestManPageDocumentsFlagsAndCommands.
var agentDocExemptFlags = map[string]string{
	"--profile":  "cosmetic run label echoed in doctor/config show; selects no behavior (config is env-only)",
	"--output":   "writes output to a file instead of stdout; agents read stdout, no effect on content",
	"--color":    "human TTY styling; agents use --format json",
	"--no-color": "human TTY styling; agents use --format json",
	"--no-cache": "live reads already bypass the SDK cache regardless; config/status surface, not part of the agent data path",
}

// flagDocumented reports whether docs reference flag as a whole token — the flag
// not immediately followed by another flag-name character — so "--output" is not
// satisfied by "--output-file" and "--search" is not satisfied by "--searchable".
func flagDocumented(docs, flag string) bool {
	return regexp.MustCompile(regexp.QuoteMeta(flag) + `([^A-Za-z0-9-]|$)`).MatchString(docs)
}

// TestAgentDocsDocumentEveryFlag guards AGENTS.md + skills/zscalerctl/SKILL.md
// against drift: every global flag is either referenced (as a whole token) in the
// agent docs or explicitly exempt with a reason — never both, and no exemption may
// name a flag that no longer exists. It is a subset check (extra prose is fine) —
// the agent-facing analogue of the man-page gate, so a new flag cannot ship
// without a deliberate decision about its agent guidance.
func TestAgentDocsDocumentEveryFlag(t *testing.T) {
	t.Parallel()

	read := func(parts ...string) string {
		rel := filepath.Join(append([]string{"..", ".."}, parts...)...)
		body, err := os.ReadFile(rel)
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		return string(body)
	}
	docs := read("AGENTS.md") + "\n" + read("skills", "zscalerctl", "SKILL.md")

	for _, flag := range completionFlags {
		documented := flagDocumented(docs, flag)
		reason, exempt := agentDocExemptFlags[flag]
		switch {
		case documented && exempt:
			t.Errorf("global flag %q is BOTH documented in the agent docs and listed in agentDocExemptFlags — remove the now-stale exemption", flag)
		case documented:
			// Documented for agents — good.
		case exempt:
			if strings.TrimSpace(reason) == "" {
				t.Errorf("global flag %q is exempt from the agent docs but has no reason", flag)
			}
		default:
			t.Errorf("global flag %q is not documented in AGENTS.md or the skill and is not in agentDocExemptFlags — document it for agents, or exempt it there with a reason", flag)
		}
	}

	// A stale exemption is drift too: an exempted flag that no longer exists must
	// be pruned so the list keeps meaning something.
	current := make(map[string]bool, len(completionFlags))
	for _, f := range completionFlags {
		current[f] = true
	}
	for f := range agentDocExemptFlags {
		if !current[f] {
			t.Errorf("agentDocExemptFlags lists %q, which is not a current global flag — remove the stale exemption", f)
		}
	}
}
