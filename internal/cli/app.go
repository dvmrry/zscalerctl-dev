package cli

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/dump"
	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/dvmrry/zscalerctl/internal/version"
	"github.com/dvmrry/zscalerctl/internal/zscaler"
)

var ErrUsage = errors.New("usage error")
var ErrPartialDump = errors.New("partial dump")
var ErrNotFound = errors.New("not found")

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

type ResourceNotFoundError struct {
	Product  resources.Product
	Resource string
}

func (e ResourceNotFoundError) Error() string {
	return fmt.Sprintf("unsupported resource %s/%s", e.Product, e.Resource)
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
	catalog := append(resources.ResourceCatalog(nil), opts.Catalog...)
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
	if len(a.catalog) > 0 {
		return append(resources.ResourceCatalog(nil), a.catalog...)
	}
	return resources.Catalog()
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
		if err != nil {
			return err
		}
		return writeOutputFile(opts.output, buffered.Bytes())
	}
	return a.runParsed(ctx, opts, rest)
}

func (a *App) runParsed(ctx context.Context, opts globalOptions, rest []string) error {
	if logger, err := newDiagLogger(a.err, opts.logLevel); err == nil {
		a.logger = logger
	}
	if opts.help {
		a.writeHelp(a.out, rest)
		return nil
	}
	if len(rest) == 0 {
		a.writeUsage(a.err)
		return UsageError{Message: "missing command"}
	}
	switch {
	case rest[0] == "help" || rest[0] == "-h" || rest[0] == "--help":
		a.writeUsage(a.out)
		return nil
	case rest[0] == "version":
		return a.runVersion(opts, rest[1:])
	case rest[0] == "completion":
		return a.runCompletion(rest[1:])
	case isRunnableCommand(rest[0]):
	default:
		a.writeUsage(a.err)
		return UsageError{Message: unknownCommandMessage(rest[0])}
	}

	cfg, err := config.LoadEnv(a.env)
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
		return a.runDump(ctx, cfg, opts, rest[1:])
	default:
		if knownProductCommand(rest[0]) {
			return a.runProduct(ctx, cfg, opts, rest[0], rest[1:])
		}
		a.writeUsage(a.err)
		return UsageError{Message: unknownCommandMessage(rest[0])}
	}
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
	format       output.Format
	output       string
	timeout      time.Duration
	redaction    redact.Mode
	redactionSet bool
	noCache      bool
	colorMode    output.ColorMode
	logLevel     string
	fields       []string
	help         bool
}

type doctorStatus struct {
	Status      string `json:"status"`
	Mode        string `json:"mode"`
	Profile     string `json:"profile"`
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
	profile := fs.String("profile", "", "profile name")
	format := fs.String("format", string(output.FormatAuto), "output format: auto, table, json, pretty")
	outputPath := fs.String("output", "", "output path")
	timeout := fs.Duration("timeout", 30*time.Second, "request timeout")
	redactionFlag := fs.String("redaction", "", "redaction mode: standard, share, paranoid")
	noCache := fs.Bool("no-cache", false, "bypass API cache where supported")
	colorFlag := fs.String("color", string(output.ColorAuto), "color output: auto, always, never")
	noColor := fs.Bool("no-color", false, "disable color output")
	logLevel := fs.String("log-level", "off", "diagnostic logging to stderr: off, error, warn, info, debug")
	fieldsFlag := fs.String("fields", "", "comma-separated output fields to keep (narrows the sanitized output)")
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
	return globalOptions{
		profile:      *profile,
		format:       parsedFormat,
		output:       *outputPath,
		timeout:      *timeout,
		redaction:    parsedRedaction,
		redactionSet: redactionSet,
		noCache:      *noCache,
		colorMode:    colorMode,
		logLevel:     *logLevel,
		fields:       parseFieldsList(*fieldsFlag),
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
	switch name {
	case "profile", "format", "output", "timeout", "redaction", "no-cache", "color", "no-color", "log-level", "fields":
		return true
	default:
		return false
	}
}

func isGlobalBoolFlag(name string) bool {
	return name == "no-cache" || name == "no-color"
}

func applyOptions(cfg *config.Config, opts globalOptions) {
	if opts.profile != "" {
		cfg.Profile = opts.profile
	}
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
	return fmt.Errorf("%s does not support %s output yet", command, format)
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
		{Key: "Auth Mode", Value: safe.AuthMode},
		{Key: "Vanity Domain", Value: setStatus(safe.VanityDomainSet)},
		{Key: "Cloud", Value: valueOrUnset(safe.Cloud)},
		{Key: "Client ID", Value: setStatus(safe.Credentials.ClientIDSet)},
		{Key: "Client Secret", Value: setStatus(safe.Credentials.ClientSecretSet || safe.Credentials.ClientSecretFileSet)},
		{Key: "ZPA Customer ID", Value: setStatus(safe.ZPA.CustomerIDSet)},
		{Key: "ZPA Microtenant ID", Value: setStatus(safe.ZPA.MicrotenantIDSet)},
		{Key: "ZIA Username", Value: setStatus(safe.ZIALegacy.UsernameSet)},
		{Key: "ZIA Password", Value: setStatus(safe.ZIALegacy.PasswordSet || safe.ZIALegacy.PasswordFileSet)},
		{Key: "ZIA API Key", Value: setStatus(safe.ZIALegacy.APIKeySet || safe.ZIALegacy.APIKeyFileSet)},
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
	// When the resource is recognized, prefer help that lists its actual
	// operations and renderable fields over the generic per-product usage.
	helpSpec, helpSpecOK := a.resourceCatalog().FindSpec(product, resource)
	usage := func() string {
		if helpSpecOK {
			return resourceUsage(product, helpSpec)
		}
		return productCommandUsage(product)
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
		return UsageError{Message: fmt.Sprintf("unsupported operation %s for %s/%s\n%s", op, product, resource, resourceUsage(product, spec))}
	}
	reader, err := a.resourceReader(cfg, opts)
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

func (a *App) resourceReader(cfg config.Config, opts globalOptions) (ResourceReader, error) {
	if a.reader != nil {
		return a.reader, nil
	}
	return zscaler.NewReader(zscaler.ReaderConfig{
		ClientID:         cfg.Credentials.ClientID,
		ClientSecret:     cfg.Credentials.ClientSecret,
		VanityDomain:     cfg.VanityDomain,
		Cloud:            cfg.Cloud,
		ZPACustomerID:    cfg.ZPA.CustomerID,
		ZPAMicrotenantID: cfg.ZPA.MicrotenantID,
		AuthMode:         zscaler.AuthMode(cfg.EffectiveAuthMode()),
		ZIALegacy: zscaler.ZIALegacyConfig{
			Username: cfg.ZIALegacy.Username,
			Password: cfg.ZIALegacy.Password,
			APIKey:   cfg.ZIALegacy.APIKey,
			Cloud:    cfg.ZIALegacy.Cloud,
		},
		Timeout: opts.timeout,
		NoCache: cfg.Defaults.NoCache,
		Proxy: zscaler.ProxyConfig{
			URL:             cfg.Proxy.URL,
			FromEnvironment: cfg.Proxy.FromEnvironment,
		},
	})
}

func (a *App) dumpResourceReader(
	ctx context.Context,
	cfg config.Config,
	opts globalOptions,
	product resources.Product,
) (ResourceReader, func(), error) {
	reader, err := a.resourceReader(cfg, opts)
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
		a.diagLogger().Debug("dump reading resource", "product", spec.Product, "resource", spec.Name)
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
	if len(opts.fields) > 0 {
		records = records.Select(fields)
	}
	switch opts.format {
	case output.FormatJSON:
		return a.renderer(cfg, opts).WriteJSON(a.out, records)
	case output.FormatTable:
		return a.renderer(cfg, opts).WriteText(a.out, renderRecordsTable(fields, records, a.style(opts)))
	case output.FormatPretty:
		return a.renderer(cfg, opts).WriteText(a.out, renderRecordsPretty(fields, records, a.style(opts)))
	default:
		return fmt.Errorf("unhandled output format %q for resource list", opts.format)
	}
}

// writeHelp prints help scoped to what the user asked for: a known resource's
// operations and renderable fields for `<product> <resource> --help`, the
// product's resources for `<product> --help`, or the global usage otherwise.
func (a *App) writeHelp(w io.Writer, rest []string) {
	if len(rest) >= 1 && knownProductCommand(rest[0]) {
		product := resources.Product(rest[0])
		if len(rest) >= 2 {
			if spec, ok := a.resourceCatalog().FindSpec(product, rest[1]); ok {
				fmt.Fprintln(w, resourceUsage(product, spec))
				return
			}
		}
		fmt.Fprintln(w, productCommandUsage(product))
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
	fmt.Fprintln(w, "  schema list")
	fmt.Fprintln(w, "  dump --out <dir> [--products names] [--resources names] [--continue-on-error]")
	fmt.Fprintln(w, "  completion bash|zsh|fish")
	fmt.Fprintln(w, "  version")
	for _, product := range knownProducts() {
		fmt.Fprintf(w, "  %s <resource> %s\n", product, strings.Join(productReadOperationNames(product), "|"))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "global flags:")
	fmt.Fprintln(w, "  --profile <name>")
	fmt.Fprintln(w, "  --format auto|table|json|pretty")
	fmt.Fprintln(w, "  --output <path>")
	fmt.Fprintln(w, "  --timeout <duration>")
	fmt.Fprintln(w, "  --redaction standard|share|paranoid")
	fmt.Fprintln(w, "  --color auto|always|never")
	fmt.Fprintln(w, "  --no-color")
	fmt.Fprintln(w, "  --no-cache")
	fmt.Fprintln(w, "  --log-level off|error|warn|info|debug")
	fmt.Fprintln(w, "  --fields <a,b,c>")
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
		return fmt.Errorf("write --output: %w", err)
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
		"usage: zscalerctl dump --out <dir> [--products %s] [--resources names] [--continue-on-error]",
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
	case "doctor", "auth", "config", "schema", "dump":
		return true
	default:
		return knownProductCommand(name)
	}
}

func productNames(products []resources.Product) []string {
	names := make([]string, len(products))
	for i, product := range products {
		names[i] = string(product)
	}
	return names
}

func productCommandUsage(product resources.Product) string {
	return fmt.Sprintf(
		"usage: zscalerctl %s <resource> %s",
		product,
		strings.Join(productReadOperationNames(product), "|"),
	)
}

// resourceUsage builds help for a known resource: its supported read operations
// plus the renderable field names (standard mode), so the operator can discover
// what to pass to --fields without consulting `schema list`.
func resourceUsage(product resources.Product, spec resources.ResourceSpec) string {
	msg := fmt.Sprintf(
		"usage: zscalerctl %s %s %s",
		product,
		spec.Name,
		strings.Join(readOperationNames(spec), "|"),
	)
	if fields := spec.FieldOrder(redact.ModeStandard); len(fields) > 0 {
		msg += "\nfields: " + strings.Join(fields, ", ")
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
		{Key: "Token", Value: status.CredentialExchange},
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
