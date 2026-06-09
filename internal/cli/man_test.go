package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestManPageDocumentsFlagsAndCommands guards man/zscalerctl.1 against drift:
// every global flag and every command must be documented. It is a subset check
// (extras in the man page are allowed), so the page can document a flag slightly
// ahead of, or behind, an in-flight flag change without flapping.
func TestManPageDocumentsFlagsAndCommands(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile(filepath.Join("..", "..", "man", "zscalerctl.1"))
	if err != nil {
		t.Fatalf("read man page: %v", err)
	}
	// roff escapes literal hyphens as "\-"; normalize so flag names match.
	content := strings.ReplaceAll(string(body), `\-`, "-")

	for _, flag := range completionFlags {
		if !strings.Contains(content, flag) {
			t.Errorf("man/zscalerctl.1 does not document global flag %q", flag)
		}
	}
	for _, cmd := range completionCommandNames() {
		if !strings.Contains(content, cmd) {
			t.Errorf("man/zscalerctl.1 does not document command %q", cmd)
		}
	}
}
