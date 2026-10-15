package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/cli"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestSchemaDescribeConfigFreeAndModeAware(t *testing.T) {
	t.Parallel()
	for _, mode := range []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid} {
		t.Run(string(mode), func(t *testing.T) {
			var out, errOut bytes.Buffer
			app := cli.NewWithOptions(&out, &errOut, []string{"ZSCALERCTL_CLIENT_SECRET_FILE=/nonexistent/secret"}, cli.Options{Reader: neverCalledReader{t: t}})
			err := app.Run(context.Background(), []string{"--config", filepath.Join(t.TempDir(), "missing.yaml"), "--profile", "missing", "--format", "json", "--redaction", string(mode), "schema", "describe", "zia", "locations"})
			if err != nil {
				t.Fatal(err)
			}
			var got resources.Semantics
			if err := json.Unmarshal(out.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.RedactionMode != mode || got.ReviewStatus != resources.SemanticsReviewed || !got.ReadOnly || got.Version != resources.SemanticsVersion {
				t.Fatalf("unexpected document: %+v", got)
			}
			spec, _ := resources.Catalog().FindSpec(resources.ProductZIA, "locations")
			for _, field := range got.Fields {
				found := false
				for _, catalogField := range spec.Fields {
					if catalogField.JSONField() == field.Name && catalogField.AllowedIn(mode) {
						found = true
					}
				}
				if !found {
					t.Errorf("field %q is not renderable in %s", field.Name, mode)
				}
			}
			if errOut.Len() != 0 {
				t.Fatalf("unexpected stderr: %s", &errOut)
			}
		})
	}
}

func TestSchemaDescribeUnreviewedIsExplicit(t *testing.T) {
	t.Parallel()
	app, out, _ := testConfigApp(t)
	if err := app.Run(context.Background(), []string{"--format", "json", "schema", "describe", "zia", "advanced-settings"}); err != nil {
		t.Fatal(err)
	}
	var got resources.Semantics
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.ReviewStatus != resources.SemanticsNotReviewed || got.Fields == nil || len(got.Fields) != 0 {
		t.Fatalf("unreviewed resource looks reviewed: %+v", got)
	}
}

func TestSchemaDescribeValidationAndFormats(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name string
		args []string
		want error
	}{
		{"missing", []string{"schema", "describe", "zia"}, cli.ErrUsage},
		{"extra", []string{"schema", "describe", "zia", "locations", "extra"}, cli.ErrUsage},
		{"unknown", []string{"schema", "describe", "zia", "unknown"}, cli.ErrNotFound},
		{"ndjson", []string{"--format", "ndjson", "schema", "describe", "zia", "locations"}, cli.ErrUsage},
		{"table", []string{"--format", "table", "schema", "describe", "zia", "locations"}, nil},
		{"pretty", []string{"--format", "pretty", "schema", "describe", "zia", "locations"}, nil},
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

func TestSchemaDescribeAdvertisesExactArguments(t *testing.T) {
	t.Parallel()
	app, _, _ := testConfigApp(t)
	for _, command := range cli.IntrospectTree(app).Commands {
		if command.Path == "schema describe" {
			if command.Args.Policy != "exact" || command.Args.N != 2 {
				t.Fatalf("schema describe args = %+v, want exact 2", command.Args)
			}
			return
		}
	}
	t.Fatal("schema describe missing from introspection")
}
