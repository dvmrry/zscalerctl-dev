package runtime

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/config"
	dumpdiff "github.com/dvmrry/zscalerctl/internal/diff"
	dumpartifact "github.com/dvmrry/zscalerctl/internal/dump"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestEngineDiffExecutesAdvertisedCapabilityWithoutConfig(t *testing.T) {
	t.Parallel()

	spec := runtimeDiffSpec()
	oldDir := writeRuntimeDiffDump(t, spec, "old")
	newDir := writeRuntimeDiffDump(t, spec, "new")
	loadCalls := 0
	engine, err := NewEngine(Options{
		Catalog: resources.ResourceCatalog{spec},
		loadConfig: func([]string, config.LoadOptions) (config.Config, error) {
			loadCalls++
			return config.Config{}, errors.New("config must not be loaded")
		},
	})
	if err != nil {
		t.Fatalf("NewEngine() error = %v, want nil", err)
	}
	if !engineManifestHasCapability(engine.EngineManifest(), machine.CapabilityDiffCompare) {
		t.Fatalf("EngineManifest() = %#v, want %q", engine.EngineManifest(), machine.CapabilityDiffCompare)
	}

	var events []machine.Event
	req := machine.DiffRequest{
		OldDir:   oldDir,
		NewDir:   newDir,
		Products: []string{"zia"},
		Resources: []machine.DiffResourceSelector{{
			Product: "zia", Resource: "locations",
		}},
	}
	result, err := engine.Diff(context.Background(), req, func(event machine.Event) error {
		events = append(events, event)
		if event.Kind == machine.EventStarted {
			req.Products[0] = "mutated"
			req.Resources[0].Resource = "mutated"
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Engine.Diff() error = %v, want nil", err)
	}
	if loadCalls != 0 {
		t.Fatalf("Engine.Diff() config loads = %d, want 0", loadCalls)
	}
	if !result.HasDrift() {
		t.Fatal("Engine.Diff().HasDrift() = false, want true")
	}
	report := result.Report()
	if report.Schema != dumpdiff.SchemaID || report.Old.Path != oldDir || report.New.Path != newDir ||
		len(report.Resources) != 1 || len(report.Resources[0].Changed) != 1 {
		t.Fatalf("Engine.Diff().Report() = %#v, want one admitted changed resource", report)
	}
	assertRuntimeEventKinds(t, events, []machine.EventKind{
		machine.EventStarted, machine.EventProgress, machine.EventCompleted,
	})
	if events[0].Total != 1 || events[1].Done != 1 || events[1].Total != 1 ||
		events[1].Product != "zia" || events[1].Resource != "locations" ||
		events[2].Resources != 1 {
		t.Fatalf("Engine.Diff() events = %#v, want one-resource progress/completion", events)
	}
}

func TestEngineDiffProgressIncludesSelectedResourceEmptyOnBothSides(t *testing.T) {
	t.Parallel()

	spec := runtimeDiffSpec()
	oldDir := writeRuntimeEmptyDiffDump(t, spec)
	newDir := writeRuntimeEmptyDiffDump(t, spec)
	engine, err := NewEngine(Options{Catalog: resources.ResourceCatalog{spec}})
	if err != nil {
		t.Fatalf("NewEngine() error = %v, want nil", err)
	}
	var events []machine.Event
	result, err := engine.Diff(context.Background(), machine.DiffRequest{
		OldDir: oldDir, NewDir: newDir,
		Resources: []machine.DiffResourceSelector{{Product: "zia", Resource: "locations"}},
	}, func(event machine.Event) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("Engine.Diff(empty resources) error = %v", err)
	}
	report := result.Report()
	if report.Summary.ResourcesCompared != 0 || len(report.Resources) != 0 {
		t.Fatalf("Engine.Diff(empty resources) report = %#v, want no compared-resource entry", report)
	}
	assertRuntimeEventKinds(t, events, []machine.EventKind{
		machine.EventStarted, machine.EventProgress, machine.EventCompleted,
	})
	if events[0].Total != 1 || events[1].Done != 1 || events[1].Total != 1 || events[2].Resources != 1 {
		t.Fatalf("Engine.Diff(empty resources) events = %#v, want one selected-resource progress", events)
	}
}

func TestEngineDiffValidatesRequestBeforeFilesystemAccess(t *testing.T) {
	t.Parallel()

	spec := runtimeDiffSpec()
	engine, err := NewEngine(Options{Catalog: resources.ResourceCatalog{spec}})
	if err != nil {
		t.Fatalf("NewEngine() error = %v, want nil", err)
	}
	const pathCanary = "/private/diff-path-canary"
	var events []machine.Event
	_, err = engine.Diff(context.Background(), machine.DiffRequest{
		OldDir: pathCanary + "-old",
		NewDir: pathCanary + "-new",
		Resources: []machine.DiffResourceSelector{{
			Product: "zia", Resource: "unknown",
		}},
	}, func(event machine.Event) error {
		events = append(events, event)
		return nil
	})
	var machineErr *machine.MachineError
	if !errors.As(err, &machineErr) || machineErr.Kind != machine.ErrorKindUsage ||
		machineErr.Operation != machine.OperationDiff || strings.Contains(err.Error(), pathCanary) {
		t.Fatalf("Engine.Diff(invalid selector) error = %#v, want static usage/diff", err)
	}
	if errors.Is(err, dumpdiff.ErrInvalidDump) {
		t.Fatalf("Engine.Diff(invalid selector) error = %v, filesystem access won request validation", err)
	}
	assertRuntimeEventKinds(t, events, []machine.EventKind{machine.EventStarted, machine.EventFailed})
}

func TestEngineDiffRejectsInvalidCatalogBeforeFilesystemAccess(t *testing.T) {
	t.Parallel()

	invalid := runtimeDiffSpec()
	invalid.Fields[0].AllowedModes = nil
	engine, err := NewEngine(Options{Catalog: resources.ResourceCatalog{invalid}})
	if err != nil {
		t.Fatalf("NewEngine() error = %v, want deferred catalog validation", err)
	}
	if engineManifestHasCapability(engine.EngineManifest(), machine.CapabilityDiffCompare) {
		t.Fatalf("EngineManifest() = %#v, want no diff capability for invalid catalog", engine.EngineManifest())
	}

	const pathCanary = "/private/diff-invalid-catalog-path-canary"
	var events []machine.Event
	_, err = engine.Diff(context.Background(), machine.DiffRequest{
		OldDir: pathCanary + "-old",
		NewDir: pathCanary + "-new",
	}, func(event machine.Event) error {
		events = append(events, event)
		return nil
	})
	var machineErr *machine.MachineError
	if !errors.As(err, &machineErr) || machineErr.Kind != machine.ErrorKindInternal ||
		machineErr.Operation != machine.OperationDiff ||
		!errors.Is(err, resources.ErrInvalidResourceSpec) ||
		strings.Contains(err.Error(), pathCanary) {
		t.Fatalf("Engine.Diff(invalid catalog) error = %#v, want static internal/diff catalog error", err)
	}
	if errors.Is(err, dumpdiff.ErrInvalidDump) {
		t.Fatalf("Engine.Diff(invalid catalog) error = %v, filesystem access won catalog validation", err)
	}
	assertRuntimeEventKinds(t, events, []machine.EventKind{machine.EventStarted, machine.EventFailed})
}

func TestEngineDiffFinishedContextWinsBeforeFilesystemAccess(t *testing.T) {
	t.Parallel()

	engine, err := NewEngine(Options{
		Catalog: resources.ResourceCatalog{runtimeDiffSpec()},
	})
	if err != nil {
		t.Fatalf("NewEngine() error = %v, want nil", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var events []machine.Event
	_, err = engine.Diff(ctx, machine.DiffRequest{
		OldDir: "/private/diff-context-canary-old",
		NewDir: "/private/diff-context-canary-new",
	}, func(event machine.Event) error {
		events = append(events, event)
		return nil
	})
	var machineErr *machine.MachineError
	if !errors.As(err, &machineErr) || machineErr.Kind != machine.ErrorKindCanceled ||
		machineErr.Operation != machine.OperationDiff || !errors.Is(err, context.Canceled) ||
		strings.Contains(err.Error(), "canary") {
		t.Fatalf("Engine.Diff(pre-canceled) error = %#v, want static canceled/diff", err)
	}
	assertRuntimeEventKinds(t, events, []machine.EventKind{machine.EventStarted, machine.EventCanceled})

	ctx, cancel = context.WithCancel(context.Background())
	events = nil
	_, err = engine.Diff(ctx, machine.DiffRequest{
		OldDir: "/private/diff-started-context-canary-old",
		NewDir: "/private/diff-started-context-canary-new",
	}, func(event machine.Event) error {
		events = append(events, event)
		if event.Kind == machine.EventStarted {
			cancel()
		}
		return nil
	})
	if !errors.As(err, &machineErr) || machineErr.Kind != machine.ErrorKindCanceled ||
		machineErr.Operation != machine.OperationDiff || !errors.Is(err, context.Canceled) ||
		strings.Contains(err.Error(), "canary") {
		t.Fatalf("Engine.Diff(canceled by started sink) error = %#v, want static canceled/diff", err)
	}
	if errors.Is(err, dumpdiff.ErrInvalidDump) {
		t.Fatalf("Engine.Diff(canceled by started sink) error = %v, filesystem access won cancellation", err)
	}
	assertRuntimeEventKinds(t, events, []machine.EventKind{machine.EventStarted, machine.EventCanceled})
}

func TestEngineDiffSanitizesLocalInputErrorAndRetainsLegacyAdapterText(t *testing.T) {
	t.Parallel()

	engine, err := NewEngine(Options{
		Catalog: resources.ResourceCatalog{runtimeDiffSpec()},
	})
	if err != nil {
		t.Fatalf("NewEngine() error = %v, want nil", err)
	}
	oldDir := filepath.Join(t.TempDir(), "local-path-canary")
	newDir := filepath.Join(t.TempDir(), "new")
	var events []machine.Event
	_, err = engine.Diff(context.Background(), machine.DiffRequest{
		OldDir: oldDir,
		NewDir: newDir,
	}, func(event machine.Event) error {
		events = append(events, event)
		return nil
	})
	var machineErr *machine.MachineError
	if !errors.As(err, &machineErr) || machineErr.Kind != machine.ErrorKindUsage ||
		machineErr.Operation != machine.OperationDiff || !errors.Is(err, dumpdiff.ErrInvalidDump) {
		t.Fatalf("Engine.Diff(invalid dump) error = %#v, want usage/diff/ErrInvalidDump", err)
	}
	if strings.Contains(err.Error(), oldDir) || strings.Contains(err.Error(), "local-path-canary") {
		t.Fatalf("Engine.Diff(invalid dump) error = %q, want static path-free typed error", err)
	}
	adapterErr, ok := LegacyDiffAdapterError(err)
	if !ok || !errors.Is(adapterErr, dumpdiff.ErrInvalidDump) {
		t.Fatalf("LegacyDiffAdapterError() = (%v, %t), want ErrInvalidDump", adapterErr, ok)
	}
	if errors.As(adapterErr, &machineErr) {
		t.Fatalf("LegacyDiffAdapterError() = %#v, want no MachineError context", machineErr)
	}
	assertRuntimeEventKinds(t, events, []machine.EventKind{machine.EventStarted, machine.EventFailed})
}

func TestEngineDiffSinkFailureAbortsWithoutSecondTerminalEvent(t *testing.T) {
	t.Parallel()

	spec := runtimeDiffSpec()
	oldDir := writeRuntimeDiffDump(t, spec, "old")
	newDir := writeRuntimeDiffDump(t, spec, "new")
	engine, err := NewEngine(Options{Catalog: resources.ResourceCatalog{spec}})
	if err != nil {
		t.Fatalf("NewEngine() error = %v, want nil", err)
	}
	const sinkCanary = "diff-sink-canary-secret"
	var events []machine.Event
	_, err = engine.Diff(context.Background(), machine.DiffRequest{
		OldDir: oldDir,
		NewDir: newDir,
	}, func(event machine.Event) error {
		events = append(events, event)
		if event.Kind == machine.EventProgress {
			return errors.New(sinkCanary)
		}
		return nil
	})
	var machineErr *machine.MachineError
	if !errors.As(err, &machineErr) || machineErr.Kind != machine.ErrorKindInternal ||
		strings.Contains(err.Error(), sinkCanary) {
		t.Fatalf("Engine.Diff(sink failure) error = %#v, want static internal error", err)
	}
	assertRuntimeEventKinds(t, events, []machine.EventKind{
		machine.EventStarted, machine.EventProgress, machine.EventFailed,
	})
}

func writeRuntimeDiffDump(t *testing.T, spec resources.ResourceSpec, name string) string {
	t.Helper()

	projected, reports, err := resources.ProjectRecordsAndVerify(
		spec,
		redact.ModeStandard,
		[]resources.SourceRecord{resources.NewSourceRecord(map[string]any{
			"id":   "1",
			"name": name,
		})},
	)
	if err != nil {
		t.Fatalf("ProjectRecordsAndVerify(%s) error = %v", name, err)
	}
	dir := filepath.Join(t.TempDir(), "dump")
	if err := dumpartifact.Write(dir, redact.ModeStandard, dumpartifact.Result{
		Entries: []dumpartifact.ResourceDump{{
			Spec:    spec,
			Records: projected,
			Reports: reports,
		}},
	}); err != nil {
		t.Fatalf("dump.Write(%s) error = %v", name, err)
	}
	return dir
}

func writeRuntimeEmptyDiffDump(t *testing.T, spec resources.ResourceSpec) string {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "dump")
	if err := dumpartifact.Write(dir, redact.ModeStandard, dumpartifact.Result{
		Entries: []dumpartifact.ResourceDump{{
			Spec: spec, Records: resources.NewProjectedRecordsFromProjectedFields([]map[string]any{}),
		}},
	}); err != nil {
		t.Fatalf("dump.Write(empty) error = %v", err)
	}
	return dir
}

func runtimeDiffSpec() resources.ResourceSpec {
	spec := runtimeDumpListSpec(resources.ProductZIA, "locations")
	spec.Operations = resources.ReadOperations()
	return spec
}
