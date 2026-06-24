package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

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
