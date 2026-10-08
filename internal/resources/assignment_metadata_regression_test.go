package resources_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestLocationDescriptionsDistinguishIdentifiersAndCredentials(t *testing.T) {
	t.Parallel()

	const compact = "b7e19c024af683d095bc72e461da8f30"
	const uuid = "123e4567-e89b-12d3-a456-426614174000"
	spec, ok := resources.FindSpec(resources.ProductZIA, "locations")
	if !ok {
		t.Fatal("FindSpec(zia, locations) ok = false, want true")
	}

	for _, tt := range []struct {
		input  string
		want   string
		canary string
	}{
		{"community=b7e19c024af683d095bc72e461da8f30", "<REDACTED:SECRET>", compact},
		{"pin=" + compact, "<REDACTED:SECRET>", compact},
		{"opaque=" + compact, "<REDACTED:SECRET>", compact},
		{"liquid=" + compact, "<REDACTED:SECRET>", compact},
		{"community=" + uuid, "<REDACTED:SECRET>", uuid},
		{"id=" + compact, "id=" + compact, ""},
		{"uuid=" + compact, "uuid=" + compact, ""},
		{"guid=" + compact, "guid=" + compact, ""},
		{"ref=" + compact, "ref=" + compact, ""},
		{"reference=" + compact, "reference=" + compact, ""},
		{"LocationId=" + uuid, "LocationId=" + uuid, ""},
		{"PartitionKey=" + compact, "PartitionKey=" + compact, ""},
		{"Token ID: " + uuid, "Token ID: " + uuid, ""},
		{"Token identifier is " + uuid, "Token identifier is " + uuid, ""},
		{"Token: " + uuid, "Token: <REDACTED:SECRET>", uuid},
		{"Token is " + uuid, "Token is <REDACTED:SECRET>", uuid},
		{"Token for the vendor portal is " + uuid, "Token for the vendor portal is <REDACTED:SECRET>", uuid},
		{"Token: " + compact, "Token: <REDACTED:SECRET>", compact},
		{"API token ID: " + uuid, "API token ID: " + uuid, ""},
		{"Access token ID: " + uuid, "Access token ID: " + uuid, ""},
		{"API key ID: " + uuid, "API key ID: " + uuid, ""},
		{"API key name is " + uuid, "API key name is " + uuid, ""},
		{"API token ID:\n" + uuid, "API token ID:\n" + uuid, ""},
		{"Access token ID:\n" + uuid, "Access token ID:\n" + uuid, ""},
		{"API key ID:\n" + uuid, "API key ID:\n" + uuid, ""},
		{"API token: " + uuid, "API token: <REDACTED:SECRET>", uuid},
		{"Access token: " + uuid, "Access token: <REDACTED:SECRET>", uuid},
		{"API key: " + uuid, "API key: <REDACTED:SECRET>", uuid},
		{"API key is " + uuid, "API key is <REDACTED:SECRET>", uuid},
		{"API key for the name is " + uuid, "API key for the name is <REDACTED:SECRET>", uuid},
	} {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			projected, _, err := resources.ProjectRecordsAndVerify(spec, redact.ModeStandard, []resources.SourceRecord{
				resources.NewSourceRecord(map[string]any{
					"id":          1,
					"name":        "HQ",
					"description": tt.input,
				}),
			})
			if err != nil {
				t.Fatalf("ProjectRecordsAndVerify() error = %v, want nil", err)
			}
			records := projected.Records()
			if len(records) != 1 {
				t.Fatalf("projected records = %d, want 1", len(records))
			}
			if value, ok := records[0].Value("description"); !ok || value != tt.want {
				t.Errorf("projected description = %#v (present %t), want %q", value, ok, tt.want)
			}

			renderer := output.NewRenderer(redact.New(redact.ModeStandard))
			var jsonOut, ndjsonOut bytes.Buffer
			if err := renderer.WriteJSON(&jsonOut, projected); err != nil {
				t.Fatalf("WriteJSON() error = %v, want nil", err)
			}
			if err := renderer.WriteNDJSON(&ndjsonOut, []output.SafeJSON{records[0]}); err != nil {
				t.Fatalf("WriteNDJSON() error = %v, want nil", err)
			}

			var jsonRecords []map[string]any
			if err := json.Unmarshal(jsonOut.Bytes(), &jsonRecords); err != nil {
				t.Fatalf("decode JSON: %v", err)
			}
			if len(jsonRecords) != 1 {
				t.Fatalf("JSON records = %d, want 1", len(jsonRecords))
			}
			var ndjsonRecord map[string]any
			if err := json.Unmarshal(ndjsonOut.Bytes(), &ndjsonRecord); err != nil {
				t.Fatalf("decode NDJSON: %v", err)
			}
			for label, record := range map[string]map[string]any{
				"json":   jsonRecords[0],
				"ndjson": ndjsonRecord,
			} {
				if value := record["description"]; value != tt.want {
					t.Errorf("%s description = %#v, want %q", label, value, tt.want)
				}
			}
			for label, body := range map[string]string{
				"json":   jsonOut.String(),
				"ndjson": ndjsonOut.String(),
			} {
				if tt.canary != "" && strings.Contains(body, tt.canary) {
					t.Errorf("%s output = %s, want credential %q absent", label, body, tt.canary)
				}
			}
		})
	}
}
