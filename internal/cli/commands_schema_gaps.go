package cli

import (
	"fmt"
	"slices"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/spf13/cobra"
)

// newSchemaGapsCmd lists the declared coverage gaps, optionally for one
// product, so an absent resource is not mistaken for an empty one. It is
// config-free catalog metadata.
func (a *App) newSchemaGapsCmd(opts globalOptions) *cobra.Command {
	return &cobra.Command{
		Use:         "gaps [product]",
		Short:       "list the Zscaler surfaces zscalerctl does not read, and why (config-free)",
		Annotations: map[string]string{"introspect/args-policy": "max:1"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 1 {
				return UsageError{Message: "usage: zscalerctl schema gaps [product]"}
			}
			if opts.format != output.FormatJSON && opts.format != output.FormatTable && opts.format != output.FormatPretty {
				return rejectUnsupportedFormat("schema gaps", opts.format)
			}
			if err := cmd.Context().Err(); err != nil {
				return err
			}
			var product resources.Product
			if len(args) == 1 {
				product = resources.Product(args[0])
				products := knownProducts(a.resourceCatalog())
				if !slices.Contains(products, product) {
					return UsageError{Message: "usage: zscalerctl schema gaps [product]; product is one of " + strings.Join(productNames(products), ", ")}
				}
			}
			doc := resources.GapsDocument{Gaps: resources.CoverageGaps(product)}
			renderer := output.NewRenderer(redact.New(redact.ModeStandard))
			if opts.format == output.FormatJSON {
				return renderer.WriteJSON(a.out, doc)
			}
			var body strings.Builder
			for _, gap := range doc.Gaps {
				instead := ""
				if len(gap.Instead) > 0 {
					names := make([]string, len(gap.Instead))
					for i, ref := range gap.Instead {
						names[i] = ref.String()
					}
					instead = "\tinstead: " + strings.Join(names, ", ")
				}
				fmt.Fprintf(&body, "%s %s\t%s\t%s\t%s%s\n", gap.Product, gap.Surface, gap.Status, gap.Reason, gap.Note, instead)
			}
			return renderer.WriteText(a.out, output.NewSafeText(body.String()))
		},
	}
}
