package resources_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/resources"
)

// TestResourcesDocDocumentsEveryCatalogResource guards docs/RESOURCES.md against
// the drift that already bit the README: a resource added to the catalog but
// never written into the reference doc. It is a presence check (every enabled
// resource name must appear as a token in the doc), not a field-level diff —
// the catalog and `schema list` remain the authoritative field model, but the
// human reference must at least mention every resource it claims to cover.
func TestResourcesDocDocumentsEveryCatalogResource(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile(filepath.Join("..", "..", "docs", "RESOURCES.md"))
	if err != nil {
		t.Fatalf("read docs/RESOURCES.md: %v", err)
	}
	content := string(body)

	for _, spec := range resources.Catalog() {
		if !docMentionsToken(content, spec.Name) {
			t.Errorf("docs/RESOURCES.md does not document resource %s/%s; add it to the resource reference", spec.Product, spec.Name)
		}
	}
}

// docMentionsToken reports whether token appears in content delimited by
// non-identifier characters on both sides, so a hyphenated resource name like
// "rule-labels" matches the whole name and not a substring of a longer token.
func docMentionsToken(content, token string) bool {
	isBoundary := func(b byte) bool {
		switch {
		case b >= 'A' && b <= 'Z', b >= 'a' && b <= 'z', b >= '0' && b <= '9', b == '_', b == '-':
			return false
		default:
			return true
		}
	}
	for offset := 0; ; {
		idx := strings.Index(content[offset:], token)
		if idx < 0 {
			return false
		}
		start := offset + idx
		end := start + len(token)
		beforeOK := start == 0 || isBoundary(content[start-1])
		afterOK := end == len(content) || isBoundary(content[end])
		if beforeOK && afterOK {
			return true
		}
		offset = start + 1
	}
}
