package resources_test

import (
	"bytes"
	"encoding/json"
	"io"
	"regexp"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// TestIncidentCorpusNeverReachesRenderedOutput replays the pasted-key leak
// class through a real catalog resource: projection of a free-text field and a
// display name, then the JSON and NDJSON renderers with their final byte scan.
// No canary may reach output bytes in any mode. Unlabeled keys are not
// detected in standard-mode display names by design (they use main's
// ScanString), so those cases keep a plain standard-mode name and are checked
// through the description only.
func TestIncidentCorpusNeverReachesRenderedOutput(t *testing.T) {
	t.Parallel()

	spec, ok := resources.FindSpec(resources.ProductZIA, "locations")
	if !ok {
		t.Fatal("FindSpec(zia, locations) ok = false, want true")
	}
	type incidentCase struct {
		input     string
		canary    string
		unlabeled bool
	}
	var cases []incidentCase
	for _, tt := range []struct {
		input  string
		canary string
	}{
		{"A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v8", "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v8"},
		{strings.ReplaceAll(`{"note":"vendor trial ^"A7b9C2d^^u0034E6f8G1h^^u0033J5k7L9m^^u0032N4p6Q8r^^u0030S2t4U6v^" until Friday"}`, "^", "\x5c"), "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"},
		{strings.ReplaceAll(`"A7b9C2d^u0034E6f8G1h^u0033J5k7L9m^u0032N4p6Q8r^u0030S2t4U6v"`, "^", "\x5c"), "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"},
	} {
		cases = append(cases, incidentCase{input: tt.input, canary: tt.canary, unlabeled: true})
	}
	for _, tt := range []struct {
		input  string
		canary string
	}{
		{"POC key: " + "A7b9C2d4E6f8G1h3J5k7", "A7b9C2d4E6f8G1h3J5k7"},
		{"key=" + "A7b9C2d4E6f8G1h3J5k7", "A7b9C2d4E6f8G1h3J5k7"},
		{"Bearer A7b9C2d4E6f8J5k7", "A7b9C2d4E6f8J5k7"},
		{"POC key: 550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440000"},
		{"token A7b9C2d4E6f8G1h3J5k7L9m2N4p6", "A7b9C2d4E6f8G1h3J5k7L9m2N4p6"},
		{"password " + "Abc123def456", "Abc123def456"},
		{"gh" + "p_1A2b3C4d5E6f7G8h9I0jKlMnOpQrStUvWxYz", "1A2b3C4d5E6f7G8h9I0jKlMnOpQrStUvWxYz"},
		{"POC key: Bearer A7b9C2d4E6f8J5k7", "A7b9C2d4E6f8J5k7"},
		{"POC key: 550e8400-e29b-41d4-a716-446655440000.", "550e8400-e29b-41d4-a716-446655440000"},
		{"POC key: (A7b9C2d4E6f8G1h3J5k7)", "A7b9C2d4E6f8G1h3J5k7"},
		{"POC key: ( A7b9C2d4E6f8J5k7 )", "A7b9C2d4E6f8J5k7"},
		{"POC key:\xc2\xa0A7b9C2d4E6f8J5k7", "A7b9C2d4E6f8J5k7"},
		{"POC key: key=" + "550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440000"},
		{"POC key: " + "A7b9G2d4E6f_keY=", "A7b9G2d4E6f"},
		{"POC key: " + "QzWpRtYuHsDfGjKlXcVbZg==", "QzWpRtYuHsDfGjKlXcVbZg"},
		{"POC **key**: A7b9C2d4E6f8J5k7", "A7b9C2d4E6f8J5k7"},
		{"POC key: " + "_550e8400-e29b-41d4-a716-446655440000_", "550e8400-e29b-41d4-a716-446655440000"},
		{"POC key: \xe2\x80\x9cA7b9C2d4E6f8J5k7\xe2\x80\x9d", "A7b9C2d4E6f8J5k7"},
		{"POC \xe2\x80\x9ckey\xe2\x80\x9d: \xe2\x80\x9cA7b9C2d4E6f8J5k7\xe2\x80\x9d", "A7b9C2d4E6f8J5k7"},
		{strings.ReplaceAll(`{"key":"^u003c550e8400-e29b-41d4-a716-446655440000^u003e"}`, "^", "\x5c"), "550e8400-e29b-41d4-a716-446655440000"},
		{strings.ReplaceAll(`{"key":"^u003c 550e8400-e29b-41d4-a716-446655440000 ^u003e"}`, "^", "\x5c"), "550e8400-e29b-41d4-a716-446655440000"},
		{strings.ReplaceAll(`POC key: "key^u003d550e8400-e29b-41d4-a716-446655440000"`, "^", "\x5c"), "550e8400-e29b-41d4-a716-446655440000"},
		{strings.ReplaceAll(`POC key: "A7b9C2d^u0034E6f8G1h^u0033J5k7L9m^u0032N4p6Q8r^u0030S2t4U6v"`, "^", "\x5c"), "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"},
	} {
		cases = append(cases, incidentCase{input: tt.input, canary: tt.canary})
	}
	for _, tt := range cases {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			for _, mode := range []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid} {
				name := tt.input
				if tt.unlabeled && mode == redact.ModeStandard {
					name = "HQ"
				}
				projected, reports, err := resources.ProjectRecordsAndVerify(spec, mode, []resources.SourceRecord{
					resources.NewSourceRecord(map[string]any{
						"id":          1,
						"name":        name,
						"description": "vendor trial note: " + tt.input,
					}),
				})
				if err != nil {
					t.Fatalf("ProjectRecordsAndVerify(%s) error = %v, want nil", mode, err)
				}

				renderer := output.NewRenderer(redact.New(mode))
				var jsonOut, ndjsonOut bytes.Buffer
				if err := renderer.WriteJSON(&jsonOut, projected); err != nil {
					t.Fatalf("WriteJSON(%s) error = %v, want nil", mode, err)
				}
				records := projected.Records()
				if mode == redact.ModeStandard {
					// The fields must still render, visibly redacted, rather
					// than the canary disappearing with the field.
					for _, field := range []string{"name", "description"} {
						if _, ok := records[0].Value(field); !ok {
							t.Errorf("standard projection dropped %s, want it rendered with a marker", field)
						}
					}
					if len(reports) != 1 || len(reports[0].RedactedFields) == 0 {
						t.Errorf("standard projection reports = %#v, want a redacted field", reports)
					}
					if !strings.Contains(jsonOut.String(), "<REDACTED:") {
						t.Errorf("standard JSON output = %s, want a redaction marker", jsonOut.String())
					}
				}
				safe := make([]output.SafeJSON, len(records))
				for i := range records {
					safe[i] = records[i]
				}
				if err := renderer.WriteNDJSON(&ndjsonOut, safe); err != nil {
					t.Fatalf("WriteNDJSON(%s) error = %v, want nil", mode, err)
				}
				for label, body := range map[string]string{"json": jsonOut.String(), "ndjson": ndjsonOut.String()} {
					if strings.Contains(body, tt.canary) {
						t.Errorf("%s output (%s) = %s, want canary %q absent", label, mode, body, tt.canary)
					}
					for _, value := range decodedOutputStrings(t, body) {
						if strings.Contains(value, tt.canary) {
							t.Errorf("%s output (%s) decoded value %q contains canary %q", label, mode, value, tt.canary)
						}
					}
				}
			}
		})
	}
}

var embeddedJSONLiteralRE = regexp.MustCompile(`"(?:\\.|[^"\\])*"`)

// decodedOutputStrings returns every string value in JSON or NDJSON output,
// plus the decoded form of any value that is itself a JSON string literal, so
// an escaped canary cannot hide from a substring check.
func decodedOutputStrings(t *testing.T, body string) []string {
	t.Helper()
	var out []string
	var walk func(any)
	walk = func(value any) {
		switch v := value.(type) {
		case string:
			out = append(out, v)
			// Decode recursively: a value that is itself a JSON document, and
			// every quoted JSON literal inside the value, so an escaped canary
			// at any nesting depth is visible to the substring check.
			var document any
			if err := json.Unmarshal([]byte(v), &document); err == nil {
				if _, isString := document.(string); !isString {
					walk(document)
				}
			}
			for _, literal := range embeddedJSONLiteralRE.FindAllString(v, -1) {
				var inner string
				if err := json.Unmarshal([]byte(literal), &inner); err == nil && inner != v {
					walk(inner)
				}
			}
		case []any:
			for _, item := range v {
				walk(item)
			}
		case map[string]any:
			for _, item := range v {
				walk(item)
			}
		}
	}
	decoder := json.NewDecoder(strings.NewReader(body))
	for {
		var document any
		if err := decoder.Decode(&document); err != nil {
			if err != io.EOF {
				t.Fatalf("decode rendered output: %v", err)
			}
			return out
		}
		walk(document)
	}
}

// TestIncidentFixKeepsOrdinaryLocationText guards the other direction: names
// and descriptions an admin really writes must survive standard projection.
func TestIncidentFixKeepsOrdinaryLocationText(t *testing.T) {
	t.Parallel()

	spec, ok := resources.FindSpec(resources.ProductZIA, "locations")
	if !ok {
		t.Fatal("FindSpec(zia, locations) ok = false, want true")
	}
	for _, tt := range []struct{ name, description string }{
		{"api-gw-prod-usw2-01", "Primary key rotation is quarterly; key: production"},
		{"ZIA-LOC-NYC-HQ-FLOOR12", "token bucket rate 100 per second, ticket CHG-123456"},
		{"vnet-hub-eastus2-0a1b2c3d4e5f6a7b8c9d", "Bearer of bad news: circuit DIA-00001234 is down"},
		{"ZscalerClientConnectorRollout2024", "Rollout of ZscalerClientConnector4 to EMEA in 2024."},
		{"Projects/2024/Q3-planning", "tag key: Projects/2024/Q3-planning. key: production."},
		{"Projects/2024/production2024", "tag key: Projects/2024/production2024"},
		{"Projects/2024/NetworkSwitch2024", "tag key: Projects/2024/NetworkSwitch2024"},
		{"Projects/2024/IPv6Firewall", "tag key: Projects/2024/IPv6Firewall; key: Switch01Port48"},
		{"Switch01Eth0", "uplink key: " + "Switch01Eth0; key: " + "IPv6Firewall01"},
		{"Catalyst01Gi0/1", "uplink tag key: Catalyst01Gi0/1"},
		{"Projects/2024/iOS", "tag key: Projects/2024/iOS; tag key: monthlypatchwindows"},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, report, err := resources.ProjectRecord(spec, redact.ModeStandard, resources.NewSourceRecord(map[string]any{
				"id":          1,
				"name":        tt.name,
				"description": tt.description,
			}))
			if err != nil {
				t.Fatalf("ProjectRecord() error = %v, want nil", err)
			}
			for field, want := range map[string]string{"name": tt.name, "description": tt.description} {
				value, _ := got.Value(field)
				if value != want {
					t.Errorf("ProjectRecord().Value(%s) = %#v, want %q unchanged", field, value, want)
				}
			}
			if len(report.RedactedFields) != 0 {
				t.Errorf("ProjectRecord().RedactedFields = %#v, want none", report.RedactedFields)
			}
		})
	}
}
