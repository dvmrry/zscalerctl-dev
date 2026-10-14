package cli

import (
	"fmt"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/spf13/cobra"
)

// newSchemaReferencesCmd lists the catalog fields that point at one resource,
// with the --filter path that holds its id. It is config-free catalog
// metadata: it answers "where is this resource used" without a live read.
func (a *App) newSchemaReferencesCmd(opts globalOptions) *cobra.Command {
	return &cobra.Command{
		Use:         "references <product> <resource>",
		Short:       "list the fields that point at a resource, with the --filter path for its id (config-free)",
		Annotations: map[string]string{"introspect/args-policy": "exact:2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return UsageError{Message: "usage: zscalerctl schema references <product> <resource>"}
			}
			if opts.format != output.FormatJSON && opts.format != output.FormatTable && opts.format != output.FormatPretty {
				return rejectUnsupportedFormat("schema references", opts.format)
			}
			if err := cmd.Context().Err(); err != nil {
				return err
			}
			catalog := a.resourceCatalog()
			spec, ok := catalog.FindSpec(resources.Product(args[0]), args[1])
			if !ok {
				// Arbitrary command inputs are not reflected into diagnostics.
				return fmt.Errorf("%w: resource is not in the catalog; use schema list", ErrNotFound)
			}
			doc := resources.ReferencesTo(catalog, resources.ResourceRef{Product: spec.Product, Resource: spec.Name})
			renderer := output.NewRenderer(redact.New(redact.ModeStandard))
			if opts.format == output.FormatJSON {
				return renderer.WriteJSON(a.out, doc)
			}
			var body strings.Builder
			if len(doc.ReferredBy) == 0 {
				fmt.Fprintf(&body, "No catalog field is known to point at %s.\n", doc.Target)
			}
			for _, site := range doc.ReferredBy {
				targets := ""
				if len(site.Targets) > 1 {
					names := make([]string, len(site.Targets))
					for i, target := range site.Targets {
						names[i] = target.String()
					}
					targets = "\tid is one of " + strings.Join(names, ", ")
				}
				if site.Match == "" {
					fmt.Fprintf(&body, "%s %s\t%s\tnot rendered in any mode%s\n", site.Product, site.Resource, site.Field, targets)
					continue
				}
				modes := make([]string, len(site.AllowedModes))
				for i, mode := range site.AllowedModes {
					modes[i] = string(mode)
				}
				fmt.Fprintf(&body, "%s %s\t--filter %s=<id>\t%s%s\n", site.Product, site.Resource, site.Match, strings.Join(modes, ","), targets)
			}
			return renderer.WriteText(a.out, output.NewSafeText(body.String()))
		},
	}
}
