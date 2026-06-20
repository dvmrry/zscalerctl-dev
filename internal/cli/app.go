package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dvmrry/zscalerctl/internal/config"
	dumpdiff "github.com/dvmrry/zscalerctl/internal/diff"
	"github.com/dvmrry/zscalerctl/internal/dump"
	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/dvmrry/zscalerctl/internal/version"
	"github.com/dvmrry/zscalerctl/internal/zscaler"
	"github.com/spf13/cobra"
)

var ErrUsage = errors.New("usage error")
var ErrPartialDump = errors.New("partial dump")
var ErrNotFound = errors.New("not found")
var ErrDriftDetected = errors.New("drift detected")

type UsageError struct {
	Message string
}

func (e UsageError) Error() string {
	return e.Message
}

func (e UsageError) Unwrap() error {
	return ErrUsage
}

type PartialDumpError struct {
	Dir    string
	Errors int
}

func (e PartialDumpError) Error() string {
	return fmt.Sprintf("partial dump written: %s (%d errors; see errors.ndjson)", e.Dir, e.Errors)
}

func (e PartialDumpError) Unwrap() error {
	return ErrPartialDump
}

type DriftDetectedError struct{}

func (e DriftDetectedError) Error() string {
	return "drift detected"
}

func (e DriftDetectedError) Unwrap() error {
	return ErrDriftDetected
}

type ResourceNotFoundError struct {
	Product  resources.Product
	Resource string
}

func (e ResourceNotFoundError) Error() string {
	// Point the caller (human or agent) at the two enumeration paths instead
	// of leaving them to guess names.
	return fmt.Sprintf("unsupported resource %s/%s; run \"zscalerctl %s --help\" or \"zscalerctl --format json schema list\" to enumerate resources", e.Product, e.Resource, e.Product)
}

func (e ResourceNotFoundError) Unwrap() error {
	return ErrNotFound
}

type App struct {
	out       io.Writer
	err       io.Writer
	env       []string
	stdoutTTY bool
	reader    ResourceReader
	catalog   resources.ResourceCatalog
	logger    *slog.Logger
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
	})
}

type ResourceReader interface {
	List(context.Context, resources.Product, string) ([]resources.SourceRecord, error)
	Get(context.Context, resources.Product, string, string) (resources.SourceRecord, error)
	Show(context.Context, resources.Product, string) (resources.SourceRecord, error)
}

type resourceSessionProvider interface {
	Session(context.Context, resources.Product) (zscaler.ResourceSession, error)
}

type Options struct {
	StdoutTTY bool
	Reader    ResourceReader
	Catalog   resources.ResourceCatalog
}

func NewWithOptions(out, err io.Writer, env []string, opts Options) *App {
	envCopy := append([]string(nil), env...)
	// Resolve the catalog once at construction: use the caller-supplied override
	// when provided (test injection), otherwise build from the full static catalog.
	// All later calls to resourceCatalog() return a cheap copy of this slice.
	var catalog resources.ResourceCatalog
	if len(opts.Catalog) > 0 {
		catalog = append(resources.ResourceCatalog(nil), opts.Catalog...)
	} else {
		catalog = resources.Catalog()
	}
	return &App{
		out:       out,
		err:       err,
		env:       envCopy,
		stdoutTTY: opts.StdoutTTY,
		reader:    opts.Reader,
		catalog:   catalog,
	}
}

func (a *App) resourceCatalog() resources.ResourceCatalog {
	return append(resources.ResourceCatalog(nil), a.catalog...)
}

func (a *App) Run(ctx context.Context, args []string) error {
	if ctx == nil {
		ctx = context.Background()
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
	// Legacy help early-return: fire only when there is no migrated command to
	// dispatch to. When rest[0] is a migrated command (e.g. "version --help"),
	// opts.help is true but we let it fall through to execCobra below, which
	// re-inserts "--help" so Cobra renders the subcommand's help instead.
	if opts.help && (len(rest) == 0 || !isMigrated(rest[0])) {
		a.writeHelp(a.out, rest)
		return nil
	}
	if len(rest) == 0 {
		a.writeUsageForHumans(opts)
		return UsageError{Message: "missing command"}
	}
	// --filter/--search narrow list results only. Reject every other invocation
	// up front — get/show/dump and non-resource commands alike — so the usage
	// error (documented exit 2) is raised before any credential or reader work.
	if name := opts.narrowingFlag(); name != "" && !isListInvocation(rest) {
		return UsageError{Message: fmt.Sprintf("%s applies to list operations only; use it with \"<product> <resource> list\"", name)}
	}
	// --fields narrows projected resource records, so it applies only to a
	// <product> <resource> list|get|show read. Reject it on any other recognized
	// command, where it would otherwise be silently ignored; an unrecognized
	// token (e.g. a product name a value-taking flag swallowed) falls through to
	// the dispatch's more specific swallowed-product hint.
	if len(opts.fields) > 0 && isKnownCommand(rest[0]) && !isResourceReadInvocation(rest) {
		return UsageError{Message: "--fields applies to resource read operations only; use it with \"<product> <resource> list|get|show\""}
	}
	// Hybrid dispatch: migrated commands go through Cobra; legacy commands continue
	// through the switch below. isMigrated gates the early-return for --help above
	// (version --help must reach Cobra) and routes here before the switch.
	if isMigrated(rest[0]) {
		return a.execCobra(ctx, opts, rest)
	}
	switch {
	case rest[0] == "help" || rest[0] == "-h" || rest[0] == "--help":
		a.writeUsage(a.out)
		return nil
	case rest[0] == "completion":
		if opts.format == output.FormatNDJSON {
			return rejectUnsupportedFormat("completion", opts.format)
		}
		return a.runCompletion(rest[1:])
	case rest[0] == "diff":
		if opts.format == output.FormatNDJSON {
			return rejectUnsupportedFormat("diff", opts.format)
		}
		return a.runDiff(opts, rest[1:])
	case rest[0] == "config" && len(rest) >= 2 && rest[1] == "init":
		// config init writes the starter config and must run before LoadConfig:
		// the target file is expected not to exist yet, and with an explicit
		// --config a missing file is otherwise a hard ErrInvalidConfig.
		return a.runConfigInit(opts, rest[2:])
	case isRunnableCommand(rest[0]):
	default:
		a.writeUsageForHumans(opts)
		return UsageError{Message: unknownCommandMessage(rest[0])}
	}

	cfg, err := config.LoadConfig(a.env, config.LoadOptions{
		Profile:    opts.profile,
		ConfigPath: opts.configPath,
	})
	if err != nil {
		return err
	}
	applyOptions(&cfg, opts)

	switch rest[0] {
	case "doctor":
		return a.runDoctor(ctx, cfg, opts, rest[1:])
	case "auth":
		return a.runAuth(ctx, cfg, opts, rest[1:])
	case "config":
		return a.runConfig(ctx, cfg, opts, rest[1:])
	case "schema":
		return a.runSchema(ctx, cfg, opts, rest[1:])
	case "dump":
		if opts.format == output.FormatNDJSON {
			return rejectUnsupportedFormat("dump", opts.format)
		}
		return a.runDump(ctx, cfg, opts, rest[1:])
	default:
		if knownProductCommand(rest[0]) {
			return a.runProduct(ctx, cfg, opts, rest[0], rest[1:])
		}
		a.writeUsageForHumans(opts)
		return UsageError{Message: unknownCommandMessage(rest[0])}
	}
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
	a.writeUsage(a.err)
}

// unknownCommandMessage reports an unknown command, and when the token is in
// fact a known resource name, hints that a value-taking flag (e.g. --fields)
// likely consumed the product name before it — the common cause of, say,
// `--fields zia locations list` being parsed as command "locations".
func unknownCommandMessage(name string) string {
	msg := fmt.Sprintf("unknown command %q", name)
	for _, resource := range allResourceNames() {
		if resource == name {
			return msg + fmt.Sprintf("; %q is a resource — run it as \"<product> %s ...\" and check that a value-taking flag (such as --fields) did not consume the product name", name, name)
		}
	}
	return msg
}

type globalOptions struct {
	profile      string
	configPath   string
	format       output.Format
	output       string
	timeout      time.Duration
	redaction    redact.Mode
	redactionSet bool
	noCache      bool
	colorMode    output.ColorMode
	logLevel     string
	fields       []string
	filters      []recordFilter
	search       string
	help         bool
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

type doctorStatus struct {
	Status      string `json:"status"`
	Mode        string `json:"mode"`
	Profile     string `json:"profile"`
	Config      string `json:"config"`
	AuthMode    string `json:"auth_mode"`
	Redaction   string `json:"redaction"`
	Timeout     string `json:"timeout"`
	Cache       string `json:"cache"`
	Proxy       string `json:"proxy"`
	Credentials string `json:"credentials"`
	LiveAPI     string `json:"live_api"`
}

func (doctorStatus) OutputSafe() {}

type authStatus struct {
	Credentials        string `json:"credentials"`
	CredentialExchange string `json:"credential_exchange"`
	LiveAPI            string `json:"live_api"`
}

func (authStatus) OutputSafe() {}

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
	globalArgs, rest, help, err := splitGlobalArgs(args)
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
		profile:      *profile,
		configPath:   *configPath,
		format:       parsedFormat,
		output:       *outputPath,
		timeout:      *timeout,
		redaction:    parsedRedaction,
		redactionSet: redactionSet,
		noCache:      *noCache,
		colorMode:    colorMode,
		logLevel:     *logLevel,
		fields:       parseFieldsList(*fieldsFlag),
		filters:      filters,
		search:       *searchFlag,
		help:         help,
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

// RequestedFormatRaw returns the --format value as parsed (auto/table/json/
// pretty), defaulting to auto, without resolving auto against a TTY. The error
// renderer in main uses it so error output follows the same format the data
// path will use.
func RequestedFormatRaw(args []string) output.Format {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			return output.FormatAuto
		}
		name, hasValue := flagName(arg)
		if name != "format" {
			continue
		}
		value := ""
		if hasValue {
			_, after, _ := strings.Cut(arg, "=")
			value = after
		} else if i+1 < len(args) {
			value = args[i+1]
		}
		if f, err := output.ParseFormat(value); err == nil {
			return f
		}
		return output.FormatAuto
	}
	return output.FormatAuto
}

func RequestedFormat(args []string) output.Format {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			return output.FormatTable
		}
		name, hasValue := flagName(arg)
		if name != "format" {
			continue
		}
		value := ""
		if hasValue {
			_, after, _ := strings.Cut(arg, "=")
			value = after
		} else if i+1 < len(args) {
			value = args[i+1]
		}
		if output.Format(strings.ToLower(strings.TrimSpace(value))) == output.FormatJSON {
			return output.FormatJSON
		}
		return output.FormatTable
	}
	return output.FormatTable
}

func splitGlobalArgs(args []string) ([]string, []string, bool, error) {
	var global []string
	var rest []string
	help := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
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
			return nil, nil, false, UsageError{Message: fmt.Sprintf("flag needs an argument: -%s", name)}
		}
		i++
		global = append(global, args[i])
	}
	return global, rest, help, nil
}

func splitDiffArgs(args []string) ([]string, []string, error) {
	boolFlags := map[string]bool{
		"ignore-operational": true,
		"detail":             true,
		"allow-partial":      true,
		"fail-on-drift":      true,
	}
	valueFlags := map[string]bool{
		"products":  true,
		"resources": true,
	}
	var flags []string
	var positionals []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		name, hasValue := flagName(arg)
		if name == "" {
			positionals = append(positionals, arg)
			continue
		}
		flags = append(flags, arg)
		if hasValue || boolFlags[name] {
			continue
		}
		if !valueFlags[name] {
			// Let flag.FlagSet produce the canonical "flag provided but not
			// defined" error for unknown flags.
			continue
		}
		if i+1 >= len(args) {
			return nil, nil, UsageError{Message: fmt.Sprintf("flag needs an argument: -%s", name)}
		}
		i++
		flags = append(flags, args[i])
	}
	return flags, positionals, nil
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

// rejectUnsupportedFormat returns an error when command does not support the
// given format. JSON is handled separately (fast-path) before this guard, so
// only non-table/non-pretty formats reach here.
func rejectUnsupportedFormat(command string, format output.Format) error {
	return UsageError{Message: fmt.Sprintf("%s does not support %s output yet", command, format)}
}

// isMigrated reports whether cmd has been migrated to Cobra dispatch. Only
// commands in this list are routed through execCobra; all others continue
// through the legacy switch in runParsed. Grows one command per phase.
func isMigrated(cmd string) bool {
	switch cmd {
	case "version":
		return true
	}
	return false
}

// execCobra builds a transient Cobra root, adds the migrated subcommand(s), and
// dispatches rest through it. It is only called when isMigrated(rest[0]) is true.
//
// --help re-insertion (v2.1 fix): parseGlobal strips --help into opts.help.
// If the caller had "version --help", rest is ["version"] and opts.help is true.
// We re-append "--help" so Cobra renders the subcommand help rather than running
// the command.
//
// TODO(phase2/3): preserve "--" for positional commands — not needed for version.
//
// Unknown-command wrap (defensive): during the hybrid phase this can't fire
// because isMigrated gates dispatch to known-migrated commands only. The wrap is
// the documented hook for when Cobra owns the full root and an unknown command
// slips through.
func (a *App) execCobra(ctx context.Context, opts globalOptions, rest []string) error {
	root := newRootCmd(a)
	root.AddCommand(a.newVersionCmd(opts))

	args := rest
	if opts.help {
		args = append(rest[:len(rest):len(rest)], "--help")
	}

	err := a.executeRoot(ctx, root, args)
	if err != nil && strings.HasPrefix(err.Error(), "unknown command") {
		// During the hybrid this can't fire (isMigrated gates to known commands),
		// but this is the documented hook for when Cobra owns the root.
		return UsageError{Message: err.Error()}
	}
	return err
}

// newVersionCmd returns the Cobra "version" subcommand. It delegates directly to
// runVersion so all format/arity/redaction behaviour is identical to the legacy
// path. No restrictive Args validator is set here — runVersion's requireNoArgs
// produces the same UsageError message as before, preserving the surface.
func (a *App) newVersionCmd(opts globalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "print version, commit, build date, and runtime info",
		RunE: func(_ *cobra.Command, args []string) error {
			return a.runVersion(opts, args)
		},
	}
}

func (a *App) runVersion(opts globalOptions, args []string) error {
	if err := requireNoArgs("version", args); err != nil {
		return err
	}
	info := version.Current()
	if opts.format == output.FormatJSON {
		return output.NewRenderer(redact.New(redact.ModeStandard)).WriteJSON(a.out, info)
	}
	if opts.format != output.FormatTable && opts.format != output.FormatPretty {
		return rejectUnsupportedFormat("version", opts.format)
	}
	body := output.RenderKeyValues([]output.KV{
		{Key: "Version", Value: info.Version},
		{Key: "Commit", Value: info.Commit},
		{Key: "Date", Value: info.Date},
		{Key: "Go", Value: info.Go},
		{Key: "Platform", Value: info.OS + "/" + info.Arch},
	}, a.style(opts))
	return output.NewRenderer(redact.New(redact.ModeStandard)).WriteText(a.out, body)
}

func (a *App) runDoctor(ctx context.Context, cfg config.Config, opts globalOptions, args []string) error {
	if err := requireNoArgs("doctor", args); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return fmt.Errorf("doctor cancelled: %w", ctx.Err())
	default:
	}
	// Doctor's job is catching problems before live calls: surface the same
	// proxy-conflict error the reader would raise on the first API request,
	// instead of reporting status OK on a configuration that cannot work.
	if err := zscaler.ValidateProxyConfig(zscaler.ProxyConfig{
		URL:             cfg.Proxy.URL,
		FromEnvironment: cfg.Proxy.FromEnvironment,
	}); err != nil {
		return err
	}
	status := newDoctorStatus(cfg, opts)
	if opts.format == output.FormatJSON {
		return a.renderer(cfg, opts).WriteJSON(a.out, status)
	}
	if opts.format != output.FormatTable && opts.format != output.FormatPretty {
		return rejectUnsupportedFormat("doctor", opts.format)
	}
	body := output.RenderKeyValues(doctorStatusRows(status), a.style(opts))
	return a.renderer(cfg, opts).WriteText(a.out, body)
}

func (a *App) runAuth(_ context.Context, cfg config.Config, opts globalOptions, args []string) error {
	if len(args) != 1 || args[0] != "status" {
		return UsageError{Message: "usage: zscalerctl auth status"}
	}
	status := newAuthStatus(cfg)
	if opts.format == output.FormatJSON {
		return a.renderer(cfg, opts).WriteJSON(a.out, status)
	}
	if opts.format != output.FormatTable && opts.format != output.FormatPretty {
		return rejectUnsupportedFormat("auth status", opts.format)
	}
	body := output.RenderKeyValues(authStatusRows(status), a.style(opts))
	return a.renderer(cfg, opts).WriteText(a.out, body)
}

func (a *App) runConfig(_ context.Context, cfg config.Config, opts globalOptions, args []string) error {
	if len(args) != 1 || args[0] != "show" {
		return UsageError{Message: "usage: zscalerctl config show"}
	}
	if opts.format == output.FormatJSON {
		return a.renderer(cfg, opts).WriteJSON(a.out, cfg.Safe())
	}
	if opts.format != output.FormatTable && opts.format != output.FormatPretty {
		return rejectUnsupportedFormat("config show", opts.format)
	}
	safe := cfg.Safe()
	body := output.RenderKeyValues([]output.KV{
		{Key: "Profile", Value: safe.Profile},
		{Key: "Config", Value: configSourceStatus(safe)},
		{Key: "Auth Mode", Value: safe.AuthMode},
		{Key: "Vanity Domain", Value: setStatus(safe.VanityDomainSet)},
		{Key: "Cloud", Value: valueOrUnset(safe.Cloud)},
		{Key: "Client ID", Value: setStatus(safe.Credentials.ClientIDSet)},
		{Key: "Client Secret", Value: secretSourceStatus(safe.Credentials.ClientSecretSet || safe.Credentials.ClientSecretFileSet, safe.Credentials.ClientSecretScheme)},
		{Key: "ZPA Customer ID", Value: setStatus(safe.ZPA.CustomerIDSet)},
		{Key: "ZPA Microtenant ID", Value: setStatus(safe.ZPA.MicrotenantIDSet)},
		{Key: "ZIA Username", Value: setStatus(safe.ZIALegacy.UsernameSet)},
		{Key: "ZIA Password", Value: secretSourceStatus(safe.ZIALegacy.PasswordSet || safe.ZIALegacy.PasswordFileSet, safe.ZIALegacy.PasswordScheme)},
		{Key: "ZIA API Key", Value: secretSourceStatus(safe.ZIALegacy.APIKeySet || safe.ZIALegacy.APIKeyFileSet, safe.ZIALegacy.APIKeyScheme)},
		{Key: "ZIA Cloud", Value: setStatus(safe.ZIALegacy.CloudSet)},
		{Key: "Proxy", Value: proxyStatus(cfg.Proxy)},
		{Key: "Redaction", Value: safe.Defaults.Redaction},
		{Key: "Cache", Value: cacheStatus(safe.Defaults.NoCache)},
	}, a.style(opts))
	return a.renderer(cfg, opts).WriteText(a.out, body)
}

func (a *App) runSchema(_ context.Context, cfg config.Config, opts globalOptions, args []string) error {
	if len(args) != 1 || args[0] != "list" {
		return UsageError{Message: "usage: zscalerctl schema list"}
	}
	catalog := a.resourceCatalog()
	if err := resources.AssertReadOnly(catalog...); err != nil {
		return err
	}
	if opts.format == output.FormatJSON {
		return a.renderer(cfg, opts).WriteJSON(a.out, catalog)
	}
	if opts.format != output.FormatTable && opts.format != output.FormatPretty {
		return rejectUnsupportedFormat("schema list", opts.format)
	}
	if len(catalog) == 0 {
		return a.renderer(cfg, opts).WriteText(a.out, output.NewSafeText("no resources enabled yet\n"))
	}
	var body strings.Builder
	for _, spec := range catalog {
		fmt.Fprintf(&body, "%s\t%s\t%s\n", spec.Product, spec.Name, strings.Join(readOperationNames(spec), ","))
	}
	return a.renderer(cfg, opts).WriteText(a.out, output.NewSafeText(body.String()))
}

func (a *App) runProduct(ctx context.Context, cfg config.Config, opts globalOptions, productName string, args []string) error {
	product := resources.Product(productName)
	resource := ""
	if len(args) >= 1 {
		resource = args[0]
	}
	// zia url-lookup is a diagnostic verb, not a catalog resource; dispatch it
	// before resource lookup so it never collides with the list/get/show model.
	if product == resources.ProductZIA && resource == urlLookupCommandName {
		return a.runURLLookup(ctx, cfg, opts, args[1:])
	}
	// When the resource is recognized, prefer help that lists its actual
	// operations and renderable fields over the generic per-product usage.
	helpSpec, helpSpecOK := a.resourceCatalog().FindSpec(product, resource)
	usage := func() string {
		if helpSpecOK {
			return resourceUsage(product, helpSpec, 0)
		}
		return productCommandUsage(product, 0)
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
	reader, err := a.resourceReader(ctx, cfg, opts)
	if err != nil {
		return err
	}
	if op == "show" {
		record, err := reader.Show(ctx, product, resource)
		if err != nil {
			return err
		}
		projected, _, err := resources.ProjectRecordAndVerify(spec, cfg.Defaults.Redaction, record)
		if err != nil {
			return err
		}
		return a.writeProjectedRecord(cfg, opts, spec, projected, op)
	}
	if op == "get" {
		record, err := reader.Get(ctx, product, resource, args[2])
		if err != nil {
			return err
		}
		projected, _, err := resources.ProjectRecordAndVerify(spec, cfg.Defaults.Redaction, record)
		if err != nil {
			return err
		}
		return a.writeProjectedRecord(cfg, opts, spec, projected, op)
	}
	records, err := reader.List(ctx, product, resource)
	if err != nil {
		return err
	}
	projected, _, err := resources.ProjectRecordsAndVerify(spec, cfg.Defaults.Redaction, records)
	if err != nil {
		return err
	}
	return a.writeProjectedRecords(cfg, opts, spec, projected)
}

func (a *App) runDump(ctx context.Context, cfg config.Config, opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("dump", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	outDir := fs.String("out", "", "dump output directory")
	productsFlag := fs.String("products", "", "comma-separated products: zia,zpa")
	resourcesFlag := fs.String("resources", "", "comma-separated resources: locations or zia/locations")
	continueOnError := fs.Bool("continue-on-error", false, "write a partial dump when individual resources fail")
	force := fs.Bool("force", false, "replace an existing zscalerctl dump directory")
	if err := fs.Parse(args); err != nil {
		return UsageError{Message: err.Error()}
	}
	if fs.NArg() != 0 {
		return UsageError{Message: dumpUsage()}
	}
	if *outDir == "" {
		return UsageError{Message: dumpUsage()}
	}
	products, err := parseProducts(*productsFlag)
	if err != nil {
		return err
	}
	selectedResources, err := parseDumpResources(*resourcesFlag, products, a.resourceCatalog())
	if err != nil {
		return err
	}
	result, err := a.collectDump(ctx, cfg, opts, products, selectedResources, *continueOnError)
	if err != nil {
		return err
	}
	for _, re := range result.Errors {
		a.diagLogger().Warn("dump resource failed",
			"product", re.Product, "resource", re.Name, "operation", re.Operation, "kind", re.Kind)
	}
	a.diagLogger().Info("dump complete",
		"resources", len(result.Entries), "errors", len(result.Errors))
	if err := prepareForcedDumpDir(*outDir, *force); err != nil {
		return err
	}
	if err := dump.Write(*outDir, cfg.Defaults.Redaction, result); err != nil {
		return err
	}
	// Dump emits no resource data on stdout (it writes files), so its status
	// notice is a diagnostic and goes to stderr, keeping stdout clean per the
	// stdout=data / stderr=diagnostics contract.
	if len(result.Errors) > 0 {
		if err := a.renderer(cfg, opts).WriteText(
			a.err,
			output.NewSafeText(fmt.Sprintf("partial dump written: %s (%d errors; see errors.ndjson)\n", *outDir, len(result.Errors))),
		); err != nil {
			return err
		}
		return PartialDumpError{Dir: *outDir, Errors: len(result.Errors)}
	}
	return a.renderer(cfg, opts).WriteText(a.err, output.NewSafeText(fmt.Sprintf("dump written: %s\n", *outDir)))
}

func (a *App) runDiff(opts globalOptions, args []string) error {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	productsFlag := fs.String("products", "", "comma-separated products")
	resourcesFlag := fs.String("resources", "", "comma-separated resources: locations or zia/locations")
	ignoreOperational := fs.Bool("ignore-operational", false, "ignore operational metadata on keyed and singleton resources")
	detail := fs.Bool("detail", false, "include record-level table details")
	allowPartial := fs.Bool("allow-partial", false, "compare partial dumps instead of rejecting them")
	failOnDrift := fs.Bool("fail-on-drift", false, "exit 7 when drift is detected")
	flagArgs, positionalArgs, err := splitDiffArgs(args)
	if err != nil {
		return err
	}
	if err := fs.Parse(flagArgs); err != nil {
		return UsageError{Message: err.Error()}
	}
	if len(positionalArgs) != 2 {
		return UsageError{Message: diffUsage()}
	}
	catalog := a.resourceCatalog()
	products, err := parseProducts(*productsFlag)
	if err != nil {
		return err
	}
	selectedResources, err := parseDumpResources(*resourcesFlag, products, catalog)
	if err != nil {
		return err
	}
	report, err := dumpdiff.Compare(positionalArgs[0], positionalArgs[1], dumpdiff.Options{
		Catalog:           catalog,
		Products:          products,
		Resources:         diffResourceSelection(selectedResources),
		IgnoreOperational: *ignoreOperational,
		AllowPartial:      *allowPartial,
	})
	if err != nil {
		if errors.Is(err, dumpdiff.ErrInvalidDump) ||
			errors.Is(err, dumpdiff.ErrPartialDumpInput) ||
			errors.Is(err, dumpdiff.ErrRedactionMismatch) {
			return UsageError{Message: err.Error()}
		}
		return err
	}
	renderer := output.NewRenderer(redact.New(redact.ModeStandard))
	switch opts.format {
	case output.FormatJSON:
		if err := renderer.WriteJSON(a.out, report); err != nil {
			return err
		}
	case output.FormatTable, output.FormatPretty:
		if err := renderer.WriteText(a.out, renderDiffTable(report, *detail, a.style(opts))); err != nil {
			return err
		}
	default:
		return rejectUnsupportedFormat("diff", opts.format)
	}
	if *failOnDrift && report.HasDrift() {
		return DriftDetectedError{}
	}
	return nil
}

func renderDiffTable(report dumpdiff.Report, detail bool, style output.Style) output.SafeText {
	var body strings.Builder
	fmt.Fprintf(
		&body,
		"%s\t%s\t%s\t%s\t%s\n",
		style.Key("RESOURCE"),
		style.Key("IDENTITY"),
		style.Key("ADDED"),
		style.Key("REMOVED"),
		style.Key("CHANGED"),
	)
	for _, resource := range report.Resources {
		resourceName := resource.Product + "/" + resource.Resource
		fmt.Fprintf(
			&body,
			"%s\t%s\t%d\t%d\t%d\n",
			resourceName,
			diffIdentityLabel(resource.Identity),
			len(resource.Added),
			len(resource.Removed),
			len(resource.Changed),
		)
		if detail && resource.HasDrift() {
			writeDiffDetailRows(&body, resourceName, resource)
		}
	}
	if len(report.Resources) == 0 {
		fmt.Fprintln(&body, "no comparable resources found")
	}
	fmt.Fprintf(
		&body,
		"\nsummary: compared=%d drifted=%d added=%d removed=%d changed=%d\n",
		report.Summary.ResourcesCompared,
		report.Summary.ResourcesWithDrift,
		report.Summary.RecordsAdded,
		report.Summary.RecordsRemoved,
		report.Summary.RecordsChanged,
	)
	return output.NewSafeText(body.String())
}

func writeDiffDetailRows(body *strings.Builder, resourceName string, resource dumpdiff.ResourceDiff) {
	for _, added := range resource.Added {
		fmt.Fprintf(body, "%s\t+\t%s\t-\t-\n", resourceName, diffRecordRefLabel(added))
	}
	for _, removed := range resource.Removed {
		fmt.Fprintf(body, "%s\t-\t%s\t-\t-\n", resourceName, diffRecordRefLabel(removed))
	}
	for _, changed := range resource.Changed {
		fmt.Fprintf(body, "%s\t~\t%s\t%s\t-\n", resourceName, changed.Key, diffFieldNames(changed.Changes))
	}
}

func diffIdentityLabel(identity dumpdiff.Identity) string {
	if identity.Field == "" {
		return identity.Mode
	}
	return identity.Mode + ":" + identity.Field
}

func diffRecordRefLabel(ref dumpdiff.RecordRef) string {
	if ref.Key != "" {
		return ref.Key
	}
	if len(ref.Hash) > 12 {
		return ref.Hash[:12]
	}
	return ref.Hash
}

func diffFieldNames(changes []dumpdiff.FieldChange) string {
	fields := make([]string, len(changes))
	for i, change := range changes {
		fields[i] = change.Field
	}
	return strings.Join(fields, ",")
}

func prepareForcedDumpDir(dir string, force bool) error {
	if !force {
		return nil
	}
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("%w: missing dump directory", dump.ErrUnsafePath)
	}
	if err := rejectDangerousForceTarget(dir); err != nil {
		return err
	}
	info, err := os.Lstat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: inspect dump directory for --force: %v", dump.ErrUnsafePath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: --force target %s is a symlink", dump.ErrUnsafePath, dir)
	}
	target, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return fmt.Errorf("%w: resolve --force target symlinks: %v", dump.ErrUnsafePath, err)
	}
	if err := rejectDangerousForceTarget(target); err != nil {
		return err
	}
	info, err = os.Lstat(target)
	if err != nil {
		return fmt.Errorf("%w: inspect resolved dump directory for --force: %v", dump.ErrUnsafePath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: --force target %s is not a directory", dump.ErrUnsafePath, dir)
	}
	empty, err := isDirEmpty(target)
	if err != nil {
		return err
	}
	if empty {
		return nil
	}
	if err := validateExistingDumpDir(target); err != nil {
		return err
	}
	// The target was resolved after rejecting a final symlink. If a same-host
	// actor swaps the directory after validation, RemoveAll on a symlink removes
	// the link itself, not its target; the command still refuses cwd/home/root
	// after symlink resolution before reaching this point.
	if err := os.RemoveAll(target); err != nil {
		return fmt.Errorf("%w: remove dump directory for --force: %v", dump.ErrUnsafePath, err)
	}
	return nil
}

func rejectDangerousForceTarget(dir string) error {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("%w: resolve --force target: %v", dump.ErrUnsafePath, err)
	}
	clean := filepath.Clean(abs)
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("%w: resolve current directory: %v", dump.ErrUnsafePath, err)
	}
	if clean == filepath.Clean(cwd) {
		return fmt.Errorf("%w: --force target cannot be the current directory", dump.ErrUnsafePath)
	}
	if filepath.Dir(clean) == clean {
		return fmt.Errorf("%w: --force target cannot be the filesystem root", dump.ErrUnsafePath)
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" && clean == filepath.Clean(home) {
		return fmt.Errorf("%w: --force target cannot be the home directory", dump.ErrUnsafePath)
	}
	return nil
}

func isDirEmpty(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, fmt.Errorf("%w: inspect dump directory for --force: %v", dump.ErrUnsafePath, err)
	}
	return len(entries) == 0, nil
}

func validateExistingDumpDir(dir string) error {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return fmt.Errorf("%w: open dump directory for --force: %v", dump.ErrUnsafePath, err)
	}
	defer root.Close()

	info, err := root.Lstat("manifest.json")
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: --force target %s is not a zscalerctl dump directory", dump.ErrUnsafePath, dir)
	}
	if err != nil {
		return fmt.Errorf("%w: inspect dump manifest for --force: %v", dump.ErrUnsafePath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%w: --force target manifest is a symlink", dump.ErrUnsafePath)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: --force target manifest is not a regular file", dump.ErrUnsafePath)
	}
	if info.Size() > 1<<20 {
		return fmt.Errorf("%w: --force target manifest is too large", dump.ErrUnsafePath)
	}
	body, err := root.ReadFile("manifest.json")
	if err != nil {
		return fmt.Errorf("%w: read dump manifest for --force: %v", dump.ErrUnsafePath, err)
	}
	var manifest struct {
		Schema string `json:"schema"`
	}
	if err := json.Unmarshal(body, &manifest); err != nil {
		return fmt.Errorf("%w: --force target %s is not a zscalerctl dump directory", dump.ErrUnsafePath, dir)
	}
	if !strings.HasPrefix(manifest.Schema, "zscalerctl.dump.manifest.") {
		return fmt.Errorf("%w: --force target %s is not a zscalerctl dump directory", dump.ErrUnsafePath, dir)
	}
	return nil
}

func (a *App) resourceReader(ctx context.Context, cfg config.Config, opts globalOptions) (ResourceReader, error) {
	if a.reader != nil {
		return a.reader, nil
	}
	clientSecret, err := cfg.Credentials.ClientSecret.Resolve(ctx)
	if err != nil {
		// Invariant: keep the credential noun LAST (parenthetical, trailing). Appending text after it can reintroduce the redactor over-redaction (see redact secret_phrase/secret_assignment rules).
		// Phrase the credential AFTER the cause (parenthesized). A "<secret>: <cause>"
		// shape makes the redactor read the nested diagnostic as a key:value secret
		// and redact the cause, hiding the real failure (redact secret_assignment rule).
		return nil, fmt.Errorf("%w: %w (while resolving the client secret)", zscaler.ErrMissingCredentials, err)
	}
	ziaPassword, err := cfg.ZIALegacy.Password.Resolve(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w (while resolving the ZIA legacy password)", zscaler.ErrMissingCredentials, err)
	}
	ziaAPIKey, err := cfg.ZIALegacy.APIKey.Resolve(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w (while resolving the ZIA legacy API key)", zscaler.ErrMissingCredentials, err)
	}
	// Surface SDK retry/backoff and session/token-renewal activity only when the
	// operator opts in with --log-level debug; otherwise the reader installs a
	// nop SDK logger and stays silent.
	var sdkDiagLogger *slog.Logger
	if opts.logLevel == "debug" {
		sdkDiagLogger = a.diagLogger()
	}
	return zscaler.NewReader(zscaler.ReaderConfig{
		ClientID:         cfg.Credentials.ClientID,
		ClientSecret:     clientSecret,
		VanityDomain:     cfg.VanityDomain,
		Cloud:            cfg.Cloud,
		ZPACustomerID:    cfg.ZPA.CustomerID,
		ZPAMicrotenantID: cfg.ZPA.MicrotenantID,
		AuthMode:         zscaler.AuthMode(cfg.EffectiveAuthMode()),
		ZIALegacy: zscaler.ZIALegacyConfig{
			Username: cfg.ZIALegacy.Username,
			Password: ziaPassword,
			APIKey:   ziaAPIKey,
			Cloud:    cfg.ZIALegacy.Cloud,
		},
		Timeout: opts.timeout,
		NoCache: cfg.Defaults.NoCache,
		Proxy: zscaler.ProxyConfig{
			URL:             cfg.Proxy.URL,
			FromEnvironment: cfg.Proxy.FromEnvironment,
		},
		DiagLogger: sdkDiagLogger,
	})
}

func (a *App) dumpResourceReader(
	ctx context.Context,
	cfg config.Config,
	opts globalOptions,
	product resources.Product,
) (ResourceReader, func(), error) {
	reader, err := a.resourceReader(ctx, cfg, opts)
	if err != nil {
		return nil, nil, err
	}
	provider, ok := reader.(resourceSessionProvider)
	if !ok {
		return reader, func() {}, nil
	}
	session, err := provider.Session(ctx, product)
	if err != nil {
		if errors.Is(err, zscaler.ErrUnsupportedResource) {
			return reader, func() {}, nil
		}
		return nil, nil, err
	}
	if session == nil {
		return nil, nil, errors.New("reader session provider returned nil session")
	}
	return session, session.Close, nil
}

func (a *App) collectDump(
	ctx context.Context,
	cfg config.Config,
	opts globalOptions,
	products map[resources.Product]bool,
	selectedResources map[dumpResourceKey]bool,
	continueOnError bool,
) (dump.Result, error) {
	result := dump.Result{}
	catalog := a.resourceCatalog()
	if err := resources.AssertReadOnly(catalog...); err != nil {
		return result, err
	}
	selectedCount := 0
	for _, spec := range catalog {
		if products[spec.Product] && dumpResourceSelected(selectedResources, spec) {
			selectedCount++
		}
	}
	// A full dump can run for minutes; at info, operators get the selection
	// size up front and one progress event per resource below.
	a.diagLogger().Info("dump starting", "resources", selectedCount)

	readers := make(map[resources.Product]ResourceReader)
	for _, spec := range catalog {
		if !products[spec.Product] {
			continue
		}
		if !dumpResourceSelected(selectedResources, spec) {
			continue
		}
		if err := ctx.Err(); err != nil {
			return result, err
		}
		reader, ok := readers[spec.Product]
		if !ok {
			var cleanup func()
			var err error
			reader, cleanup, err = a.dumpResourceReader(ctx, cfg, opts, spec.Product)
			if err != nil {
				return result, err
			}
			readers[spec.Product] = reader
			// Register cleanup once per product session, not once per resource.
			defer cleanup()
		}
		a.diagLogger().Info("dump reading resource", "product", spec.Product, "resource", spec.Name)
		if spec.SupportsReadOperation("show") {
			record, err := reader.Show(ctx, spec.Product, spec.Name)
			if err != nil {
				if ctxErr := ctx.Err(); ctxErr != nil {
					return result, ctxErr
				}
				if continueOnError {
					result.Errors = append(result.Errors, dump.NewResourceError(spec.Product, spec.Name, "show", "show_failed"))
					continue
				}
				return result, fmt.Errorf("dump %s/%s show failed: %w", spec.Product, spec.Name, err)
			}
			projected, report, err := resources.ProjectRecordAndVerify(spec, cfg.Defaults.Redaction, record)
			if err != nil {
				operation := "project"
				kind := "projection_failed"
				if errors.Is(err, resources.ErrUnexpectedField) {
					operation = "validate"
					kind = "subset_failed"
				}
				if continueOnError {
					result.Errors = append(result.Errors, dump.NewResourceError(spec.Product, spec.Name, operation, kind))
					continue
				}
				return result, fmt.Errorf("dump %s/%s %s failed: %w", spec.Product, spec.Name, operation, err)
			}
			result.Entries = append(result.Entries, dump.ResourceDump{
				Spec:    spec,
				Record:  &projected,
				Reports: []resources.ProjectionReport{report},
			})
			continue
		}
		records, err := reader.List(ctx, spec.Product, spec.Name)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return result, ctxErr
			}
			if continueOnError {
				result.Errors = append(result.Errors, dump.NewResourceError(spec.Product, spec.Name, "list", "list_failed"))
				continue
			}
			return result, fmt.Errorf("dump %s/%s list failed: %w", spec.Product, spec.Name, err)
		}
		projected, reports, err := resources.ProjectRecordsAndVerify(spec, cfg.Defaults.Redaction, records)
		if err != nil {
			operation := "project"
			kind := "projection_failed"
			if errors.Is(err, resources.ErrUnexpectedField) {
				operation = "validate"
				kind = "subset_failed"
			}
			if continueOnError {
				result.Errors = append(result.Errors, dump.NewResourceError(spec.Product, spec.Name, operation, kind))
				continue
			}
			return result, fmt.Errorf("dump %s/%s %s failed: %w", spec.Product, spec.Name, operation, err)
		}
		result.Entries = append(result.Entries, dump.ResourceDump{
			Spec:    spec,
			Records: projected,
			Reports: reports,
		})
	}
	return result, nil
}

// effectiveFields returns the field order to render: the renderable fields for
// the mode, optionally narrowed to --fields. --fields can only select from the
// already-renderable set; an unknown field name (not in the catalog at all) is
// a usage error, while a known-but-not-rendered field (secret or mode-excluded)
// is silently skipped, so --fields can never widen the sanitized output.
func effectiveFields(spec resources.ResourceSpec, mode redact.Mode, requested []string) ([]string, error) {
	order := spec.FieldOrder(mode)
	if len(requested) == 0 {
		return order, nil
	}
	catalog := make(map[string]struct{}, len(spec.Fields))
	for _, field := range spec.Fields {
		catalog[field.JSONField()] = struct{}{}
	}
	renderable := make(map[string]struct{}, len(order))
	for _, name := range order {
		renderable[name] = struct{}{}
	}
	out := make([]string, 0, len(requested))
	for _, name := range requested {
		if _, ok := catalog[name]; !ok {
			return nil, UsageError{Message: fmt.Sprintf("--fields: %q is not a field of %s/%s", name, spec.Product, spec.Name)}
		}
		if _, ok := renderable[name]; ok {
			out = append(out, name)
		}
	}
	return out, nil
}

func (a *App) writeProjectedRecord(
	cfg config.Config,
	opts globalOptions,
	spec resources.ResourceSpec,
	record resources.ProjectedRecord,
	operation string,
) error {
	fields, err := effectiveFields(spec, cfg.Defaults.Redaction, opts.fields)
	if err != nil {
		return err
	}
	if len(opts.fields) > 0 {
		record = record.Select(fields)
	}
	switch opts.format {
	case output.FormatJSON:
		return a.renderer(cfg, opts).WriteJSON(a.out, record)
	case output.FormatNDJSON:
		return a.renderer(cfg, opts).WriteNDJSON(a.out, []output.SafeJSON{record})
	case output.FormatTable:
		if operation == "show" {
			return a.renderer(cfg, opts).WriteText(a.out, renderRecordKeyValues(fields, record, a.style(opts)))
		}
		return a.renderer(cfg, opts).WriteText(a.out, renderRecordsTable(fields, resources.NewProjectedRecords([]resources.ProjectedRecord{record}), a.style(opts)))
	case output.FormatPretty:
		if operation == "show" {
			return a.renderer(cfg, opts).WriteText(a.out, renderRecordPretty(fields, record, a.style(opts)))
		}
		return a.renderer(cfg, opts).WriteText(a.out, renderRecordsPretty(fields, resources.NewProjectedRecords([]resources.ProjectedRecord{record}), a.style(opts)))
	default:
		return fmt.Errorf("unhandled output format %q for resource %s", opts.format, operation)
	}
}

// isListInvocation reports whether rest is a product resource list command —
// the only invocation shape --filter/--search apply to.
func isListInvocation(rest []string) bool {
	return len(rest) >= 3 && knownProductCommand(rest[0]) && rest[2] == "list"
}

// isResourceReadInvocation reports whether rest is a record-projecting resource
// read (<product> <resource> list|get|show) — the only invocation shape --fields
// applies to.
func isResourceReadInvocation(rest []string) bool {
	if len(rest) >= 3 && knownProductCommand(rest[0]) {
		switch rest[2] {
		case "list", "get", "show":
			return true
		}
	}
	return false
}

// narrowRecords applies --filter and --search to an already-projected record
// set. SAFETY PROPERTY: narrowing runs strictly post-projection. The records
// here have already been allow-list projected and per-field redacted for the
// active redaction mode, so a dropped or secret-classified field is simply
// absent: a filter naming it matches no record, and search only ever sees the
// sanitized rendered values. Narrowing can reduce the record set but can never
// resurrect a field or widen exposure in any redaction mode.
func narrowRecords(records resources.ProjectedRecords, filters []recordFilter, search string) resources.ProjectedRecords {
	if len(filters) == 0 && search == "" {
		return records
	}
	kept := make([]resources.ProjectedRecord, 0)
	for _, record := range records.Records() {
		if recordMatches(record.Fields(), filters, search) {
			kept = append(kept, record)
		}
	}
	return resources.NewProjectedRecords(kept)
}

// recordMatches evaluates all narrowing conditions against one record's
// projected fields: every --filter must match (AND), and when --search is set
// at least one rendered field value must contain the term. Values are compared
// in their rendered string form (the same formatting the table renderer uses),
// so arrays and nested values participate via their rendered text.
func recordMatches(fields map[string]any, filters []recordFilter, search string) bool {
	for _, filter := range filters {
		value, ok := fields[filter.key]
		if !ok {
			// A record lacking the key does not match. This is also the
			// fail-closed path for secret/dropped field names, which never
			// appear in projected records.
			return false
		}
		rendered := formatTableValue(value)
		if filter.substring {
			if !strings.Contains(strings.ToLower(rendered), strings.ToLower(filter.value)) {
				return false
			}
			continue
		}
		if rendered != filter.value {
			return false
		}
	}
	if search == "" {
		return true
	}
	term := strings.ToLower(search)
	for _, value := range fields {
		if strings.Contains(strings.ToLower(formatTableValue(value)), term) {
			return true
		}
	}
	return false
}

func (a *App) writeProjectedRecords(
	cfg config.Config,
	opts globalOptions,
	spec resources.ResourceSpec,
	records resources.ProjectedRecords,
) error {
	fields, err := effectiveFields(spec, cfg.Defaults.Redaction, opts.fields)
	if err != nil {
		return err
	}
	warnUnknownFilterKeys(a.err, spec, opts.filters)
	// Narrow rows before --fields narrows columns, so a filter may reference
	// any projected field even when it is not selected for display. An empty
	// match is success: every format renders its empty form and exits 0.
	records = narrowRecords(records, opts.filters, opts.search)
	if len(opts.fields) > 0 {
		records = records.Select(fields)
	}
	switch opts.format {
	case output.FormatJSON:
		return a.renderer(cfg, opts).WriteJSON(a.out, records)
	case output.FormatNDJSON:
		return a.renderer(cfg, opts).WriteNDJSON(a.out, safeJSONRecords(records))
	case output.FormatTable:
		return a.renderer(cfg, opts).WriteText(a.out, renderRecordsTable(fields, records, a.style(opts)))
	case output.FormatPretty:
		return a.renderer(cfg, opts).WriteText(a.out, renderRecordsPretty(fields, records, a.style(opts)))
	default:
		return fmt.Errorf("unhandled output format %q for resource list", opts.format)
	}
}

func warnUnknownFilterKeys(w io.Writer, spec resources.ResourceSpec, filters []recordFilter) {
	if len(filters) == 0 {
		return
	}
	catalog := make(map[string]struct{}, len(spec.Fields))
	for _, field := range spec.Fields {
		catalog[field.JSONField()] = struct{}{}
	}
	warned := make(map[string]struct{}, len(filters))
	for _, filter := range filters {
		if _, ok := catalog[filter.key]; ok {
			continue
		}
		if _, ok := warned[filter.key]; ok {
			continue
		}
		warned[filter.key] = struct{}{}
		fmt.Fprintf(w, "warning: --filter key %q is not a field of %s/%s\n", filter.key, spec.Product, spec.Name)
	}
}

// safeJSONRecords adapts projected records to the output layer's SafeJSON slice
// for NDJSON rendering (one element per line). It preserves order; an empty set
// yields an empty slice, which WriteNDJSON renders as zero lines.
func safeJSONRecords(records resources.ProjectedRecords) []output.SafeJSON {
	recs := records.Records()
	out := make([]output.SafeJSON, len(recs))
	for i := range recs {
		out[i] = recs[i]
	}
	return out
}

// writeHelp prints help scoped to what the user asked for: a known resource's
// operations and renderable fields for `<product> <resource> --help`, the
// product's resources for `<product> --help`, or the global usage otherwise.
func (a *App) writeHelp(w io.Writer, rest []string) {
	if len(rest) >= 1 {
		switch rest[0] {
		case "doctor":
			fmt.Fprintln(w, "usage: zscalerctl doctor")
			return
		case "auth":
			fmt.Fprintln(w, "usage: zscalerctl auth status")
			return
		case "config":
			fmt.Fprintln(w, "usage: zscalerctl config show")
			fmt.Fprintln(w, "       zscalerctl config init [--force]")
			return
		case "schema":
			fmt.Fprintln(w, "usage: zscalerctl schema list")
			return
		case "dump":
			fmt.Fprintln(w, dumpUsage())
			return
		case "diff":
			fmt.Fprintln(w, diffUsage())
			return
		case "completion":
			fmt.Fprintf(w, "usage: zscalerctl completion %s\n", completionShellNames())
			return
		case "version":
			fmt.Fprintln(w, "usage: zscalerctl version")
			return
		}
	}
	if len(rest) >= 1 && knownProductCommand(rest[0]) {
		product := resources.Product(rest[0])
		if len(rest) >= 2 {
			if product == resources.ProductZIA && rest[1] == urlLookupCommandName {
				fmt.Fprintln(w, urlLookupUsageMessage)
				return
			}
			if spec, ok := a.resourceCatalog().FindSpec(product, rest[1]); ok {
				fmt.Fprintln(w, resourceUsage(product, spec, output.TerminalWidth(w)))
				return
			}
		}
		fmt.Fprintln(w, productCommandUsage(product, output.TerminalWidth(w)))
		return
	}
	a.writeUsage(w)
}

func (a *App) writeUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: zscalerctl [global flags] <command> [args]")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "products: %s\n", strings.Join(productNames(knownProducts()), ", "))
	fmt.Fprintln(w)
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  doctor")
	fmt.Fprintln(w, "  auth status")
	fmt.Fprintln(w, "  config show")
	fmt.Fprintln(w, "  config init [--force]")
	fmt.Fprintln(w, "  zia url-lookup <url> [url...]")
	fmt.Fprintln(w, "  schema list")
	fmt.Fprintln(w, "  dump --out <dir> [--products names] [--resources names] [--continue-on-error] [--force]")
	fmt.Fprintln(w, "  diff <old-dump-dir> <new-dump-dir> [--products names] [--resources names] [--ignore-operational] [--detail] [--allow-partial] [--fail-on-drift]")
	fmt.Fprintf(w, "  completion %s\n", completionShellNames())
	fmt.Fprintln(w, "  version")
	for _, product := range knownProducts() {
		fmt.Fprintf(w, "  %s <resource> %s\n", product, strings.Join(productReadOperationNames(product), "|"))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "global flags:")
	fmt.Fprintln(w, "  --profile <name>")
	fmt.Fprintln(w, "  --config <path>")
	fmt.Fprintln(w, "  --format auto|table|json|ndjson|pretty")
	fmt.Fprintln(w, "  --output <path>")
	fmt.Fprintln(w, "  --timeout <duration>")
	fmt.Fprintln(w, "  --redaction standard|share|paranoid")
	fmt.Fprintln(w, "  --color auto|always|never")
	fmt.Fprintln(w, "  --no-color")
	fmt.Fprintln(w, "  --no-cache")
	fmt.Fprintln(w, "  --log-level off|error|warn|info|debug")
	fmt.Fprintln(w, "  --fields <a,b,c>")
	fmt.Fprintln(w, "  --filter <key=value|key~value>   (list only; repeatable, all must match)")
	fmt.Fprintln(w, "  --search <term>                  (list only; case-insensitive, any field)")
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

func (a *App) renderer(cfg config.Config, _ globalOptions) output.Renderer {
	return output.NewRenderer(redact.New(cfg.Defaults.Redaction))
}

func renderRecordsTable(
	fields []string,
	records resources.ProjectedRecords,
	style output.Style,
) output.SafeText {
	var body strings.Builder
	for i, field := range fields {
		if i > 0 {
			body.WriteByte('\t')
		}
		body.WriteString(style.Key(field))
	}
	body.WriteByte('\n')
	for _, record := range records.Records() {
		values := record.Fields()
		for i, field := range fields {
			if i > 0 {
				body.WriteByte('\t')
			}
			body.WriteString(style.Value(field, formatTableValue(values[field])))
		}
		body.WriteByte('\n')
	}
	return output.NewSafeText(body.String())
}

func renderRecordKeyValues(
	fields []string,
	record resources.ProjectedRecord,
	style output.Style,
) output.SafeText {
	values := record.Fields()
	rows := make([]output.KV, 0, len(fields))
	for _, field := range fields {
		rows = append(rows, output.KV{
			Key:   field,
			Kind:  field,
			Value: formatTableValue(values[field]),
		})
	}
	return output.RenderKeyValues(rows, style)
}

func renderRecordsPretty(
	fields []string,
	records resources.ProjectedRecords,
	style output.Style,
) output.SafeText {
	rows := make([][]string, 0, len(records.Records()))
	for _, record := range records.Records() {
		values := record.Fields()
		row := make([]string, len(fields))
		for i, field := range fields {
			row[i] = formatTableValue(values[field])
		}
		rows = append(rows, row)
	}
	return output.RenderRecordsPretty(fields, rows, style)
}

func renderRecordPretty(
	fields []string,
	record resources.ProjectedRecord,
	style output.Style,
) output.SafeText {
	values := record.Fields()
	rows := make([]output.KV, 0, len(fields))
	for _, field := range fields {
		rows = append(rows, output.KV{
			Key:   field,
			Kind:  field,
			Value: formatTableValue(values[field]),
		})
	}
	return output.RenderRecordPretty(rows, style)
}

// formatTableValue renders a value for the text-based renderers (table, pretty,
// key-value). It sanitizes control characters so a value containing a newline or
// tab cannot break the tab-separated or border-delimited layout; the JSON
// renderer uses a separate path and keeps raw values.
func formatTableValue(value any) string {
	return sanitizeCellValue(rawTableValue(value))
}

func rawTableValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case []string:
		return strings.Join(v, ",")
	case []any:
		parts := make([]string, len(v))
		for i, item := range v {
			parts[i] = rawTableValue(item)
		}
		return strings.Join(parts, ",")
	default:
		return fmt.Sprint(v)
	}
}

// sanitizeCellValue collapses control characters (newline, tab, carriage return,
// and other C0/DEL bytes) to single spaces so multi-line or tabbed values render
// on one logical cell instead of corrupting row or column boundaries.
func sanitizeCellValue(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' || r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, s)
}

// resolveFormat collapses the auto format to a concrete one at the point where
// the destination is known: a real stdout TTY (and no --output file) gets the
// pretty human renderer, everything else (pipe, redirect, --output file) gets
// json so pipelines stay machine-parseable. Explicit --format choices pass
// through untouched.
func (a *App) resolveFormat(opts globalOptions) output.Format {
	if opts.format != output.FormatAuto {
		return opts.format
	}
	if a.stdoutTTY && opts.output == "" {
		return output.FormatPretty
	}
	return output.FormatJSON
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

func dumpUsage() string {
	return fmt.Sprintf(
		"usage: zscalerctl dump --out <dir> [--products %s] [--resources names] [--continue-on-error] [--force]\n"+
			"tip: add --log-level info to see start, per-resource, and completion progress on stderr during a long dump",
		strings.Join(productNames(knownProducts()), ","),
	)
}

func diffUsage() string {
	return fmt.Sprintf(
		"usage: zscalerctl diff <old-dump-dir> <new-dump-dir> [--products %s] [--resources names] [--ignore-operational] [--detail] [--allow-partial] [--fail-on-drift]",
		strings.Join(productNames(knownProducts()), ","),
	)
}

func knownProducts() []resources.Product {
	// Derive from the enabled catalog so help and command dispatch always reflect
	// the products that actually have resources, instead of a hardcoded list that
	// drifts as batches merge.
	seen := make(map[resources.Product]bool)
	var products []resources.Product
	for _, spec := range resources.Catalog() {
		if !seen[spec.Product] {
			seen[spec.Product] = true
			products = append(products, spec.Product)
		}
	}
	return products
}

func knownProductCommand(name string) bool {
	for _, product := range knownProducts() {
		if name == string(product) {
			return true
		}
	}
	return false
}

func isRunnableCommand(name string) bool {
	switch name {
	case "doctor", "auth", "config", "schema", "dump", "diff":
		return true
	default:
		return knownProductCommand(name)
	}
}

// isKnownCommand reports whether name is one of the top-level commands the
// dispatch switch in runParsed recognizes. The --fields guard uses it so that
// an unrecognized token — for example a product name a value-taking flag
// swallowed — still reaches the dispatch's more specific swallowed-product hint
// instead of the generic --fields usage error.
func isKnownCommand(name string) bool {
	switch name {
	case "help", "-h", "--help", "version", "completion":
		return true
	}
	return isRunnableCommand(name)
}

func productNames(products []resources.Product) []string {
	names := make([]string, len(products))
	for i, product := range products {
		names[i] = string(product)
	}
	return names
}

// columnize lays out names in a left-aligned, column-major grid (alphabetical
// down each column, like `ls`) indented two spaces, packed to fit width
// columns. width <= 0 falls back to 80, keeping error messages and
// non-terminal output deterministic. Returns the block without a trailing
// newline.
func columnize(names []string, width int) string {
	if len(names) == 0 {
		return ""
	}
	if width <= 0 {
		width = 80
	}
	const indent, gap = 2, 2
	longest := 0
	for _, n := range names {
		if len(n) > longest {
			longest = len(n)
		}
	}
	colWidth := longest + gap
	cols := (width - indent + gap) / colWidth
	if cols < 1 {
		cols = 1
	}
	rows := (len(names) + cols - 1) / cols
	var b strings.Builder
	for r := 0; r < rows; r++ {
		var line strings.Builder
		line.WriteString(strings.Repeat(" ", indent))
		for c := 0; c < cols; c++ {
			i := c*rows + r
			if i >= len(names) {
				break
			}
			line.WriteString(names[i])
			line.WriteString(strings.Repeat(" ", colWidth-len(names[i])))
		}
		b.WriteString(strings.TrimRight(line.String(), " "))
		if r < rows-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func productCommandUsage(product resources.Product, width int) string {
	// Enumerate the product's resources so a cold caller (human or agent) can
	// discover real names from --help or a usage error instead of guessing;
	// `schema list` remains the machine-readable source of truth.
	names := make([]string, 0, 64)
	for _, spec := range resources.Catalog() {
		if spec.Product == product {
			names = append(names, spec.Name)
		}
	}
	sort.Strings(names)
	msg := fmt.Sprintf(
		"usage: zscalerctl %s <resource> %s\n\nresources (%d; see also: zscalerctl --format json schema list):\n%s",
		product,
		strings.Join(productReadOperationNames(product), "|"),
		len(names),
		columnize(names, width),
	)
	if product == resources.ProductZIA {
		msg += "\n\ndiagnostics:\n  zscalerctl zia url-lookup <url> [url...]"
	}
	return msg
}

// resourceUsage builds help for a known resource: its supported read operations
// plus the renderable field names (standard mode), so the operator can discover
// what to pass to --fields without consulting `schema list`.
func resourceUsage(product resources.Product, spec resources.ResourceSpec, width int) string {
	msg := fmt.Sprintf(
		"usage: zscalerctl %s %s %s",
		product,
		spec.Name,
		strings.Join(readOperationNames(spec), "|"),
	)
	if fields := spec.FieldOrder(redact.ModeStandard); len(fields) > 0 {
		msg += "\nfields:\n" + columnize(fields, width)
	}
	return msg
}

func productReadOperationNames(product resources.Product) []string {
	seen := make(map[string]bool)
	for _, spec := range resources.Catalog() {
		if spec.Product != product {
			continue
		}
		for _, op := range spec.Operations {
			if op.Capability == resources.CapabilityRead {
				seen[op.Name] = true
			}
		}
	}

	var names []string
	for _, name := range []string{"list", "get", "show"} {
		if seen[name] {
			names = append(names, name)
		}
	}
	return names
}

func newDoctorStatus(cfg config.Config, opts globalOptions) doctorStatus {
	return doctorStatus{
		Status:      "OK",
		Mode:        "read-only",
		Profile:     cfg.Profile,
		Config:      configSourceStatus(cfg.Safe()),
		AuthMode:    string(cfg.EffectiveAuthMode()),
		Redaction:   string(cfg.Defaults.Redaction),
		Timeout:     opts.timeout.String(),
		Cache:       cacheStatus(cfg.Defaults.NoCache),
		Proxy:       proxyStatus(cfg.Proxy),
		Credentials: credentialStatus(cfg),
		LiveAPI:     liveAPIStatus(cfg),
	}
}

func doctorStatusRows(status doctorStatus) []output.KV {
	return []output.KV{
		{Key: "Status", Value: status.Status, Kind: "ok"},
		{Key: "Mode", Value: status.Mode, Kind: "mode"},
		{Key: "Profile", Value: status.Profile},
		{Key: "Config", Value: status.Config},
		{Key: "Auth Mode", Value: status.AuthMode},
		{Key: "Redaction", Value: status.Redaction},
		{Key: "Timeout", Value: status.Timeout},
		{Key: "Cache", Value: status.Cache},
		{Key: "Proxy", Value: status.Proxy},
		{Key: "Credentials", Value: status.Credentials},
		{Key: "Live API", Value: status.LiveAPI},
	}
}

func newAuthStatus(cfg config.Config) authStatus {
	return authStatus{
		Credentials:        credentialStatus(cfg),
		CredentialExchange: "not requested",
		LiveAPI:            liveAPIStatus(cfg),
	}
}

func authStatusRows(status authStatus) []output.KV {
	return []output.KV{
		{Key: "Credentials", Value: status.Credentials},
		{Key: "Credential Exchange", Value: status.CredentialExchange},
		{Key: "Live API", Value: status.LiveAPI},
	}
}

func credentialStatus(cfg config.Config) string {
	switch cfg.EffectiveAuthMode() {
	case config.AuthModeZIALegacy:
		if cfg.ZIALegacy.Configured() {
			return "configured"
		}
		if cfg.ZIALegacy.AnySet() {
			return "partial"
		}
		return "not configured"
	default:
		if cfg.Credentials.Configured(cfg.VanityDomain) {
			return "configured"
		}
		if cfg.Credentials.AnySet() || cfg.VanityDomain != "" {
			return "partial"
		}
		return "not configured"
	}
}

func liveAPIStatus(cfg config.Config) string {
	if credentialStatus(cfg) == "configured" {
		if cfg.EffectiveAuthMode() != config.AuthModeZIALegacy && strings.TrimSpace(cfg.ZPA.CustomerID) == "" {
			return "available for ZIA read-only commands; ZPA resources require ZSCALERCTL_ZPA_CUSTOMER_ID"
		}
		return "available for read-only commands"
	}
	if cfg.EffectiveAuthMode() == config.AuthModeZIALegacy {
		return "requires ZSCALERCTL_ZIA_USERNAME, ZSCALERCTL_ZIA_PASSWORD, ZSCALERCTL_ZIA_API_KEY, and ZSCALERCTL_ZIA_CLOUD"
	}
	return "requires ZSCALERCTL_CLIENT_ID, ZSCALERCTL_CLIENT_SECRET, and ZSCALERCTL_VANITY_DOMAIN; ZPA resources also require ZSCALERCTL_ZPA_CUSTOMER_ID"
}

func setStatus(set bool) string {
	if set {
		return "set"
	}
	return "unset"
}

func secretSourceStatus(set bool, scheme string) string {
	if !set {
		return "unset"
	}
	if scheme == "" {
		return "set"
	}
	return "set (" + scheme + ")"
}

func configSourceStatus(safe config.SafeConfig) string {
	if safe.ConfigFileSet {
		return "config file"
	}
	return "environment"
}

func cacheStatus(noCache bool) string {
	if noCache {
		return "bypass"
	}
	return "default"
}

func proxyStatus(proxy config.Proxy) string {
	switch {
	case proxy.FromEnvironment:
		return "environment"
	case strings.TrimSpace(proxy.URL) != "":
		return "explicit"
	default:
		return "direct"
	}
}

func valueOrUnset(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unset"
	}
	return value
}

func parseProducts(value string) (map[resources.Product]bool, error) {
	if strings.TrimSpace(value) == "" {
		products := map[resources.Product]bool{}
		for _, product := range knownProducts() {
			products[product] = true
		}
		return products, nil
	}
	products := map[resources.Product]bool{}
	for _, item := range strings.Split(value, ",") {
		product := resources.Product(strings.TrimSpace(strings.ToLower(item)))
		if knownProductCommand(string(product)) {
			products[product] = true
		} else {
			return nil, UsageError{Message: fmt.Sprintf("unsupported product %q", item)}
		}
	}
	return products, nil
}

type dumpResourceKey struct {
	product resources.Product
	name    string
}

func parseDumpResources(
	value string,
	products map[resources.Product]bool,
	catalog resources.ResourceCatalog,
) (map[dumpResourceKey]bool, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	selected := map[dumpResourceKey]bool{}
	for _, raw := range strings.Split(value, ",") {
		item := strings.TrimSpace(strings.ToLower(raw))
		if item == "" {
			return nil, UsageError{Message: "empty resource in --resources"}
		}
		keys, err := matchDumpResources(item, products, catalog)
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			selected[key] = true
		}
	}
	return selected, nil
}

func matchDumpResources(
	item string,
	products map[resources.Product]bool,
	catalog resources.ResourceCatalog,
) ([]dumpResourceKey, error) {
	if strings.Contains(item, "/") {
		parts := strings.Split(item, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, UsageError{Message: fmt.Sprintf("invalid resource %q", item)}
		}
		product := resources.Product(parts[0])
		if !catalogHasProduct(catalog, product) {
			return nil, UsageError{Message: fmt.Sprintf("unsupported product %q", parts[0])}
		}
		if !products[product] {
			return nil, UsageError{Message: fmt.Sprintf("resource %s is not selected by --products", item)}
		}
		key := dumpResourceKey{product: product, name: parts[1]}
		if !catalogHasDumpResource(catalog, key) {
			return nil, UsageError{Message: fmt.Sprintf("unsupported dump resource %s", item)}
		}
		return []dumpResourceKey{key}, nil
	}

	var matches []dumpResourceKey
	knownOutsideSelection := false
	for _, spec := range catalog {
		if spec.Name != item || !resourceSupportsDump(spec) {
			continue
		}
		if !products[spec.Product] {
			knownOutsideSelection = true
			continue
		}
		matches = append(matches, dumpResourceKey{product: spec.Product, name: spec.Name})
	}
	switch {
	case len(matches) == 1:
		return matches, nil
	case len(matches) > 1:
		return nil, UsageError{Message: fmt.Sprintf("ambiguous dump resource %q; use product/name", item)}
	case knownOutsideSelection:
		return nil, UsageError{Message: fmt.Sprintf("resource %s is not selected by --products", item)}
	default:
		return nil, UsageError{Message: fmt.Sprintf("unsupported dump resource %q", item)}
	}
}

func catalogHasDumpResource(catalog resources.ResourceCatalog, key dumpResourceKey) bool {
	for _, spec := range catalog {
		if spec.Product == key.product && spec.Name == key.name && resourceSupportsDump(spec) {
			return true
		}
	}
	return false
}

func catalogHasProduct(catalog resources.ResourceCatalog, product resources.Product) bool {
	for _, spec := range catalog {
		if spec.Product == product {
			return true
		}
	}
	return false
}

func resourceSupportsDump(spec resources.ResourceSpec) bool {
	return spec.SupportsReadOperation("list") || spec.SupportsReadOperation("show")
}

func readOperationNames(spec resources.ResourceSpec) []string {
	var names []string
	for _, op := range spec.Operations {
		if op.Capability == resources.CapabilityRead {
			names = append(names, op.Name)
		}
	}
	return names
}

func dumpResourceSelected(selected map[dumpResourceKey]bool, spec resources.ResourceSpec) bool {
	if selected == nil {
		return true
	}
	return selected[dumpResourceKey{product: spec.Product, name: spec.Name}]
}

func diffResourceSelection(selected map[dumpResourceKey]bool) map[dumpdiff.ResourceKey]bool {
	if selected == nil {
		return nil
	}
	out := make(map[dumpdiff.ResourceKey]bool, len(selected))
	for key := range selected {
		out[dumpdiff.ResourceKey{Product: key.product, Name: key.name}] = true
	}
	return out
}
