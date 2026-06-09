package cli

import (
	"bytes"
	"strings"
	"testing"

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
