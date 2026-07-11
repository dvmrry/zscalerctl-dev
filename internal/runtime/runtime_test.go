package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dvmrry/zscalerctl/internal/browser"
	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/dvmrry/zscalerctl/internal/zscaler"
)

func TestNewMachineAssemblesReaderConfigAndExecutes(t *testing.T) {
	t.Parallel()

	catalog := runtimeTestCatalog(t, resources.ProductZIA, "locations")
	reader := &runtimeFakeReader{
		list: map[runtimeResourceKey][]resources.SourceRecord{
			{product: resources.ProductZIA, resource: "locations"}: {
				resources.NewSourceRecord(map[string]any{
					"id":     "loc-1",
					"name":   "HQ",
					"status": "ACTIVE",
					"raw":    "not-rendered",
				}),
			},
		},
	}
	var gotReaderConfig zscaler.ReaderConfig
	rt, err := NewMachine(context.Background(), Options{
		Env: []string{
			config.EnvClientID + "=client-id",
			config.EnvClientSecret + "=client-secret",
			config.EnvVanityDomain + "=example",
			config.EnvCloud + "=PRODUCTION",
			config.EnvZPACustomerID + "=customer-id",
			config.EnvZPAMicrotenantID + "=microtenant-id",
			config.EnvRedaction + "=share",
			config.EnvNoCache + "=true",
			config.EnvProxyURL + "=https://proxy.example.invalid:8443",
		},
		Timeout: 7 * time.Second,
		Catalog: catalog,
		newReader: func(cfg zscaler.ReaderConfig) (browser.RecordReader, error) {
			gotReaderConfig = cfg
			return reader, nil
		},
	})
	if err != nil {
		t.Fatalf("NewMachine(env runtime) error = %v, want nil", err)
	}

	if got := gotReaderConfig.ClientID.Reveal(); got != "client-id" {
		t.Errorf("NewMachine(env runtime) ClientID = %q, want client-id", got)
	}
	if got := gotReaderConfig.ClientSecret.Reveal(); got != "client-secret" {
		t.Errorf("NewMachine(env runtime) ClientSecret = %q, want client-secret", got)
	}
	if gotReaderConfig.VanityDomain != "example" ||
		gotReaderConfig.Cloud != "PRODUCTION" ||
		gotReaderConfig.ZPACustomerID != "customer-id" ||
		gotReaderConfig.ZPAMicrotenantID != "microtenant-id" {
		t.Errorf("NewMachine(env runtime) reader config = %+v, want env-derived tenant fields", gotReaderConfig)
	}
	if gotReaderConfig.AuthMode != zscaler.AuthMode(config.AuthModeOneAPI) {
		t.Errorf("NewMachine(env runtime) AuthMode = %q, want %q", gotReaderConfig.AuthMode, config.AuthModeOneAPI)
	}
	if gotReaderConfig.Timeout != 7*time.Second {
		t.Errorf("NewMachine(env runtime) Timeout = %s, want 7s", gotReaderConfig.Timeout)
	}
	if !gotReaderConfig.NoCache {
		t.Errorf("NewMachine(env runtime) NoCache = false, want true")
	}
	if gotReaderConfig.Proxy.URL != "https://proxy.example.invalid:8443" {
		t.Errorf("NewMachine(env runtime) Proxy.URL = %q, want configured proxy", gotReaderConfig.Proxy.URL)
	}
	if got := rt.Redaction(); got != redact.ModeShare {
		t.Fatalf("Machine.Redaction() = %q, want %q", got, redact.ModeShare)
	}

	resp, err := rt.Execute(context.Background(), machine.Request{
		RequestID:  "req-1",
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationList,
		Input:      &machine.Input{Product: "zia", Resource: "locations"},
	})
	if err != nil {
		t.Fatalf("Machine.Execute(list locations) error = %v, want nil", err)
	}
	wantRecords := []map[string]any{{"id": "loc-1", "name": "HQ"}}
	if !reflect.DeepEqual(resp.Records, wantRecords) {
		t.Fatalf("Machine.Execute(list locations).Records = %#v, want %#v", resp.Records, wantRecords)
	}
	wantCalls := []string{"list:zia/locations"}
	if !reflect.DeepEqual(reader.calls, wantCalls) {
		t.Fatalf("Machine.Execute(list locations) reader calls = %#v, want %#v", reader.calls, wantCalls)
	}
}

func TestNewMachineFromConfigAssemblesReaderConfig(t *testing.T) {
	t.Parallel()

	cfg, err := config.LoadConfig([]string{
		config.EnvClientID + "=client-id",
		config.EnvClientSecret + "=client-secret",
		config.EnvVanityDomain + "=example",
		config.EnvRedaction + "=paranoid",
	}, config.LoadOptions{})
	if err != nil {
		t.Fatalf("LoadConfig(runtime fixture) error = %v, want nil", err)
	}

	var gotReaderConfig zscaler.ReaderConfig
	rt, err := NewMachineFromConfig(context.Background(), cfg, Options{
		Timeout: 3 * time.Second,
		Catalog: runtimeTestCatalog(t, resources.ProductZIA, "locations"),
		newReader: func(cfg zscaler.ReaderConfig) (browser.RecordReader, error) {
			gotReaderConfig = cfg
			return &runtimeFakeReader{}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewMachineFromConfig(effective config) error = %v, want nil", err)
	}
	if gotReaderConfig.Timeout != 3*time.Second {
		t.Fatalf("NewMachineFromConfig(effective config) Timeout = %s, want 3s", gotReaderConfig.Timeout)
	}
	if got := rt.Redaction(); got != redact.ModeParanoid {
		t.Fatalf("Machine.Redaction() = %q, want %q", got, redact.ModeParanoid)
	}
}

func TestNewDumpCollectorAssemblesReaderConfigAndCollects(t *testing.T) {
	t.Parallel()

	catalog := resources.ResourceCatalog{
		runtimeDumpListSpec(resources.ProductZIA, "locations"),
	}
	reader := &runtimeDumpReader{
		list: map[runtimeResourceKey][]resources.SourceRecord{
			{product: resources.ProductZIA, resource: "locations"}: {
				resources.NewSourceRecord(map[string]any{
					"id":       "loc-1",
					"name":     "HQ",
					"rawNoise": "dropped",
				}),
			},
		},
	}
	var gotReaderConfig zscaler.ReaderConfig
	collector, err := NewDumpCollector(context.Background(), Options{
		Env: []string{
			config.EnvClientID + "=client-id",
			config.EnvClientSecret + "=client-secret",
			config.EnvVanityDomain + "=example",
			config.EnvCloud + "=PRODUCTION",
			config.EnvRedaction + "=share",
			config.EnvNoCache + "=true",
		},
		Timeout: 9 * time.Second,
		Catalog: catalog,
		newReader: func(cfg zscaler.ReaderConfig) (browser.RecordReader, error) {
			gotReaderConfig = cfg
			return reader, nil
		},
	})
	if err != nil {
		t.Fatalf("NewDumpCollector(env runtime) error = %v, want nil", err)
	}
	if got := gotReaderConfig.ClientID.Reveal(); got != "client-id" {
		t.Errorf("NewDumpCollector(env runtime) ClientID = %q, want client-id", got)
	}
	if got := gotReaderConfig.ClientSecret.Reveal(); got != "client-secret" {
		t.Errorf("NewDumpCollector(env runtime) ClientSecret = %q, want client-secret", got)
	}
	if gotReaderConfig.Timeout != 9*time.Second {
		t.Errorf("NewDumpCollector(env runtime) Timeout = %s, want 9s", gotReaderConfig.Timeout)
	}
	if !gotReaderConfig.NoCache {
		t.Errorf("NewDumpCollector(env runtime) NoCache = false, want true")
	}

	result, err := collector.Collect(context.Background(), catalog, DumpCollectOptions{})
	if err != nil {
		t.Fatalf("DumpCollector.Collect(locations) error = %v, want nil", err)
	}
	if got, want := len(result.Entries), 1; got != want {
		t.Fatalf("DumpCollector.Collect(locations) entries = %d, want %d", got, want)
	}
	gotRecords := result.Entries[0].Records.Records()
	if got, want := len(gotRecords), 1; got != want {
		t.Fatalf("DumpCollector.Collect(locations) records = %d, want %d", got, want)
	}
	if gotFields := gotRecords[0].Fields(); !reflect.DeepEqual(gotFields, map[string]any{"id": "loc-1", "name": "HQ"}) {
		t.Fatalf("DumpCollector.Collect(locations) record = %#v, want id/name only", gotFields)
	}
	if gotCalls := reader.calls; !reflect.DeepEqual(gotCalls, []string{"list:zia/locations"}) {
		t.Fatalf("DumpCollector.Collect(locations) reader calls = %#v, want list call", gotCalls)
	}
}

func TestDumpCollectorCollectStreamEmitsProjectedLifecycle(t *testing.T) {
	t.Parallel()

	catalog := resources.ResourceCatalog{
		runtimeDumpListSpec(resources.ProductZIA, "locations"),
	}
	reader := &runtimeDumpReader{
		list: map[runtimeResourceKey][]resources.SourceRecord{
			{product: resources.ProductZIA, resource: "locations"}: {
				resources.NewSourceRecord(map[string]any{
					"id":       "loc-1",
					"name":     "HQ",
					"rawNoise": "must-not-cross-event-boundary",
				}),
				resources.NewSourceRecord(map[string]any{"id": "loc-2", "name": "Branch"}),
			},
		},
	}
	collector := NewDumpCollectorFromReader(reader, catalog, redact.ModeStandard)

	var events []machine.Event
	result, err := collector.CollectStream(context.Background(), catalog, DumpCollectOptions{}, func(event machine.Event) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("DumpCollector.CollectStream(locations) error = %v, want nil", err)
	}
	assertRuntimeEventKinds(t, events, []machine.EventKind{
		machine.EventStarted,
		machine.EventProgress,
		machine.EventRecord,
		machine.EventRecord,
		machine.EventCompleted,
	})
	if events[0].Total != 1 {
		t.Errorf("CollectStream started total = %d, want 1", events[0].Total)
	}
	if events[1].Done != 1 || events[1].Total != 1 || events[1].Product != "zia" || events[1].Resource != "locations" {
		t.Errorf("CollectStream progress = %#v, want 1/1 zia/locations", events[1])
	}
	for i, event := range events[2:4] {
		if event.Record == nil {
			t.Fatalf("CollectStream record event %d has nil record", i)
		}
		if _, ok := event.Record.Value("rawNoise"); ok {
			t.Errorf("CollectStream record event %d exposes dropped rawNoise field", i)
		}
	}
	completed := events[len(events)-1]
	if completed.Records != 2 || completed.Resources != 1 || completed.Warnings != 0 {
		t.Errorf("CollectStream completed counts = records:%d resources:%d warnings:%d, want 2/1/0",
			completed.Records, completed.Resources, completed.Warnings)
	}
	if got, want := len(result.Entries), 1; got != want {
		t.Errorf("CollectStream result entries = %d, want %d", got, want)
	}
}

func TestDumpCollectorCollectStreamCountsShowResource(t *testing.T) {
	t.Parallel()

	spec := resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "advanced-settings",
		Operations: resources.ShowOperation(),
		Fields: []resources.FieldSpec{
			{
				Name:           "id",
				Classification: resources.ClassOperational,
				AllowedModes:   []redact.Mode{redact.ModeStandard},
			},
			{
				Name:           "name",
				Classification: resources.ClassTenantConfig,
				AllowedModes:   []redact.Mode{redact.ModeStandard},
			},
		},
	}
	catalog := resources.ResourceCatalog{spec}
	reader := &runtimeFakeReader{
		show: map[runtimeResourceKey]resources.SourceRecord{
			{product: spec.Product, resource: spec.Name}: resources.NewSourceRecord(map[string]any{
				"id":   "settings-1",
				"name": "Tenant settings",
			}),
		},
	}
	collector := NewDumpCollectorFromReader(reader, catalog, redact.ModeStandard)

	var events []machine.Event
	result, err := collector.CollectStream(context.Background(), catalog, DumpCollectOptions{}, func(event machine.Event) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("DumpCollector.CollectStream(show) error = %v, want nil", err)
	}
	assertRuntimeEventKinds(t, events, []machine.EventKind{
		machine.EventStarted,
		machine.EventProgress,
		machine.EventRecord,
		machine.EventCompleted,
	})
	completed := events[len(events)-1]
	if completed.Records != 1 || completed.Resources != 1 || completed.Warnings != 0 {
		t.Errorf("CollectStream(show) completed counts = records:%d resources:%d warnings:%d, want 1/1/0",
			completed.Records, completed.Resources, completed.Warnings)
	}
	if got, want := len(result.Entries), 1; got != want || result.Entries[0].Record == nil {
		t.Errorf("CollectStream(show) result = %#v, want one show entry", result)
	}
}

func TestDumpCollectorCollectStreamEmitsValueFreeWarning(t *testing.T) {
	t.Parallel()

	catalog := resources.ResourceCatalog{
		runtimeDumpListSpec(resources.ProductZIA, "locations"),
		runtimeDumpListSpec(resources.ProductZIA, "rule-labels"),
	}
	const rawBackendError = "client_secret=must-not-cross-event-boundary"
	reader := &runtimeDumpReader{
		list: map[runtimeResourceKey][]resources.SourceRecord{
			{product: resources.ProductZIA, resource: "locations"}: {
				resources.NewSourceRecord(map[string]any{"id": "loc-1", "name": "HQ"}),
			},
		},
		failures: map[runtimeResourceKey]error{
			{product: resources.ProductZIA, resource: "rule-labels"}: errors.New(rawBackendError),
		},
	}
	collector := NewDumpCollectorFromReader(reader, catalog, redact.ModeStandard)

	var events []machine.Event
	result, err := collector.CollectStream(context.Background(), catalog, DumpCollectOptions{
		ContinueOnError: true,
	}, func(event machine.Event) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("DumpCollector.CollectStream(continue on error) error = %v, want nil", err)
	}
	assertRuntimeEventKinds(t, events, []machine.EventKind{
		machine.EventStarted,
		machine.EventProgress,
		machine.EventRecord,
		machine.EventProgress,
		machine.EventWarning,
		machine.EventCompleted,
	})
	warning := events[4]
	if warning.Err == nil {
		t.Fatal("CollectStream warning error = nil, want value-free MachineError")
	}
	if warning.Product != "zia" || warning.Resource != "rule-labels" ||
		warning.Err.Kind != "list_failed" || warning.Err.Operation != machine.OperationList ||
		warning.Err.Product != "zia" || warning.Err.Resource != "rule-labels" {
		t.Errorf("CollectStream warning = %#v, want list_failed metadata for zia/rule-labels", warning)
	}
	if strings.Contains(warning.Err.Message, rawBackendError) || strings.Contains(warning.Err.Error(), rawBackendError) {
		t.Errorf("CollectStream warning error = %q, want no backend error value", warning.Err.Message)
	}
	completed := events[len(events)-1]
	if completed.Records != 1 || completed.Resources != 1 || completed.Warnings != 1 {
		t.Errorf("CollectStream completed counts = records:%d resources:%d warnings:%d, want 1/1/1",
			completed.Records, completed.Resources, completed.Warnings)
	}
	if got, want := len(result.Errors), 1; got != want {
		t.Fatalf("CollectStream result errors = %d, want %d", got, want)
	}
	resourceErr := result.Errors[0]
	if resourceErr.Product != warning.Err.Product || resourceErr.Name != warning.Err.Resource ||
		resourceErr.Operation != string(warning.Err.Operation) || resourceErr.Kind != warning.Err.Kind {
		t.Errorf("CollectStream warning metadata = %#v, want errors.ndjson metadata %#v", warning.Err, resourceErr)
	}
}

func TestDumpCollectorCollectStreamPreservesContextErrorWithTerminalEvent(t *testing.T) {
	t.Parallel()

	catalog := resources.ResourceCatalog{
		runtimeDumpListSpec(resources.ProductZIA, "locations"),
	}
	collector := NewDumpCollectorFromReader(&runtimeDumpReader{}, catalog, redact.ModeStandard)
	tests := []struct {
		name         string
		ctx          context.Context
		wantErr      error
		wantKind     string
		wantTerminal machine.EventKind
	}{
		{
			name:         "canceled",
			ctx:          canceledRuntimeContext(),
			wantErr:      context.Canceled,
			wantKind:     machine.ErrorKindCanceled,
			wantTerminal: machine.EventCanceled,
		},
		{
			name:         "deadline",
			ctx:          expiredRuntimeContext(),
			wantErr:      context.DeadlineExceeded,
			wantKind:     machine.ErrorKindDeadlineExceeded,
			wantTerminal: machine.EventFailed,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var events []machine.Event
			_, err := collector.CollectStream(tt.ctx, catalog, DumpCollectOptions{}, func(event machine.Event) error {
				events = append(events, event)
				return nil
			})
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("CollectStream(%s) error = %v, want errors.Is(%v)", tt.name, err, tt.wantErr)
			}
			assertRuntimeEventKinds(t, events, []machine.EventKind{machine.EventStarted, tt.wantTerminal})
			terminal := events[1]
			if terminal.Err == nil || terminal.Err.Kind != tt.wantKind {
				t.Errorf("CollectStream(%s) terminal error = %#v, want kind %q", tt.name, terminal.Err, tt.wantKind)
			}
		})
	}
}

func TestDumpCollectorCollectStreamPreservesFatalErrorIdentityAndSanitizesEvent(t *testing.T) {
	t.Parallel()

	catalog := resources.ResourceCatalog{
		runtimeDumpListSpec(resources.ProductZIA, "locations"),
	}
	const rawBackendError = "client_secret=trusted-in-process-cause"
	sentinel := errors.New(rawBackendError)
	reader := &runtimeDumpReader{
		failures: map[runtimeResourceKey]error{
			{product: resources.ProductZIA, resource: "locations"}: sentinel,
		},
	}
	collector := NewDumpCollectorFromReader(reader, catalog, redact.ModeStandard)

	var events []machine.Event
	_, err := collector.CollectStream(context.Background(), catalog, DumpCollectOptions{}, func(event machine.Event) error {
		events = append(events, event)
		return nil
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("CollectStream(fatal read) error = %v, want original sentinel identity", err)
	}
	assertRuntimeEventKinds(t, events, []machine.EventKind{
		machine.EventStarted,
		machine.EventProgress,
		machine.EventFailed,
	})
	terminal := events[len(events)-1]
	if terminal.Err == nil || terminal.Err.Kind != machine.ErrorKindLiveAccessFailed ||
		terminal.Err.Operation != machine.OperationList || terminal.Err.Product != "zia" || terminal.Err.Resource != "locations" {
		t.Fatalf("CollectStream(fatal read) terminal error = %#v, want sanitized list failure", terminal.Err)
	}
	if strings.Contains(terminal.Err.Message, rawBackendError) || strings.Contains(terminal.Err.Error(), rawBackendError) {
		t.Errorf("CollectStream(fatal read) terminal error = %q, want no backend value", terminal.Err.Message)
	}
}

func TestDumpCollectorCollectStreamPreservesFatalErrorWhenTerminalDeliveryFails(t *testing.T) {
	t.Parallel()

	catalog := resources.ResourceCatalog{
		runtimeDumpListSpec(resources.ProductZIA, "locations"),
	}
	tests := []struct {
		name            string
		failTerminal    func() error
		rawDelivery     string
		wantDeliveryMsg string
	}{
		{
			name:        "sink error",
			rawDelivery: "consumer error containing raw value",
			failTerminal: func() error {
				return errors.New("consumer error containing raw value")
			},
			wantDeliveryMsg: "event sink failed",
		},
		{
			name:        "sink panic",
			rawDelivery: "consumer panic containing raw value",
			failTerminal: func() error {
				panic("consumer panic containing raw value")
			},
			wantDeliveryMsg: "event sink panicked",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sentinel := errors.New("trusted backend sentinel")
			session := &runtimeDumpSession{err: sentinel}
			reader := &runtimeDumpSessionProvider{session: session}
			collector := NewDumpCollectorFromReader(reader, catalog, redact.ModeStandard)

			var events []machine.Event
			_, err := collector.CollectStream(context.Background(), catalog, DumpCollectOptions{}, func(event machine.Event) error {
				events = append(events, event)
				if event.Kind == machine.EventFailed {
					return tt.failTerminal()
				}
				return nil
			})
			if !errors.Is(err, sentinel) {
				t.Fatalf("CollectStream(%s) error = %v, want original sentinel identity", tt.name, err)
			}
			var deliveryErr *machine.MachineError
			if !errors.As(err, &deliveryErr) {
				t.Fatalf("CollectStream(%s) error = %T %v, want joined *machine.MachineError", tt.name, err, err)
			}
			if deliveryErr.Kind != machine.ErrorKindInternal || deliveryErr.Message != tt.wantDeliveryMsg {
				t.Errorf("CollectStream(%s) delivery error = %#v, want internal/%q", tt.name, deliveryErr, tt.wantDeliveryMsg)
			}
			if strings.Contains(err.Error(), tt.rawDelivery) {
				t.Errorf("CollectStream(%s) joined error = %q, want no raw sink failure value", tt.name, err)
			}
			assertRuntimeEventKinds(t, events, []machine.EventKind{
				machine.EventStarted,
				machine.EventProgress,
				machine.EventFailed,
			})
			terminal := events[len(events)-1]
			if terminal.Err == nil || terminal.Err.Kind != machine.ErrorKindLiveAccessFailed || terminal.Err.Message != "resource read failed" {
				t.Errorf("CollectStream(%s) terminal event error = %#v, want sanitized live-access failure", tt.name, terminal.Err)
			}
			if session.closeCalls != 1 {
				t.Errorf("CollectStream(%s) session close calls = %d, want 1", tt.name, session.closeCalls)
			}
		})
	}
}

func TestDumpCollectorUsesOneProductSession(t *testing.T) {
	t.Parallel()

	catalog := resources.ResourceCatalog{
		runtimeDumpListSpec(resources.ProductZIA, "locations"),
		runtimeDumpListSpec(resources.ProductZIA, "rule-labels"),
	}
	session := &runtimeDumpSession{
		list: []resources.SourceRecord{
			resources.NewSourceRecord(map[string]any{"id": "1", "name": "one"}),
		},
	}
	reader := &runtimeDumpSessionProvider{session: session}
	collector := NewDumpCollectorFromReader(reader, catalog, redact.ModeStandard)

	_, err := collector.Collect(context.Background(), catalog, DumpCollectOptions{})
	if err != nil {
		t.Fatalf("DumpCollector.Collect(session catalog) error = %v, want nil", err)
	}
	if reader.sessionCalls != 1 {
		t.Errorf("runtimeDumpSessionProvider.Session calls = %d, want 1", reader.sessionCalls)
	}
	if reader.directListCalls != 0 {
		t.Errorf("runtimeDumpSessionProvider.List calls = %d, want 0", reader.directListCalls)
	}
	if session.listCalls != 2 {
		t.Errorf("runtimeDumpSession.List calls = %d, want 2", session.listCalls)
	}
	if session.closeCalls != 1 {
		t.Errorf("runtimeDumpSession.Close calls = %d, want 1", session.closeCalls)
	}
}

func TestDumpCollectorContinueOnErrorRecordsValueFreeListFailure(t *testing.T) {
	t.Parallel()

	catalog := resources.ResourceCatalog{
		runtimeDumpListSpec(resources.ProductZIA, "locations"),
		runtimeDumpListSpec(resources.ProductZIA, "rule-labels"),
	}
	reader := &runtimeDumpReader{
		list: map[runtimeResourceKey][]resources.SourceRecord{
			{product: resources.ProductZIA, resource: "locations"}: {
				resources.NewSourceRecord(map[string]any{"id": "loc-1", "name": "HQ"}),
			},
		},
		failures: map[runtimeResourceKey]error{
			{product: resources.ProductZIA, resource: "rule-labels"}: errors.New("client_secret=raw-value"),
		},
	}
	collector := NewDumpCollectorFromReader(reader, catalog, redact.ModeStandard)

	result, err := collector.Collect(context.Background(), catalog, DumpCollectOptions{
		ContinueOnError: true,
	})
	if err != nil {
		t.Fatalf("DumpCollector.Collect(continue on list error) error = %v, want nil", err)
	}
	if got, want := len(result.Entries), 1; got != want {
		t.Fatalf("DumpCollector.Collect(continue on list error) entries = %d, want %d", got, want)
	}
	if got, want := len(result.Errors), 1; got != want {
		t.Fatalf("DumpCollector.Collect(continue on list error) errors = %d, want %d", got, want)
	}
	got := result.Errors[0]
	if got.Product != "zia" || got.Name != "rule-labels" || got.Operation != "list" || got.Kind != "list_failed" {
		t.Fatalf("DumpCollector.Collect(continue on list error) error record = %#v, want value-free list_failed", got)
	}
}

func TestDumpCollectorContinueOnErrorRecordsProjectionFailure(t *testing.T) {
	t.Parallel()

	catalog := resources.ResourceCatalog{{
		Product:    resources.ProductZIA,
		Name:       "bad-shape",
		Shape:      resources.ResourceShape("invalid"),
		Operations: resources.ListOperations(),
		Fields: []resources.FieldSpec{{
			Name:           "id",
			Classification: resources.ClassOperational,
			AllowedModes:   []redact.Mode{redact.ModeStandard},
		}},
	}}
	reader := &runtimeDumpReader{
		list: map[runtimeResourceKey][]resources.SourceRecord{
			{product: resources.ProductZIA, resource: "bad-shape"}: {
				resources.NewSourceRecord(map[string]any{"id": "1"}),
			},
		},
	}
	collector := NewDumpCollectorFromReader(reader, catalog, redact.ModeStandard)

	result, err := collector.Collect(context.Background(), catalog, DumpCollectOptions{
		ContinueOnError: true,
	})
	if err != nil {
		t.Fatalf("DumpCollector.Collect(continue on projection error) error = %v, want nil", err)
	}
	if got, want := len(result.Entries), 0; got != want {
		t.Fatalf("DumpCollector.Collect(continue on projection error) entries = %d, want %d", got, want)
	}
	if got, want := len(result.Errors), 1; got != want {
		t.Fatalf("DumpCollector.Collect(continue on projection error) errors = %d, want %d", got, want)
	}
	got := result.Errors[0]
	if got.Product != "zia" || got.Name != "bad-shape" || got.Operation != "project" || got.Kind != "projection_failed" {
		t.Fatalf("DumpCollector.Collect(continue on projection error) error record = %#v, want projection_failed", got)
	}
}

func TestNewMachineWrapsDeferredSecretResolutionErrors(t *testing.T) {
	t.Parallel()

	configPath := runtimeWriteConfig(t, `
profiles:
  default:
    vanity_domain: example
    client_id: client-id
    client_secret_ref: env:ZSCALERCTL_TEST_MISSING_SECRET
`)
	_, err := NewMachine(context.Background(), Options{
		ConfigPath: configPath,
		newReader: func(zscaler.ReaderConfig) (browser.RecordReader, error) {
			t.Fatal("NewMachine(deferred secret error) called reader factory, want failure before reader construction")
			return nil, nil
		},
	})
	if !errors.Is(err, zscaler.ErrMissingCredentials) {
		t.Fatalf("NewMachine(deferred secret error) error = %v, want ErrMissingCredentials", err)
	}
}

func TestNewMachineRejectsInvalidRuntimeOptionsBeforeReader(t *testing.T) {
	t.Parallel()

	_, err := NewMachine(context.Background(), Options{
		Env: []string{
			config.EnvClientID + "=client-id",
			config.EnvClientSecret + "=client-secret",
			config.EnvVanityDomain + "=example",
		},
		Redaction:    redact.Mode("verbose"),
		RedactionSet: true,
		newReader: func(zscaler.ReaderConfig) (browser.RecordReader, error) {
			t.Fatal("NewMachine(invalid redaction) called reader factory, want validation first")
			return nil, nil
		},
	})
	if !errors.Is(err, config.ErrInvalidConfig) {
		t.Fatalf("NewMachine(invalid redaction) error = %v, want ErrInvalidConfig", err)
	}
}

func TestMachineExecuteReturnsMachineLiveLoadError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("backend sentinel")
	rt := newMachineFromReader(&runtimeFakeReader{listErr: sentinel},
		runtimeTestCatalog(t, resources.ProductZIA, "locations"), redact.ModeStandard)

	resp, err := rt.Execute(context.Background(), machine.Request{
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationList,
		Input:      &machine.Input{Product: "zia", Resource: "locations"},
	})
	var machineErr *machine.MachineError
	if !errors.As(err, &machineErr) {
		t.Fatalf("Machine.Execute(live load error) error = %T %v, want *machine.MachineError", err, err)
	}
	if machineErr.Kind != machine.ErrorKindLiveAccessFailed {
		t.Fatalf("Machine.Execute(live load error) MachineError.Kind = %q, want %q", machineErr.Kind, machine.ErrorKindLiveAccessFailed)
	}
	if errors.Is(err, sentinel) {
		t.Fatalf("Machine.Execute(live load error) error = %v, want machine error instead of original loader sentinel", err)
	}
	if resp.Error == nil || resp.Error.Kind != machine.ErrorKindLiveAccessFailed {
		t.Fatalf("Machine.Execute(live load error) response error = %#v, want live_access_failed", resp.Error)
	}
}

func TestMachineExecuteReturnsMachineNotFoundError(t *testing.T) {
	t.Parallel()

	reader := &runtimeFakeReader{getErr: resources.ErrRecordNotFound}
	rt := newMachineFromReader(reader, runtimeTestCatalog(t, resources.ProductZIA, "locations"), redact.ModeStandard)

	resp, err := rt.Execute(context.Background(), machine.Request{
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationGet,
		Input:      &machine.Input{Product: "zia", Resource: "locations", RecordID: "missing"},
	})
	var machineErr *machine.MachineError
	if !errors.As(err, &machineErr) {
		t.Fatalf("Machine.Execute(missing record) error = %T %v, want *machine.MachineError", err, err)
	}
	if machineErr.Kind != machine.ErrorKindNotFound {
		t.Fatalf("Machine.Execute(missing record) MachineError.Kind = %q, want %q", machineErr.Kind, machine.ErrorKindNotFound)
	}
	if errors.Is(err, resources.ErrRecordNotFound) {
		t.Fatalf("Machine.Execute(missing record) error = %v, want machine error instead of original loader sentinel", err)
	}
	if resp.Error == nil || resp.Error.Kind != machine.ErrorKindNotFound {
		t.Fatalf("Machine.Execute(missing record) response error = %#v, want not_found", resp.Error)
	}
	wantCalls := []string{"get:zia/locations/missing"}
	if !reflect.DeepEqual(reader.calls, wantCalls) {
		t.Fatalf("Machine.Execute(missing record) reader calls = %#v, want %#v", reader.calls, wantCalls)
	}
}

func TestMachineExecuteStreamForwardsProjectedEvents(t *testing.T) {
	catalog := runtimeDeepCopyCatalog()
	reader := &runtimeFakeReader{
		list: map[runtimeResourceKey][]resources.SourceRecord{
			{product: resources.ProductZIA, resource: "locations"}: {
				resources.NewSourceRecord(map[string]any{
					"outer": map[string]any{"inner": "value"},
				}),
			},
		},
	}
	rt := NewMachineFromReader(reader, catalog, redact.ModeStandard)
	req := machine.Request{
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationList,
		Input:      &machine.Input{Product: "zia", Resource: "locations"},
	}

	var events []machine.Event
	err := rt.ExecuteStream(context.Background(), req, func(event machine.Event) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("Machine.ExecuteStream(list request) error = %v, want nil", err)
	}
	wantKinds := []machine.EventKind{
		machine.EventStarted,
		machine.EventRecord,
		machine.EventCompleted,
	}
	gotKinds := make([]machine.EventKind, len(events))
	for i, event := range events {
		gotKinds[i] = event.Kind
	}
	if !reflect.DeepEqual(gotKinds, wantKinds) {
		t.Fatalf("Machine.ExecuteStream(list request) event kinds = %#v, want %#v", gotKinds, wantKinds)
	}
	if events[1].Record == nil {
		t.Fatal("Machine.ExecuteStream(list request) record event = nil, want projected record")
	}
	wantRecord := map[string]any{"outer": map[string]any{"inner": "value"}}
	if got := events[1].Record.Fields(); !reflect.DeepEqual(got, wantRecord) {
		t.Errorf("Machine.ExecuteStream(list request) record = %#v, want %#v", got, wantRecord)
	}
}

func TestMachineManifestAndCatalogAreDefensiveCopies(t *testing.T) {
	t.Parallel()

	catalog := runtimeDeepCopyCatalog()
	reader := &runtimeFakeReader{
		list: map[runtimeResourceKey][]resources.SourceRecord{
			{product: resources.ProductZIA, resource: "locations"}: {
				resources.NewSourceRecord(map[string]any{
					"outer": map[string]any{
						"inner": "value",
					},
				}),
			},
		},
	}
	rt := NewMachineFromReader(reader, catalog, redact.ModeStandard)

	catalog[0].Name = "mutated"
	catalog[0].Operations[0].Capability = resources.CapabilityWrite
	catalog[0].Fields[0].Name = "mutated"
	catalog[0].Fields[0].AllowedModes[0] = redact.ModeShare
	catalog[0].Fields[0].Fields[0].Name = "mutated-inner"
	catalog[0].Fields[0].Fields[0].AllowedModes[0] = redact.ModeParanoid
	assertRuntimeCatalogUnchanged(t, rt, "after input catalog mutation")

	gotCatalog := rt.Catalog()
	gotCatalog[0].Name = "changed"
	gotCatalog[0].Operations[0].Capability = resources.CapabilityWrite
	gotCatalog[0].Fields[0].Name = "changed"
	gotCatalog[0].Fields[0].AllowedModes[0] = redact.ModeShare
	gotCatalog[0].Fields[0].Fields[0].Name = "changed-inner"
	gotCatalog[0].Fields[0].Fields[0].AllowedModes[0] = redact.ModeParanoid
	assertRuntimeCatalogUnchanged(t, rt, "after returned catalog mutation")
}

func assertRuntimeCatalogUnchanged(t *testing.T, rt *Machine, phase string) {
	t.Helper()
	manifest := rt.Manifest()
	if len(manifest.Capabilities) != 1 {
		t.Fatalf("Machine.Manifest(%s) capabilities = %d, want 1", phase, len(manifest.Capabilities))
	}
	if got := manifest.Capabilities[0].Input.Resource; got != "locations" {
		t.Fatalf("Machine.Manifest(%s) resource = %q, want locations", phase, got)
	}
	if got := rt.Redaction(); got != redact.ModeStandard {
		t.Fatalf("Machine.Redaction(%s) = %q, want %q", phase, got, redact.ModeStandard)
	}
	gotCatalog := rt.Catalog()
	wantCatalog := runtimeDeepCopyCatalog()
	if !reflect.DeepEqual(gotCatalog, wantCatalog) {
		t.Fatalf("Machine.Catalog(%s) = %#v, want %#v", phase, gotCatalog, wantCatalog)
	}

	resp, err := rt.Execute(context.Background(), machine.Request{
		Capability: machine.CapabilityResourcesRead,
		Operation:  machine.OperationList,
		Input:      &machine.Input{Product: "zia", Resource: "locations"},
	})
	if err != nil {
		t.Fatalf("Machine.Execute(%s) error = %v, want nil", phase, err)
	}
	wantRecords := []map[string]any{{
		"outer": map[string]any{
			"inner": "value",
		},
	}}
	if !reflect.DeepEqual(resp.Records, wantRecords) {
		t.Fatalf("Machine.Execute(%s).Records = %#v, want %#v", phase, resp.Records, wantRecords)
	}
}

func runtimeDeepCopyCatalog() resources.ResourceCatalog {
	return resources.ResourceCatalog{{
		Product: resources.ProductZIA,
		Name:    "locations",
		Operations: []resources.Operation{{
			Name:       "list",
			Capability: resources.CapabilityRead,
		}},
		Fields: []resources.FieldSpec{{
			Name:           "outer",
			Classification: resources.ClassTenantConfig,
			AllowedModes:   []redact.Mode{redact.ModeStandard},
			Fields: []resources.FieldSpec{{
				Name:           "inner",
				Classification: resources.ClassOperational,
				AllowedModes:   []redact.Mode{redact.ModeStandard},
			}},
		}},
	}}
}

type runtimeFakeReader struct {
	list    map[runtimeResourceKey][]resources.SourceRecord
	get     map[runtimeResourceIDKey]resources.SourceRecord
	show    map[runtimeResourceKey]resources.SourceRecord
	listErr error
	getErr  error
	showErr error
	calls   []string
}

func (r *runtimeFakeReader) List(_ context.Context, product resources.Product, resource string) ([]resources.SourceRecord, error) {
	r.calls = append(r.calls, "list:"+string(product)+"/"+resource)
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.list[runtimeResourceKey{product: product, resource: resource}], nil
}

func (r *runtimeFakeReader) Get(_ context.Context, product resources.Product, resource string, id string) (resources.SourceRecord, error) {
	r.calls = append(r.calls, "get:"+string(product)+"/"+resource+"/"+id)
	if r.getErr != nil {
		return resources.SourceRecord{}, r.getErr
	}
	return r.get[runtimeResourceIDKey{product: product, resource: resource, id: id}], nil
}

func (r *runtimeFakeReader) Show(_ context.Context, product resources.Product, resource string) (resources.SourceRecord, error) {
	r.calls = append(r.calls, "show:"+string(product)+"/"+resource)
	if r.showErr != nil {
		return resources.SourceRecord{}, r.showErr
	}
	return r.show[runtimeResourceKey{product: product, resource: resource}], nil
}

type runtimeResourceKey struct {
	product  resources.Product
	resource string
}

type runtimeResourceIDKey struct {
	product  resources.Product
	resource string
	id       string
}

func runtimeTestCatalog(t *testing.T, product resources.Product, resource string) resources.ResourceCatalog {
	t.Helper()
	spec, ok := resources.Catalog().FindSpec(product, resource)
	if !ok {
		t.Fatalf("resources.Catalog().FindSpec(%s, %q) ok = false, want true", product, resource)
	}
	return resources.ResourceCatalog{spec}
}

func runtimeWriteConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}
	return path
}

func canceledRuntimeContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

func expiredRuntimeContext() context.Context {
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(0, 0))
	cancel()
	return ctx
}

func assertRuntimeEventKinds(t *testing.T, events []machine.Event, want []machine.EventKind) {
	t.Helper()
	if len(events) != len(want) {
		t.Fatalf("event count = %d, want %d (events = %#v)", len(events), len(want), events)
	}
	for i, kind := range want {
		if events[i].Kind != kind {
			t.Fatalf("event[%d].Kind = %q, want %q (events = %#v)", i, events[i].Kind, kind, events)
		}
	}
}

func runtimeDumpListSpec(product resources.Product, resource string) resources.ResourceSpec {
	return resources.ResourceSpec{
		Product:    product,
		Name:       resource,
		Operations: resources.ListOperations(),
		Fields: []resources.FieldSpec{
			{
				Name:           "id",
				Classification: resources.ClassOperational,
				AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
			},
			{
				Name:           "name",
				Classification: resources.ClassTenantConfig,
				AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
			},
		},
	}
}

type runtimeDumpReader struct {
	list            map[runtimeResourceKey][]resources.SourceRecord
	failures        map[runtimeResourceKey]error
	calls           []string
	directListCalls int
}

func (r *runtimeDumpReader) List(_ context.Context, product resources.Product, resource string) ([]resources.SourceRecord, error) {
	r.calls = append(r.calls, "list:"+string(product)+"/"+resource)
	r.directListCalls++
	key := runtimeResourceKey{product: product, resource: resource}
	if err := r.failures[key]; err != nil {
		return nil, err
	}
	return r.list[key], nil
}

func (r *runtimeDumpReader) Get(_ context.Context, _ resources.Product, _ string, _ string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, errors.New("runtimeDumpReader.Get must not be called")
}

func (r *runtimeDumpReader) Show(_ context.Context, _ resources.Product, _ string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, errors.New("runtimeDumpReader.Show must not be called")
}

type runtimeDumpSessionProvider struct {
	session         *runtimeDumpSession
	sessionCalls    int
	directListCalls int
}

func (r *runtimeDumpSessionProvider) Session(_ context.Context, _ resources.Product) (zscaler.ResourceSession, error) {
	r.sessionCalls++
	return r.session, nil
}

func (r *runtimeDumpSessionProvider) List(_ context.Context, _ resources.Product, _ string) ([]resources.SourceRecord, error) {
	r.directListCalls++
	return nil, nil
}

func (r *runtimeDumpSessionProvider) Get(_ context.Context, _ resources.Product, _ string, _ string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, errors.New("runtimeDumpSessionProvider.Get must not be called")
}

func (r *runtimeDumpSessionProvider) Show(_ context.Context, _ resources.Product, _ string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, errors.New("runtimeDumpSessionProvider.Show must not be called")
}

type runtimeDumpSession struct {
	list       []resources.SourceRecord
	err        error
	listCalls  int
	closeCalls int
}

func (s *runtimeDumpSession) List(_ context.Context, _ resources.Product, _ string) ([]resources.SourceRecord, error) {
	s.listCalls++
	if s.err != nil {
		return nil, s.err
	}
	return s.list, nil
}

func (s *runtimeDumpSession) Get(_ context.Context, _ resources.Product, _ string, _ string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, errors.New("runtimeDumpSession.Get must not be called")
}

func (s *runtimeDumpSession) Show(_ context.Context, _ resources.Product, _ string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, errors.New("runtimeDumpSession.Show must not be called")
}

func (s *runtimeDumpSession) Close() {
	s.closeCalls++
}
