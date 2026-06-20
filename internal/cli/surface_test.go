package cli

import (
	"io"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

// TestCompletionFlagsMatchGlobalFlagDefs verifies that completionFlags is fully
// in sync with globalFlagDefs. Since completionFlags is derived directly from
// globalFlagDefs in surface.go, this test acts as a sanity check: if the
// derivation formula ever changes, this catches a name-set mismatch.
func TestCompletionFlagsMatchGlobalFlagDefs(t *testing.T) {
	t.Parallel()

	want := make([]string, 0, len(globalFlagDefs))
	for _, d := range globalFlagDefs {
		want = append(want, "--"+d.name)
	}
	sort.Strings(want)

	got := make([]string, len(completionFlags))
	copy(got, completionFlags)
	sort.Strings(got)

	if len(got) != len(want) {
		t.Fatalf("completionFlags has %d entries, globalFlagDefs yields %d; got=%v want=%v",
			len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("completionFlags entry mismatch at index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

// TestCompletionDumpFlagsMatchCobraDumpCommand cross-checks completionDumpFlags
// against the live Cobra dump command's local flags (minus --help). If a flag is
// added to or removed from newDumpCmd without updating completionDumpFlags in
// surface.go, this test fails.
func TestCompletionDumpFlagsMatchCobraDumpCommand(t *testing.T) {
	t.Parallel()

	a := New(io.Discard, io.Discard, nil)
	cobraNames := cobraLocalFlagNames(a.newDumpCmd(globalOptions{}).Flags())

	want := sortedWithPrefix(cobraNames)
	got := sortedCopy(completionDumpFlags)

	assertFlagListsMatch(t, "completionDumpFlags", "newDumpCmd local flags", want, got)
}

// TestCompletionDiffFlagsMatchCobraDiffCommand cross-checks completionDiffFlags
// against the live Cobra diff command's local flags (minus --help). If a flag is
// added to or removed from newDiffCmd without updating completionDiffFlags in
// surface.go, this test fails.
func TestCompletionDiffFlagsMatchCobraDiffCommand(t *testing.T) {
	t.Parallel()

	a := New(io.Discard, io.Discard, nil)
	cobraNames := cobraLocalFlagNames(a.newDiffCmd(globalOptions{}).Flags())

	want := sortedWithPrefix(cobraNames)
	got := sortedCopy(completionDiffFlags)

	assertFlagListsMatch(t, "completionDiffFlags", "newDiffCmd local flags", want, got)
}

// cobraLocalFlagNames returns the sorted flag names from fs, excluding --help
// (which Cobra adds implicitly to every command).
func cobraLocalFlagNames(fs *pflag.FlagSet) []string {
	var names []string
	fs.VisitAll(func(f *pflag.Flag) {
		if f.Name == "help" {
			return
		}
		names = append(names, f.Name)
	})
	sort.Strings(names)
	return names
}

// sortedWithPrefix returns sorted "--name" tokens from a list of bare flag names.
func sortedWithPrefix(names []string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = "--" + n
	}
	sort.Strings(out)
	return out
}

// sortedCopy returns a sorted copy of tokens (already "--name" prefixed).
func sortedCopy(tokens []string) []string {
	out := make([]string, len(tokens))
	copy(out, tokens)
	sort.Strings(out)
	return out
}

// assertFlagListsMatch fails the test if got and want differ in length or content.
func assertFlagListsMatch(t *testing.T, gotLabel, wantLabel string, want, got []string) {
	t.Helper()

	// Find entries only in got (stale in the explicit list).
	wantSet := make(map[string]bool, len(want))
	for _, w := range want {
		wantSet[w] = true
	}
	gotSet := make(map[string]bool, len(got))
	for _, g := range got {
		gotSet[g] = true
	}

	var missing, extra []string
	for _, w := range want {
		if !gotSet[w] {
			missing = append(missing, w)
		}
	}
	for _, g := range got {
		if !wantSet[g] {
			extra = append(extra, g)
		}
	}

	if len(missing) > 0 {
		t.Errorf("%s is missing flags that %s declares: %s",
			gotLabel, wantLabel, strings.Join(missing, ", "))
	}
	if len(extra) > 0 {
		t.Errorf("%s has stale flags not in %s: %s",
			gotLabel, wantLabel, strings.Join(extra, ", "))
	}
}
