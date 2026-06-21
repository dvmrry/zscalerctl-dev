package cli

// globalflags_test.go — drift guard between parseGlobal and the Cobra mirror.
//
// TestCobraGlobalsMirrorParseGlobal asserts that:
//  1. The set of flag names registered by defineGlobalFlags (the source of
//     truth that parseGlobal calls) equals the set registered by
//     registerGlobalPersistentFlags (the Cobra mirror).
//  2. Per-flag type and default value agree across both sides.
//
// How the cross-check works (not a tautology):
//   - The stdlib side is populated by calling defineGlobalFlags, which is the
//     SAME function parseGlobal calls — so any flag present in parseGlobal is
//     automatically present here. Since parseGlobal no longer has inline
//     flag registrations, flags can only be added to it by adding them to
//     globalFlagDefs; that same change is automatically reflected in both
//     this test's stdlib enumeration AND in registerGlobalPersistentFlags.
//   - The pflag side is populated by calling registerGlobalPersistentFlags,
//     which also derives from globalFlagDefs.
//   - If a name appears on one side but not the other, the test fails.
//   - If the type or default drifts (e.g. someone changes a default in one
//     place), the test reports the mismatch per flag.

import (
	"flag"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/spf13/pflag"
)

// flagMeta summarises a registered flag for equality comparison.
type flagMeta struct {
	name       string
	kind       string // "string" | "bool" | "duration" | "stringArray"
	defaultVal string // canonical default as a string
}

// stdlibFlagKind maps a stdlib flag.Value to a kind string matching globalFlagDef.kind.
func stdlibFlagKind(f *flag.Flag) string {
	// repeatableFlag (our custom Var) implements flag.Value but not flag.Getter.
	if _, ok := f.Value.(*repeatableFlag); ok {
		return "stringArray"
	}
	// All stdlib flag types implement flag.Getter.
	g, ok := f.Value.(flag.Getter)
	if !ok {
		return fmt.Sprintf("unknown(%T)", f.Value)
	}
	switch g.Get().(type) {
	case string:
		return "string"
	case bool:
		return "bool"
	case time.Duration:
		return "duration"
	default:
		return fmt.Sprintf("unknown(%T)", g.Get())
	}
}

// pflagKind maps a pflag.Flag type name to a kind string matching globalFlagDef.kind.
func pflagKind(f *pflag.Flag) string {
	switch f.Value.Type() {
	case "string":
		return "string"
	case "bool":
		return "bool"
	case "duration":
		return "duration"
	case "stringArray":
		return "stringArray"
	default:
		return "unknown(" + f.Value.Type() + ")"
	}
}

func TestCobraGlobalsMirrorParseGlobal(t *testing.T) {
	t.Parallel()

	// --- Canonical (stdlib) side ---
	// defineGlobalFlags is the same function parseGlobal calls, so this
	// enumeration is the ground truth for what parseGlobal actually registers.
	stdFS := flag.NewFlagSet("test-canonical", flag.ContinueOnError)
	var dummyFilter repeatableFlag // sentinel; we only need the name/type
	defineGlobalFlags(stdFS, &dummyFilter)

	stdMeta := map[string]flagMeta{}
	stdFS.VisitAll(func(f *flag.Flag) {
		stdMeta[f.Name] = flagMeta{
			name:       f.Name,
			kind:       stdlibFlagKind(f),
			defaultVal: f.DefValue,
		}
	})

	// --- Mirror (pflag / Cobra) side ---
	pFS := pflag.NewFlagSet("test-mirror", pflag.ContinueOnError)
	registerGlobalPersistentFlags(pFS)

	pfMeta := map[string]flagMeta{}
	pFS.VisitAll(func(f *pflag.Flag) {
		pfMeta[f.Name] = flagMeta{
			name:       f.Name,
			kind:       pflagKind(f),
			defaultVal: f.DefValue,
		}
	})

	// --- Name parity ---
	var allNames []string
	seen := map[string]bool{}
	for n := range stdMeta {
		if !seen[n] {
			allNames = append(allNames, n)
			seen[n] = true
		}
	}
	for n := range pfMeta {
		if !seen[n] {
			allNames = append(allNames, n)
			seen[n] = true
		}
	}
	sort.Strings(allNames)

	for _, name := range allNames {
		s, inStd := stdMeta[name]
		p, inPf := pfMeta[name]

		if !inStd {
			t.Errorf("flag %q: present in Cobra mirror but missing from parseGlobal (stdlib side)", name)
			continue
		}
		if !inPf {
			t.Errorf("flag %q: present in parseGlobal (stdlib side) but missing from Cobra mirror", name)
			continue
		}

		// --- Type parity ---
		if s.kind != p.kind {
			t.Errorf("flag %q: type mismatch: stdlib=%q pflag=%q", name, s.kind, p.kind)
		}

		// --- Default parity ---
		// Duration: stdlib formats as "30s", pflag formats as "30s" — match.
		// Bool: stdlib "false", pflag "false" — match.
		// String: empty string "" on both sides.
		// stringArray: stdlib repeatableFlag.String() returns "" for empty (comma-join
		// of zero elements), pflag StringArray returns "[]". Both represent an empty
		// default, so we normalise both to "" for the comparison.
		sd, pd := s.defaultVal, p.defaultVal
		if s.kind == "stringArray" {
			if sd == "[]" {
				sd = ""
			}
			if pd == "[]" {
				pd = ""
			}
		}
		if sd != pd {
			t.Errorf("flag %q: default mismatch: stdlib=%q pflag=%q", name, s.defaultVal, p.defaultVal)
		}
	}

	// Verify the total count matches globalFlagDefs so a flag added only on one
	// side (stdlib OR pflag) fails immediately.
	wantCount := len(globalFlagDefs)
	if got := len(allNames); got != wantCount {
		t.Errorf("total global flag count: got %d, want %d; flags seen: %v", got, wantCount, allNames)
	}
}

// TestSplitGlobalArgsBoolFlag guards the isGlobalBoolFlag derivation: a known
// global bool flag must be extracted as a global arg (consuming no next token),
// while an unrecognised flag must pass through to rest.
func TestSplitGlobalArgsBoolFlag(t *testing.T) {
	t.Parallel()

	t.Run("global bool flag is extracted", func(t *testing.T) {
		t.Parallel()
		// --no-cache is a bool flag: it needs no following value.
		globalArgs, rest, help, err := splitGlobalArgs([]string{"--no-cache", "version"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if help {
			t.Error("help should be false")
		}
		if len(globalArgs) != 1 || globalArgs[0] != "--no-cache" {
			t.Errorf("globalArgs: got %v, want [--no-cache]", globalArgs)
		}
		if len(rest) != 1 || rest[0] != "version" {
			t.Errorf("rest: got %v, want [version]", rest)
		}
	})

	t.Run("unknown flag passes through to rest", func(t *testing.T) {
		t.Parallel()
		// --nope is not a global flag; it should be left for the subcommand.
		globalArgs, rest, help, err := splitGlobalArgs([]string{"--nope", "version"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if help {
			t.Error("help should be false")
		}
		if len(globalArgs) != 0 {
			t.Errorf("globalArgs: got %v, want []", globalArgs)
		}
		if len(rest) != 2 || rest[0] != "--nope" || rest[1] != "version" {
			t.Errorf("rest: got %v, want [--nope version]", rest)
		}
	})
}
