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
	commands := []string{"doctor", "auth", "config", "schema", "dump", "diff", "completion", "version", "help"}
	commands = append(commands, productNames(knownProducts())...)
	sort.Strings(commands)
	return commands
}

func completionProductValues() []string {
	products := productNames(knownProducts())
	values := append([]string(nil), products...)
	if len(products) > 1 {
		values = append(values, strings.Join(products, ","))
	}
	return values
}

// completionDiagnosticVerbs lists per-product diagnostic verbs that are
// dispatched directly in app.go rather than registered as catalog resources, so
// resourceNames (which reads the catalog) omits them. They still need shell
// completion.
//
// url-lookup was listed here before Phase 2b. Phase 2b registered it as a real
// Cobra subcommand of "zia" via newURLLookupCmd, so Cobra now completes it
// automatically from the subcommand list. Keeping it here too produced a
// duplicate entry in __complete zia output. It has been removed.
var completionDiagnosticVerbs = map[resources.Product][]string{}

// completionResourceNames returns the completion candidates for a product's
// second positional word: its catalog resources plus any diagnostic verbs that
// live outside the catalog.
func completionResourceNames(product resources.Product) []string {
	names := resourceNames(product)
	names = append(names, completionDiagnosticVerbs[product]...)
	sort.Strings(names)
	return names
}

func resourceNames(product resources.Product) []string {
	var names []string
	for _, spec := range resources.Catalog() {
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

func dumpResourceNames() []string {
	seen := map[string]struct{}{}
	for _, spec := range resources.Catalog() {
		if !resourceSupportsDump(spec) {
			continue
		}
		seen[spec.Name] = struct{}{}
		seen[string(spec.Product)+"/"+spec.Name] = struct{}{}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func operationNames() []string {
	seen := map[string]struct{}{}
	var names []string
	for _, spec := range resources.Catalog() {
		for _, op := range spec.Operations {
			if op.Capability == resources.CapabilityRead {
				if _, ok := seen[op.Name]; ok {
					continue
				}
				seen[op.Name] = struct{}{}
				names = append(names, op.Name)
			}
		}
	}
	return names
}

func words(values []string) string {
	return strings.Join(values, " ")
}

func powershellArray(values []string) string {
	if len(values) == 0 {
		return "@()"
	}
	quoted := make([]string, len(values))
	for i, value := range values {
		quoted[i] = "'" + strings.ReplaceAll(value, "'", "''") + "'"
	}
	return "@(" + strings.Join(quoted, ", ") + ")"
}
