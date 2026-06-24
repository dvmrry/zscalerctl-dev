package cli

import (
	"io"
	"sort"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/spf13/cobra"
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
// drift gate and agent-docs drift gate. It derives the set from the live Cobra
// tree so it cannot drift from the actual command surface.
func completionCommandNames() []string {
	a := New(io.Discard, io.Discard, nil)
	root := BuildCommandTree(a)
	root.InitDefaultCompletionCmd()

	var commands []string
	WalkCobraTree(root, func(cmd *cobra.Command, path string) {
		// Only top-level (depth-1) commands.
		if strings.Contains(path, " ") {
			return
		}
		if cmd.Hidden || strings.HasPrefix(cmd.Name(), "__complete") {
			return
		}
		commands = append(commands, path)
	})

	sort.Strings(commands)
	return commands
}

// completionResourceNames returns the completion candidates for a product's
// second positional word from the resolved catalog. It is a method on App so the
// test-injected catalog (set via NewWithOptions) is respected consistently.
func (a *App) completionResourceNames(product resources.Product) []string {
	return a.resourceNames(product)
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

func allResourceNames(catalog resources.ResourceCatalog) []string {
	seen := map[string]struct{}{}
	for _, spec := range catalog {
		seen[spec.Name] = struct{}{}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
