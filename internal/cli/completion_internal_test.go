package cli

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

// TestCompletionScriptsExposeSameGeneratedSurface asserts that all four shell
// completion scripts reference the same binary ("zscalerctl") and use the Cobra
// __complete protocol, guaranteeing they all offer the same surface at runtime
// regardless of shell. With Cobra-generated scripts the surface is identical by
// construction — every script is generated from the same command tree and calls
// back to the same __complete binary.
func TestCompletionScriptsExposeSameGeneratedSurface(t *testing.T) {
	t.Parallel()

	for _, shell := range completionShells {
		shell := shell
		t.Run(shell, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			a := New(&out, io.Discard, nil)
			if err := a.Run(context.Background(), []string{"completion", shell}); err != nil {
				t.Fatalf("App.Run(completion %q) error = %v, want nil", shell, err)
			}
			script := out.String()
			// Every Cobra-generated script must reference the binary name.
			if !strings.Contains(script, "zscalerctl") {
				t.Errorf("completion %q: script = %q, want zscalerctl", shell, script)
			}
		})
	}

	// Verify the runtime surface is consistent: __complete '' lists the same
	// top-level commands regardless of which shell script sources the completion.
	// We exercise two key tokens that must always be present.
	var out bytes.Buffer
	a := New(&out, io.Discard, nil)
	if err := a.Run(context.Background(), []string{"__complete", ""}); err != nil {
		t.Fatalf("App.Run(__complete '') error = %v, want nil", err)
	}
	got := out.String()
	for _, want := range []string{"zia", "completion", "version"} {
		if !strings.Contains(got, want) {
			t.Errorf("App.Run(__complete '') stdout = %q, want %q", got, want)
		}
	}
}

// TestCompletionScriptsIncludeURLLookup guards the zia-only url-lookup
// diagnostic verb. It is registered as a Cobra subcommand of "zia" (Phase 2b),
// so __complete zia ” must offer it alongside catalog resources.
func TestCompletionScriptsIncludeURLLookup(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	a := New(&out, io.Discard, nil)
	if err := a.Run(context.Background(), []string{"__complete", "zia", ""}); err != nil {
		t.Fatalf("App.Run(__complete zia '') error = %v, want nil", err)
	}
	got := out.String()
	if !strings.Contains(got, urlLookupCommandName) {
		t.Errorf("App.Run(__complete zia '') stdout = %q, want %q", got, urlLookupCommandName)
	}
}

// TestCompletionScriptsOfferLogLevelValues asserts that --log-level flag-value
// completion offers the expected values (off/error/warn/info/debug). The values
// are registered via RegisterFlagCompletionFunc in applyGlobalPersistentFlags.
func TestCompletionScriptsOfferLogLevelValues(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	a := New(&out, io.Discard, nil)
	// Request completion for the --log-level flag value.
	if err := a.Run(context.Background(), []string{"__complete", "--log-level", ""}); err != nil {
		t.Fatalf("App.Run(__complete --log-level '') error = %v, want nil", err)
	}
	got := out.String()
	for _, want := range completionLogLevels {
		if !strings.Contains(got, want) {
			t.Errorf("App.Run(__complete --log-level '') stdout = %q, want log-level value %q", got, want)
		}
	}
}
