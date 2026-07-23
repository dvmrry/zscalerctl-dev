package zscaler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	zsdk "github.com/zscaler/zscaler-sdk-go/v3/zscaler"
)

func TestParseZPATotalPages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     json.RawMessage
		want    int
		wantErr bool
	}{
		{name: "number", raw: json.RawMessage(`3`), want: 3},
		{name: "quoted number", raw: json.RawMessage(`"3"`), want: 3},
		{name: "zero", raw: json.RawMessage(`0`), want: 0},
		{name: "missing", raw: nil, wantErr: true},
		{name: "null", raw: json.RawMessage(`null`), wantErr: true},
		{name: "negative", raw: json.RawMessage(`-1`), wantErr: true},
		{name: "not a number", raw: json.RawMessage(`"many"`), wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseZPATotalPages(test.raw)
			if gotErr := err != nil; gotErr != test.wantErr {
				t.Fatalf("parseZPATotalPages(%q) error = %v, want error presence = %t", test.raw, err, test.wantErr)
			}
			if got != test.want {
				t.Errorf("parseZPATotalPages(%q) = %d, want %d", test.raw, got, test.want)
			}
		})
	}
}

func TestZPAPaginateCollectsDeclaredPages(t *testing.T) {
	t.Parallel()

	var requested []int
	got, _, err := zpaPaginate(context.Background(), func(_ context.Context, pageNumber, pageSize int) (zpaPage[int], error) {
		requested = append(requested, pageNumber)
		if pageSize != zpaPageSize {
			t.Errorf("zpaPaginate fetch pageSize = %d, want %d", pageSize, zpaPageSize)
		}
		return zpaPage[int]{
			records:    []int{pageNumber},
			totalPages: 3,
		}, nil
	})
	if err != nil {
		t.Fatalf("zpaPaginate(3 pages) error = %v, want nil", err)
	}
	if want := []int{1, 2, 3}; !reflect.DeepEqual(got, want) {
		t.Errorf("zpaPaginate(3 pages) = %v, want %v", got, want)
	}
	if want := []int{1, 2, 3}; !reflect.DeepEqual(requested, want) {
		t.Errorf("zpaPaginate requested pages = %v, want %v", requested, want)
	}
}

func TestZPAPaginateRejectsMetadataDrift(t *testing.T) {
	t.Parallel()

	got, _, err := zpaPaginate(context.Background(), func(_ context.Context, pageNumber, _ int) (zpaPage[int], error) {
		totalPages := 2
		if pageNumber == 2 {
			totalPages = 3
		}
		return zpaPage[int]{
			records:    []int{pageNumber},
			totalPages: totalPages,
		}, nil
	})
	if err == nil {
		t.Fatal("zpaPaginate(changing totalPages) error = nil, want error")
	}
	if got != nil {
		t.Errorf("zpaPaginate(changing totalPages) result = %v, want nil", got)
	}
}

func TestZPAPaginateDiscardsPartialResultsOnPageError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("page 2 failed")
	got, _, err := zpaPaginate(context.Background(), func(_ context.Context, pageNumber, _ int) (zpaPage[int], error) {
		if pageNumber == 2 {
			return zpaPage[int]{}, wantErr
		}
		return zpaPage[int]{
			records:    []int{1},
			totalPages: 2,
		}, nil
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("zpaPaginate(page error) error = %v, want %v", err, wantErr)
	}
	if got != nil {
		t.Errorf("zpaPaginate(page error) result = %v, want nil", got)
	}
}

func TestZPAPaginateRejectsEmptyDeclaredPage(t *testing.T) {
	t.Parallel()

	got, _, err := zpaPaginate(context.Background(), func(_ context.Context, pageNumber, _ int) (zpaPage[int], error) {
		records := []int{1}
		if pageNumber == 2 {
			records = nil
		}
		return zpaPage[int]{
			records:    records,
			totalPages: 2,
		}, nil
	})
	if err == nil {
		t.Fatal("zpaPaginate(empty declared page) error = nil, want error")
	}
	if got != nil {
		t.Errorf("zpaPaginate(empty declared page) result = %v, want nil", got)
	}
}

func TestZPAPaginateRejectsRepeatedPage(t *testing.T) {
	t.Parallel()

	got, _, err := zpaPaginate(context.Background(), func(_ context.Context, _, _ int) (zpaPage[int], error) {
		return zpaPage[int]{
			records:    []int{7},
			totalPages: 2,
		}, nil
	})
	if err == nil {
		t.Fatal("zpaPaginate(repeated page) error = nil, want error")
	}
	if got != nil {
		t.Errorf("zpaPaginate(repeated page) result = %v, want nil", got)
	}
}

func TestZPAPaginateRejectsDeclaredPageCountAboveCeiling(t *testing.T) {
	t.Parallel()

	calls := 0
	got, _, err := zpaPaginate(context.Background(), func(_ context.Context, _, _ int) (zpaPage[int], error) {
		calls++
		return zpaPage[int]{
			records:    []int{1},
			totalPages: zpaMaxPages + 1,
		}, nil
	})
	if err == nil {
		t.Fatal("zpaPaginate(above ceiling) error = nil, want error")
	}
	if got != nil {
		t.Errorf("zpaPaginate(above ceiling) result = %v, want nil", got)
	}
	if calls != 1 {
		t.Errorf("zpaPaginate(above ceiling) fetch calls = %d, want 1", calls)
	}
}

func TestGetZPAAllPagesPreservesEndpointPageAndMicrotenantQueries(t *testing.T) {
	cfg := validReaderConfig()
	sdkCfg := newSDKConfiguration(context.Background(), cfg)

	var productRequests []*http.Request
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := `{"access_token":"test-token","expires_in":60}`
		statusCode := http.StatusOK
		if request.URL.Path == "/zpa/mgmtconfig/v1/admin/customers/zscalerctl-zpa-customer-id/serverGroup" {
			cloned := request.Clone(request.Context())
			clonedURL := *request.URL
			cloned.URL = &clonedURL
			productRequests = append(productRequests, cloned)

			switch request.URL.Query().Get("page") {
			case "1":
				body = `{"totalPages":2,"list":[{"id":"1","name":"first"}]}`
			case "2":
				body = `{"totalPages":"2","list":[{"id":"2","name":"second"}]}`
			default:
				statusCode = http.StatusBadRequest
				body = `{"message":"unexpected page"}`
			}
		} else if request.URL.Path == "/zpa/mgmtconfig/v2/admin/customers/zscalerctl-zpa-customer-id/network" {
			cloned := request.Clone(request.Context())
			clonedURL := *request.URL
			cloned.URL = &clonedURL
			productRequests = append(productRequests, cloned)
			body = `{"totalPages":1,"list":[{"id":"3","name":"third"}]}`
		} else if request.URL.Path != "/oauth2/v1/token" {
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
	sdkCfg.HTTPClient.Transport = transport
	sdkCfg.ZPAHTTPClient.Transport = transport

	service, err := zsdk.NewOneAPIClient(sdkCfg)
	if err != nil {
		t.Fatalf("NewOneAPIClient() error = %v, want nil", err)
	}
	t.Cleanup(service.Client.Close)

	type item struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	items, _, err := getZPAAllPages[item](context.Background(), service, zpaAPIV1, "/serverGroup")
	if err != nil {
		t.Fatalf("getZPAAllPages(serverGroup) error = %v, want nil", err)
	}
	if got, want := len(productRequests), 2; got != want {
		t.Fatalf("getZPAAllPages(serverGroup) request count = %d, want %d", got, want)
	}
	for index, request := range productRequests {
		query := request.URL.Query()
		for key, want := range map[string]string{
			"microtenantId": "zscalerctl-zpa-microtenant-id",
			"page":          strconv.Itoa(index + 1),
			"pagesize":      strconv.Itoa(zpaPageSize),
		} {
			if got := query.Get(key); got != want {
				t.Errorf("request %d query[%q] = %q, want %q", index+1, key, got, want)
			}
		}
	}
	if want := []item{{ID: "1", Name: "first"}, {ID: "2", Name: "second"}}; !reflect.DeepEqual(items, want) {
		t.Errorf("getZPAAllPages(serverGroup) = %#v, want %#v", items, want)
	}

	items, _, err = getZPAAllPages[item](context.Background(), service, zpaAPIV2, "/network")
	if err != nil {
		t.Fatalf("getZPAAllPages(network) error = %v, want nil", err)
	}
	if got, want := len(productRequests), 3; got != want {
		t.Fatalf("getZPAAllPages(all calls) request count = %d, want %d", got, want)
	}
	networkRequest := productRequests[2]
	if got, want := networkRequest.URL.Query().Get("microtenantId"), "zscalerctl-zpa-microtenant-id"; got != want {
		t.Errorf("network request query[microtenantId] = %q, want %q", got, want)
	}
	if got, want := networkRequest.URL.Query().Get("page"), "1"; got != want {
		t.Errorf("network request query[page] = %q, want %q", got, want)
	}
	if got, want := networkRequest.URL.Query().Get("pagesize"), strconv.Itoa(zpaPageSize); got != want {
		t.Errorf("network request query[pagesize] = %q, want %q", got, want)
	}
	if want := []item{{ID: "3", Name: "third"}}; !reflect.DeepEqual(items, want) {
		t.Errorf("getZPAAllPages(network) = %#v, want %#v", items, want)
	}
}

func TestZPAListHandlersAvoidUnboundedSDKPagination(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("reader_zpa.go")
	if err != nil {
		t.Fatalf("ReadFile(reader_zpa.go) error = %v, want nil", err)
	}
	source := string(body)

	for _, banned := range []string{
		"zpaservergroup.GetAll(ctx, service)",
		"zpamicrotenants.GetAll(ctx, service)",
		"zpaversionprofile.GetAll(ctx, service)",
		"zpaisolationprofile.GetAll(ctx, service)",
		"zpabranchconnector.GetAll(ctx, service)",
		"zpauserportal.GetAll(ctx, service)",
		"zpauserportalaup.GetAll(ctx, service)",
		"zpauserportallink.GetAll(ctx, service)",
		"zpaappsegmentba.GetAll(ctx, service)",
		"zpaappsegmentinspection.GetAll(ctx, service)",
		"zpaappsegmentpra.GetAll(ctx, service)",
		"zpasegmentgroup.GetAll(ctx, service)",
		"zpaapplicationsegment.GetAll(ctx, service)",
		"zpaappconnectorcontroller.GetAll(ctx, service)",
		"zpaappconnectorgroup.GetAll(ctx, service)",
		"zpaappservercontroller.GetAll(ctx, service)",
		"zpamachinegroup.GetAll(ctx, service)",
		"zpatrustednetwork.GetAll(ctx, service)",
		"zpaserviceedgegroup.GetAll(ctx, service)",
		"zpaserviceedgecontroller.GetAll(ctx, service)",
		"zpacloudconnectorgroup.GetAll(ctx, service)",
		"zpacloudconnector.GetAll(ctx, service)",
		"zpapostureprofile.GetAll(ctx, service)",
		"zpaconfigoverride.GetAll(ctx, service)",
	} {
		if strings.Contains(source, banned) {
			t.Errorf("reader_zpa.go still calls SDK pagination with an unverified totalPages contract: %q", banned)
		}
	}
	if got, want := strings.Count(source, "getZPAAllPages["), 24; got != want {
		t.Errorf("reader_zpa.go bounded paginator wiring count = %d, want %d", got, want)
	}
}
