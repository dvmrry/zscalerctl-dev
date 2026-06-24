package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/dvmrry/zscalerctl/internal/tui/browserdata"
	"github.com/dvmrry/zscalerctl/internal/tui/launcher"
)

// newBrowseCmd returns the experimental "browse" command. It is hidden from
// normal help and only launches the TUI when explicitly requested with --tui.
// The command loads config, resolves credentials, builds a real resource reader,
// and runs the collector to produce BrowserData before launching the TUI.
//
// The fixture/demo path (scripts/tui-browser-demo.go) is separate and still
// requires no credentials; this command exercises the real CLI wiring.
func (a *App) newBrowseCmd(opts globalOptions) *cobra.Command {
	var (
		tuiFlag         bool
		productsFlag    string
		resourcesFlag   string
		continueOnError bool
	)
	cmd := &cobra.Command{
		Use:    "browse",
		Short:  "experimental TUI browser",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !tuiFlag {
				return UsageError{Message: "browse currently requires --tui"}
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			// stdinTTY/stdoutTTY/stderrTTY were captured when the App was constructed
			// from the user's real terminal streams. They are not affected by the
			// --output-file wrapper or by muteProcessOutput.
			launchCfg := launcher.Config{
				Requested:  tuiFlag,
				StdinTTY:   a.stdinTTY,
				StdoutTTY:  a.stdoutTTY,
				StderrTTY:  a.stderrTTY,
				Format:     opts.format,
				ColorMode:  opts.colorMode,
				OutputPath: opts.output,
				Env:        a.env,
				Input:      os.Stdin,
				Output:     a.out,
			}

			// Evaluate the TUI gate before any config, credential, or reader work.
			// This ensures --format json/ndjson, --output, non-TTY, and color-disabled
			// invocations fail with a usage error before we touch secrets or the network.
			if err := launcher.CheckGate(launchCfg); err != nil {
				var launchErr launcher.LaunchError
				if errors.As(err, &launchErr) {
					return UsageError{Message: launchErr.Error()}
				}
				return err
			}

			cfg, err := config.LoadConfig(a.env, config.LoadOptions{
				Profile:    opts.profile,
				ConfigPath: opts.configPath,
			})
			if err != nil {
				return err
			}
			applyOptions(&cfg, opts)

			reader, err := a.resourceReader(ctx, cfg, opts)
			if err != nil {
				return err
			}

			catalog := a.resourceCatalog()
			products, err := parseBrowseProducts(productsFlag, catalog)
			if err != nil {
				return err
			}
			resourceNames, err := parseBrowseResources(resourcesFlag, products, catalog)
			if err != nil {
				return err
			}

			launchCfg.Collector = &browserdata.Collector{
				Catalog: catalog,
				Reader:  reader,
				Mode:    cfg.Defaults.Redaction,
			}
			launchCfg.CollectOptions = browserdata.CollectOptions{
				Products:        products,
				Resources:       resourceNames,
				ContinueOnError: continueOnError,
			}

			err = a.launchBrowser(ctx, launchCfg)
			var launchErr launcher.LaunchError
			if errors.As(err, &launchErr) {
				return UsageError{Message: launchErr.Error()}
			}
			return err
		},
	}
	cmd.Flags().BoolVar(&tuiFlag, "tui", false, "launch the interactive TUI browser")
	cmd.Flags().StringVar(&productsFlag, "products", "", "comma-separated products to browse")
	cmd.Flags().StringVar(&resourcesFlag, "resources", "", "comma-separated resources to browse")
	cmd.Flags().BoolVar(&continueOnError, "continue-on-error", false, "continue collecting resources after an error")
	return cmd
}

func parseBrowseProducts(value string, catalog resources.ResourceCatalog) ([]resources.Product, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	productSet, err := parseProducts(value, catalog)
	if err != nil {
		return nil, err
	}
	products := make([]resources.Product, 0, len(productSet))
	for p := range productSet {
		products = append(products, p)
	}
	return products, nil
}

func parseBrowseResources(value string, products []resources.Product, catalog resources.ResourceCatalog) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	allowed := make(map[string]bool, len(catalog))
	productSet := make(map[resources.Product]bool, len(products))
	for _, p := range products {
		productSet[p] = true
	}
	for _, spec := range catalog {
		if len(productSet) > 0 && !productSet[spec.Product] {
			continue
		}
		allowed[spec.Name] = true
	}
	var names []string
	for _, raw := range strings.Split(value, ",") {
		name := strings.TrimSpace(strings.ToLower(raw))
		if name == "" {
			return nil, UsageError{Message: "empty resource in --resources"}
		}
		if !allowed[name] {
			return nil, UsageError{Message: fmt.Sprintf("unsupported resource %q", raw)}
		}
		names = append(names, name)
	}
	return names, nil
}
