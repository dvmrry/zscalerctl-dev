package cli_test

import (
	"context"
	"strings"
	"testing"
)

// TestProductHelpListsResources pins the target of the unknown-resource
// error's "run zscalerctl <product> --help" hint: product help lists the
// catalog resources with their read operations and points at schema gaps.
func TestProductHelpListsResources(t *testing.T) {
	t.Parallel()
	app, out, _ := testConfigApp(t)
	if err := app.Run(context.Background(), []string{"zpa", "--help"}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, want := range []string{
		"Resources (zscalerctl zpa <resource> <operation>",
		"\n  application-segments ",
		"schema gaps zpa",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("zpa --help missing %q:\n%s", want, text)
		}
	}
	if resources, usage := strings.Index(text, "Resources ("), strings.Index(text, "Usage:"); resources < 0 || usage < resources {
		t.Errorf("zpa --help lists resources after the usage block; want them first:\n%s", text)
	}
}
