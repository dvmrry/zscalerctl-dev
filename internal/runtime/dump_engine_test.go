package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/browser"
	"github.com/dvmrry/zscalerctl/internal/config"
	dumpartifact "github.com/dvmrry/zscalerctl/internal/dump"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/dvmrry/zscalerctl/internal/zscaler"
)

func TestEngineDumpValidatesBeforeConfig(t *testing.T) {
	t.Parallel()

	catalog := resources.ResourceCatalog{runtimeDumpListSpec(resources.ProductZIA, "locations")}
	configLoads := 0
	engine, err := NewEngine(Options{
		Catalog: catalog,
		loadConfig: func([]string, config.LoadOptions) (config.Config, error) {
			configLoads++
			return config.Config{}, errors.New("config loader must not run")
		},
	})
	if err != nil {
		t.Fatalf("NewEngine() error = %v, want nil", err)
	}

	tests := []machine.DumpRequest{
		{},
		{OutputDir: "   "},
		{OutputDir: t.TempDir(), Products: []string{"unknown-product-canary"}},
		{
			OutputDir: t.TempDir(),
			Products:  []string{"zia"},
			Resources: []machine.DumpResourceSelector{{Product: "zia", Resource: "unknown-resource-canary"}},
		},
		{OutputDir: t.TempDir(), Products: []string{"zia", "zia"}},
	}
	for _, req := range tests {
		var events []machine.Event
		_, err := engine.Dump(context.Background(), req, func(event machine.Event) error {
			events = append(events, event)
			return nil
		})
		var machineErr *machine.MachineError
		if !errors.As(err, &machineErr) || machineErr.Kind != machine.ErrorKindUsage {
			t.Fatalf("Engine.Dump(%#v) error = %v, want usage MachineError", req, err)
		}
		if configLoads != 0 {
			t.Fatalf("Engine.Dump(%#v) config loads = %d, want 0", req, configLoads)
		}
		assertRuntimeEventKinds(t, events, []machine.EventKind{machine.EventStarted, machine.EventFailed})
		for _, canary := range []string{"unknown-product-canary", "unknown-resource-canary"} {
			if strings.Contains(err.Error(), canary) {
				t.Fatalf("Engine.Dump(%#v) error = %q, want no selector canary", req, err)
			}
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var events []machine.Event
	_, err = engine.Dump(ctx, machine.DumpRequest{OutputDir: t.TempDir()}, func(event machine.Event) error {
		events = append(events, event)
		return nil
	})
	var machineErr *machine.MachineError
	if !errors.As(err, &machineErr) || machineErr.Kind != machine.ErrorKindCanceled ||
		!errors.Is(err, context.Canceled) {
		t.Fatalf("Engine.Dump(canceled) error = %v, want canceled MachineError", err)
	}
	if configLoads != 0 {
		t.Fatalf("Engine.Dump(canceled) config loads = %d, want 0", configLoads)
	}
	assertRuntimeEventKinds(t, events, []machine.EventKind{machine.EventStarted, machine.EventCanceled})

	ctx, cancel = context.WithCancel(context.Background())
	events = nil
	_, err = engine.Dump(ctx, machine.DumpRequest{OutputDir: filepath.Join(t.TempDir(), "dump")}, func(event machine.Event) error {
		events = append(events, event)
		if event.Kind == machine.EventStarted {
			cancel()
		}
		return nil
	})
	if !errors.As(err, &machineErr) || machineErr.Kind != machine.ErrorKindCanceled ||
		!errors.Is(err, context.Canceled) {
		t.Fatalf("Engine.Dump(canceled by started sink) error = %v, want canceled MachineError", err)
	}
	if configLoads != 0 {
		t.Fatalf("Engine.Dump(canceled by started sink) config loads = %d, want 0", configLoads)
	}
	assertRuntimeEventKinds(t, events, []machine.EventKind{machine.EventStarted, machine.EventCanceled})
}

func TestEngineDumpExecutesAdvertisedCapabilityAndCompletesAfterWrite(t *testing.T) {
	t.Parallel()

	catalog := resources.ResourceCatalog{runtimeDumpListSpec(resources.ProductZIA, "locations")}
	reader := &runtimeDumpReader{list: map[runtimeResourceKey][]resources.SourceRecord{
		{product: resources.ProductZIA, resource: "locations"}: {
			resources.NewSourceRecord(map[string]any{"id": "1", "name": "HQ"}),
			resources.NewSourceRecord(map[string]any{"id": "2", "name": "Branch"}),
		},
	}}
	cfg, err := config.LoadConfig([]string{
		config.EnvClientID + "=client-id",
		config.EnvClientSecret + "=client-secret",
		config.EnvVanityDomain + "=example",
	}, config.LoadOptions{})
	if err != nil {
		t.Fatalf("config.LoadConfig(dump fixture) error = %v, want nil", err)
	}
	engine, err := NewEngine(Options{
		Catalog: catalog,
		loadConfig: func([]string, config.LoadOptions) (config.Config, error) {
			return cfg, nil
		},
		newReader: func(zscaler.ReaderConfig) (browser.RecordReader, error) {
			return reader, nil
		},
	})
	if err != nil {
		t.Fatalf("NewEngine() error = %v, want nil", err)
	}
	manifest := engine.EngineManifest()
	if !engineManifestHasCapability(manifest, machine.CapabilityDumpWrite) {
		t.Fatalf("EngineManifest() = %#v, want %s", manifest, machine.CapabilityDumpWrite)
	}

	outDir := filepath.Join(t.TempDir(), "dump")
	req := machine.DumpRequest{
		OutputDir: outDir,
		Products:  []string{"zia"},
		Resources: []machine.DumpResourceSelector{{Product: "zia", Resource: "locations"}},
	}
	var events []machine.Event
	result, err := engine.Dump(context.Background(), req, func(event machine.Event) error {
		events = append(events, event)
		if event.Kind == machine.EventStarted {
			req.Products[0] = "mutated"
			req.Resources[0].Resource = "mutated"
		}
		if event.Kind == machine.EventCompleted {
			for _, path := range []string{
				filepath.Join(outDir, "manifest.json"),
				filepath.Join(outDir, "redaction_report.json"),
				filepath.Join(outDir, "resources", "zia", "locations.json"),
			} {
				if _, statErr := os.Stat(path); statErr != nil {
					t.Errorf("completed event before artifact %q existed: %v", path, statErr)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Engine.Dump() error = %v, want nil", err)
	}
	assertRuntimeEventKinds(t, events, []machine.EventKind{
		machine.EventStarted,
		machine.EventProgress,
		machine.EventRecord,
		machine.EventRecord,
		machine.EventCompleted,
	})
	if result.Records() != 2 || result.Resources() != 1 || result.Warnings() != 0 ||
		result.Partial() || result.Redaction() != string(redact.ModeStandard) || result.Errors() == nil {
		t.Fatalf("Engine.Dump() result = records:%d resources:%d warnings:%d partial:%t redaction:%q errors:%#v",
			result.Records(), result.Resources(), result.Warnings(), result.Partial(), result.Redaction(), result.Errors())
	}
	wantCalls := []string{"list:zia/locations"}
	if !reflect.DeepEqual(reader.calls, wantCalls) {
		t.Fatalf("Engine.Dump() reader calls = %#v, want %#v", reader.calls, wantCalls)
	}
}

func TestTypedDumpSanitizesFatalCollectionError(t *testing.T) {
	t.Parallel()

	const canary = "dump-reader-client_secret-canary"
	catalog := resources.ResourceCatalog{runtimeDumpListSpec(resources.ProductZIA, "locations")}
	reader := &runtimeDumpReader{failures: map[runtimeResourceKey]error{
		{product: resources.ProductZIA, resource: "locations"}: errors.New(canary),
	}}
	collector := NewDumpCollectorFromReader(reader, catalog, redact.ModeStandard)
	outDir := filepath.Join(t.TempDir(), "dump")
	var events []machine.Event
	_, err := collector.Dump(context.Background(), machine.DumpRequest{OutputDir: outDir}, func(event machine.Event) error {
		events = append(events, event)
		return nil
	})
	var machineErr *machine.MachineError
	if !errors.As(err, &machineErr) || machineErr.Kind != machine.ErrorKindLiveAccessFailed ||
		!errors.Is(err, zscaler.ErrLiveAccessFailed) {
		t.Fatalf("DumpCollector.Dump(fatal reader) error = %v, want safe live MachineError", err)
	}
	if strings.Contains(err.Error(), canary) {
		t.Fatalf("DumpCollector.Dump(fatal reader) error = %q, want no backend canary", err)
	}
	assertRuntimeEventKinds(t, events, []machine.EventKind{
		machine.EventStarted, machine.EventProgress, machine.EventFailed,
	})
	terminal := events[len(events)-1]
	if terminal.Err == nil || terminal.Err.Product != "zia" || terminal.Err.Resource != "locations" ||
		strings.Contains(terminal.Err.Error(), canary) {
		t.Fatalf("DumpCollector.Dump(fatal reader) terminal = %#v, want safe catalog context", terminal)
	}
	if _, statErr := os.Stat(outDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("os.Stat(%q) error = %v, want no output after collection failure", outDir, statErr)
	}
}

func TestTypedDumpAllowListsMissingCredentialNames(t *testing.T) {
	t.Parallel()

	catalog := resources.ResourceCatalog{runtimeDumpListSpec(resources.ProductZPA, "app-segments")}
	rawErr := &zscaler.MissingCredentialsError{Missing: []string{
		config.EnvZPACustomerID,
		"UNSAFE_PROVIDER_CANARY",
		config.EnvZPACustomerID,
	}}
	reader := &runtimeDumpReader{failures: map[runtimeResourceKey]error{
		{product: resources.ProductZPA, resource: "app-segments"}: rawErr,
	}}
	collector := NewDumpCollectorFromReader(reader, catalog, redact.ModeStandard)
	var events []machine.Event
	_, err := collector.Dump(context.Background(), machine.DumpRequest{
		OutputDir: filepath.Join(t.TempDir(), "dump"),
	}, func(event machine.Event) error {
		events = append(events, event)
		return nil
	})
	var machineErr *machine.MachineError
	if !errors.As(err, &machineErr) || machineErr.Kind != machine.ErrorKindMissingCredentials ||
		!reflect.DeepEqual(machineErr.Missing, []string{config.EnvZPACustomerID}) {
		t.Fatalf("DumpCollector.Dump(missing credentials) error = %#v, want one allow-listed ZPA name", machineErr)
	}
	var missingErr *zscaler.MissingCredentialsError
	if !errors.As(err, &missingErr) || !reflect.DeepEqual(missingErr.Missing, []string{config.EnvZPACustomerID}) {
		t.Fatalf("DumpCollector.Dump(missing credentials) sentinel = %#v, want safe copied ZPA name", missingErr)
	}
	if strings.Contains(err.Error(), "UNSAFE_PROVIDER_CANARY") {
		t.Fatalf("DumpCollector.Dump(missing credentials) error = %q, want no unknown name", err)
	}
	assertRuntimeEventKinds(t, events, []machine.EventKind{
		machine.EventStarted, machine.EventProgress, machine.EventFailed,
	})
	terminal := events[len(events)-1]
	if terminal.Err == nil || !reflect.DeepEqual(terminal.Err.Missing, []string{config.EnvZPACustomerID}) {
		t.Fatalf("DumpCollector.Dump(missing credentials) terminal = %#v, want safe missing name", terminal)
	}
}

func TestTypedDumpReturnsValueFreePartialSummary(t *testing.T) {
	t.Parallel()

	const canary = "partial-dump-backend-canary"
	catalog := resources.ResourceCatalog{
		runtimeDumpListSpec(resources.ProductZIA, "locations"),
		runtimeDumpListSpec(resources.ProductZIA, "rule-labels"),
	}
	reader := &runtimeDumpReader{
		list: map[runtimeResourceKey][]resources.SourceRecord{
			{product: resources.ProductZIA, resource: "locations"}: {
				resources.NewSourceRecord(map[string]any{"id": "1", "name": "HQ"}),
			},
		},
		failures: map[runtimeResourceKey]error{
			{product: resources.ProductZIA, resource: "rule-labels"}: errors.New(canary),
		},
	}
	collector := NewDumpCollectorFromReader(reader, catalog, redact.ModeShare)
	outDir := filepath.Join(t.TempDir(), "dump")
	var events []machine.Event
	result, err := collector.Dump(context.Background(), machine.DumpRequest{
		OutputDir:       outDir,
		ContinueOnError: true,
	}, func(event machine.Event) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("DumpCollector.Dump(partial) error = %v, want nil", err)
	}
	if result.Records() != 1 || result.Resources() != 1 || result.Warnings() != 1 ||
		!result.Partial() || result.Redaction() != string(redact.ModeShare) {
		t.Fatalf("partial DumpResult = records:%d resources:%d warnings:%d partial:%t redaction:%q",
			result.Records(), result.Resources(), result.Warnings(), result.Partial(), result.Redaction())
	}
	wantErrors := []machine.DumpResourceError{{
		Product: "zia", Resource: "rule-labels", Operation: machine.OperationList, Kind: "list_failed",
	}}
	if got := result.Errors(); !reflect.DeepEqual(got, wantErrors) {
		t.Fatalf("partial DumpResult.Errors() = %#v, want %#v", got, wantErrors)
	}
	assertRuntimeEventKinds(t, events, []machine.EventKind{
		machine.EventStarted,
		machine.EventProgress,
		machine.EventRecord,
		machine.EventProgress,
		machine.EventWarning,
		machine.EventCompleted,
	})
	for _, event := range events {
		if event.Err != nil && strings.Contains(event.Err.Error(), canary) {
			t.Fatalf("partial dump event = %#v, want no backend canary", event)
		}
	}
	manifestBody, readErr := os.ReadFile(filepath.Join(outDir, "manifest.json"))
	if readErr != nil {
		t.Fatalf("os.ReadFile(partial manifest) error = %v", readErr)
	}
	if !strings.Contains(string(manifestBody), `"status": "partial"`) || strings.Contains(string(manifestBody), canary) {
		t.Fatalf("partial manifest = %s, want partial and no backend canary", manifestBody)
	}
	if body, readErr := os.ReadFile(filepath.Join(outDir, "errors.ndjson")); readErr != nil ||
		strings.Contains(string(body), canary) {
		t.Fatalf("partial errors.ndjson = %q, %v; want value-free", body, readErr)
	}
}

func TestTypedDumpOutputFailureIsStaticAndHasNoCompletedEvent(t *testing.T) {
	t.Parallel()

	catalog := resources.ResourceCatalog{runtimeDumpListSpec(resources.ProductZIA, "locations")}
	reader := &runtimeDumpReader{list: map[runtimeResourceKey][]resources.SourceRecord{
		{product: resources.ProductZIA, resource: "locations"}: {
			resources.NewSourceRecord(map[string]any{"id": "1", "name": "HQ"}),
		},
	}}
	collector := NewDumpCollectorFromReader(reader, catalog, redact.ModeStandard)
	parent := t.TempDir()
	outDir := filepath.Join(parent, "output-path-canary")
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", outDir, err)
	}
	manifestPath := filepath.Join(outDir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte("existing"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", manifestPath, err)
	}

	var events []machine.Event
	_, err := collector.Dump(context.Background(), machine.DumpRequest{OutputDir: outDir}, func(event machine.Event) error {
		events = append(events, event)
		return nil
	})
	var machineErr *machine.MachineError
	if !errors.As(err, &machineErr) || machineErr.Kind != machine.ErrorKindInternal ||
		!errors.Is(err, dumpartifact.ErrUnsafeOverwrite) {
		t.Fatalf("DumpCollector.Dump(existing output) error = %v, want internal/ErrUnsafeOverwrite", err)
	}
	if strings.Contains(err.Error(), "output-path-canary") {
		t.Fatalf("DumpCollector.Dump(existing output) error = %q, want no path", err)
	}
	assertRuntimeEventKinds(t, events, []machine.EventKind{
		machine.EventStarted, machine.EventProgress, machine.EventRecord, machine.EventFailed,
	})
	body, readErr := os.ReadFile(manifestPath)
	if readErr != nil || string(body) != "existing" {
		t.Fatalf("existing manifest after failed dump = %q, %v; want unchanged", body, readErr)
	}
}

func TestTypedDumpCancellationBeforeForcePreservesPreviousArtifact(t *testing.T) {
	t.Parallel()

	outDir := filepath.Join(t.TempDir(), "dump")
	if err := dumpartifact.Write(outDir, redact.ModeStandard, dumpartifact.Result{}); err != nil {
		t.Fatalf("dump.Write(previous artifact) error = %v, want nil", err)
	}
	manifestPath := filepath.Join(outDir, "manifest.json")
	before, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("os.ReadFile(previous manifest) error = %v", err)
	}

	catalog := resources.ResourceCatalog{runtimeDumpListSpec(resources.ProductZIA, "locations")}
	reader := &runtimeDumpReader{list: map[runtimeResourceKey][]resources.SourceRecord{
		{product: resources.ProductZIA, resource: "locations"}: {
			resources.NewSourceRecord(map[string]any{"id": "1", "name": "HQ"}),
		},
	}}
	collector := NewDumpCollectorFromReader(reader, catalog, redact.ModeStandard)
	ctx, cancel := context.WithCancel(context.Background())
	var events []machine.Event
	_, err = collector.Dump(ctx, machine.DumpRequest{OutputDir: outDir, Force: true}, func(event machine.Event) error {
		events = append(events, event)
		if event.Kind == machine.EventRecord {
			cancel()
		}
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("DumpCollector.Dump(canceled before force) error = %v, want context.Canceled", err)
	}
	assertRuntimeEventKinds(t, events, []machine.EventKind{
		machine.EventStarted, machine.EventProgress, machine.EventRecord, machine.EventCanceled,
	})
	after, readErr := os.ReadFile(manifestPath)
	if readErr != nil || !reflect.DeepEqual(after, before) {
		t.Fatalf("previous manifest after cancellation = %q, %v; want unchanged %q", after, readErr, before)
	}
}

func TestEngineDumpSanitizesConfigFailure(t *testing.T) {
	t.Parallel()

	const canary = "/private/dump-config-canary.yaml"
	catalog := resources.ResourceCatalog{runtimeDumpListSpec(resources.ProductZIA, "locations")}
	engine, err := NewEngine(Options{
		Catalog: catalog,
		loadConfig: func([]string, config.LoadOptions) (config.Config, error) {
			return config.Config{}, fmt.Errorf("%w: %s", config.ErrInvalidConfig, canary)
		},
	})
	if err != nil {
		t.Fatalf("NewEngine() error = %v, want nil", err)
	}
	var events []machine.Event
	_, err = engine.Dump(context.Background(), machine.DumpRequest{
		OutputDir: filepath.Join(t.TempDir(), "dump"),
	}, func(event machine.Event) error {
		events = append(events, event)
		return nil
	})
	var machineErr *machine.MachineError
	if !errors.As(err, &machineErr) || machineErr.Kind != machine.ErrorKindInvalidConfig ||
		!errors.Is(err, config.ErrInvalidConfig) || strings.Contains(err.Error(), canary) {
		t.Fatalf("Engine.Dump(config failure) error = %v, want static invalid-config sentinel", err)
	}
	assertRuntimeEventKinds(t, events, []machine.EventKind{machine.EventStarted, machine.EventFailed})
}

func engineManifestHasCapability(manifest machine.EngineManifest, name string) bool {
	for _, capability := range manifest.Capabilities {
		if capability.Name == name {
			return true
		}
	}
	return false
}
