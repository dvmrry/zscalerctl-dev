package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/spf13/cobra"
)

// newRefsCmd answers "where is this object referenced" from records, live or
// from --from-dump. It reports which records name which ids; it never says
// what a reference does.
func (a *App) newRefsCmd(opts globalOptions) *cobra.Command {
	parent := &cobra.Command{
		Use:         "refs",
		Short:       "find the records that reference an object, or referenced ids not found in what was read",
		Annotations: map[string]string{effectsAnnotation: credentialedReadEffects},
	}
	parent.AddCommand(&cobra.Command{
		Use:   "to <product> <resource> <id>",
		Short: "list the records whose reference fields hold <id> of <product> <resource>",
		Annotations: map[string]string{
			effectsAnnotation:        credentialedReadEffects,
			"introspect/args-policy": "exact:3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 3 || strings.TrimSpace(args[2]) == "" {
				return UsageError{Message: "usage: zscalerctl refs to <product> <resource> <id>"}
			}
			target, err := a.refsSpec(opts, args[0], args[1])
			if err != nil {
				return err
			}
			return a.runRefs(cmd.Context(), opts, args, func(read resources.RecordReader, rt machineRuntime) (output.SafeJSON, output.SafeText) {
				doc := resources.FindRefsTo(a.resourceCatalog(), resources.ResourceRef{Product: target.Product, Resource: target.Name}, args[2], rt.Redaction(), read, refsFailure)
				return doc, renderRefsTo(doc)
			})
		},
	})
	parent.AddCommand(&cobra.Command{
		Use:   "check <product> <resource>",
		Short: "list reference ids on <product> <resource> records that are not in the target lists as read",
		Annotations: map[string]string{
			effectsAnnotation:        credentialedReadEffects,
			"introspect/args-policy": "exact:2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return UsageError{Message: "usage: zscalerctl refs check <product> <resource>"}
			}
			source, err := a.refsSpec(opts, args[0], args[1])
			if err != nil {
				return err
			}
			var checkErr error
			err = a.runRefs(cmd.Context(), opts, args, func(read resources.RecordReader, rt machineRuntime) (output.SafeJSON, output.SafeText) {
				doc, err := resources.CheckRefs(a.resourceCatalog(), source, rt.Redaction(), read, refsFailure)
				if err != nil {
					checkErr = err
					return nil, ""
				}
				return doc, renderRefsCheck(doc)
			})
			if checkErr != nil {
				return checkErr
			}
			return err
		},
	})
	return parent
}

func (a *App) refsSpec(opts globalOptions, product, resource string) (resources.ResourceSpec, error) {
	if opts.format != output.FormatJSON && opts.format != output.FormatTable && opts.format != output.FormatPretty {
		return resources.ResourceSpec{}, rejectUnsupportedFormat("refs", opts.format)
	}
	spec, ok := a.resourceCatalog().FindSpec(resources.Product(product), resource)
	if !ok {
		return resources.ResourceSpec{}, ResourceNotFoundError{Product: resources.Product(product), Resource: resource}
	}
	return spec, nil
}

// runRefs opens the live or saved-dump runtime, hands build a reader that
// reads each whole resource (list, or show for a singleton), and renders the
// document build returns.
func (a *App) runRefs(
	ctx context.Context,
	opts globalOptions,
	args []string,
	build func(resources.RecordReader, machineRuntime) (output.SafeJSON, output.SafeText),
) error {
	var (
		rt  machineRuntime
		cfg config.Config
		err error
	)
	if opts.fromDumpSet || opts.fromDump != "" {
		rt, cfg, err = a.savedDumpRuntime(ctx, opts, "refs", args)
	} else {
		cfg, err = config.LoadConfig(a.env, config.LoadOptions{Profile: opts.profile, ConfigPath: opts.configPath})
		if err == nil {
			applyOptions(&cfg, opts)
			rt, err = a.machineRuntime(ctx, cfg, opts)
		}
	}
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return err
	}
	// Whole-resource reads: refs never narrows the records it compares.
	readOpts := opts
	readOpts.fields, readOpts.filters, readOpts.search = nil, nil, ""
	read := func(spec resources.ResourceSpec) (resources.ProjectedRecords, error) {
		if err := ctx.Err(); err != nil {
			return resources.ProjectedRecords{}, err
		}
		op := "list"
		if !spec.SupportsReadOperation(op) {
			op = "show"
		}
		// Progress for --log-level info: a refs run can read dozens of
		// resources over several minutes.
		a.diagLogger().Info("refs reading resource", "product", spec.Product, "resource", spec.Name)
		return a.executeMachineRead(ctx, spec, op, "", rt, readOpts)
	}
	message := "reading references"
	var doc output.SafeJSON
	var text output.SafeText
	_, err = callWithSpinner(a, opts, message, func() (struct{}, error) {
		doc, text = build(read, rt)
		return struct{}{}, ctx.Err()
	})
	if err != nil {
		return err
	}
	if doc == nil {
		return nil
	}
	renderer := a.renderer(cfg, opts)
	if opts.format == output.FormatJSON {
		return renderer.WriteJSON(a.out, doc)
	}
	return renderer.WriteText(a.out, text)
}

// refsFailure names why a resource read failed, from the live failure reason
// when there is one, else the machine error kind.
func refsFailure(err error) string {
	machineErr, ok := asMachineError(err)
	if !ok {
		return "unknown"
	}
	if machineErr.Detail.Reason != "" {
		return machineErr.Detail.Reason
	}
	return machineErr.Kind
}

func renderRefsTo(doc resources.RefsTo) output.SafeText {
	var body strings.Builder
	fmt.Fprintf(&body, "%s: %d reference fields checked, %d references to the id found.\n", doc.Target, doc.SitesChecked, len(doc.ReferencedBy))
	for _, hit := range doc.ReferencedBy {
		fmt.Fprintf(&body, "%s %s\t%s\t%s\n", hit.Product, hit.Resource, hit.Field, refsRecordText(hit.Record))
	}
	writeRefsNotChecked(&body, doc.NotChecked)
	return output.NewSafeText(body.String())
}

func renderRefsCheck(doc resources.RefsCheck) output.SafeText {
	var body strings.Builder
	fmt.Fprintf(&body, "%s: %d records, %d references checked, %d not found in the target lists read.\n",
		doc.Source, doc.Records, doc.ReferencesChecked, len(doc.NotFoundInRead))
	for _, miss := range doc.NotFoundInRead {
		targets := make([]string, len(miss.Targets))
		for i, target := range miss.Targets {
			targets[i] = target.String()
		}
		fmt.Fprintf(&body, "%s\t%s=%s\tnot in %s\n", refsRecordText(miss.Record), miss.Field,
			output.TerminalCell(resources.ProjectedValueText(miss.ID)), strings.Join(targets, ", "))
	}
	writeRefsNotChecked(&body, doc.NotChecked)
	return output.NewSafeText(body.String())
}

func refsRecordText(record resources.ProjectedRecord) string {
	parts := []string{}
	for _, key := range []string{"id", "name"} {
		if value, ok := record.Value(key); ok {
			parts = append(parts, key+"="+output.TerminalCell(resources.ProjectedValueText(value)))
		}
	}
	return strings.Join(parts, " ")
}

func writeRefsNotChecked(body *strings.Builder, skips []resources.RefSkip) {
	for _, skip := range skips {
		field := ""
		if skip.Field != "" {
			field = " " + skip.Field
		}
		reason := string(skip.Reason)
		if skip.Failure != "" {
			reason += ": " + skip.Failure
		}
		fmt.Fprintf(body, "not checked: %s %s%s (%s)\n", skip.Product, skip.Resource, field, reason)
	}
}
