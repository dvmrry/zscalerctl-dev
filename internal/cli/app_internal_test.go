package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/spf13/cobra"
)

func TestParseDumpResourcesSupportsCatalogDerivedQualifiedProducts(t *testing.T) {
	t.Parallel()

	product := resources.Product("ztw")
	catalog := resources.ResourceCatalog{
		{
			Product:    product,
			Name:       "workload-groups",
			Operations: resources.ReadOperations(),
		},
	}
	products := map[resources.Product]bool{product: true}

	selected, err := parseDumpResources("ztw/workload-groups", products, catalog)
	if err != nil {
		t.Fatalf("parseDumpResources(ztw/workload-groups) error = %v, want nil", err)
	}
	want := dumpResourceKey{product: product, name: "workload-groups"}
	if len(selected) != 1 || !selected[want] {
		t.Fatalf("parseDumpResources(ztw/workload-groups) = %#v, want only %#v", selected, want)
	}
}

func TestNewDiagLoggerLevelsAndLeakSafety(t *testing.T) {
	t.Parallel()

	// off (default) discards everything.
	var off bytes.Buffer
	logger, err := newDiagLogger(&off, "off")
	if err != nil {
		t.Fatalf("newDiagLogger(off) error = %v", err)
	}
	logger.Error("should not appear")
	logger.Info("should not appear")
	if off.Len() != 0 {
		t.Fatalf("log-level off wrote %q, want empty", off.String())
	}

	// info emits info+ to the provided writer (stderr in production) but not debug.
	var buf bytes.Buffer
	logger, err = newDiagLogger(&buf, "info")
	if err != nil {
		t.Fatalf("newDiagLogger(info) error = %v", err)
	}
	logger.Debug("debug line")
	logger.Info("dump complete", "resources", 11, "errors", 0)
	out := buf.String()
	if strings.Contains(out, "debug line") {
		t.Errorf("info level emitted a debug line: %q", out)
	}
	if !strings.Contains(out, "dump complete") || !strings.Contains(out, "resources=11") {
		t.Errorf("info level missing expected metadata: %q", out)
	}

	// Invalid level is rejected.
	if _, err := newDiagLogger(&buf, "bogus"); err == nil {
		t.Error("newDiagLogger(bogus) error = nil, want validation error")
	}
}

func TestEffectiveFieldsNarrowsValidatesAndCannotWiden(t *testing.T) {
	t.Parallel()
	spec := resources.ResourceSpec{
		Product: "zia",
		Name:    "things",
		Fields: []resources.FieldSpec{
			{Name: "id", Classification: resources.ClassOperational, AllowedModes: []redact.Mode{redact.ModeStandard, redact.ModeShare}},
			{Name: "name", Classification: resources.ClassTenantConfig, AllowedModes: []redact.Mode{redact.ModeStandard, redact.ModeShare}},
			{Name: "token", Classification: resources.ClassSecret},
		},
	}

	// No --fields: full renderable order.
	got, err := effectiveFields(spec, redact.ModeStandard, nil)
	if err != nil || strings.Join(got, ",") != "id,name" {
		t.Fatalf("default = %v, %v; want [id name], nil", got, err)
	}

	// Narrowed and reordered to the request.
	got, err = effectiveFields(spec, redact.ModeStandard, []string{"name", "id"})
	if err != nil || strings.Join(got, ",") != "name,id" {
		t.Fatalf("narrow = %v, %v; want [name id], nil", got, err)
	}

	// A secret (non-rendered) field is known but silently dropped: cannot widen.
	got, err = effectiveFields(spec, redact.ModeStandard, []string{"token"})
	if err != nil || len(got) != 0 {
		t.Fatalf("secret request = %v, %v; want [], nil", got, err)
	}

	// An unknown field is a usage error.
	if _, err := effectiveFields(spec, redact.ModeStandard, []string{"nope"}); err == nil {
		t.Fatal("unknown field: err = nil, want usage error")
	}
}

func TestRunProductReadOperationsRouteThroughMachineExecutor(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		response     machine.Response
		wantOp       machine.Operation
		wantProduct  string
		wantResource string
		wantRecordID string
		wantArray    bool
	}{
		{
			name: "list",
			args: []string{"--format", "json", "zia", "locations", "list"},
			response: machine.Response{
				Records: []map[string]any{{"id": "1", "name": "From machine list"}},
			},
			wantOp:       machine.OperationList,
			wantProduct:  "zia",
			wantResource: "locations",
			wantArray:    true,
		},
		{
			name: "show",
			args: []string{"--format", "json", "zia", "advanced-settings", "show"},
			response: machine.Response{
				Records: []map[string]any{{"id": "settings", "name": "From machine show"}},
			},
			wantOp:       machine.OperationShow,
			wantProduct:  "zia",
			wantResource: "advanced-settings",
			wantArray:    false,
		},
		{
			name: "get",
			args: []string{"--format", "json", "zia", "locations", "get", "42"},
			response: machine.Response{
				Records: []map[string]any{{"id": "42", "name": "From machine get"}},
			},
			wantOp:       machine.OperationGet,
			wantProduct:  "zia",
			wantResource: "locations",
			wantRecordID: "42",
			wantArray:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			app := NewWithOptions(&out, &errOut, nil, Options{
				Reader:  &machineRouteReader{},
				Catalog: machineRouteCatalog(),
			})
			executor := &recordingMachineReadExecutor{response: tt.response}
			loaderFactoryCalled := false
			app.machineReadExecutorFactory = func(loader machine.BrowserLoader) machineReadExecutor {
				if loader == nil {
					t.Fatal("machine executor factory received nil loader")
				}
				loaderFactoryCalled = true
				return executor
			}

			if err := app.Run(context.Background(), tt.args); err != nil {
				t.Fatalf("App.Run(%s) error = %v, want nil", tt.name, err)
			}
			if !loaderFactoryCalled {
				t.Fatal("machine executor factory was not called")
			}
			if len(executor.calls) != 1 {
				t.Fatalf("machine executor calls = %d, want 1", len(executor.calls))
			}
			req := executor.calls[0]
			if req.Capability != machine.CapabilityResourcesRead ||
				req.Operation != tt.wantOp ||
				req.Input == nil ||
				req.Input.Product != tt.wantProduct ||
				req.Input.Resource != tt.wantResource ||
				req.Input.RecordID != tt.wantRecordID {
				t.Fatalf("machine request = %#v, want resources.read %s %s/%s record_id=%q",
					req, tt.wantOp, tt.wantProduct, tt.wantResource, tt.wantRecordID)
			}
			if errOut.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", errOut.String())
			}
			if tt.wantArray {
				var decoded []map[string]any
				if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
					t.Fatalf("json.Unmarshal(list output) error = %v; output = %q", err, out.String())
				}
				if len(decoded) != 1 || decoded[0]["name"] != tt.response.Records[0]["name"] {
					t.Fatalf("list output = %#v, want machine response record", decoded)
				}
				return
			}
			var decoded map[string]any
			if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
				t.Fatalf("json.Unmarshal(%s output) error = %v; output = %q", tt.name, err, out.String())
			}
			if decoded["name"] != tt.response.Records[0]["name"] {
				t.Fatalf("%s output = %#v, want machine response record", tt.name, decoded)
			}
		})
	}
}

func TestRunProductMachineLoaderErrorPreservesOriginalError(t *testing.T) {
	sentinel := errors.New("backend sentinel")
	var out, errOut bytes.Buffer
	app := NewWithOptions(&out, &errOut, nil, Options{
		Reader: &machineRouteReader{listErr: sentinel},
		Catalog: resources.ResourceCatalog{{
			Product:    resources.ProductZIA,
			Name:       "locations",
			Operations: resources.ListOperations(),
			Fields:     machineRouteFields(),
		}},
	})

	err := app.Run(context.Background(), []string{"--format", "json", "zia", "locations", "list"})
	if !errors.Is(err, sentinel) {
		t.Fatalf("App.Run(machine loader error) error = %v, want original sentinel", err)
	}
	var machineErr *machine.MachineError
	if errors.As(err, &machineErr) {
		t.Fatalf("App.Run(machine loader error) error = %#v, want original loader error", machineErr)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
	if errOut.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", errOut.String())
	}
}

func TestRunProductMachineGetLoaderErrorPreservesOriginalError(t *testing.T) {
	sentinel := errors.New("backend get sentinel")
	reader := &machineRouteReader{getErr: sentinel}
	var out, errOut bytes.Buffer
	app := NewWithOptions(&out, &errOut, nil, Options{
		Reader:  reader,
		Catalog: machineRouteCatalog(),
	})

	err := app.Run(context.Background(), []string{"--format", "json", "zia", "locations", "get", "42"})
	if !errors.Is(err, sentinel) {
		t.Fatalf("App.Run(machine get loader error) error = %v, want original sentinel", err)
	}
	var machineErr *machine.MachineError
	if errors.As(err, &machineErr) {
		t.Fatalf("App.Run(machine get loader error) error = %#v, want original loader error", machineErr)
	}
	if reader.getCalls != 1 {
		t.Fatalf("reader get calls = %d, want 1", reader.getCalls)
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
	if errOut.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", errOut.String())
	}
}

func TestRunProductMachineExecutorUsageErrorMapsToCLIUsage(t *testing.T) {
	var out, errOut bytes.Buffer
	app := NewWithOptions(&out, &errOut, nil, Options{
		Reader:  &machineRouteReader{},
		Catalog: machineRouteCatalog(),
	})
	app.machineReadExecutorFactory = func(machine.BrowserLoader) machineReadExecutor {
		return &recordingMachineReadExecutor{err: &machine.MachineError{
			Kind:    machine.ErrorKindUsage,
			Message: "missing required input: input.product",
		}}
	}

	err := app.Run(context.Background(), []string{"--format", "json", "zia", "locations", "list"})
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("App.Run(machine executor usage error) error = %v, want ErrUsage", err)
	}
	if !strings.Contains(err.Error(), "missing required input: input.product") {
		t.Fatalf("App.Run(machine executor usage error) error = %q, want machine message", err.Error())
	}
	if out.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", out.String())
	}
	if errOut.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", errOut.String())
	}
}

func TestRunProductGetRoutesThroughMachineExecutorWithoutDirectReaderGet(t *testing.T) {
	reader := &machineRouteReader{
		get: resources.NewSourceRecord(map[string]any{
			"id":   "42",
			"name": "Direct get",
		}),
	}
	var out, errOut bytes.Buffer
	app := NewWithOptions(&out, &errOut, nil, Options{
		Reader:  reader,
		Catalog: machineRouteCatalog(),
	})
	executor := &recordingMachineReadExecutor{
		response: machine.Response{
			Records: []map[string]any{{"id": "42", "name": "Machine get"}},
		},
	}
	app.machineReadExecutorFactory = func(loader machine.BrowserLoader) machineReadExecutor {
		if _, ok := loader.(machine.ProjectedRecordGetter); !ok {
			t.Fatal("machine executor factory loader does not implement ProjectedRecordGetter")
		}
		return executor
	}

	err := app.Run(context.Background(), []string{"--format", "json", "zia", "locations", "get", "42"})
	if err != nil {
		t.Fatalf("App.Run(zia locations get 42) error = %v, want nil", err)
	}
	if reader.getCalls != 0 {
		t.Fatalf("reader get calls = %d, want 0", reader.getCalls)
	}
	if reader.listCalls != 0 || reader.showCalls != 0 {
		t.Fatalf("reader list/show calls = %d/%d, want 0/0", reader.listCalls, reader.showCalls)
	}
	if len(executor.calls) != 1 {
		t.Fatalf("machine executor calls = %d, want 1", len(executor.calls))
	}
	req := executor.calls[0]
	if req.Operation != machine.OperationGet ||
		req.Input == nil ||
		req.Input.Product != "zia" ||
		req.Input.Resource != "locations" ||
		req.Input.RecordID != "42" {
		t.Fatalf("machine request = %#v, want get zia/locations record_id=42", req)
	}
	var decoded map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(get output) error = %v; output = %q", err, out.String())
	}
	if decoded["name"] != "Machine get" {
		t.Fatalf("get output = %#v, want machine response record", decoded)
	}
	if errOut.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", errOut.String())
	}
}

func TestSanitizeCellValueCollapsesControlChars(t *testing.T) {
	t.Parallel()

	got := sanitizeCellValue("line one\nline two\twith tab\rend")
	if strings.ContainsAny(got, "\n\t\r") {
		t.Errorf("sanitizeCellValue = %q, want no control chars", got)
	}
	if got != "line one line two with tab end" {
		t.Errorf("sanitizeCellValue = %q, want control chars collapsed to spaces", got)
	}
}

func TestFormatTableValueSanitizesNestedAndScalarValues(t *testing.T) {
	t.Parallel()

	if got := formatTableValue("a\nb"); got != "a b" {
		t.Errorf("formatTableValue(string with newline) = %q, want %q", got, "a b")
	}
	if got := formatTableValue([]any{"x\ty", "z"}); got != "x y,z" {
		t.Errorf("formatTableValue([]any with tab) = %q, want %q", got, "x y,z")
	}
}

type recordingMachineReadExecutor struct {
	response machine.Response
	err      error
	calls    []machine.Request
}

func (e *recordingMachineReadExecutor) Execute(_ context.Context, req machine.Request) (machine.Response, error) {
	e.calls = append(e.calls, req)
	if e.err != nil {
		return machine.Response{}, e.err
	}
	return e.response, nil
}

type machineRouteReader struct {
	list      []resources.SourceRecord
	get       resources.SourceRecord
	show      resources.SourceRecord
	listErr   error
	getErr    error
	showErr   error
	listCalls int
	getCalls  int
	showCalls int
}

func (r *machineRouteReader) List(_ context.Context, _ resources.Product, _ string) ([]resources.SourceRecord, error) {
	r.listCalls++
	if r.listErr != nil {
		return nil, r.listErr
	}
	if r.list != nil {
		return r.list, nil
	}
	return []resources.SourceRecord{resources.NewSourceRecord(map[string]any{
		"id":   "1",
		"name": "Direct list",
	})}, nil
}

func (r *machineRouteReader) Get(_ context.Context, _ resources.Product, _ string, _ string) (resources.SourceRecord, error) {
	r.getCalls++
	if r.getErr != nil {
		return resources.SourceRecord{}, r.getErr
	}
	return r.get, nil
}

func (r *machineRouteReader) Show(_ context.Context, _ resources.Product, _ string) (resources.SourceRecord, error) {
	r.showCalls++
	if r.showErr != nil {
		return resources.SourceRecord{}, r.showErr
	}
	return r.show, nil
}

func machineRouteCatalog() resources.ResourceCatalog {
	return resources.ResourceCatalog{
		{
			Product:    resources.ProductZIA,
			Name:       "locations",
			Operations: resources.ReadOperations(),
			Fields:     machineRouteFields(),
		},
		{
			Product:    resources.ProductZIA,
			Name:       "advanced-settings",
			Operations: resources.ShowOperation(),
			Fields:     machineRouteFields(),
		},
	}
}

func machineRouteFields() []resources.FieldSpec {
	allModes := []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid}
	return []resources.FieldSpec{
		{
			Name:           "id",
			Classification: resources.ClassOperational,
			AllowedModes:   allModes,
		},
		{
			Name:           "name",
			Classification: resources.ClassTenantConfig,
			AllowedModes:   allModes,
		},
	}
}

func TestColumnize(t *testing.T) {
	t.Parallel()

	// width 12, longest name 2 → colWidth 4 → 3 columns, column-major (down each
	// column), trailing pad trimmed per line.
	if got, want := columnize([]string{"aa", "bb", "cc", "dd", "ee"}, 12), "  aa  cc  ee\n  bb  dd"; got != want {
		t.Errorf("columnize(3-col) = %q, want %q", got, want)
	}
	// Too narrow for two columns → one per line.
	if got, want := columnize([]string{"aaaa", "bbbb"}, 3), "  aaaa\n  bbbb"; got != want {
		t.Errorf("columnize(1-col) = %q, want %q", got, want)
	}
	if got := columnize(nil, 80); got != "" {
		t.Errorf("columnize(nil) = %q, want empty", got)
	}
}

// TestWriteUsageCoversAllCobraCommands asserts that every non-hidden,
// non-__complete, depth-1 utility command (i.e. not a product group node) is
// mentioned somewhere in the global usage text. This is the durability gate
// that caught the missing `introspect` line.
func TestWriteUsageCoversAllCobraCommands(t *testing.T) {
	t.Parallel()

	a := New(io.Discard, io.Discard, nil)
	root := BuildCommandTree(a)
	root.InitDefaultCompletionCmd()

	productSet := make(map[string]struct{})
	for _, product := range knownProducts(a.resourceCatalog()) {
		productSet[string(product)] = struct{}{}
	}

	var want []string
	WalkCobraTree(root, func(cmd *cobra.Command, path string) {
		// Only depth-1 commands.
		if strings.Contains(path, " ") {
			return
		}
		if cmd.Hidden || strings.HasPrefix(cmd.Name(), "__complete") {
			return
		}
		// Product group nodes are handled separately in the usage text.
		if _, ok := productSet[path]; ok {
			return
		}
		want = append(want, path)
	})

	var b bytes.Buffer
	a.writeUsage(&b, a.resourceCatalog())
	usage := b.String()

	containsCommand := func(usage, name string) bool {
		for _, line := range strings.Split(usage, "\n") {
			trimmed := strings.TrimLeft(line, " \t")
			if trimmed == name || strings.HasPrefix(trimmed, name+" ") || strings.HasPrefix(trimmed, name+"\t") {
				return true
			}
		}
		return false
	}

	for _, name := range want {
		if !containsCommand(usage, name) {
			t.Errorf("writeUsage output missing top-level command %q as a standalone entry", name)
		}
	}

	// Negative check: a fake top-level command must not be spuriously matched.
	t.Run("fake command not matched", func(t *testing.T) {
		if containsCommand(usage, "auth-status") {
			t.Errorf("coverage check spuriously accepted fake command %q", "auth-status")
		}
	})
}

// TestCatalogSourceOfTruth verifies that an injected catalog is the single source
// of truth for the command tree, introspection output, and shell completion.
func TestCatalogSourceOfTruth(t *testing.T) {
	t.Parallel()

	product := resources.Product("testp")
	catalog := resources.ResourceCatalog{
		{
			Product:    product,
			Name:       "widgets",
			Operations: resources.ReadOperations(),
			Fields: []resources.FieldSpec{
				{Name: "id", Classification: resources.ClassOperational, AllowedModes: []redact.Mode{redact.ModeStandard}},
				{Name: "name", Classification: resources.ClassTenantConfig, AllowedModes: []redact.Mode{redact.ModeStandard}},
			},
		},
		{
			Product:    product,
			Name:       "gadgets",
			Operations: []resources.Operation{{Name: "show", Capability: resources.CapabilityRead}},
			Fields: []resources.FieldSpec{
				{Name: "id", Classification: resources.ClassOperational, AllowedModes: []redact.Mode{redact.ModeStandard}},
			},
		},
	}

	var out, errBuf bytes.Buffer
	a := NewWithOptions(&out, &errBuf, nil, Options{
		Catalog: catalog,
	})

	// Command tree should contain exactly the product from the injected catalog.
	root := a.buildCommandTree(globalOptions{})
	var productNames []string
	for _, cmd := range root.Commands() {
		if !cmd.Hidden {
			productNames = append(productNames, cmd.Name())
		}
	}
	if !contains(productNames, string(product)) {
		t.Errorf("command tree missing product %q; got commands %v", product, productNames)
	}

	// Introspection catalog should match the injected catalog.
	doc := IntrospectTree(a)
	if len(doc.Catalog.Products) != 1 || doc.Catalog.Products[0] != string(product) {
		t.Errorf("introspect products = %v, want [%s]", doc.Catalog.Products, product)
	}
	if len(doc.Catalog.Resources) != 2 {
		t.Errorf("introspect resources count = %d, want 2", len(doc.Catalog.Resources))
	}

	// Completion should use the injected catalog.
	resources := a.completionResourceNames(product)
	wantResources := []string{"gadgets", "widgets"}
	if strings.Join(resources, ",") != strings.Join(wantResources, ",") {
		t.Errorf("completion resources = %v, want %v", resources, wantResources)
	}

	// Resource hints in usage errors should use the injected catalog.
	msg := unknownCommandMessage("widgets", catalog)
	if !strings.Contains(msg, "is a resource") {
		t.Errorf("unknownCommandMessage for injected resource = %q, want resource hint", msg)
	}

	err := a.Run(context.Background(), []string{"widgets"})
	if err == nil {
		t.Fatal("App.Run(widgets) error = nil, want usage error with resource hint")
	}
	if !strings.Contains(err.Error(), "is a resource") || !strings.Contains(err.Error(), "<product> widgets") {
		t.Errorf("App.Run(widgets) error = %q, want injected resource hint", err)
	}
}

// TestEmptyCatalogProducesConsistentSurface verifies that an injected empty
// catalog does not panic and produces a consistent, empty surface.
func TestEmptyCatalogProducesConsistentSurface(t *testing.T) {
	t.Parallel()

	var out, errBuf bytes.Buffer
	a := NewWithOptions(&out, &errBuf, nil, Options{
		Catalog: resources.ResourceCatalog{},
	})

	// Command tree should contain no product commands.
	root := a.buildCommandTree(globalOptions{})
	for _, cmd := range root.Commands() {
		if cmd.Name() == "testp" {
			t.Errorf("command tree contains product %q from empty catalog", cmd.Name())
		}
	}

	// Introspection should report an empty catalog.
	doc := IntrospectTree(a)
	if len(doc.Catalog.Products) != 0 {
		t.Errorf("introspect products from empty catalog = %v, want []", doc.Catalog.Products)
	}
	if len(doc.Catalog.Resources) != 0 {
		t.Errorf("introspect resources from empty catalog = %d, want 0", len(doc.Catalog.Resources))
	}

	// Bare invocation with no args should still exit 2 without panicking.
	if err := a.Run(context.Background(), nil); err == nil {
		t.Error("Run with empty args and empty catalog error = nil, want usage error")
	} else if !errors.Is(err, ErrUsage) {
		t.Errorf("Run with empty args and empty catalog error = %v, want ErrUsage", err)
	}

	// Help should still render without panicking.
	if err := a.Run(context.Background(), []string{"--help"}); err != nil {
		t.Errorf("Run --help with empty catalog error = %v, want nil", err)
	}
	if !strings.Contains(out.String(), "zscalerctl") {
		t.Errorf("Run --help output = %q, want zscalerctl help", out.String())
	}
}

func contains(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}
