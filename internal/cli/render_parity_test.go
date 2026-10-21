package cli_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/cli"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// A credential pasted into a record field is redacted the same way in every
// output format: table and pretty scan each value as JSON output does,
// instead of scanning the rendered layout.
func TestPastedCredentialRedactedInEveryFormat(t *testing.T) {
	t.Parallel()
	const canary = "ghp_" + "A1b2C3d4E5f6G7h8I9j0K1l2M3n4O5p6Q7r8"
	reader := fakeResourceReader{list: []resources.SourceRecord{resources.NewSourceRecord(map[string]any{
		"id": 7, "name": "Example label", "description": "rotate: token=" + canary,
	})}}
	for _, format := range []string{"json", "table", "pretty"} {
		for _, mode := range []string{"standard", "share", "paranoid"} {
			var out, errOut bytes.Buffer
			app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})
			args := []string{"--format", format, "--redaction", mode, "zia", "rule-labels", "list"}
			if err := app.Run(context.Background(), args); err != nil {
				t.Fatalf("App.Run(%v) error = %v", args, err)
			}
			if strings.Contains(out.String(), canary) || strings.Contains(out.String(), "A1b2C3d4E5f6") {
				t.Errorf("App.Run(%v) output leaks the pasted token:\n%s", args, out.String())
			}
		}
	}
}
