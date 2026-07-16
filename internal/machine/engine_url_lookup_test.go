package machine_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/machine"
)

func TestURLLookupResultReturnsDeepDefensiveCopies(t *testing.T) {
	t.Parallel()

	source := []machine.URLClassification{{
		URL:                          "example.com",
		Classifications:              []string{"TECHNOLOGY"},
		SecurityAlertClassifications: []string{"MALWARE_SITE"},
		Application:                  "EXAMPLE_APP",
	}}
	result := machine.NewURLLookupResult(source)
	source[0].URL = "source-mutated"
	source[0].Classifications[0] = "source-mutated"
	source[0].SecurityAlertClassifications[0] = "source-mutated"

	first := result.Classifications()
	want := []machine.URLClassification{{
		URL:                          "example.com",
		Classifications:              []string{"TECHNOLOGY"},
		SecurityAlertClassifications: []string{"MALWARE_SITE"},
		Application:                  "EXAMPLE_APP",
	}}
	if !reflect.DeepEqual(first, want) {
		t.Fatalf("URLLookupResult.Classifications() = %#v, want %#v", first, want)
	}
	first[0].URL = "result-mutated"
	first[0].Classifications[0] = "result-mutated"
	first[0].SecurityAlertClassifications[0] = "result-mutated"
	if second := result.Classifications(); !reflect.DeepEqual(second, want) {
		t.Fatalf("URLLookupResult after returned mutation = %#v, want %#v", second, want)
	}

	empty := machine.NewURLLookupResult([]machine.URLClassification{{URL: "empty.example"}}).Classifications()
	if empty[0].Classifications == nil || empty[0].SecurityAlertClassifications == nil {
		t.Fatalf("URLLookupResult empty slices = %#v, want non-nil empty slices", empty[0])
	}
}

func TestURLLookupEngineValuesRejectDirectJSON(t *testing.T) {
	t.Parallel()

	values := []struct {
		name  string
		value any
	}{
		{
			name: "request",
			value: machine.URLLookupRequest{
				RequestID: "must-not-render", URLs: []string{"must-not-render"},
			},
		},
		{
			name: "result",
			value: machine.NewURLLookupResult([]machine.URLClassification{{
				URL: "must-not-render",
			}}),
		},
	}
	for _, tt := range values {
		body, err := json.Marshal(tt.value)
		if err == nil {
			t.Fatalf("json.Marshal(URLLookup %s) error = nil; body = %s, want no wire format", tt.name, body)
		}
		if strings.Contains(string(body), "must-not-render") {
			t.Fatalf("json.Marshal(URLLookup %s) body = %q, want no value bytes", tt.name, body)
		}
	}

	var request machine.URLLookupRequest
	if err := json.Unmarshal([]byte(`{"URLs":["must-not-render"]}`), &request); err == nil {
		t.Fatalf("json.Unmarshal(URLLookupRequest) error = nil; request = %#v, want no wire format", request)
	}
	var result machine.URLLookupResult
	if err := json.Unmarshal([]byte(`{"classifications":[]}`), &result); err == nil {
		t.Fatalf("json.Unmarshal(URLLookupResult) error = nil; result = %#v, want no wire format", result)
	}
}
