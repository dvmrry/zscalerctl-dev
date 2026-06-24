package cli

// export_test.go exposes internal symbols for use by the cli_test package.
// It is compiled only during testing (file name ends in _test.go).

import (
	"io"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/resources"
)

// ProductCmdCompletions calls the ValidArgsFunction wired to the product command
// for the given product name, with the given already-completed args. It returns
// the completion strings and the raw directive integer.
//
// This helper lets cli_test tests exercise the ValidArgsFunction directly
// without routing through App.Run (which would dispatch to the Cobra __complete
// protocol and complicate the test setup).
//
// SECURITY contract: the call must not panic, must not call config.LoadConfig,
// and must not return an error — it returns only catalog-derived strings.
func ProductCmdCompletions(t *testing.T, productName string, args []string) ([]string, int) {
	t.Helper()
	a := New(io.Discard, io.Discard, nil)
	product := resources.Product(productName)
	cmd := a.newProductCmd(product, globalOptions{})
	if cmd.ValidArgsFunction == nil {
		t.Fatal("newProductCmd returned a command with nil ValidArgsFunction")
	}
	completions, directive := cmd.ValidArgsFunction(cmd, args, "")
	out := make([]string, len(completions))
	for i, c := range completions {
		out[i] = string(c)
	}
	return out, int(directive)
}

// KnownProductNames returns the product command names derived from the live
// catalog, so tests can iterate every product without hardcoding the list
// (a new product is auto-covered). Test-only; the App arg is used to read the
// App catalog so tests can also inject empty/narrowed catalogs.
func KnownProductNames(a *App) []string {
	return productNames(knownProducts(a.resourceCatalog()))
}

// ExportedGlobalFlagDefs exposes the internal globalFlagDefs slice so the
// cli_test package can assert that IntrospectTree returns exactly one GlobalFlag
// entry per definition without duplicating the count.
var ExportedGlobalFlagDefs = globalFlagDefs
