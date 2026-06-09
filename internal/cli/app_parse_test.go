package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestFlagNameAcceptsSingleAndDoubleDash(t *testing.T) {
	cases := []struct {
		arg       string
		wantName  string
		wantValue bool
	}{
		{"--format", "format", false},
		{"-format", "format", false},
		{"--format=json", "format", true},
		{"-format=json", "format", true},
		{"--profile", "profile", false},
		{"-profile", "profile", false},
		{"--", "", false},
		{"-", "", false},
		{"locations", "", false},
		{"get", "", false},
	}
	for _, tc := range cases {
		name, hasValue := flagName(tc.arg)
		if name != tc.wantName || hasValue != tc.wantValue {
			t.Errorf("flagName(%q) = (%q, %v), want (%q, %v)", tc.arg, name, hasValue, tc.wantName, tc.wantValue)
		}
	}
}

func TestRequestedFormatHandlesDashStylesAndBoundary(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want output.Format
	}{
		{"double dash space", []string{"zia", "locations", "list", "--format", "json"}, output.FormatJSON},
		{"single dash space", []string{"zia", "locations", "list", "-format", "json"}, output.FormatJSON},
		{"double dash equals", []string{"--format=json", "schema", "list"}, output.FormatJSON},
		{"single dash equals", []string{"-format=json", "schema", "list"}, output.FormatJSON},
		{"explicit table", []string{"--format", "table", "schema", "list"}, output.FormatTable},
		{"no format flag", []string{"schema", "list"}, output.FormatTable},
		{"terminator stops scan", []string{"--", "--format", "json"}, output.FormatTable},
		{"format after positionals", []string{"schema", "list", "-format", "json"}, output.FormatJSON},
	}
	for _, tc := range cases {
		if got := RequestedFormat(tc.args); got != tc.want {
			t.Errorf("%s: RequestedFormat(%v) = %q, want %q", tc.name, tc.args, got, tc.want)
		}
	}
}

func TestRequestedFormatRawPreservesAutoAndPretty(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want output.Format
	}{
		{"default is auto", []string{"zia", "locations", "list"}, output.FormatAuto},
		{"explicit auto", []string{"--format", "auto", "schema", "list"}, output.FormatAuto},
		{"explicit pretty", []string{"--format", "pretty", "schema", "list"}, output.FormatPretty},
		{"explicit pretty equals", []string{"--format=pretty", "schema", "list"}, output.FormatPretty},
		{"explicit json", []string{"--format", "json", "schema", "list"}, output.FormatJSON},
		{"explicit table", []string{"--format", "table", "schema", "list"}, output.FormatTable},
		{"unparseable falls back to auto", []string{"--format", "yaml", "schema", "list"}, output.FormatAuto},
		{"terminator stops scan", []string{"--", "--format", "json"}, output.FormatAuto},
	}
	for _, tc := range cases {
		if got := RequestedFormatRaw(tc.args); got != tc.want {
			t.Errorf("%s: RequestedFormatRaw(%v) = %q, want %q", tc.name, tc.args, got, tc.want)
		}
	}
}

func TestParseDumpResourcesRejectsAmbiguousUnqualifiedName(t *testing.T) {
	products := map[resources.Product]bool{
		resources.ProductZIA: true,
		resources.ProductZPA: true,
	}
	catalog := resources.ResourceCatalog{
		dumpListSpec(resources.ProductZIA, "locations"),
		dumpListSpec(resources.ProductZPA, "locations"),
	}

	_, err := parseDumpResources("locations", products, catalog)
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("parseDumpResources(locations) error = %v, want ErrUsage", err)
	}
	if !strings.Contains(err.Error(), "ambiguous") || !strings.Contains(err.Error(), "product/name") {
		t.Errorf("parseDumpResources(locations) error = %q, want ambiguous product/name guidance", err.Error())
	}
}

func TestParseDumpResourcesRejectsMalformedResourceNames(t *testing.T) {
	products := map[resources.Product]bool{
		resources.ProductZIA: true,
		resources.ProductZPA: true,
	}
	catalog := resources.ResourceCatalog{
		dumpListSpec(resources.ProductZIA, "locations"),
	}
	tests := []string{
		"zia/locations/extra",
		"/locations",
		"zia/",
		"locations,",
		"locations,,rule-labels",
	}

	for _, value := range tests {
		value := value
		t.Run(value, func(t *testing.T) {
			_, err := parseDumpResources(value, products, catalog)
			if !errors.Is(err, ErrUsage) {
				t.Fatalf("parseDumpResources(%q) error = %v, want ErrUsage", value, err)
			}
		})
	}
}

func dumpListSpec(product resources.Product, name string) resources.ResourceSpec {
	return resources.ResourceSpec{
		Product: product,
		Name:    name,
		Operations: []resources.Operation{{
			Name:       "list",
			Capability: resources.CapabilityRead,
		}},
	}
}
