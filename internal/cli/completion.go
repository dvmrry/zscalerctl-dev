package cli

import (
	"sort"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/resources"
)

// completionFlags, completionDumpFlags, and completionDiffFlags are defined in
// surface.go — the durable, drift-proof surface inventory for completion,
// man-page, and agent-docs gates.
var (
	completionFormats   = []string{"auto", "table", "json", "ndjson", "pretty"}
	completionRedaction = []string{"standard", "share", "paranoid"}
	completionColors    = []string{"auto", "always", "never"}
	completionLogLevels = []string{"off", "error", "warn", "info", "debug"}
	completionShells    = []string{"bash", "zsh", "fish", "powershell"}
)

func completionShellNames() string {
	return strings.Join(completionShells, "|")
}

// completionCommandNames returns the top-level command names for the man-page
// drift gate and agent-docs drift gate. It must stay in sync with the actual
// command tree.
func completionCommandNames() []string {
	commands := []string{"doctor", "auth", "config", "schema", "dump", "diff", "completion", "version", "help", "introspect"}
	commands = append(commands, productNames(knownProducts())...)
	sort.Strings(commands)
	return commands
}

// completionResourceNames returns the completion candidates for a product's
// second positional word. It is a method on App so the test-injected catalog
// (set via NewWithOptions) is respected consistently (N-2).
func (a *App) completionResourceNames(product resources.Product) []string {
	names := a.resourceNames(product)
	sort.Strings(names)
	return names
}

func (a *App) resourceNames(product resources.Product) []string {
	var names []string
	for _, spec := range a.resourceCatalog() {
		if spec.Product == product {
			names = append(names, spec.Name)
		}
	}
	sort.Strings(names)
	return names
}

func allResourceNames() []string {
	seen := map[string]struct{}{}
	for _, spec := range resources.Catalog() {
		seen[spec.Name] = struct{}{}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
