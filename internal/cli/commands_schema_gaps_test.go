package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/cli"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestSchemaGapsConfigFree(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, []string{"ZSCALERCTL_CLIENT_SECRET_FILE=/nonexistent/secret"}, cli.Options{Reader: neverCalledReader{t: t}})
	err := app.Run(context.Background(), []string{"--config", filepath.Join(t.TempDir(), "missing.yaml"), "--profile", "missing", "--format", "json", "schema", "gaps", "zpa"})
	if err != nil {
		t.Fatal(err)
	}
	var got resources.GapsDocument
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := resources.CoverageGaps(resources.ProductZPA)
	if len(got.Gaps) == 0 || len(got.Gaps) != len(want) {
		t.Fatalf("schema gaps zpa has %d entries, want %d (non-zero)", len(got.Gaps), len(want))
	}
	for _, gap := range got.Gaps {
		if gap.Product != resources.ProductZPA || gap.Status == "" || gap.Reason == "" || gap.Note == "" {
			t.Errorf("gap %+v is incomplete or not zpa", gap)
		}
	}
	if errOut.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", &errOut)
	}
	// Aliases are for matching only; the document names each surface once.
	if strings.Contains(out.String(), "access-policies") {
		t.Errorf("schema gaps output renders aliases: %s", out.String())
	}
}

func TestSchemaGapsRejectsUnknownProduct(t *testing.T) {
	t.Parallel()
	app, _, _ := testConfigApp(t)
	err := app.Run(context.Background(), []string{"--format", "json", "schema", "gaps", "nope"})
	if !errors.Is(err, cli.ErrUsage) {
		t.Fatalf("schema gaps nope error = %v, want usage error", err)
	}
}

// TestUnknownResourceNamesDeclaredGap pins the point of the gap table: asking
// for a surface zscalerctl does not read says so and why, instead of a bare
// "unsupported resource" that a caller keeps guessing around.
func TestUnknownResourceNamesDeclaredGap(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		args []string
		want []string
	}{
		{[]string{"zpa", "access-policies", "list"},
			[]string{"zscalerctl does not read zpa/policy-rules (deferred: not_modeled)", "schema gaps zpa"}},
		{[]string{"ztw", "locations-lite", "list"},
			[]string{"(out_of_scope: alternate_view)", "Covered instead: ztw/locations."}},
		{[]string{"zpa", "no-such-thing", "list"},
			[]string{"unsupported resource zpa/no-such-thing", "schema list", "schema gaps zpa"}},
	} {
		app, _, _ := testConfigApp(t)
		err := app.Run(context.Background(), append([]string{"--format", "json"}, tc.args...))
		if !errors.Is(err, cli.ErrNotFound) {
			t.Fatalf("%v error = %v, want not found", tc.args, err)
		}
		for _, want := range tc.want {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("%v error = %q, want it to contain %q", tc.args, err.Error(), want)
			}
		}
	}
}

// TestEveryGapRoutesToItsGapError runs each declared gap through the real
// command router, so a gap cannot name a special command (such as
// "zia admin-audit-logs") that the catalog-level test cannot see.
func TestEveryGapRoutesToItsGapError(t *testing.T) {
	t.Parallel()
	for _, gap := range resources.CoverageGaps("") {
		for _, name := range append([]string{gap.Surface}, gap.Aliases...) {
			app, _, _ := testConfigApp(t)
			err := app.Run(context.Background(), []string{"--format", "json", string(gap.Product), name, "list"})
			want := "zscalerctl does not read " + string(gap.Product) + "/" + gap.Surface + " "
			if !errors.Is(err, cli.ErrNotFound) || !strings.Contains(err.Error(), want) {
				t.Errorf("%s %s list error = %v, want not found naming the gap", gap.Product, name, err)
			}
		}
	}
}
