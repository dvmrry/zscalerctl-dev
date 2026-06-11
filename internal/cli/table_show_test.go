package cli_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/cli"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// TestTableShowRendersKeyValues covers the table-format show path
// (renderRecordKeyValues): singleton records render as one key-value row per
// allow-listed field instead of a one-row column table.
func TestTableShowRendersKeyValues(t *testing.T) {
	t.Parallel()

	reader := fakeResourceReader{
		show: resources.NewSourceRecord(map[string]any{
			"apiSessionTimeout":     30,
			"enableAdminRankAccess": true,
		}),
	}
	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})

	if err := app.Run(context.Background(), []string{"--format", "table", "zia", "advanced-settings", "show"}); err != nil {
		t.Fatalf("App.Run(table show) error = %v, want nil", err)
	}
	got := out.String()
	if strings.Contains(got, "\x1b[") {
		t.Errorf("table show output = %q, want no ANSI escapes", got)
	}
	if hasBoxDrawing(got) {
		t.Errorf("table show output = %q, want plain key-value lines without borders", got)
	}

	// Every populated field renders as a "key<whitespace>value" row.
	rows := map[string]string{}
	for _, line := range strings.Split(got, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		rows[fields[0]] = strings.Join(fields[1:], " ")
	}
	if rows["apiSessionTimeout"] != "30" {
		t.Errorf("table show apiSessionTimeout row = %q, want value 30", rows["apiSessionTimeout"])
	}
	if rows["enableAdminRankAccess"] != "true" {
		t.Errorf("table show enableAdminRankAccess row = %q, want value true", rows["enableAdminRankAccess"])
	}
	// Allow-listed fields missing from the record still get a key column.
	if _, ok := rows["blockConnectHostSniMismatch"]; !ok {
		t.Errorf("table show output = %q, want blockConnectHostSniMismatch key row", got)
	}
}
