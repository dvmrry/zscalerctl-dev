package cli

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
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
}

func New(out, err io.Writer, env []string) *App {
	return NewWithOptions(out, err, env, Options{
		StdoutTTY: output.IsTerminal(out),
	})
}

type ResourceReader interface {
	List(context.Context, resources.Product, string) ([]resources.SourceRecord, error)
	Get(context.Context, resources.Product, string, string) (resources.SourceRecord, error)
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
	if opts.help {
		a.writeUsage(a.out)
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
		return UsageError{Message: fmt.Sprintf("unknown command %q", rest[0])}
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
		return UsageError{Message: fmt.Sprintf("unknown command %q", rest[0])}
	}
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
	format := fs.String("format", string(output.FormatTable), "output format: table, json")
	outputPath := fs.String("output", "", "output path")
	timeout := fs.Duration("timeout", 30*time.Second, "request timeout")
	redactionFlag := fs.String("redaction", "", "redaction mode: standard, share, paranoid")
	noCache := fs.Bool("no-cache", false, "bypass API cache where supported")
	colorFlag := fs.String("color", string(output.ColorAuto), "color output: auto, always, never")
	noColor := fs.Bool("no-color", false, "disable color output")
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
	return globalOptions{
		profile:      *profile,
		format:       parsedFormat,
		output:       *outputPath,
		timeout:      *timeout,
		redaction:    parsedRedaction,
		redactionSet: redactionSet,
		noCache:      *noCache,
		colorMode:    colorMode,
		help:         help,
	}, rest, nil
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
	case "profile", "format", "output", "timeout", "redaction", "no-cache", "color", "no-color":
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

func (a *App) runVersion(opts globalOptions, args []string) error {
	if err := requireNoArgs("version", args); err != nil {
		return err
	}
	info := version.Current()
	if opts.format == output.FormatJSON {
		return output.NewRenderer(redact.New(redact.ModeStandard)).WriteJSON(a.out, info)
	}
	if opts.format != output.FormatTable {
		return fmt.Errorf("version does not support %s output yet", opts.format)
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
	if opts.format != output.FormatTable {
		return fmt.Errorf("doctor does not support %s output yet", opts.format)
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
	if opts.format != output.FormatTable {
		return fmt.Errorf("auth status does not support %s output yet", opts.format)
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
	if opts.format != output.FormatTable {
		return fmt.Errorf("config show does not support %s output yet", opts.format)
	}
	safe := cfg.Safe()
	body := output.RenderKeyValues([]output.KV{
		{Key: "Profile", Value: safe.Profile},
		{Key: "Auth Mode", Value: safe.AuthMode},
		{Key: "Vanity Domain", Value: setStatus(safe.VanityDomainSet)},
		{Key: "Cloud", Value: valueOrUnset(safe.Cloud)},
		{Key: "Client ID", Value: setStatus(safe.Credentials.ClientIDSet)},
		{Key: "Client Secret", Value: setStatus(safe.Credentials.ClientSecretSet || safe.Credentials.ClientSecretFileSet)},
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
	if opts.format != output.FormatTable {
		return fmt.Errorf("schema list does not support %s output yet", opts.format)
	}
	if len(catalog) == 0 {
		return a.renderer(cfg, opts).WriteText(a.out, output.NewSafeText("no resources enabled yet\n"))
	}
	var body strings.Builder
	for _, spec := range catalog {
		fmt.Fprintf(&body, "%s\t%s\n", spec.Product, spec.Name)
	}
	return a.renderer(cfg, opts).WriteText(a.out, output.NewSafeText(body.String()))
}

func (a *App) runProduct(ctx context.Context, cfg config.Config, opts globalOptions, productName string, args []string) error {
	if len(args) < 2 {
		return UsageError{Message: fmt.Sprintf("usage: zscalerctl %s <resource> list|get", productName)}
	}
	product := resources.Product(productName)
	resource := args[0]
	op := args[1]
	if op == "list" && len(args) != 2 {
		return UsageError{Message: fmt.Sprintf("usage: zscalerctl %s <resource> list", productName)}
	}
	if op == "get" && len(args) != 3 {
		return UsageError{Message: fmt.Sprintf("usage: zscalerctl %s <resource> get <id>", productName)}
	}
	if op != "list" && op != "get" {
		return UsageError{Message: fmt.Sprintf("usage: zscalerctl %s <resource> list|get", productName)}
	}
	spec, ok := a.resourceCatalog().FindSpec(product, resource)
	if !ok {
		return ResourceNotFoundError{Product: product, Resource: resource}
	}
	if err := resources.AssertReadOnly(spec); err != nil {
		return err
	}
	if !spec.SupportsReadOperation(op) {
		return UsageError{Message: fmt.Sprintf("unsupported operation %s for %s/%s", op, product, resource)}
	}
	reader, err := a.resourceReader(cfg, opts)
	if err != nil {
		return err
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
		return a.writeProjectedRecord(cfg, opts, spec, projected)
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
	if err := dump.Write(*outDir, cfg.Defaults.Redaction, result); err != nil {
		return err
	}
	if len(result.Errors) > 0 {
		if err := a.renderer(cfg, opts).WriteText(
			a.out,
			output.NewSafeText(fmt.Sprintf("partial dump written: %s (%d errors; see errors.ndjson)\n", *outDir, len(result.Errors))),
		); err != nil {
			return err
		}
		return PartialDumpError{Dir: *outDir, Errors: len(result.Errors)}
	}
	return a.renderer(cfg, opts).WriteText(a.out, output.NewSafeText(fmt.Sprintf("dump written: %s\n", *outDir)))
}

func (a *App) resourceReader(cfg config.Config, opts globalOptions) (ResourceReader, error) {
	if a.reader != nil {
		return a.reader, nil
	}
	return zscaler.NewReader(zscaler.ReaderConfig{
		ClientID:     cfg.Credentials.ClientID,
		ClientSecret: cfg.Credentials.ClientSecret,
		VanityDomain: cfg.VanityDomain,
		Cloud:        cfg.Cloud,
		AuthMode:     zscaler.AuthMode(cfg.EffectiveAuthMode()),
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
		records, err := reader.List(ctx, spec.Product, spec.Name)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return result, ctxErr
			}
			if continueOnError {
				result.Errors = append(result.Errors, dump.NewResourceError(spec.Product, spec.Name, "list", "list_failed"))
				continue
			}
			return result, fmt.Errorf("dump %s/%s list failed", spec.Product, spec.Name)
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
			return result, fmt.Errorf("dump %s/%s %s failed", spec.Product, spec.Name, operation)
		}
		result.Entries = append(result.Entries, dump.ResourceDump{
			Spec:    spec,
			Records: projected,
			Reports: reports,
		})
	}
	return result, nil
}

func (a *App) writeProjectedRecord(
	cfg config.Config,
	opts globalOptions,
	spec resources.ResourceSpec,
	record resources.ProjectedRecord,
) error {
	switch opts.format {
	case output.FormatJSON:
		return a.renderer(cfg, opts).WriteJSON(a.out, record)
	case output.FormatTable:
		return a.renderer(cfg, opts).WriteText(a.out, renderRecordsTable(spec, cfg.Defaults.Redaction, resources.NewProjectedRecords([]resources.ProjectedRecord{record}), a.style(opts)))
	default:
		return fmt.Errorf("unhandled output format %q for resource get", opts.format)
	}
}

func (a *App) writeProjectedRecords(
	cfg config.Config,
	opts globalOptions,
	spec resources.ResourceSpec,
	records resources.ProjectedRecords,
) error {
	switch opts.format {
	case output.FormatJSON:
		return a.renderer(cfg, opts).WriteJSON(a.out, records)
	case output.FormatTable:
		return a.renderer(cfg, opts).WriteText(a.out, renderRecordsTable(spec, cfg.Defaults.Redaction, records, a.style(opts)))
	default:
		return fmt.Errorf("unhandled output format %q for resource list", opts.format)
	}
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
	fmt.Fprintln(w, "  dump --out <dir> [--resources names] [--continue-on-error]")
	fmt.Fprintln(w, "  completion bash|zsh|fish")
	fmt.Fprintln(w, "  version")
	for _, product := range knownProducts() {
		fmt.Fprintf(w, "  %s <resource> list|get|show\n", product)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "global flags:")
	fmt.Fprintln(w, "  --profile <name>")
	fmt.Fprintln(w, "  --format table|json")
	fmt.Fprintln(w, "  --output <path>")
	fmt.Fprintln(w, "  --timeout <duration>")
	fmt.Fprintln(w, "  --redaction standard|share|paranoid")
	fmt.Fprintln(w, "  --color auto|always|never")
	fmt.Fprintln(w, "  --no-color")
	fmt.Fprintln(w, "  --no-cache")
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
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("write --output: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(body); err != nil {
		return fmt.Errorf("write --output: %w", err)
	}
	return nil
}

func (a *App) renderer(cfg config.Config, _ globalOptions) output.Renderer {
	return output.NewRenderer(redact.New(cfg.Defaults.Redaction))
}

func renderRecordsTable(
	spec resources.ResourceSpec,
	mode redact.Mode,
	records resources.ProjectedRecords,
	style output.Style,
) output.SafeText {
	fields := spec.FieldOrder(mode)
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

func formatTableValue(value any) string {
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
			parts[i] = formatTableValue(item)
		}
		return strings.Join(parts, ",")
	default:
		return fmt.Sprint(v)
	}
}

func (a *App) style(opts globalOptions) output.Style {
	stdoutTTY := a.stdoutTTY && opts.output == ""
	color := output.ShouldColor(opts.colorMode, a.env, stdoutTTY)
	return output.NewStyle(color, output.Supports256Color(a.env))
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
		return "available for read-only commands"
	}
	if cfg.EffectiveAuthMode() == config.AuthModeZIALegacy {
		return "requires ZSCALERCTL_ZIA_USERNAME, ZSCALERCTL_ZIA_PASSWORD, ZSCALERCTL_ZIA_API_KEY, and ZSCALERCTL_ZIA_CLOUD"
	}
	return "requires ZSCALERCTL_CLIENT_ID, ZSCALERCTL_CLIENT_SECRET, and ZSCALERCTL_VANITY_DOMAIN"
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
	for _, op := range spec.Operations {
		if op.Name == "list" && op.Capability == resources.CapabilityRead {
			return true
		}
	}
	return false
}

func dumpResourceSelected(selected map[dumpResourceKey]bool, spec resources.ResourceSpec) bool {
	if selected == nil {
		return true
	}
	return selected[dumpResourceKey{product: spec.Product, name: spec.Name}]
}
