package cli_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/cli"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// TestResourceReadNamesFieldsTheModeOmits pins the omitted-field notice: a
// successful read in a mode that suppresses catalog fields says which ones on
// stderr, so an absent key is never mistaken for empty data. Standard mode
// suppresses nothing in this catalog and stays quiet; stdout is unchanged.
func TestResourceReadNamesFieldsTheModeOmits(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "share without fields names every omitted field",
			args:       []string{"--redaction", "share"},
			wantStderr: "zscalerctl: share mode omits zia/locations fields: hostname; they render with --redaction standard\n",
		},
		{
			name:       "requested omitted field",
			args:       []string{"--redaction", "share", "--fields", "id,hostname"},
			wantStderr: "zscalerctl: requested zia/locations fields omitted in share mode: hostname; they render with --redaction standard\n",
		},
		{
			name:       "requested secret field",
			args:       []string{"--fields", "id,internalToken"},
			wantStderr: "zscalerctl: requested zia/locations fields are secret and never render: internalToken\n",
		},
		{
			name:       "requested fields all rendered",
			args:       []string{"--redaction", "share", "--fields", "id,name"},
			wantStderr: "",
		},
		{
			name:       "standard mode",
			args:       nil,
			wantStderr: "",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reader := fakeResourceReader{list: []resources.SourceRecord{
				resources.NewSourceRecord(map[string]any{"id": "1", "name": "HQ", "hostname": "host-01", "internalToken": "raw-token-value"}),
			}}
			var out, errOut bytes.Buffer
			app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader, Catalog: filterWarningCatalog()})
			args := append([]string{"--format", "json"}, tt.args...)
			args = append(args, "zia", "locations", "list")
			if err := app.Run(context.Background(), args); err != nil {
				t.Fatalf("App.Run(%v) error = %v", args, err)
			}
			if errOut.String() != tt.wantStderr {
				t.Errorf("App.Run(%v) stderr = %q, want %q", args, errOut.String(), tt.wantStderr)
			}
			if strings.Contains(out.String(), "raw-token-value") || strings.Contains(errOut.String(), "raw-token-value") {
				t.Errorf("App.Run(%v) leaked the secret value", args)
			}
		})
	}
}

// TestOmittedFieldNoticeIsNotWrittenOnFailure keeps a failing read to a single
// error envelope on stderr: notices are flushed only when the run succeeds.
func TestOmittedFieldNoticeIsNotWrittenOnFailure(t *testing.T) {
	t.Parallel()

	reader := selectiveErrorResourceReader{failures: map[string]error{"zia/locations": errors.New("upstream failed")}}
	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader, Catalog: filterWarningCatalog()})
	args := []string{"--format", "json", "--redaction", "share", "zia", "locations", "list"}
	if err := app.Run(context.Background(), args); err == nil {
		t.Fatalf("App.Run(%v) error = nil, want failure", args)
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(%v) stderr = %q, want empty (the caller writes the error envelope)", args, errOut.String())
	}
}
