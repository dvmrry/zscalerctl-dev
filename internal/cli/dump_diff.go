package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/config"
	dumpdiff "github.com/dvmrry/zscalerctl/internal/diff"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	machineruntime "github.com/dvmrry/zscalerctl/internal/runtime"
)

// dumpOptions holds the parsed local flags for the dump command. The Cobra RunE
// path populates it from cmd.Flags().
type dumpOptions struct {
	out             string
	products        string
	resources       string
	continueOnError bool
	force           bool
}

// runDumpWithOptions resolves CLI shorthand, adapts the typed dump operation,
// and retains status rendering and partial-dump exit policy at the CLI layer.
func (a *App) runDumpWithOptions(ctx context.Context, opts globalOptions, d dumpOptions) error {
	if d.out == "" {
		return UsageError{Message: dumpUsage(a.resourceCatalog())}
	}
	products, err := parseProducts(d.products, a.resourceCatalog())
	if err != nil {
		return err
	}
	selectedResources, err := parseDumpResources(d.resources, products, a.resourceCatalog())
	if err != nil {
		return err
	}
	req := newDumpRequest(d, products, selectedResources, a.resourceCatalog())
	s := a.newSpinner(opts)
	s.Start("dumping")
	// defer s.Stop() covers a panic inside the typed dump operation: without it,
	// the spinner goroutine would stay live and keep writing to stderr after main.run
	// recovers the panic. The explicit s.Stop() below still runs on the normal
	// path (and is a no-op for the deferred call) to preserve Stop-before-render
	// ordering: the status notice that follows must not race with a live spinner.
	defer s.Stop()
	result, err := a.executeDump(ctx, opts, req,
		func(event machine.Event) error {
			if event.Kind == machine.EventProgress {
				s.Update(fmt.Sprintf("[%d/%d] %s/%s", event.Done, event.Total, event.Product, event.Resource))
			}
			return nil
		})
	s.Stop()
	if err != nil {
		return cliErrorFromDump(err, a.resourceCatalog())
	}
	for _, re := range result.Errors() {
		a.diagLogger().Warn("dump resource failed",
			"product", re.Product, "resource", re.Resource, "operation", re.Operation, "kind", re.Kind)
	}
	a.diagLogger().Info("dump complete",
		"resources", result.Resources(), "errors", result.Warnings())
	renderer := output.NewRenderer(redact.New(redact.Mode(result.Redaction())))
	// Dump emits no resource data on stdout (it writes files), so its status
	// notice is a diagnostic and goes to stderr, keeping stdout clean per the
	// stdout=data / stderr=diagnostics contract.
	if result.Partial() {
		if err := renderer.WriteText(
			a.err,
			output.NewSafeText(fmt.Sprintf("partial dump written: %s (%d errors; see errors.ndjson)\n", d.out, result.Warnings())),
		); err != nil {
			return err
		}
		return PartialDumpError{Dir: d.out, Errors: result.Warnings()}
	}
	return renderer.WriteText(a.err, output.NewSafeText(fmt.Sprintf("dump written: %s\n", d.out)))
}

func newDumpRequest(
	d dumpOptions,
	products map[resources.Product]bool,
	selected map[dumpResourceKey]bool,
	catalog resources.ResourceCatalog,
) machine.DumpRequest {
	req := machine.DumpRequest{
		OutputDir:       d.out,
		ContinueOnError: d.continueOnError,
		Force:           d.force,
	}
	for _, product := range knownProducts(catalog) {
		if products[product] {
			req.Products = append(req.Products, string(product))
		}
	}
	if selected != nil {
		for _, spec := range catalog {
			key := dumpResourceKey{product: spec.Product, name: spec.Name}
			if selected[key] {
				req.Resources = append(req.Resources, machine.DumpResourceSelector{
					Product:  string(spec.Product),
					Resource: spec.Name,
				})
			}
		}
	}
	return req
}

func (a *App) executeDump(
	ctx context.Context,
	opts globalOptions,
	req machine.DumpRequest,
	observer machine.EventSink,
) (machine.DumpResult, error) {
	sink := func(event machine.Event) error {
		if observer != nil {
			if err := observer(event); err != nil {
				return err
			}
		}
		switch event.Kind {
		case machine.EventStarted:
			a.diagLogger().Info("dump starting", "resources", event.Total)
		case machine.EventProgress:
			a.diagLogger().Info("dump reading resource", "product", event.Product, "resource", event.Resource)
		}
		return nil
	}

	if a.reader != nil {
		cfg, err := config.LoadConfig(a.env, config.LoadOptions{
			Profile:    opts.profile,
			ConfigPath: opts.configPath,
		})
		if err != nil {
			return machine.DumpResult{}, err
		}
		applyOptions(&cfg, opts)
		collector := machineruntime.NewDumpCollectorFromReader(
			a.reader,
			a.resourceCatalog(),
			cfg.Defaults.Redaction,
		)
		return collector.Dump(ctx, req, sink)
	}

	engine, err := machineruntime.NewEngine(machineruntime.Options{
		Env:          a.env,
		Profile:      opts.profile,
		ConfigPath:   opts.configPath,
		Timeout:      opts.timeout,
		Redaction:    opts.redaction,
		RedactionSet: opts.redactionSet,
		NoCache:      opts.noCache,
		Catalog:      a.resourceCatalog(),
		DiagLogger:   a.sdkDiagLogger(opts),
	})
	if err != nil {
		return machine.DumpResult{}, err
	}
	return engine.Dump(ctx, req, sink)
}

func cliErrorFromDump(err error, catalog resources.ResourceCatalog) error {
	if err == nil {
		return nil
	}
	if adapterErr, ok := machineruntime.LegacyDumpAdapterError(err); ok {
		return adapterErr
	}
	machineErr, ok := asMachineError(err)
	if ok && machineErr.Kind == machine.ErrorKindUsage {
		return UsageError{Message: dumpUsage(catalog)}
	}
	if ok && machineErr.Kind == machine.ErrorKindInternal &&
		machineErr.Operation == machine.OperationDump && machineErr.Message == "dump output failed" {
		return errors.New("dump output failed")
	}
	return err
}

// diffOptions holds the parsed local flags for the diff command.
// The struct is populated by the Cobra RunE path reading cmd.Flags().
type diffOptions struct {
	products          string
	resources         string
	ignoreOperational bool
	detail            bool
	allowPartial      bool
	failOnDrift       bool
}

// runDiffWithOptions executes the diff logic after flags have been parsed into d.
// All Compare/error-mapping/ModeStandard render/DriftDetectedError behaviour is
// identical to the former inline runDiff; only flag parsing has moved to the
// Cobra RunE. Config-FREE: diff compares two local dump dirs and never needs
// LoadConfig.
func (a *App) runDiffWithOptions(opts globalOptions, d diffOptions, oldDir, newDir string) error {
	catalog := a.resourceCatalog()
	products, err := parseProducts(d.products, catalog)
	if err != nil {
		return err
	}
	selectedResources, err := parseDumpResources(d.resources, products, catalog)
	if err != nil {
		return err
	}
	report, err := dumpdiff.Compare(oldDir, newDir, dumpdiff.Options{
		Catalog:           catalog,
		Products:          products,
		Resources:         diffResourceSelection(selectedResources),
		IgnoreOperational: d.ignoreOperational,
		AllowPartial:      d.allowPartial,
	})
	if err != nil {
		if errors.Is(err, dumpdiff.ErrInvalidDump) ||
			errors.Is(err, dumpdiff.ErrPartialDumpInput) ||
			errors.Is(err, dumpdiff.ErrRedactionMismatch) {
			return UsageError{Message: err.Error()}
		}
		return err
	}
	// ModeStandard is always used for diff — independent of any configured
	// redaction mode (diff compares local dump dirs, not live API data).
	renderer := output.NewRenderer(redact.New(redact.ModeStandard))
	switch opts.format {
	case output.FormatJSON:
		if err := renderer.WriteJSON(a.out, report); err != nil {
			return err
		}
	case output.FormatTable, output.FormatPretty:
		if err := renderer.WriteText(a.out, renderDiffTable(report, d.detail, a.style(opts))); err != nil {
			return err
		}
	default:
		return rejectUnsupportedFormat("diff", opts.format)
	}
	if d.failOnDrift && report.HasDrift() {
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
	resourceName = terminalCell(resourceName)
	for _, added := range resource.Added {
		fmt.Fprintf(body, "%s\t+\t%s\t-\t-\n", resourceName, terminalCell(diffRecordRefLabel(added)))
	}
	for _, removed := range resource.Removed {
		fmt.Fprintf(body, "%s\t-\t%s\t-\t-\n", resourceName, terminalCell(diffRecordRefLabel(removed)))
	}
	for _, changed := range resource.Changed {
		fmt.Fprintf(body, "%s\t~\t%s\t%s\t-\n", resourceName, terminalCell(changed.Key), diffFieldNames(changed.Changes))
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
		fields[i] = terminalCell(change.Field)
	}
	return strings.Join(fields, ",")
}

func terminalCell(value string) string {
	var out strings.Builder
	for _, r := range value {
		switch {
		case r == '\n':
			out.WriteString(`\n`)
		case r == '\r':
			out.WriteString(`\r`)
		case r == '\t':
			out.WriteString(`\t`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&out, `\x%02x`, r)
		case r >= 0x80 && r <= 0x9f:
			fmt.Fprintf(&out, `\u%04x`, r)
		case isBidiControl(r):
			fmt.Fprintf(&out, `\u%04x`, r)
		default:
			out.WriteRune(r)
		}
	}
	return out.String()
}

func isBidiControl(r rune) bool {
	return (r >= 0x202a && r <= 0x202e) || (r >= 0x2066 && r <= 0x2069)
}

func parseProducts(value string, catalog resources.ResourceCatalog) (map[resources.Product]bool, error) {
	if strings.TrimSpace(value) == "" {
		products := map[resources.Product]bool{}
		for _, product := range knownProducts(catalog) {
			products[product] = true
		}
		return products, nil
	}
	products := map[resources.Product]bool{}
	for _, item := range strings.Split(value, ",") {
		product := resources.Product(strings.TrimSpace(strings.ToLower(item)))
		if knownProductCommand(string(product), catalog) {
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
