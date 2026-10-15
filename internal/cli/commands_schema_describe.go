package cli

import (
	"fmt"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/spf13/cobra"
)

// newSchemaDescribeCmd exposes the bounded semantics pilot independently of
// configuration and the existing catalog wire contract.
func (a *App) newSchemaDescribeCmd(opts globalOptions) *cobra.Command {
	return &cobra.Command{
		Use:         "describe <product> <resource>",
		Short:       "describe reviewed field semantics for one resource (config-free pilot)",
		Annotations: map[string]string{"introspect/args-policy": "exact:2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return UsageError{Message: "usage: zscalerctl schema describe <product> <resource>"}
			}
			if opts.format != output.FormatJSON && opts.format != output.FormatTable && opts.format != output.FormatPretty {
				return rejectUnsupportedFormat("schema describe", opts.format)
			}
			if err := cmd.Context().Err(); err != nil {
				return err
			}
			spec, ok := a.resourceCatalog().FindSpec(resources.Product(args[0]), args[1])
			if !ok {
				// Arbitrary command inputs are not reflected into diagnostics.
				return fmt.Errorf("%w: resource is not in the catalog; use schema list", ErrNotFound)
			}
			doc, err := resources.DescribeSemantics(spec, opts.redaction)
			if err != nil {
				return err
			}
			renderer := output.NewRenderer(redact.New(redact.ModeStandard))
			if opts.format == output.FormatJSON {
				return renderer.WriteJSON(a.out, doc)
			}
			var body strings.Builder
			fmt.Fprintf(&body, "%s/%s\treview: %s\tmode: %s\n", doc.Product, doc.Resource, doc.ReviewStatus, doc.RedactionMode)
			if doc.ReviewStatus == resources.SemanticsNotReviewed {
				body.WriteString("No reviewed field semantics in this pilot; use schema list for catalog fields.\n")
			}
			for _, field := range doc.Fields {
				fmt.Fprintf(&body, "%s\t%s\t%s\n", field.Name, field.Type, field.Description)
			}
			writeRuleMatchingText(&body, doc.Matching)
			return renderer.WriteText(a.out, output.NewSafeText(body.String()))
		},
	}
}

func writeRuleMatchingText(body *strings.Builder, matching *resources.RuleMatching) {
	if matching == nil {
		return
	}
	fmt.Fprintf(body, "\nMatching (%s):\n", matching.Source)
	for _, group := range matching.Groups {
		fmt.Fprintf(body, "%s\t%s\t%s\n", group.Category, group.Operator, strings.Join(group.Fields, ", "))
	}
	for _, line := range []struct {
		label  string
		values []string
	}{
		{"Unplaced criteria", matching.Unplaced},
		{"Not rendered in this mode", matching.NotRendered},
		{"Not read", matching.NotRead},
	} {
		if len(line.values) > 0 {
			fmt.Fprintf(body, "%s: %s\n", line.label, strings.Join(line.values, ", "))
		}
	}
	for _, note := range matching.Notes {
		fmt.Fprintf(body, "Note: %s\n", note)
	}
	fmt.Fprintf(body, "%s\n", matching.Logic)
}
