package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// writeHelp prints the global usage. It is only reachable when rest is empty
// (all recognized commands, including products, are routed through Cobra in
// runParsed before writeHelp is called). The per-command and per-product cases
// that previously lived here were dead code.
func (a *App) writeHelp(w io.Writer, rest []string) {
	a.writeUsage(w, a.resourceCatalog())
}

func (a *App) writeUsage(w io.Writer, catalog resources.ResourceCatalog) {
	fmt.Fprintln(w, "usage: zscalerctl [global flags] <command> [args]")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "products: %s\n", strings.Join(productNames(knownProducts(catalog)), ", "))
	fmt.Fprintln(w)
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  help [command]")
	fmt.Fprintln(w, "  doctor")
	fmt.Fprintln(w, "  auth status")
	fmt.Fprintln(w, "  config show")
	fmt.Fprintln(w, "  config init [--force]")
	fmt.Fprintln(w, "  zia url-lookup <url> [url...]")
	fmt.Fprintln(w, "  schema list")
	fmt.Fprintln(w, "  introspect")
	fmt.Fprintln(w, "  machine manifest")
	fmt.Fprintln(w, "  dump --out <dir> [--products names] [--resources names] [--continue-on-error] [--force]")
	fmt.Fprintln(w, "  diff <old-dump-dir> <new-dump-dir> [--products names] [--resources names] [--ignore-operational] [--detail] [--allow-partial] [--fail-on-drift]")
	fmt.Fprintf(w, "  completion %s\n", completionShellNames())
	fmt.Fprintln(w, "  version")
	for _, product := range knownProducts(catalog) {
		fmt.Fprintf(w, "  %s <resource> %s\n", product, strings.Join(productReadOperationNames(product, catalog), "|"))
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "global flags:")
	fmt.Fprintln(w, "  --profile <name>")
	fmt.Fprintln(w, "  --config <path>")
	fmt.Fprintln(w, "  --format auto|table|json|ndjson|pretty")
	fmt.Fprintln(w, "  --output <path>")
	fmt.Fprintln(w, "  --timeout <duration>")
	fmt.Fprintln(w, "  --redaction standard|share|paranoid")
	fmt.Fprintln(w, "  --color auto|always|never")
	fmt.Fprintln(w, "  --no-color")
	fmt.Fprintln(w, "  --no-cache")
	fmt.Fprintln(w, "  --log-level off|error|warn|info|debug")
	fmt.Fprintln(w, "  --fields <a,b,c>")
	fmt.Fprintln(w, "  --filter <key=value|key~value>   (list only; repeatable, all must match)")
	fmt.Fprintln(w, "  --search <term>                  (list only; case-insensitive, any field)")
}

func dumpUsage(catalog resources.ResourceCatalog) string {
	return fmt.Sprintf(
		"usage: zscalerctl dump --out <dir> [--products %s] [--resources names] [--continue-on-error] [--force]\n"+
			"tip: add --log-level info to see start, per-resource, and completion progress on stderr during a long dump",
		strings.Join(productNames(knownProducts(catalog)), ","),
	)
}

func diffUsage(catalog resources.ResourceCatalog) string {
	return fmt.Sprintf(
		"usage: zscalerctl diff <old-dump-dir> <new-dump-dir> [--products %s] [--resources names] [--ignore-operational] [--detail] [--allow-partial] [--fail-on-drift]",
		strings.Join(productNames(knownProducts(catalog)), ","),
	)
}

// columnize lays out names in a left-aligned, column-major grid (alphabetical
// down each column, like `ls`) indented two spaces, packed to fit width
// columns. width <= 0 falls back to 80, keeping error messages and
// non-terminal output deterministic. Returns the block without a trailing
// newline.
func columnize(names []string, width int) string {
	if len(names) == 0 {
		return ""
	}
	if width <= 0 {
		width = 80
	}
	const indent, gap = 2, 2
	longest := 0
	for _, n := range names {
		if len(n) > longest {
			longest = len(n)
		}
	}
	colWidth := longest + gap
	cols := (width - indent + gap) / colWidth
	if cols < 1 {
		cols = 1
	}
	rows := (len(names) + cols - 1) / cols
	var b strings.Builder
	for r := 0; r < rows; r++ {
		var line strings.Builder
		line.WriteString(strings.Repeat(" ", indent))
		for c := 0; c < cols; c++ {
			i := c*rows + r
			if i >= len(names) {
				break
			}
			line.WriteString(names[i])
			line.WriteString(strings.Repeat(" ", colWidth-len(names[i])))
		}
		b.WriteString(strings.TrimRight(line.String(), " "))
		if r < rows-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func productCommandUsage(product resources.Product, width int, catalog resources.ResourceCatalog) string {
	// Enumerate the product's resources so a cold caller (human or agent) can
	// discover real names from --help or a usage error instead of guessing;
	// `schema list` remains the machine-readable source of truth.
	names := make([]string, 0, 64)
	for _, spec := range catalog {
		if spec.Product == product {
			names = append(names, spec.Name)
		}
	}
	sort.Strings(names)
	msg := fmt.Sprintf(
		"usage: zscalerctl %s <resource> %s\n\nresources (%d; see also: zscalerctl --format json schema list):\n%s",
		product,
		strings.Join(productReadOperationNames(product, catalog), "|"),
		len(names),
		columnize(names, width),
	)
	if product == resources.ProductZIA {
		msg += "\n\ndiagnostics:\n  zscalerctl zia url-lookup <url> [url...]"
	}
	return msg
}

// resourceUsage builds help for a known resource: its supported read operations
// plus the renderable field names (standard mode), so the operator can discover
// what to pass to --fields without consulting `schema list`.
func resourceUsage(product resources.Product, spec resources.ResourceSpec, width int) string {
	msg := fmt.Sprintf(
		"usage: zscalerctl %s %s %s",
		product,
		spec.Name,
		strings.Join(readOperationNames(spec), "|"),
	)
	if fields := spec.FieldOrder(redact.ModeStandard); len(fields) > 0 {
		msg += "\nfields:\n" + columnize(fields, width)
	}
	return msg
}
