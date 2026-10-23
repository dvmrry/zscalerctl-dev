package zscaler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

// /locations/lite omits keys per location; an omitted boolean must not render
// as false, and a negative predefined id must survive exactly.
func TestLiteLocationSourceRecordKeepsAbsentDistinctFromFalse(t *testing.T) {
	t.Parallel()
	var items []ziaLiteLocation
	body := `[{"id":-3,"name":"Road Warrior","ofwEnabled":false},{"id":101,"name":"HQ","parentId":0,"ipsControl":true}]`
	if err := json.Unmarshal([]byte(body), &items); err != nil {
		t.Fatal(err)
	}
	spec, _ := resources.FindSpec(resources.ProductZIA, resourceLocationsLite)
	var source []resources.SourceRecord
	for _, item := range items {
		source = append(source, liteLocationSourceRecord(item))
	}
	projected, _, err := resources.ProjectRecordsAndVerify(spec, redact.ModeShare, source)
	if err != nil {
		t.Fatal(err)
	}
	records := projected.Records()
	road := records[0].Fields()
	if id, _ := json.Marshal(road["id"]); string(id) != "-3" {
		t.Errorf("id = %s, want -3", id)
	}
	if v, ok := road["ofwEnabled"]; !ok || v != false {
		t.Errorf("ofwEnabled = %v, %v; want false present", v, ok)
	}
	if _, ok := road["ipsControl"]; ok {
		t.Errorf("ipsControl rendered for a location that omitted it")
	}
	if _, ok := road["parentId"]; ok {
		t.Errorf("parentId rendered for a location that omitted it")
	}
	if v, ok := records[1].Fields()["parentId"]; !ok || v == nil {
		t.Errorf("parentId 0 dropped: %v, %v", v, ok)
	}
}

// The registered handler reads /locations/lite unfiltered (no
// includeSubLocations/includeParentLocations, which RESOURCES.md relies on),
// walks it as a ZIA page list, and decodes through liteLocationSourceRecord.
func TestLocationsLiteHandlerReadsUnfilteredLitePages(t *testing.T) {
	t.Parallel()
	var liteRequests []*http.Request
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := `{"access_token":"test-token","expires_in":60}`
		statusCode := http.StatusOK
		switch request.URL.Path {
		case "/zia/api/v1/locations/lite":
			liteRequests = append(liteRequests, request.Clone(request.Context()))
			body = `[]`
			if request.URL.Query().Get("page") == "1" {
				body = `[{"id":-3,"name":"Road Warrior"},{"id":101,"name":"HQ","ipsControl":true}]`
			}
		case "/oauth2/v1/token":
		default:
			statusCode = http.StatusNotFound
			body = `{}`
		}
		return &http.Response{
			StatusCode: statusCode,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    request,
		}, nil
	})
	service := newZIATestService(t, transport)
	handlers := make(map[resourceKey]resourceHandler)
	addZIAHandlers(handlers, sdkClient{services: fixedService{cfg: validReaderConfig(), sdkService: service}})
	handler := handlers[resourceKey{product: resources.ProductZIA, name: resourceLocationsLite}]
	if handler == nil {
		t.Fatal("locations-lite handler = nil, want registered handler")
	}

	records, err := handler.List(context.Background())
	if err != nil {
		t.Fatalf("locations-lite handler List() error = %v, want nil", err)
	}
	if got, want := len(records), 2; got != want {
		t.Fatalf("locations-lite record count = %d, want %d", got, want)
	}
	if records[0].Has("ipsControl") || !records[1].Has("ipsControl") {
		t.Errorf("ipsControl presence = %v, %v, want absent then present", records[0].Has("ipsControl"), records[1].Has("ipsControl"))
	}
	if len(liteRequests) == 0 {
		t.Fatal("locations-lite requests = 0, want the lite endpoint read")
	}
	for index, request := range liteRequests {
		for key := range request.URL.Query() {
			if key != "page" && key != "pageSize" {
				t.Errorf("request %d query key %q, want only page and pageSize", index+1, key)
			}
		}
	}
}
