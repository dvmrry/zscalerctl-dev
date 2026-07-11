package cli

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	machineruntime "github.com/dvmrry/zscalerctl/internal/runtime"
	"github.com/spf13/cobra"
)

type App struct {
	out       io.Writer
	err       io.Writer
	env       []string
	stdoutTTY bool
	stderrTTY bool
	reader    ResourceReader
	catalog   resources.ResourceCatalog
	logger    *slog.Logger

	machineRuntimeFactory machineRuntimeFactory
}

// diagLogger returns the diagnostic logger, defaulting to a disabled one so log
// calls are always safe even before --log-level is parsed.
func (a *App) diagLogger() *slog.Logger {
	if a.logger == nil {
		return disabledLogger()
	}
	return a.logger
}

func disabledLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newDiagLogger builds a stderr diagnostic logger at the requested level.
// Diagnostics are metadata-only and go to stderr so stdout stays clean for
// data; "off" (the default) discards everything.
func newDiagLogger(w io.Writer, level string) (*slog.Logger, error) {
	var lvl slog.Level
	switch level {
	case "", "off":
		return disabledLogger(), nil
	case "error":
		lvl = slog.LevelError
	case "warn":
		lvl = slog.LevelWarn
	case "info":
		lvl = slog.LevelInfo
	case "debug":
		lvl = slog.LevelDebug
	default:
		return nil, fmt.Errorf("invalid log level %q: want off, error, warn, info, or debug", level)
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: lvl})), nil
}

func New(out, err io.Writer, env []string) *App {
	return NewWithOptions(out, err, env, Options{
		StdoutTTY: output.IsTerminal(out),
		StderrTTY: output.IsTerminal(err),
	})
}

type ResourceReader interface {
	List(context.Context, resources.Product, string) ([]resources.SourceRecord, error)
	Get(context.Context, resources.Product, string, string) (resources.SourceRecord, error)
	Show(context.Context, resources.Product, string) (resources.SourceRecord, error)
}

type machineReadExecutor interface {
	Read(context.Context, machine.ResourceReadRequest) (machine.ResourceReadResult, error)
}

type machineRuntime interface {
	machineReadExecutor
	Redaction() redact.Mode
}

type machineRuntimeFactory func(context.Context, config.Config, globalOptions) (machineRuntime, error)

type Options struct {
	StdoutTTY bool
	StderrTTY bool
	Reader    ResourceReader
	Catalog   resources.ResourceCatalog
}

func NewWithOptions(out, err io.Writer, env []string, opts Options) *App {
	envCopy := append([]string(nil), env...)
	// Resolve the catalog once at construction: use the caller-supplied
	// catalog when explicitly provided (even if empty — a non-nil slice is an
	// intentional injection, e.g. an empty catalog for the empty-catalog
	// schema-list test path); otherwise build from the full static catalog.
	// All later calls to resourceCatalog() return a cheap copy of this slice.
	var catalog resources.ResourceCatalog
	if opts.Catalog != nil {
		catalog = append(resources.ResourceCatalog(nil), opts.Catalog...)
	} else {
		catalog = resources.Catalog()
	}
	return &App{
		out:       out,
		err:       err,
		env:       envCopy,
		stdoutTTY: opts.StdoutTTY,
		stderrTTY: opts.StderrTTY,
		reader:    opts.Reader,
		catalog:   catalog,
	}
}

func (a *App) resourceCatalog() resources.ResourceCatalog {
	catalog := append(resources.ResourceCatalog(nil), a.catalog...)
	// Guarantee a non-nil slice so an empty catalog serialises to JSON as
	// "[]" rather than "null" (e.g. the empty-catalog schema-list path).
	if catalog == nil {
		return resources.ResourceCatalog{}
	}
	return catalog
}

// spinnerActive reports whether a progress spinner should render. Three
// conditions must all be true:
//
//  1. stderr is an interactive TTY (a.stderrTTY). This is an EXPLICIT gate:
//     even if --color always is set, we never write braille bytes to a
//     non-TTY stderr (e.g. piped to a file). ColorAlways overrides the isTTY
//     arg inside ShouldColor, so without this explicit check --color always
//     would activate the spinner on non-TTY stderr.
//  2. No diagnostic logging is active (logLevel "" or "off"). Log lines share
//     stderr and would clash with the \r-redrawn spinner line.
//  3. ShouldColor returns true. This folds in --color never/always, NO_COLOR=1,
//     and TERM=dumb — all of which signal plain output where \r overwriting is
//     unsafe or unwanted on a real TTY.
func (a *App) spinnerActive(opts globalOptions) bool {
	return a.stderrTTY &&
		(opts.logLevel == "" || opts.logLevel == "off") &&
		output.ShouldColor(opts.colorMode, a.env, a.stderrTTY)
}

// newSpinner returns a Spinner bound to stderr, active only when spinnerActive
// returns true. The caller is responsible for calling Start/Stop.
func (a *App) newSpinner(opts globalOptions) *output.Spinner {
	return output.NewSpinner(a.err, a.spinnerActive(opts))
}

// callWithSpinner runs fn while showing an indeterminate progress spinner on
// stderr (gated by spinnerActive), clearing it before fn's result is used.
// Stop is called synchronously before returning so the caller can safely check
// the error and render to stdout without racing with a live spinner on stderr.
//
// defer s.Stop() is registered immediately after Start so that a panic inside
// fn does not orphan the spinner goroutine (which would otherwise keep writing
// to stderr until process exit). Stop is idempotent, so the deferred call is
// a safe no-op if fn returns normally and Stop has already been called.
func callWithSpinner[T any](a *App, opts globalOptions, msg string, fn func() (T, error)) (T, error) {
	s := a.newSpinner(opts)
	s.Start(msg)
	defer s.Stop()
	return fn()
}

func (a *App) Run(ctx context.Context, args []string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	// __complete / __completeNoDesc: Cobra's shell-completion protocol. These tokens
	// must bypass parseGlobal entirely so that global flags appearing AFTER
	// __complete (e.g. "__complete --log-level ''") are not consumed by the
	// global-flag scanner. Cobra's completion engine owns the arg stream from here.
	// Security: execCobra never calls LoadConfig; the Cobra completion path
	// short-circuits RunE and never reaches any credential-loading code.
	if len(args) > 0 && (args[0] == "__complete" || args[0] == "__completeNoDesc") {
		return a.execCobra(ctx, globalOptions{}, args)
	}
	opts, rest, err := parseGlobal(args)
	if err != nil {
		return err
	}
	opts.format = a.resolveFormat(opts)
	if opts.output != "" && !opts.help && len(rest) > 0 && rest[0] == "dump" {
		return UsageError{Message: "usage: zscalerctl dump --out <dir>; --output cannot be used with dump"}
	}
	if opts.output != "" {
		originalOut := a.out
		var buffered bytes.Buffer
		a.out = &buffered
		err := a.runParsed(ctx, opts, rest)
		a.out = originalOut
		if err != nil && !errors.Is(err, ErrDriftDetected) {
			return err
		}
		if writeErr := writeOutputFile(opts.output, buffered.Bytes()); writeErr != nil {
			return writeErr
		}
		return err
	}
	return a.runParsed(ctx, opts, rest)
}

func (a *App) runParsed(ctx context.Context, opts globalOptions, rest []string) error {
	if logger, err := newDiagLogger(a.err, opts.logLevel); err == nil {
		a.logger = logger
	}
	// __complete / __completeNoDesc: Cobra's internal shell-completion protocol.
	// Route them straight to execCobra BEFORE any narrowing-flag validation or
	// help-gating. This is SECURITY-CRITICAL: the config-free path must not call
	// LoadConfig or construct a reader. execCobra itself never calls LoadConfig;
	// LoadConfig only runs inside individual RunE callbacks (newProductCmd,
	// newDoctorCmd, etc.) — shell completion short-circuits RunE and never reaches
	// those callbacks, so no credentials are ever loaded during completion.
	if len(rest) > 0 && (rest[0] == "__complete" || rest[0] == "__completeNoDesc") {
		return a.execCobra(ctx, opts, rest)
	}
	// Help routing:
	//   - No command (empty rest) renders root help or the machine error path.
	//   - Unknown command with --help keeps the resource-aware usage hint.
	//   - Recognized command with --help routes straight to execCobra BEFORE the
	//     narrowing/format gates below. This matches the legacy short-circuit where
	//     opts.help fired before any flag validation, so combinations such as
	//     "--filter name=x version --help", "--fields id zia locations --help", and
	//     "--format ndjson completion --help" all show help (exit 0) rather than
	//     hitting the narrowing/format gates (exit 2).
	//
	// CRITICAL: only the opts.help branch is affected. The non-help variants
	// ("--filter name=x version", "--format ndjson version") must still hit the
	// gates below → exit 2.
	if opts.help {
		if len(rest) == 0 {
			// Bare --help: let Cobra render the root help.
			return a.execCobra(ctx, opts, []string{"--help"})
		}
		if !isKnownCommand(rest[0], a.resourceCatalog()) {
			// Unknown command: keep the resource-aware usage hint.
			a.writeHelp(a.out, rest)
			return nil
		}
		// A --help request on a recognized command is a meta-request: route it to
		// Cobra's help before the narrowing/format gates, matching the legacy
		// behaviour where opts.help short-circuited prior to flag validation.
		return a.execCobra(ctx, opts, rest)
	}
	if len(rest) == 0 {
		// Scoped flags require a command even when none is given; raise the usage
		// error (exit 2) BEFORE the bare-help fallback, so "--filter x" /
		// "--search x" / "--fields x" with no command is rejected rather than
		// silently turned into a help display.
		if name := opts.narrowingFlag(); name != "" {
			return UsageError{Message: fmt.Sprintf("%s applies to list operations only; use it with \"<product> <resource> list\"", name)}
		}
		if len(opts.fields) > 0 {
			return UsageError{Message: "--fields applies to resource read operations only; use it with \"<product> <resource> list|get|show\""}
		}
		// Bare invocation: an interactive terminal gets Cobra's root help; a
		// machine/piped context treats a missing command as an error rendered
		// through the configured format (e.g. a JSON envelope), per the
		// machine-first contract.
		if a.stdoutTTY {
			return a.execCobra(ctx, opts, []string{"--help"})
		}
		a.writeUsageForHumans(opts)
		return UsageError{Message: "missing command"}
	}
	// --filter/--search narrow list results only. Reject every other invocation
	// up front — get/show/dump and non-resource commands alike — so the usage
	// error (documented exit 2) is raised before any credential or reader work.
	if name := opts.narrowingFlag(); name != "" && !isListInvocation(rest, a.resourceCatalog()) {
		return UsageError{Message: fmt.Sprintf("%s applies to list operations only; use it with \"<product> <resource> list\"", name)}
	}
	// --fields narrows projected resource records, so it applies only to a
	// <product> <resource> list|get|show read. Reject it on any other recognized
	// command, where it would otherwise be silently ignored; an unrecognized
	// token (e.g. a product name a value-taking flag swallowed) falls through to
	// the dispatch's more specific swallowed-product hint.
	if len(opts.fields) > 0 && isKnownCommand(rest[0], a.resourceCatalog()) && !isResourceReadInvocation(rest, a.resourceCatalog()) {
		return UsageError{Message: "--fields applies to resource read operations only; use it with \"<product> <resource> list|get|show\""}
	}
	// completion does not produce a record stream, so --format ndjson is rejected
	// here, before execCobra. This check must come
	// before the Cobra dispatch so the format gate fires even when Cobra owns the
	// completion command.
	if rest[0] == "completion" && opts.format == output.FormatNDJSON {
		return rejectUnsupportedFormat("completion", opts.format)
	}
	// Cobra dispatch: all recognized commands go through the unified Cobra tree.
	// Unknown commands fall through to unknownCommandMessage so
	// the CLI continues to give product/resource hints rather than Cobra's generic
	// "unknown command" output.
	if isKnownCommand(rest[0], a.resourceCatalog()) {
		return a.execCobra(ctx, opts, rest)
	}
	a.writeUsageForHumans(opts)
	return UsageError{Message: unknownCommandMessage(rest[0], a.resourceCatalog())}
}

// writeUsageForHumans writes the usage block to stderr only when the
// command-boundary error will be rendered as plain text. With an explicit
// --format json — or the auto default off a terminal — main emits a JSON
// envelope on the same stderr, and a prepended text block would make the
// stream unparseable for the automation consumers the envelope exists for.
// Mirrors main's errorFormat decision.
func (a *App) writeUsageForHumans(opts globalOptions) {
	if opts.format == output.FormatJSON || opts.format == output.FormatNDJSON || (opts.format == output.FormatAuto && !a.stdoutTTY) {
		return
	}
	a.writeUsage(a.err, a.resourceCatalog())
}

// unknownCommandMessage reports an unknown command, and when the token is in
// fact a known resource name, hints that a value-taking flag (e.g. --fields)
// likely consumed the product name before it — the common cause of, say,
// `--fields zia locations list` being parsed as command "locations".
func unknownCommandMessage(name string, catalog resources.ResourceCatalog) string {
	msg := fmt.Sprintf("unknown command %q", name)
	for _, resource := range allResourceNames(catalog) {
		if resource == name {
			return msg + fmt.Sprintf("; %q is a resource — run it as \"<product> %s ...\" and check that a value-taking flag (such as --fields) did not consume the product name", name, name)
		}
	}
	return msg
}

type globalOptions struct {
	profile            string
	configPath         string
	format             output.Format
	output             string
	timeout            time.Duration
	redaction          redact.Mode
	redactionSet       bool
	noCache            bool
	colorMode          output.ColorMode
	logLevel           string
	fields             []string
	filters            []recordFilter
	search             string
	help               bool
	argTerminatorIndex int
}

// narrowingFlag names the first result-narrowing flag in use (--filter or
// --search), or "" when neither is set. Used to scope both flags to list
// operations with a usage error that names the offending flag.
func (o globalOptions) narrowingFlag() string {
	if len(o.filters) > 0 {
		return "--filter"
	}
	if o.search != "" {
		return "--search"
	}
	return ""
}

// recordFilter is one parsed --filter expression: key=value (exact match on
// the rendered field value) or key~value (case-insensitive substring).
type recordFilter struct {
	key       string
	value     string
	substring bool
}

// repeatableFlag collects every occurrence of a flag instead of keeping only
// the last one, so --filter can be repeated and the filters AND together.
type repeatableFlag []string

func (f *repeatableFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *repeatableFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

// parseFilterExpr splits one --filter expression at its first operator
// character: '=' selects exact matching, '~' case-insensitive substring
// matching. Everything after the operator is the value verbatim, so values may
// themselves contain '=' or '~'.
func parseFilterExpr(raw string) (recordFilter, error) {
	idx := strings.IndexAny(raw, "=~")
	if idx < 0 {
		return recordFilter{}, UsageError{Message: fmt.Sprintf("--filter %q: want key=value (exact) or key~value (substring)", raw)}
	}
	key := strings.TrimSpace(raw[:idx])
	if key == "" {
		return recordFilter{}, UsageError{Message: fmt.Sprintf("--filter %q: missing field name before %q", raw, string(raw[idx]))}
	}
	return recordFilter{
		key:       key,
		value:     raw[idx+1:],
		substring: raw[idx] == '~',
	}, nil
}

func parseGlobal(args []string) (globalOptions, []string, error) {
	fs := flag.NewFlagSet("zscalerctl", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	// All 13 global flags are registered via defineGlobalFlags (globalflags.go),
	// which derives from globalFlagDefs — the single source of truth. The drift
	// test calls defineGlobalFlags on a fresh flag.FlagSet to enumerate canonical
	// names/types; any flag added here must be added to globalFlagDefs first.
	var filterFlags repeatableFlag
	gp := defineGlobalFlags(fs, &filterFlags)
	profile := gp.profile
	configPath := gp.configPath
	format := gp.format
	outputPath := gp.outputPath
	timeout := gp.timeout
	redactionFlag := gp.redaction
	noCache := gp.noCache
	colorFlag := gp.colorFlag
	noColor := gp.noColor
	logLevel := gp.logLevel
	fieldsFlag := gp.fieldsFlag
	searchFlag := gp.searchFlag
	globalArgs, rest, help, terminatorIndex, err := splitGlobalArgs(args)
	if err != nil {
		return globalOptions{}, nil, err
	}
	if err := fs.Parse(globalArgs); err != nil {
		return globalOptions{}, nil, UsageError{Message: err.Error()}
	}
	parsedFormat, err := output.ParseFormat(*format)
	if err != nil {
		return globalOptions{}, nil, UsageError{Message: err.Error()}
	}
	var parsedRedaction redact.Mode
	redactionSet := *redactionFlag != ""
	if redactionSet {
		var err error
		parsedRedaction, err = redact.ParseMode(*redactionFlag)
		if err != nil {
			return globalOptions{}, nil, UsageError{Message: err.Error()}
		}
	}
	if *timeout <= 0 {
		return globalOptions{}, nil, UsageError{Message: "timeout must be positive"}
	}
	colorMode, err := output.ParseColorMode(*colorFlag)
	if err != nil {
		return globalOptions{}, nil, UsageError{Message: err.Error()}
	}
	if *noColor {
		colorMode = output.ColorNever
	}
	if _, err := newDiagLogger(io.Discard, *logLevel); err != nil {
		return globalOptions{}, nil, UsageError{Message: err.Error()}
	}
	filters := make([]recordFilter, 0, len(filterFlags))
	for _, raw := range filterFlags {
		filter, err := parseFilterExpr(raw)
		if err != nil {
			return globalOptions{}, nil, err
		}
		filters = append(filters, filter)
	}
	return globalOptions{
		profile:            *profile,
		configPath:         *configPath,
		format:             parsedFormat,
		output:             *outputPath,
		timeout:            *timeout,
		redaction:          parsedRedaction,
		redactionSet:       redactionSet,
		noCache:            *noCache,
		colorMode:          colorMode,
		logLevel:           *logLevel,
		fields:             parseFieldsList(*fieldsFlag),
		filters:            filters,
		search:             *searchFlag,
		help:               help,
		argTerminatorIndex: terminatorIndex,
	}, rest, nil
}

func parseFieldsList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func splitGlobalArgs(args []string) ([]string, []string, bool, int, error) {
	var global []string
	var rest []string
	help := false
	argTerminatorIndex := 0
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			// Record the index in the stripped rest slice where the terminator
			// should be reinserted. A value of 0 means the terminator was at the
			// very beginning (e.g. "zscalerctl -- version"), which should NOT be
			// reinserted before Cobra.
			argTerminatorIndex = len(rest)
			rest = append(rest, args[i+1:]...)
			break
		}
		if arg == "-h" || arg == "--help" {
			help = true
			continue
		}
		name, hasValue := flagName(arg)
		if !isGlobalFlag(name) {
			rest = append(rest, arg)
			continue
		}
		global = append(global, arg)
		if hasValue || isGlobalBoolFlag(name) {
			continue
		}
		if i+1 >= len(args) {
			return nil, nil, false, 0, UsageError{Message: fmt.Sprintf("flag needs an argument: -%s", name)}
		}
		i++
		global = append(global, args[i])
	}
	return global, rest, help, argTerminatorIndex, nil
}

func flagName(arg string) (string, bool) {
	var name string
	switch {
	case strings.HasPrefix(arg, "--"):
		if arg == "--" {
			return "", false
		}
		name = strings.TrimPrefix(arg, "--")
	case strings.HasPrefix(arg, "-"):
		// Accept single-dash flags too (Go's flag package treats -flag and --flag
		// equivalently); rejecting them gave agents a confusing usage error.
		if arg == "-" {
			return "", false
		}
		name = strings.TrimPrefix(arg, "-")
	default:
		return "", false
	}
	before, _, found := strings.Cut(name, "=")
	if found {
		return before, true
	}
	return name, false
}

func isGlobalFlag(name string) bool {
	return globalFlagNameSet[name]
}

func isGlobalBoolFlag(name string) bool {
	return globalBoolFlagNameSet[name]
}

func applyOptions(cfg *config.Config, opts globalOptions) {
	if opts.redactionSet {
		cfg.Defaults.Redaction = opts.redaction
	}
	if opts.noCache {
		cfg.Defaults.NoCache = true
	}
}

// buildCommandTree constructs the full Cobra command tree — root command plus all
// subcommands — wired for the given opts. This is the SINGLE definition of the
// tree: execCobra and BuildCommandTree both call it so the tree can never drift
// between the live dispatch path and the generator / introspection path.
func (a *App) buildCommandTree(opts globalOptions) *cobra.Command {
	root := newRootCmd(a)
	configureHelp(root, a.style(opts))
	root.AddCommand(a.newVersionCmd(opts), a.newDoctorCmd(opts), a.newDumpCmd(opts), a.newDiffCmd(opts),
		a.newConfigCmd(opts), a.newSchemaCmd(opts), a.newAuthCmd(opts), a.newIntrospectCmd(opts),
		a.newMachineCmd(opts))
	catalog := a.resourceCatalog()
	for _, p := range knownProducts(catalog) {
		root.AddCommand(a.newProductCmd(p, opts))
	}
	root.InitDefaultHelpCmd()
	for _, c := range root.Commands() {
		if c.Name() == "help" {
			c.Annotations = map[string]string{"introspect/args-policy": "arbitrary"}
			break
		}
	}
	return root
}

// BuildCommandTree is the exported entry point for the CLI-reference generator
// (scripts/gen-cli-docs.go). It constructs the full Cobra command tree with
// zero-value global options so the tree is config-free and introspectable
// without credentials or a live config file. The caller must not execute the
// tree — the RunE closures capture a real App; they are present for Cobra's
// metadata (Use/Short/Long/Flags) only.
func BuildCommandTree(a *App) *cobra.Command {
	return a.buildCommandTree(globalOptions{})
}

// execCobra builds a transient Cobra root and dispatches rest through it.
// All recognized commands reach here after the pre-Cobra guards in runParsed.
//
// --help re-insertion (v2.1 fix): parseGlobal strips --help into opts.help.
// If the caller had "version --help", rest is ["version"] and opts.help is true.
// We re-append "--help" so Cobra renders the subcommand help rather than running
// the command.
//
// -- separator re-insertion: splitGlobalArgs records where "--" appeared while
// stripping globals. If it came after at least one command word, reinsert it so
// Cobra does not parse dash-prefixed positionals as flags. If it came
// immediately after a value-taking local flag, do NOT reinsert it: the next
// token is already in the right place to become the flag's value (e.g.
// "dump --out -- -weird-path" must set --out to -weird-path, not fail on an
// unknown flag).
//
// Unknown-command wrap (defensive): runParsed already filters unknown commands
// to the resource-aware unknownCommandMessage path, but this hook remains a safety net
// if a command ever reaches Cobra without passing through that filter.
func (a *App) execCobra(ctx context.Context, opts globalOptions, rest []string) error {
	root := a.buildCommandTree(opts)

	args := rest
	// Re-insert --help only for non-completion args: injecting --help into the
	// __complete stream would corrupt the completion output (L-15).
	// If a "--" terminator was present, --help must be inserted before the
	// terminator position so Cobra still sees it as a flag, not as a positional.
	if opts.help && !isCompletionArgs(rest) {
		insertAt := len(args)
		if opts.argTerminatorIndex > 0 && opts.argTerminatorIndex <= len(args) {
			insertAt = opts.argTerminatorIndex
		}
		args = append(args[:insertAt], append([]string{"--help"}, args[insertAt:]...)...)
	}

	// Re-insert the "--" arg terminator before Cobra's flag parser when needed.
	// Not used for dynamic completion because the completion protocol never has
	// local flag values.
	if opts.argTerminatorIndex > 0 && opts.argTerminatorIndex <= len(args) && !isCompletionArgs(args) {
		if shouldReinsertTerminator(root, args, opts.argTerminatorIndex) {
			insertAt := opts.argTerminatorIndex
			if opts.help {
				insertAt++
			}
			args = append(args[:insertAt], append([]string{"--"}, args[insertAt:]...)...)
		}
	}

	// Completion paths (static script generation and the __complete runtime
	// protocol) must bypass the stdout redactor: the redactor's high-entropy
	// heuristic false-positives on shell variable assignments such as
	// "local shellCompDirectiveFilterFileExt=8", corrupting the script.
	// stderr remains redacted — errors may echo user-supplied tokens.
	// Safety proof: TestCompletionScriptsDoNotReadCredentialFilesOrUseReader
	// demonstrates that completion never resolves credentials, so bypassing the
	// redactor on stdout cannot leak anything.
	var err error
	if isCompletionArgs(args) {
		err = a.executeRootCompletion(ctx, root, args)
	} else {
		err = a.executeRoot(ctx, root, args)
	}
	if err != nil && strings.HasPrefix(err.Error(), "unknown command") {
		// Safety net: if a command somehow reaches Cobra without runParsed's
		// unknown-command filter, still exit 2 with a usage error.
		return UsageError{Message: err.Error()}
	}
	return err
}

// shouldReinsertTerminator reports whether the "--" arg terminator should be
// reinserted before Cobra dispatch. It returns false when the token immediately
// before the terminator is a value-taking local flag on the target command;
// in that case the following token is already positioned to become the flag's
// value, so reinserting "--" would cause Cobra to consume it as the value and
// then reject the real value as an unknown flag.
func shouldReinsertTerminator(root *cobra.Command, args []string, terminatorIndex int) bool {
	if terminatorIndex <= 0 || terminatorIndex > len(args) {
		return false
	}
	// Walk the tree with the tokens before the terminator to find the target command.
	target, remaining, err := root.Find(args[:terminatorIndex])
	if err != nil || target == root {
		return true
	}
	// The token immediately before the terminator is the last token after the
	// command path (or the command name itself if the terminator is right after it).
	var prev string
	switch {
	case len(remaining) > 0:
		prev = remaining[len(remaining)-1]
	case len(args) > 0:
		prev = args[0]
	default:
		return true
	}
	if !strings.HasPrefix(prev, "-") {
		return true
	}
	flagName := strings.TrimPrefix(strings.TrimPrefix(prev, "--"), "-")
	if flag := target.Flags().Lookup(flagName); flag != nil {
		return flag.Value.Type() == "bool"
	}
	// Unknown flag: preserve Cobra's normal error path by reinserting the terminator.
	return true
}

// isCompletionArgs reports whether args represents a completion invocation:
// the static script generators ("completion bash|zsh|fish|powershell") or
// Cobra's dynamic completion protocol ("__complete", "__completeNoDesc").
// These paths require executeRootCompletion (raw stdout, no redactor).
func isCompletionArgs(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "completion", "__complete", "__completeNoDesc":
		return true
	}
	return false
}

func (a *App) runProduct(ctx context.Context, cfg config.Config, opts globalOptions, productName string, args []string) error {
	product := resources.Product(productName)
	resource := ""
	if len(args) >= 1 {
		resource = args[0]
	}
	// zia url-lookup is a diagnostic verb, not a catalog resource; dispatch it
	// before resource lookup so it never collides with the list/get/show model.
	//
	// Defensive fallback: via normal dispatch this branch is unreachable because
	// "zia url-lookup" routes to newURLLookupCmd. It remains here for callers
	// that invoke runProduct directly (e.g. tests) and as protection against any
	// future routing changes.
	if product == resources.ProductZIA && resource == urlLookupCommandName {
		return a.runURLLookup(ctx, cfg, opts, args[1:])
	}
	// When the resource is recognized, prefer help that lists its actual
	// operations and renderable fields over the generic per-product usage.
	catalog := a.resourceCatalog()
	helpSpec, helpSpecOK := catalog.FindSpec(product, resource)
	usage := func() string {
		if helpSpecOK {
			return resourceUsage(product, helpSpec, 0)
		}
		return productCommandUsage(product, 0, catalog)
	}
	if len(args) < 2 {
		return UsageError{Message: usage()}
	}
	op := args[1]
	if op == "list" && len(args) != 2 {
		return UsageError{Message: fmt.Sprintf("usage: zscalerctl %s %s list", productName, resource)}
	}
	if op == "get" && len(args) != 3 {
		return UsageError{Message: fmt.Sprintf("usage: zscalerctl %s %s get <id>", productName, resource)}
	}
	if op == "show" && len(args) != 2 {
		return UsageError{Message: fmt.Sprintf("usage: zscalerctl %s %s show", productName, resource)}
	}
	if op != "list" && op != "get" && op != "show" {
		return UsageError{Message: usage()}
	}
	spec, ok := a.resourceCatalog().FindSpec(product, resource)
	if !ok {
		return ResourceNotFoundError{Product: product, Resource: resource}
	}
	if err := resources.AssertReadOnly(spec); err != nil {
		return err
	}
	if !spec.SupportsReadOperation(op) {
		return UsageError{Message: fmt.Sprintf("unsupported operation %s for %s/%s\n%s", op, product, resource, resourceUsage(product, spec, 0))}
	}
	rt, err := a.machineRuntime(ctx, cfg, opts)
	if err != nil {
		return err
	}
	recordID := ""
	if op == "get" {
		recordID = args[2]
	}
	projected, err := callWithSpinner(a, opts, "contacting Zscaler", func() (resources.ProjectedRecords, error) {
		return a.executeMachineRead(ctx, spec, op, recordID, rt, opts)
	})
	if err != nil {
		return err
	}
	if op == "list" {
		errW := redact.NewWriter(a.err, cfg.Defaults.Redaction)
		warnUnknownFilterKeys(errW, spec, opts.filters)
		if err := errW.Close(); err != nil {
			return err
		}
	}
	renderOpts := opts
	// The machine/core path has already applied row narrowing. The render path
	// only keeps fields for text-format presentation order.
	renderOpts.filters = nil
	renderOpts.search = ""
	if op == "show" || op == "get" {
		records := projected.Records()
		if len(records) != 1 {
			return fmt.Errorf("resource %s %s/%s returned %d projected records, want 1", op, product, resource, len(records))
		}
		return a.writeProjectedRecord(cfg, renderOpts, spec, records[0], op)
	}
	return a.writeProjectedRecords(cfg, renderOpts, spec, projected)
}

func (a *App) executeMachineRead(
	ctx context.Context,
	spec resources.ResourceSpec,
	op string,
	recordID string,
	rt machineRuntime,
	opts globalOptions,
) (resources.ProjectedRecords, error) {
	result, err := rt.Read(ctx, machineReadRequest(spec.Product, spec.Name, op, recordID, opts))
	if err != nil {
		return resources.ProjectedRecords{}, cliErrorFromMachineRead(err)
	}
	return verifiedProjectedRecordsFromMachineResult(spec, rt.Redaction(), result)
}

func (a *App) machineRuntime(ctx context.Context, cfg config.Config, opts globalOptions) (machineRuntime, error) {
	if a.machineRuntimeFactory != nil {
		return a.machineRuntimeFactory(ctx, cfg, opts)
	}
	return a.defaultMachineRuntime(ctx, cfg, opts)
}

func (a *App) defaultMachineRuntime(ctx context.Context, cfg config.Config, opts globalOptions) (machineRuntime, error) {
	if a.reader != nil {
		return machineruntime.NewMachineFromReader(a.reader, a.resourceCatalog(), cfg.Defaults.Redaction), nil
	}
	return machineruntime.NewMachineFromConfig(ctx, cfg, machineruntime.Options{
		Timeout:    opts.timeout,
		Catalog:    a.resourceCatalog(),
		DiagLogger: a.sdkDiagLogger(opts),
	})
}

func machineReadRequest(
	product resources.Product,
	resource string,
	op string,
	recordID string,
	opts globalOptions,
) machine.ResourceReadRequest {
	input := machine.ResourceReadInput{
		Product:  string(product),
		Resource: resource,
		Fields:   opts.fields,
		Filters:  machineFilters(opts.filters),
		Search:   opts.search,
	}
	if op == string(machine.OperationGet) {
		input.RecordID = recordID
	}
	return machine.ResourceReadRequest{
		Operation: machine.Operation(op),
		Input:     input,
	}
}

func machineFilters(filters []recordFilter) []machine.Filter {
	out := make([]machine.Filter, 0, len(filters))
	for _, filter := range filters {
		operator := "="
		if filter.substring {
			operator = "~"
		}
		out = append(out, machine.Filter{
			Field:    filter.key,
			Operator: operator,
			Value:    filter.value,
		})
	}
	return out
}

func verifiedProjectedRecordsFromMachineResult(
	spec resources.ResourceSpec,
	mode redact.Mode,
	result machine.ResourceReadResult,
) (resources.ProjectedRecords, error) {
	projected := result.Records()
	if err := resources.VerifyProjectedRecords(spec, mode, projected); err != nil {
		return resources.ProjectedRecords{}, fmt.Errorf("machine result verification failed for %s/%s: %w", spec.Product, spec.Name, err)
	}
	return projected, nil
}

func (a *App) sdkDiagLogger(opts globalOptions) *slog.Logger {
	// Surface SDK retry/backoff and session/token-renewal activity only when the
	// operator opts in with --log-level debug; otherwise the reader installs a
	// nop SDK logger and stays silent.
	if opts.logLevel == "debug" {
		return a.diagLogger()
	}
	return nil
}

// isListInvocation reports whether rest is a product resource list command —
// the only invocation shape --filter/--search apply to.
func isListInvocation(rest []string, catalog resources.ResourceCatalog) bool {
	return len(rest) >= 3 && knownProductCommand(rest[0], catalog) && rest[2] == "list"
}

// isResourceReadInvocation reports whether rest is a record-projecting resource
// read (<product> <resource> list|get|show) — the only invocation shape --fields
// applies to.
func isResourceReadInvocation(rest []string, catalog resources.ResourceCatalog) bool {
	if len(rest) >= 3 && knownProductCommand(rest[0], catalog) {
		switch rest[2] {
		case "list", "get", "show":
			return true
		}
	}
	return false
}

func writeOutputFile(path string, body []byte) error {
	if strings.TrimSpace(path) == "" {
		return UsageError{Message: "--output requires a path"}
	}
	// Refuse to write through a symlink (keep the no-follow posture), but allow
	// overwriting a regular file so re-running a pipeline to the same path works.
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("write --output: %s is a symlink", path)
	}
	// Write to a temp file in the same directory, fsync it, then atomically
	// rename it over the destination, so an interrupted write never leaves a
	// truncated file at the final path. Overwriting an existing regular file is
	// still allowed (rename replaces it) so re-running a pipeline to the same
	// path works; rename targets the path itself, never through a symlink.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-"+filepath.Base(path)+"-*")
	if err != nil {
		// The destination is a user-supplied argument, so an unwritable or
		// missing directory is a usage error (documented exit 2). Report the
		// directory the user gave, not the generated temp-file name, which is
		// an implementation detail.
		return UsageError{Message: fmt.Sprintf("--output: cannot write to %s: %v", filepath.Dir(path), pathErrorReason(err))}
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write --output: %w", err)
	}
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write --output: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write --output: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write --output: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("write --output: %w", err)
	}
	cleanup = false
	return nil
}

// pathErrorReason extracts the underlying OS reason from a path error so the
// message can name the user's path instead of echoing an internal temp name.
func pathErrorReason(err error) string {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err.Error()
	}
	return err.Error()
}

func (a *App) style(opts globalOptions) output.Style {
	stdoutTTY := a.stdoutTTY && opts.output == ""
	colorMode := opts.colorMode
	// Never write ANSI escapes into a file: --output is a non-terminal sink, so
	// even an explicit --color always is suppressed. Otherwise escapes land in
	// the saved file, which the byte-scan does not strip.
	if opts.output != "" {
		colorMode = output.ColorNever
	}
	color := output.ShouldColor(colorMode, a.env, stdoutTTY)
	style := output.NewStyle(color, output.Supports256Color(a.env))
	if stdoutTTY {
		style.Width = output.TerminalWidth(a.out)
	}
	return style
}

func requireNoArgs(command string, args []string) error {
	if len(args) != 0 {
		return UsageError{Message: fmt.Sprintf("usage: zscalerctl %s", command)}
	}
	return nil
}
