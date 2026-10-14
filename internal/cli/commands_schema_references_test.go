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

func TestSchemaReferencesConfigFree(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, []string{"ZSCALERCTL_CLIENT_SECRET_FILE=/nonexistent/secret"}, cli.Options{Reader: neverCalledReader{t: t}})
	err := app.Run(context.Background(), []string{"--config", filepath.Join(t.TempDir(), "missing.yaml"), "--profile", "missing", "--format", "json", "schema", "references", "zia", "locations"})
	if err != nil {
		t.Fatal(err)
	}
	var got resources.References
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Target != (resources.ResourceRef{Product: resources.ProductZIA, Resource: "locations"}) {
		t.Errorf("Target = %+v, want zia/locations", got.Target)
	}
	want := resources.ReferencesTo(resources.Catalog(), got.Target)
	if len(got.ReferredBy) == 0 || len(got.ReferredBy) != len(want.ReferredBy) {
		t.Fatalf("ReferredBy has %d sites, want %d (non-zero)", len(got.ReferredBy), len(want.ReferredBy))
	}
	if errOut.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", &errOut)
	}
}

func TestSchemaReferencesTextNamesFilterOrSecret(t *testing.T) {
	t.Parallel()
	app, out, _ := testConfigApp(t)
	if err := app.Run(context.Background(), []string{"--format", "table", "schema", "references", "zia", "admin-users"}); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	if !strings.Contains(text, "url-filtering-rules\tlastModifiedBy\tsecret; not rendered in any mode") {
		t.Errorf("text output = %q, want the secret lastModifiedBy reference without a --filter hint", text)
	}

	app, out, _ = testConfigApp(t)
	if err := app.Run(context.Background(), []string{"--format", "table", "schema", "references", "zia", "users"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "zia url-filtering-rules\t--filter users.id=<id>\tstandard") {
		t.Errorf("text output = %q, want a --filter users.id hint", out.String())
	}
}

func TestSchemaReferencesValidationAndFormats(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		args []string
		want error
	}{
		{"missing", []string{"schema", "references", "zia"}, cli.ErrUsage},
		{"extra", []string{"schema", "references", "zia", "locations", "extra"}, cli.ErrUsage},
		{"unknown", []string{"schema", "references", "zia", "unknown"}, cli.ErrNotFound},
		{"ndjson", []string{"--format", "ndjson", "schema", "references", "zia", "locations"}, cli.ErrUsage},
		{"table", []string{"--format", "table", "schema", "references", "zia", "locations"}, nil},
		{"pretty", []string{"--format", "pretty", "schema", "references", "zia", "locations"}, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			app, out, _ := testConfigApp(t)
			err := app.Run(context.Background(), tt.args)
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
			if tt.want != nil && out.Len() != 0 {
				t.Fatalf("failure emitted output: %s", out)
			}
		})
	}
}

func TestSchemaReferencesAdvertisesExactArguments(t *testing.T) {
	t.Parallel()
	app, _, _ := testConfigApp(t)
	for _, command := range cli.IntrospectTree(app).Commands {
		if command.Path == "schema references" {
			if command.Args.Policy != "exact" || command.Args.N != 2 {
				t.Fatalf("schema references args = %+v, want exact 2", command.Args)
			}
			return
		}
	}
	t.Fatal("schema references missing from introspection")
}
