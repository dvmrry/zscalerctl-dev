package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/config"
	dumpdiff "github.com/dvmrry/zscalerctl/internal/diff"
	"github.com/dvmrry/zscalerctl/internal/dump"
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

// runDumpWithOptions executes the dump logic after flags have been parsed into d.
// All validation/collect/write/status/PartialDumpError behaviour is identical to
// the former inline runDump; only flag parsing has moved to the Cobra RunE.
func (a *App) runDumpWithOptions(ctx context.Context, cfg config.Config, opts globalOptions, d dumpOptions) error {
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
	s := a.newSpinner(opts)
	s.Start("dumping")
	// defer s.Stop() covers a panic inside collectDump: without it, the spinner
	// goroutine would stay live and keep writing to stderr after main.run
	// recovers the panic. The explicit s.Stop() below still runs on the normal
	// path (and is a no-op for the deferred call) to preserve Stop-before-render
	// ordering: the status notice that follows must not race with a live spinner.
	defer s.Stop()
	result, err := a.collectDump(ctx, cfg, opts, products, selectedResources, d.continueOnError,
		func(event machine.Event) error {
			if event.Kind == machine.EventProgress {
				s.Update(fmt.Sprintf("[%d/%d] %s/%s", event.Done, event.Total, event.Product, event.Resource))
			}
			return nil
		})
	s.Stop()
	if err != nil {
		return err
	}
	for _, re := range result.Errors {
		a.diagLogger().Warn("dump resource failed",
			"product", re.Product, "resource", re.Name, "operation", re.Operation, "kind", re.Kind)
	}
	a.diagLogger().Info("dump complete",
		"resources", len(result.Entries), "errors", len(result.Errors))
	if err := prepareForcedDumpDir(d.out, d.force); err != nil {
		return err
	}
	if err := dump.Write(d.out, cfg.Defaults.Redaction, result); err != nil {
		return err
	}
	// Dump emits no resource data on stdout (it writes files), so its status
	// notice is a diagnostic and goes to stderr, keeping stdout clean per the
	// stdout=data / stderr=diagnostics contract.
	if len(result.Errors) > 0 {
		if err := a.renderer(cfg, opts).WriteText(
			a.err,
			output.NewSafeText(fmt.Sprintf("partial dump written: %s (%d errors; see errors.ndjson)\n", d.out, len(result.Errors))),
		); err != nil {
			return err
		}
		return PartialDumpError{Dir: d.out, Errors: len(result.Errors)}
	}
	return a.renderer(cfg, opts).WriteText(a.err, output.NewSafeText(fmt.Sprintf("dump written: %s\n", d.out)))
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

func (a *App) dumpCollector(
	ctx context.Context,
	cfg config.Config,
	opts globalOptions,
) (*machineruntime.DumpCollector, error) {
	if a.reader != nil {
		return machineruntime.NewDumpCollectorFromReader(
			a.reader,
			a.resourceCatalog(),
			cfg.Defaults.Redaction,
		), nil
	}
	return machineruntime.NewDumpCollectorFromConfig(ctx, cfg, machineruntime.Options{
		Timeout:    opts.timeout,
		Catalog:    a.resourceCatalog(),
		DiagLogger: a.sdkDiagLogger(opts),
	})
}

func (a *App) collectDump(
	ctx context.Context,
	cfg config.Config,
	opts globalOptions,
	products map[resources.Product]bool,
	selectedResources map[dumpResourceKey]bool,
	continueOnError bool,
	sink machine.EventSink,
) (dump.Result, error) {
	catalog := a.resourceCatalog()
	selectedSpecs := selectedDumpSpecs(catalog, products, selectedResources)
	// A full dump can run for minutes; at info, operators get the selection
	// size up front and one progress event per resource below.
	a.diagLogger().Info("dump starting", "resources", len(selectedSpecs))

	collector, err := a.dumpCollector(ctx, cfg, opts)
	if err != nil {
		return dump.Result{}, err
	}
	return collector.CollectStream(ctx, selectedSpecs, machineruntime.DumpCollectOptions{
		ContinueOnError: continueOnError,
	}, func(event machine.Event) error {
		if sink != nil {
			if err := sink(event); err != nil {
				return err
			}
		}
		if event.Kind == machine.EventProgress {
			a.diagLogger().Info("dump reading resource", "product", event.Product, "resource", event.Resource)
		}
		return nil
	})
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

func dumpResourceSelected(selected map[dumpResourceKey]bool, spec resources.ResourceSpec) bool {
	if selected == nil {
		return true
	}
	return selected[dumpResourceKey{product: spec.Product, name: spec.Name}]
}

func selectedDumpSpecs(
	catalog resources.ResourceCatalog,
	products map[resources.Product]bool,
	selected map[dumpResourceKey]bool,
) []resources.ResourceSpec {
	specs := make([]resources.ResourceSpec, 0)
	for _, spec := range catalog {
		if !products[spec.Product] {
			continue
		}
		if !dumpResourceSelected(selected, spec) {
			continue
		}
		specs = append(specs, spec)
	}
	return specs
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
