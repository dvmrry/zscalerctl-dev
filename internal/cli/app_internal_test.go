package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
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
