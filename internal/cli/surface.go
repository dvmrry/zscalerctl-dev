package cli

// surface.go — durable completion-surface inventory.
//
// This file is the single source of truth for the flag-name slices used by
// shell completion scripts, the man-page drift gate, and the agent-docs drift
// gate.  Each inventory is made drift-proof in one of two ways:
//
//   - completionFlags: DERIVED at init time from globalFlagDefs, so it can
//     never drift from the canonical global-flag list. Adding or removing an
//     entry in globalFlagDefs automatically updates completionFlags with no
//     further edits required here.
//
//   - completionDumpFlags / completionDiffFlags: EXPLICIT lists that mirror the
//     Cobra local flags declared in newDumpCmd / newDiffCmd. A drift test in
//     surface_test.go cross-checks each list against the corresponding Cobra
//     command's LocalFlags() and fails the build if they diverge.

// completionFlags is the list of global flag tokens (with "--" prefix) exposed
// to shell completion, the man-page drift gate, and the agent-docs drift gate.
// It is derived from globalFlagDefs so it always matches the canonical 13 globals.
var completionFlags = func() []string {
	flags := make([]string, 0, len(globalFlagDefs))
	for _, d := range globalFlagDefs {
		flags = append(flags, "--"+d.name)
	}
	return flags
}()

// completionDumpFlags is the list of dump-command local flag tokens exposed to
// shell completion and the man-page drift gate. It mirrors the Cobra local flags
// declared in newDumpCmd. A drift test (TestCompletionDumpFlagsMatchCobraDumpCommand)
// cross-checks this list against the live Cobra command to prevent silent desync.
var completionDumpFlags = []string{
	"--continue-on-error",
	"--force",
	"--out",
	"--products",
	"--resources",
}

// completionDiffFlags is the list of diff-command local flag tokens exposed to
// shell completion and the man-page drift gate. It mirrors the Cobra local flags
// declared in newDiffCmd. A drift test (TestCompletionDiffFlagsMatchCobraDiffCommand)
// cross-checks this list against the live Cobra command to prevent silent desync.
var completionDiffFlags = []string{
	"--allow-partial",
	"--detail",
	"--fail-on-drift",
	"--ignore-operational",
	"--products",
	"--resources",
}
