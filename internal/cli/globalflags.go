package cli

// globalflags.go — single source-of-truth for the 13 global flags.
//
// Architecture:
//   - globalFlagDefs is the canonical list of all 13 global flags (name, kind,
//     default, usage). It is the ONLY place where a global flag is defined; every
//     other registration derives from it.
//   - defineGlobalFlags registers all 13 on a stdlib *flag.FlagSet (using
//     repeatableFlag for filter) and returns typed pointers so parseGlobal can
//     read parsed values without duplicating defaults or usage strings.
//   - registerGlobalPersistentFlags mirrors all 13 onto a pflag.FlagSet for
//     Cobra (persistent, root-level) so --help, shell completion, and tree
//     introspection show the correct flags. Cobra NEVER parses these; App.Run
//     strips globals via splitGlobalArgs before any Cobra dispatch.
//   - The drift test (globalflags_test.go) calls defineGlobalFlags on a fresh
//     flag.FlagSet and registerGlobalPersistentFlags on a fresh pflag.FlagSet,
//     then asserts name, kind, and default match across both sides. Adding a flag
//     to globalFlagDefs automatically makes parseGlobal AND the drift test see it;
//     a flag added only to globalFlagDefs (but not globalFlagPointers / returned
//     by defineGlobalFlags) will fail to compile, making silent drift impossible.

import (
	"flag"
	"time"

	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// globalFlagDef holds every definition field needed to register a global flag
// on both stdlib flag and pflag. Adding or removing an entry here propagates
// to registerGlobalPersistentFlags and the drift check simultaneously.
//
// kind values: "string" | "bool" | "duration" | "stringArray"
// defaultVal: canonical string representation (pflag stores defaults as strings).
type globalFlagDef struct {
	name       string
	kind       string
	defaultVal string // as registered (pflag stores defaults as strings)
	usage      string
}

// globalFlagDefs is the canonical definition of all 13 global flags.
// Order is alphabetical to make drift diffs easy to read.
var globalFlagDefs = []globalFlagDef{
	{
		name:       "color",
		kind:       "string",
		defaultVal: string(output.ColorAuto),
		usage:      "color output: auto, always, never",
	},
	{
		name:       "config",
		kind:       "string",
		defaultVal: "",
		usage:      "config file path",
	},
	{
		name:       "fields",
		kind:       "string",
		defaultVal: "",
		usage:      "comma-separated output fields to keep (narrows the sanitized output)",
	},
	{
		// filter is repeatable: stringArray in pflag, repeatableFlag Var in stdlib.
		// defineGlobalFlags handles it specially via the filterVar parameter.
		// defaultVal is "" because repeatableFlag.String() returns "" for empty
		// (comma-join of zero elements); pflag StringArray returns "[]" but the
		// drift test normalises both to "" for comparison.
		name:       "filter",
		kind:       "stringArray",
		defaultVal: "",
		usage:      "narrow list results: key=value (exact) or key~value (substring); repeatable, all must match",
	},
	{
		name:       "format",
		kind:       "string",
		defaultVal: string(output.FormatAuto),
		usage:      "output format: auto, table, json, ndjson, pretty",
	},
	{
		name:       "log-level",
		kind:       "string",
		defaultVal: "off",
		usage:      "diagnostic logging to stderr: off, error, warn, info, debug",
	},
	{
		name:       "no-cache",
		kind:       "bool",
		defaultVal: "false",
		usage:      "bypass API cache where supported",
	},
	{
		name:       "no-color",
		kind:       "bool",
		defaultVal: "false",
		usage:      "disable color output",
	},
	{
		name:       "output",
		kind:       "string",
		defaultVal: "",
		usage:      "output path",
	},
	{
		name:       "profile",
		kind:       "string",
		defaultVal: "",
		usage:      "profile name",
	},
	{
		name:       "redaction",
		kind:       "string",
		defaultVal: "",
		usage:      "redaction mode: standard, share, paranoid",
	},
	{
		name:       "search",
		kind:       "string",
		defaultVal: "",
		usage:      "narrow list results to records whose rendered values contain term (case-insensitive)",
	},
	{
		name:       "timeout",
		kind:       "duration",
		defaultVal: "30s",
		usage:      "request timeout",
	},
}

// globalFlagNameSet and globalBoolFlagNameSet are derived once from globalFlagDefs
// so splitGlobalArgs (isGlobalFlag / isGlobalBoolFlag) never has its own hardcoded
// list. Any flag added to globalFlagDefs is automatically recognised by the parser.
var (
	globalFlagNameSet     map[string]bool
	globalBoolFlagNameSet map[string]bool
)

func init() {
	globalFlagNameSet = make(map[string]bool, len(globalFlagDefs))
	globalBoolFlagNameSet = make(map[string]bool)
	for _, d := range globalFlagDefs {
		globalFlagNameSet[d.name] = true
		if d.kind == "bool" {
			globalBoolFlagNameSet[d.name] = true
		}
	}
}

// globalFlagPointers holds the typed pointers returned by defineGlobalFlags.
// parseGlobal reads values through these pointers after fs.Parse.
type globalFlagPointers struct {
	profile     *string
	configPath  *string
	format      *string
	outputPath  *string
	timeout     *time.Duration
	redaction   *string
	noCache     *bool
	colorFlag   *string
	noColor     *bool
	logLevel    *string
	fieldsFlag  *string
	filterFlags *repeatableFlag // points at the Var registered on fs
	searchFlag  *string
}

// defineGlobalFlags registers all 13 global flags on a stdlib flag.FlagSet
// derived from globalFlagDefs and returns typed pointers for use by parseGlobal.
// The repeatableFlag for --filter is passed in by the caller (parseGlobal creates
// it; the drift test passes a zero-value sentinel).
//
// This function is the canonical registration point for parseGlobal. Any change
// to global flags must go through globalFlagDefs, which automatically propagates
// to both this function and registerGlobalPersistentFlags.
func defineGlobalFlags(fs *flag.FlagSet, filterVar *repeatableFlag) globalFlagPointers {
	var p globalFlagPointers
	for _, d := range globalFlagDefs {
		switch d.kind {
		case "string":
			ptr := fs.String(d.name, d.defaultVal, d.usage)
			switch d.name {
			case "profile":
				p.profile = ptr
			case "config":
				p.configPath = ptr
			case "format":
				p.format = ptr
			case "output":
				p.outputPath = ptr
			case "redaction":
				p.redaction = ptr
			case "color":
				p.colorFlag = ptr
			case "log-level":
				p.logLevel = ptr
			case "fields":
				p.fieldsFlag = ptr
			case "search":
				p.searchFlag = ptr
			}
		case "bool":
			ptr := fs.Bool(d.name, d.defaultVal == "true", d.usage)
			switch d.name {
			case "no-cache":
				p.noCache = ptr
			case "no-color":
				p.noColor = ptr
			}
		case "duration":
			ptr := fs.Duration(d.name, parseDurationDefault(d), d.usage)
			if d.name == "timeout" {
				p.timeout = ptr
			}
		case "stringArray":
			// filter uses a repeatableFlag Var (not a stdlib StringSlice) to
			// collect every --filter occurrence without comma-splitting.
			fs.Var(filterVar, d.name, d.usage)
			p.filterFlags = filterVar
		default:
			panic("globalFlagDef: unknown kind " + d.kind + " for flag --" + d.name)
		}
	}
	return p
}

// applyGlobalPersistentFlags registers all 13 global flags as persistent flags
// on cmd and wires flag-value completion so Cobra's __complete protocol offers
// the correct enum choices (e.g. --log-level <TAB> → off|error|warn|info|debug).
// Completion functions are registered AFTER flag registration so the flag already
// exists in the pflag set (RegisterFlagCompletionFunc looks up by name).
func applyGlobalPersistentFlags(cmd *cobra.Command) {
	registerGlobalPersistentFlags(cmd.PersistentFlags())
	registerGlobalFlagCompletions(cmd)
}

// registerGlobalFlagCompletions wires Cobra's flag-value completion for the
// enum-valued global flags. The completion values exactly mirror the hand-written
// completion scripts so __complete and the generated scripts agree.
func registerGlobalFlagCompletions(cmd *cobra.Command) {
	enumFlags := []struct {
		name   string
		values []string
	}{
		{"format", completionFormats},
		{"redaction", completionRedaction},
		{"color", completionColors},
		{"log-level", completionLogLevels},
	}
	for _, f := range enumFlags {
		values := append([]string(nil), f.values...)
		_ = cmd.RegisterFlagCompletionFunc(f.name, func(_ *cobra.Command, _ []string, _ string) ([]cobra.Completion, cobra.ShellCompDirective) {
			completions := make([]cobra.Completion, len(values))
			for i, v := range values {
				completions[i] = cobra.Completion(v)
			}
			return completions, cobra.ShellCompDirectiveNoFileComp
		})
	}
}

// registerGlobalPersistentFlags registers mirror pflag persistent flags on fs.
// These are display-only: Cobra never parses them (splitGlobalArgs strips globals
// before Execute). They exist so --help and shell completion show the global flags.
//
// For "filter" (repeatable in parseGlobal), we use pflag.StringArray so the help
// text shows the flag as repeatable. The empty default prints as "[]" in pflag.
func registerGlobalPersistentFlags(fs *pflag.FlagSet) {
	for _, d := range globalFlagDefs {
		switch d.kind {
		case "string":
			fs.String(d.name, d.defaultVal, d.usage)
		case "bool":
			fs.Bool(d.name, d.defaultVal == "true", d.usage)
		case "duration":
			fs.Duration(d.name, parseDurationDefault(d), d.usage)
		case "stringArray":
			fs.StringArray(d.name, nil, d.usage)
		default:
			panic("globalFlagDef: unknown kind " + d.kind + " for flag --" + d.name)
		}
	}
}

// parseDurationDefault parses the duration default value for d and panics with
// a descriptive message if the value is malformed. Shared by defineGlobalFlags
// and registerGlobalPersistentFlags so both sites stay in sync.
func parseDurationDefault(d globalFlagDef) time.Duration {
	dur, err := time.ParseDuration(d.defaultVal)
	if err != nil {
		panic("globalFlagDef: bad duration default for " + d.name + ": " + err.Error())
	}
	return dur
}
