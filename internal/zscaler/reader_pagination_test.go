package zscaler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"

	zsdk "github.com/zscaler/zscaler-sdk-go/v3/zscaler"
	zcccommon "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zcc/services/common"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/location/locationgroups"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/urlfilteringpolicies"
)

// TestZCCPaginateCeilingFailsClosed drives the zccPaginate page ceiling: an
// endpoint that keeps returning a persistently-full page must error rather than
// loop forever.
func TestZCCPaginateCeilingFailsClosed(t *testing.T) {
	t.Parallel()

	calls := 0
	full := make([]int, zccPageSize) // always a full page -> never short-terminates
	_, err := zccPaginate(context.Background(), func(_ context.Context, page, pageSize int) ([]int, error) {
		calls++
		if pageSize != zccPageSize {
			t.Fatalf("fetchPage pageSize = %d, want %d", pageSize, zccPageSize)
		}
		return full, nil
	})
	if err == nil {
		t.Fatal("zccPaginate(always-full) error = nil, want ceiling error")
	}
	if !strings.Contains(err.Error(), "ceiling") {
		t.Errorf("zccPaginate error = %q, want it to mention the ceiling", err.Error())
	}
	// It must stop at the ceiling, not run away.
	if calls != zccMaxPages {
		t.Errorf("zccPaginate fetched %d pages, want exactly the ceiling of %d", calls, zccMaxPages)
	}
}

// TestZCCPaginateStopsOnShortPage confirms the normal termination path is
// unaffected by the ceiling.
func TestZCCPaginateStopsOnShortPage(t *testing.T) {
	t.Parallel()

	pages := [][]int{make([]int, zccPageSize), {1, 2, 3}}
	got, err := zccPaginate(context.Background(), func(_ context.Context, page, _ int) ([]int, error) {
		return pages[page-1], nil
	})
	if err != nil {
		t.Fatalf("zccPaginate error = %v, want nil", err)
	}
	if len(got) != zccPageSize+3 {
		t.Errorf("zccPaginate returned %d records, want %d", len(got), zccPageSize+3)
	}
}

func TestGetZCCAllPagesPreservesFiltersAndReadsEveryPage(t *testing.T) {
	cfg := validReaderConfig()
	sdkCfg := newSDKConfiguration(context.Background(), cfg)

	type item struct {
		ID int `json:"id"`
	}

	var productRequests []*http.Request
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := []byte(`{"access_token":"test-token","expires_in":60}`)
		statusCode := http.StatusOK
		if request.URL.Path == "/zcc/papi/public/v1/getDevices" {
			cloned := request.Clone(request.Context())
			clonedURL := *request.URL
			cloned.URL = &clonedURL
			productRequests = append(productRequests, cloned)

			switch request.URL.Query().Get("page") {
			case "1":
				page := make([]item, zccPageSize)
				for index := range page {
					page[index].ID = index + 1
				}
				var err error
				body, err = json.Marshal(page)
				if err != nil {
					return nil, err
				}
			case "2":
				body = []byte(`[{"id":1001}]`)
			default:
				statusCode = http.StatusBadRequest
				body = []byte(`{"message":"unexpected page"}`)
			}
		} else if request.URL.Path != "/oauth2/v1/token" {
			statusCode = http.StatusNotFound
			body = []byte(`{}`)
		}
		return &http.Response{
			StatusCode: statusCode,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(string(body))),
			Request:    request,
		}, nil
	})
	sdkCfg.HTTPClient.Transport = transport
	sdkCfg.ZCCHTTPClient.Transport = transport

	service, err := zsdk.NewOneAPIClient(sdkCfg)
	if err != nil {
		t.Fatalf("NewOneAPIClient() error = %v, want nil", err)
	}
	t.Cleanup(service.Client.Close)

	items, err := getZCCAllPages[item](
		context.Background(),
		service,
		"/zcc/papi/public/v1/getDevices",
		zcccommon.QueryParams{Username: "operator@example.invalid"},
	)
	if err != nil {
		t.Fatalf("getZCCAllPages(getDevices) error = %v, want nil", err)
	}
	if got, want := len(productRequests), 2; got != want {
		t.Fatalf("getZCCAllPages(getDevices) request count = %d, want %d", got, want)
	}
	for index, request := range productRequests {
		query := request.URL.Query()
		for key, want := range map[string]string{
			"page":     strconv.Itoa(index + 1),
			"pageSize": strconv.Itoa(zccPageSize),
			"username": "operator@example.invalid",
		} {
			if got := query.Get(key); got != want {
				t.Errorf("request %d query[%q] = %q, want %q", index+1, key, got, want)
			}
		}
	}
	if got, want := len(items), zccPageSize+1; got != want {
		t.Fatalf("getZCCAllPages(getDevices) record count = %d, want %d", got, want)
	}
	if got, want := items[len(items)-1].ID, zccPageSize+1; got != want {
		t.Errorf("getZCCAllPages(getDevices) last ID = %d, want %d", got, want)
	}
}

func TestGetZCCAllPagesPreservesAdminRolePageSize(t *testing.T) {
	cfg := validReaderConfig()
	sdkCfg := newSDKConfiguration(context.Background(), cfg)

	type item struct {
		ID int `json:"id"`
	}

	const pageSize = zcccommon.DefaultPageSize
	var productRequests []*http.Request
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := []byte(`{"access_token":"test-token","expires_in":60}`)
		statusCode := http.StatusOK
		if request.URL.Path == "/zcc/papi/public/v1/getAdminRoles" {
			cloned := request.Clone(request.Context())
			clonedURL := *request.URL
			cloned.URL = &clonedURL
			productRequests = append(productRequests, cloned)

			switch request.URL.Query().Get("page") {
			case "1":
				page := make([]item, pageSize)
				for index := range page {
					page[index].ID = index + 1
				}
				var err error
				body, err = json.Marshal(page)
				if err != nil {
					return nil, err
				}
			case "2":
				body = []byte(`[{"id":51}]`)
			default:
				statusCode = http.StatusBadRequest
				body = []byte(`{"message":"unexpected page"}`)
			}
		} else if request.URL.Path != "/oauth2/v1/token" {
			statusCode = http.StatusNotFound
			body = []byte(`{}`)
		}
		return &http.Response{
			StatusCode: statusCode,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(string(body))),
			Request:    request,
		}, nil
	})
	sdkCfg.HTTPClient.Transport = transport
	sdkCfg.ZCCHTTPClient.Transport = transport

	service, err := zsdk.NewOneAPIClient(sdkCfg)
	if err != nil {
		t.Fatalf("NewOneAPIClient() error = %v, want nil", err)
	}
	t.Cleanup(service.Client.Close)

	items, err := getZCCAllPagesWithSize[item](
		context.Background(),
		service,
		"/zcc/papi/public/v1/getAdminRoles",
		zcccommon.QueryParams{},
		pageSize,
	)
	if err != nil {
		t.Fatalf("getZCCAllPagesWithSize(getAdminRoles) error = %v, want nil", err)
	}
	if got, want := len(productRequests), 2; got != want {
		t.Fatalf("getZCCAllPagesWithSize(getAdminRoles) request count = %d, want %d", got, want)
	}
	for index, request := range productRequests {
		query := request.URL.Query()
		for key, want := range map[string]string{
			"page":     strconv.Itoa(index + 1),
			"pageSize": strconv.Itoa(pageSize),
		} {
			if got := query.Get(key); got != want {
				t.Errorf("request %d query[%q] = %q, want %q", index+1, key, got, want)
			}
		}
	}
	if got, want := len(items), pageSize+1; got != want {
		t.Fatalf("getZCCAllPagesWithSize(getAdminRoles) record count = %d, want %d", got, want)
	}
}

func TestZCCListHandlersAvoidUnboundedSDKPagination(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("reader_zcc.go")
	if err != nil {
		t.Fatalf("ReadFile(reader_zcc.go) error = %v, want nil", err)
	}
	source := string(body)
	for _, banned := range []string{
		"zccdevices.GetAll(ctx, service",
		"zccadminroles.GetAdminRoles(ctx, service",
		"zccfailopen.GetFailOpenPolicy(ctx, service",
	} {
		if strings.Contains(source, banned) {
			t.Errorf("reader_zcc.go still calls unbounded SDK pagination: %q", banned)
		}
	}
	for _, required := range []string{
		"return getZCCAllPages[zccfailopen.WebFailOpenPolicy]",
		"return getZCCAllPages[zccdevices.GetDevices]",
		"return getZCCAllPagesWithSize[zccadminroles.AdminRole]",
	} {
		if !strings.Contains(source, required) {
			t.Errorf("reader_zcc.go missing bounded paginator wiring %q", required)
		}
	}
}

func TestZTWPaginateCeilingFailsClosed(t *testing.T) {
	t.Parallel()

	calls := 0
	full := make([]int, ztwPageSize)
	got, err := ztwPaginate(context.Background(), func(_ context.Context, _ int) ([]int, error) {
		calls++
		return full, nil
	})
	if err == nil {
		t.Fatal("ztwPaginate(always-full) error = nil, want ceiling error")
	}
	if got != nil {
		t.Errorf("ztwPaginate(always-full) result = %v, want nil", got)
	}
	if calls != ztwMaxPages {
		t.Errorf("ztwPaginate(always-full) fetch calls = %d, want %d", calls, ztwMaxPages)
	}
}

func TestZTWPaginateDiscardsPartialResultsOnPageError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("ztw page 2 failed")
	got, err := ztwPaginate(context.Background(), func(_ context.Context, page int) ([]int, error) {
		if page == 2 {
			return nil, wantErr
		}
		return make([]int, ztwPageSize), nil
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("ztwPaginate(page error) error = %v, want %v", err, wantErr)
	}
	if got != nil {
		t.Errorf("ztwPaginate(page error) result = %v, want nil", got)
	}
}

func TestGetZTWAllPagesPreservesQueryAndReadsEveryPage(t *testing.T) {
	cfg := validReaderConfig()
	sdkCfg := newSDKConfiguration(context.Background(), cfg)

	type item struct {
		ID int `json:"id"`
	}

	var productRequests []*http.Request
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := []byte(`{"access_token":"test-token","expires_in":60}`)
		statusCode := http.StatusOK
		if request.URL.Path == "/ztw/api/v1/adminUsers" {
			cloned := request.Clone(request.Context())
			clonedURL := *request.URL
			cloned.URL = &clonedURL
			productRequests = append(productRequests, cloned)

			switch request.URL.Query().Get("page") {
			case "1":
				page := make([]item, ztwPageSize)
				for index := range page {
					page[index].ID = index + 1
				}
				var err error
				body, err = json.Marshal(page)
				if err != nil {
					return nil, err
				}
			case "2":
				body = []byte(`[{"id":1001}]`)
			default:
				statusCode = http.StatusBadRequest
				body = []byte(`{"message":"unexpected page"}`)
			}
		} else if request.URL.Path != "/oauth2/v1/token" {
			statusCode = http.StatusNotFound
			body = []byte(`{}`)
		}
		return &http.Response{
			StatusCode: statusCode,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(string(body))),
			Request:    request,
		}, nil
	})
	sdkCfg.HTTPClient.Transport = transport
	sdkCfg.ZTWHTTPClient.Transport = transport

	service, err := zsdk.NewOneAPIClient(sdkCfg)
	if err != nil {
		t.Fatalf("NewOneAPIClient() error = %v, want nil", err)
	}
	t.Cleanup(service.Client.Close)

	items, err := getZTWAllPages[item](
		context.Background(),
		service,
		"/ztw/api/v1/adminUsers?includeAuditorUsers=true&includeAdminUsers=true",
	)
	if err != nil {
		t.Fatalf("getZTWAllPages(adminUsers) error = %v, want nil", err)
	}
	if got, want := len(productRequests), 2; got != want {
		t.Fatalf("getZTWAllPages(adminUsers) request count = %d, want %d", got, want)
	}
	for index, request := range productRequests {
		query := request.URL.Query()
		for key, want := range map[string]string{
			"includeAdminUsers":   "true",
			"includeAuditorUsers": "true",
			"page":                strconv.Itoa(index + 1),
			"pageSize":            strconv.Itoa(ztwPageSize),
		} {
			if got := query.Get(key); got != want {
				t.Errorf("request %d query[%q] = %q, want %q", index+1, key, got, want)
			}
		}
	}
	if got, want := len(items), ztwPageSize+1; got != want {
		t.Fatalf("getZTWAllPages(adminUsers) record count = %d, want %d", got, want)
	}
	if got, want := items[len(items)-1].ID, ztwPageSize+1; got != want {
		t.Errorf("getZTWAllPages(adminUsers) last ID = %d, want %d", got, want)
	}
}

func TestZTWListHandlersAvoidUnboundedSDKPagination(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("reader_ztw.go")
	if err != nil {
		t.Fatalf("ReadFile(reader_ztw.go) error = %v, want nil", err)
	}
	source := string(body)
	for _, banned := range []string{
		"ztwworkloadgroups.GetAll(ctx, service)",
		"ztwpubliccloudaccount.GetAll(ctx, service)",
		"ztwdnsgateway.GetAll(ctx, service)",
		"ztwziaforwardinggateway.GetAll(ctx, service)",
		"ztwecgroup.GetAll(ctx, service)",
		"ztwipsourcegroups.GetAll(ctx, service)",
		"ztwipdestinationgroups.GetAll(ctx, service)",
		"ztwipgroups.GetAll(ctx, service)",
		"ztwnetworkservices.GetAllNetworkServices(ctx, service)",
		"ztwnetworkservicegroups.GetAllNetworkServiceGroups(ctx, service)",
		"ztwadminusers.GetAllAdminUsers(ctx, service)",
		"ztwlocation.GetAll(ctx, service)",
		"ztwlocationtemplate.GetAll(ctx, service)",
		"ztwaccountgroups.GetAllAccountGroups(ctx, service)",
		"ztwpubliccloudinfo.GetAllPublicCloudInfo(ctx, service)",
		"ztwzparesources.GetZPAApplicationSegments(ctx, service)",
		"ztwforwardingrules.GetAll(ctx, service)",
		"ztwtrafficdnsrules.GetAll(ctx, service)",
		"ztwtrafficlogrules.GetAll(ctx, service)",
	} {
		if strings.Contains(source, banned) {
			t.Errorf("reader_ztw.go still calls unbounded SDK pagination: %q", banned)
		}
	}
	if got, want := strings.Count(source, "return getZTWAllPages["), 19; got != want {
		t.Errorf("reader_ztw.go bounded paginator wiring count = %d, want %d", got, want)
	}
	if strings.Contains(source, "ztwcommon.ReadPage") {
		t.Error("reader_ztw.go uses SDK ReadPage, which routes through the generic read method instead of ReadResource")
	}
	if !strings.Contains(source, "service.Client.ReadResource") {
		t.Error("reader_ztw.go missing ZTW-specific ReadResource pagination path")
	}
}

// TestZIAPaginateCeilingFailsClosed drives the ziaPaginate page ceiling.
func TestZIAPaginateCeilingFailsClosed(t *testing.T) {
	t.Parallel()

	const pageSize = 1
	calls := 0
	_, err := ziaPaginate(context.Background(), pageSize, func(_ context.Context, page, _ int) ([]int, error) {
		calls++
		return []int{page}, nil
	})
	if err == nil {
		t.Fatal("ziaPaginate(always-full) error = nil, want ceiling error")
	}
	if !strings.Contains(err.Error(), "ceiling") {
		t.Errorf("ziaPaginate error = %q, want it to mention the ceiling", err.Error())
	}
	if calls != ziaMaxPages {
		t.Errorf("ziaPaginate fetched %d pages, want exactly the ceiling of %d", calls, ziaMaxPages)
	}
}

func TestZIAPaginateRejectsInvalidPageSize(t *testing.T) {
	t.Parallel()

	calls := 0
	got, err := ziaPaginate(context.Background(), 0, func(_ context.Context, _, _ int) ([]int, error) {
		calls++
		return nil, nil
	})
	if err == nil || !strings.Contains(err.Error(), "page size must be positive") {
		t.Fatalf("ziaPaginate(page size 0) error = %v, want positive-page-size error", err)
	}
	if got != nil {
		t.Errorf("ziaPaginate(page size 0) result = %#v, want nil", got)
	}
	if calls != 0 {
		t.Errorf("ziaPaginate(page size 0) calls = %d, want 0", calls)
	}
}

func TestZIAPaginateStopsOnShortPage(t *testing.T) {
	t.Parallel()

	const pageSize = 1000
	pages := [][]int{make([]int, pageSize), make([]int, 5)}
	got, err := ziaPaginate(context.Background(), pageSize, func(_ context.Context, page, _ int) ([]int, error) {
		return pages[page-1], nil
	})
	if err != nil {
		t.Fatalf("ziaPaginate error = %v, want nil", err)
	}
	if len(got) != pageSize+5 {
		t.Errorf("ziaPaginate returned %d records, want %d", len(got), pageSize+5)
	}
}

func TestZIAPaginateContinuesWhenServerClampsPageSize(t *testing.T) {
	t.Parallel()

	const requestedPageSize = 1000
	first := make([]int, 20)
	second := make([]int, 20)
	for index := range first {
		first[index] = index + 1
		second[index] = index + 21
	}
	pages := [][]int{
		first,
		second,
		{41, 42, 43},
	}
	calls := 0
	got, err := ziaPaginate(context.Background(), requestedPageSize, func(_ context.Context, page, size int) ([]int, error) {
		calls++
		if size != requestedPageSize {
			t.Errorf("ziaPaginate(clamped) requested page size = %d, want %d", size, requestedPageSize)
		}
		return pages[page-1], nil
	})
	if err != nil {
		t.Fatalf("ziaPaginate(clamped) error = %v, want nil", err)
	}
	if got, want := len(got), 43; got != want {
		t.Errorf("ziaPaginate(clamped) record count = %d, want %d", got, want)
	}
	if calls != len(pages) {
		t.Errorf("ziaPaginate(clamped) calls = %d, want %d", calls, len(pages))
	}
}

func TestZIAPaginateConfirmsShortFirstPageWithEmptySecondPage(t *testing.T) {
	t.Parallel()

	const requestedPageSize = 1000
	pages := [][]int{{1, 2, 3}, nil}
	calls := 0
	got, err := ziaPaginate(context.Background(), requestedPageSize, func(_ context.Context, page, _ int) ([]int, error) {
		calls++
		return pages[page-1], nil
	})
	if err != nil {
		t.Fatalf("ziaPaginate(short first page) error = %v, want nil", err)
	}
	if got, want := len(got), 3; got != want {
		t.Errorf("ziaPaginate(short first page) record count = %d, want %d", got, want)
	}
	if calls != len(pages) {
		t.Errorf("ziaPaginate(short first page) calls = %d, want %d", calls, len(pages))
	}
}

func TestZIAPaginateRepeatedPageFailsClosed(t *testing.T) {
	t.Parallel()

	const requestedPageSize = 20
	pageItems := make([]int, requestedPageSize)
	for index := range pageItems {
		pageItems[index] = index + 1
	}
	calls := 0
	got, err := ziaPaginate(context.Background(), requestedPageSize, func(_ context.Context, _, _ int) ([]int, error) {
		calls++
		return pageItems, nil
	})
	if err == nil || !strings.Contains(err.Error(), "repeated page") {
		t.Fatalf("ziaPaginate(repeated page) error = %v, want repeated-page error", err)
	}
	if got != nil {
		t.Errorf("ziaPaginate(repeated page) result = %#v, want nil", got)
	}
	if calls != 2 {
		t.Errorf("ziaPaginate(repeated page) calls = %d, want 2", calls)
	}
}

func TestZIAPaginatePageWidthGrowthFailsClosed(t *testing.T) {
	t.Parallel()

	const requestedPageSize = 1000
	pages := [][]int{make([]int, 20), make([]int, 21)}
	got, err := ziaPaginate(context.Background(), requestedPageSize, func(_ context.Context, page, _ int) ([]int, error) {
		return pages[page-1], nil
	})
	if err == nil || !strings.Contains(err.Error(), "page width changed") {
		t.Fatalf("ziaPaginate(growing page width) error = %v, want page-width error", err)
	}
	if got != nil {
		t.Errorf("ziaPaginate(growing page width) result = %#v, want nil", got)
	}
}

func TestZIAPaginateDiscardsPartialResultsOnPageError(t *testing.T) {
	t.Parallel()

	const pageSize = 100
	wantErr := errors.New("page 2 failed")
	got, err := ziaPaginate(context.Background(), pageSize, func(_ context.Context, page, _ int) ([]int, error) {
		if page == 1 {
			return make([]int, pageSize), nil
		}
		return nil, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("ziaPaginate(page error) error = %v, want %v", err, wantErr)
	}
	if got != nil {
		t.Errorf("ziaPaginate(page error) result = %#v, want nil", got)
	}
}

func TestGetZIAAllPagesPreservesQueryAndReadsEveryPage(t *testing.T) {
	cfg := validReaderConfig()
	sdkCfg := newSDKConfiguration(context.Background(), cfg)

	type item struct {
		ID int `json:"id"`
	}

	const pageSize = 2
	var productRequests []*http.Request
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := []byte(`{"access_token":"test-token","expires_in":60}`)
		statusCode := http.StatusOK
		if request.URL.Path == "/zia/api/v1/adminUsers" {
			cloned := request.Clone(request.Context())
			clonedURL := *request.URL
			cloned.URL = &clonedURL
			productRequests = append(productRequests, cloned)

			switch request.URL.Query().Get("page") {
			case "1":
				body = []byte(`[{"id":1},{"id":2}]`)
			case "2":
				body = []byte(`[{"id":3}]`)
			default:
				statusCode = http.StatusBadRequest
				body = []byte(`{"message":"unexpected page"}`)
			}
		} else if request.URL.Path != "/oauth2/v1/token" {
			statusCode = http.StatusNotFound
			body = []byte(`{}`)
		}
		return &http.Response{
			StatusCode: statusCode,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(string(body))),
			Request:    request,
		}, nil
	})
	sdkCfg.HTTPClient.Transport = transport
	sdkCfg.ZIAHTTPClient.Transport = transport

	service, err := zsdk.NewOneAPIClient(sdkCfg)
	if err != nil {
		t.Fatalf("NewOneAPIClient() error = %v, want nil", err)
	}
	t.Cleanup(service.Client.Close)

	items, err := getZIAAllPagesWithSize[item](
		context.Background(),
		service,
		"/zia/api/v1/adminUsers?includeAuditorUsers=true&includeAdminUsers=true",
		pageSize,
	)
	if err != nil {
		t.Fatalf("getZIAAllPagesWithSize(adminUsers) error = %v, want nil", err)
	}
	if got, want := len(productRequests), 2; got != want {
		t.Fatalf("getZIAAllPagesWithSize(adminUsers) request count = %d, want %d", got, want)
	}
	for index, request := range productRequests {
		query := request.URL.Query()
		for key, want := range map[string]string{
			"includeAdminUsers":   "true",
			"includeAuditorUsers": "true",
			"page":                strconv.Itoa(index + 1),
			"pageSize":            strconv.Itoa(pageSize),
		} {
			if got := query.Get(key); got != want {
				t.Errorf("request %d query[%q] = %q, want %q", index+1, key, got, want)
			}
		}
	}
	if got, want := len(items), 3; got != want {
		t.Fatalf("getZIAAllPagesWithSize(adminUsers) record count = %d, want %d", got, want)
	}
	if got, want := items[len(items)-1].ID, 3; got != want {
		t.Errorf("getZIAAllPagesWithSize(adminUsers) last ID = %d, want %d", got, want)
	}
}

func TestNormalizeZIABrowserIsolationListError(t *testing.T) {
	t.Parallel()

	subscriptionErr := errors.New(
		"request failed: Cloud Browser Isolation subscription is required (status 403)",
	)
	got := normalizeZIABrowserIsolationListError(subscriptionErr)
	if want := "NOT_SUBSCRIBED: Cloud Browser Isolation subscription is required"; got == nil || got.Error() != want {
		t.Errorf("normalize subscription error = %v, want %q", got, want)
	}

	otherErr := errors.New("ordinary browser isolation failure")
	if got := normalizeZIABrowserIsolationListError(otherErr); !errors.Is(got, otherErr) {
		t.Errorf("normalize ordinary error = %v, want original error", got)
	}
	if got := normalizeZIABrowserIsolationListError(nil); got != nil {
		t.Errorf("normalize nil error = %v, want nil", got)
	}
}

func TestGetZIAUsersAllPagesPreservesSDKSortDefaults(t *testing.T) {
	cfg := validReaderConfig()
	sdkCfg := newSDKConfiguration(context.Background(), cfg)

	var productRequest *http.Request
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := `{"access_token":"test-token","expires_in":60}`
		statusCode := http.StatusOK
		if request.URL.Path == "/zia/api/v1/users" {
			cloned := request.Clone(request.Context())
			clonedURL := *request.URL
			cloned.URL = &clonedURL
			productRequest = cloned
			body = `[]`
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
	sdkCfg.ZIAHTTPClient.Transport = transport

	service, err := zsdk.NewOneAPIClient(sdkCfg)
	if err != nil {
		t.Fatalf("NewOneAPIClient() error = %v, want nil", err)
	}
	t.Cleanup(service.Client.Close)

	if _, err := getZIAUsersAllPages(context.Background(), service); err != nil {
		t.Fatalf("getZIAUsersAllPages() error = %v, want nil", err)
	}
	if productRequest == nil {
		t.Fatal("getZIAUsersAllPages() product request = nil, want users request")
	}
	query := productRequest.URL.Query()
	for key, want := range map[string]string{
		"page":      "1",
		"pageSize":  "10000",
		"sortBy":    "name",
		"sortOrder": "asc",
	} {
		if got := query.Get(key); got != want {
			t.Errorf("getZIAUsersAllPages() query[%q] = %q, want %q", key, got, want)
		}
	}
}

func TestGetZIAURLCategoriesAllRequestsAllCategoryTypes(t *testing.T) {
	cfg := validReaderConfig()
	sdkCfg := newSDKConfiguration(context.Background(), cfg)

	var productRequests []*http.Request
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := []byte(`{"access_token":"test-token","expires_in":60}`)
		statusCode := http.StatusOK
		if request.URL.Path == "/zia/api/v1/urlCategories" {
			cloned := request.Clone(request.Context())
			clonedURL := *request.URL
			cloned.URL = &clonedURL
			productRequests = append(productRequests, cloned)
			switch request.URL.Query().Get("page") {
			case "1":
				categories := make([]map[string]any, 20)
				for index := range categories {
					categories[index] = map[string]any{
						"id":   fmt.Sprintf("URL_CATEGORY_%02d", index+1),
						"type": "URL_CATEGORY",
					}
				}
				var err error
				body, err = json.Marshal(categories)
				if err != nil {
					return nil, err
				}
			case "2":
				body = []byte(`[
					{"id":"CUSTOM_URL","type":"URL_CATEGORY"},
					{"id":"CUSTOM_TLD","type":"TLD_CATEGORY","customUrlsCount":1}
				]`)
			default:
				statusCode = http.StatusBadRequest
				body = []byte(`{"message":"unexpected page"}`)
			}
		} else if request.URL.Path != "/oauth2/v1/token" {
			statusCode = http.StatusNotFound
			body = []byte(`{}`)
		}
		return &http.Response{
			StatusCode: statusCode,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(string(body))),
			Request:    request,
		}, nil
	})
	sdkCfg.HTTPClient.Transport = transport
	sdkCfg.ZIAHTTPClient.Transport = transport

	service, err := zsdk.NewOneAPIClient(sdkCfg)
	if err != nil {
		t.Fatalf("NewOneAPIClient() error = %v, want nil", err)
	}
	t.Cleanup(service.Client.Close)

	categories, err := getZIAURLCategoriesAll(context.Background(), service)
	if err != nil {
		t.Fatalf("getZIAURLCategoriesAll() error = %v, want nil", err)
	}
	if got, want := len(productRequests), 2; got != want {
		t.Fatalf("getZIAURLCategoriesAll() product request count = %d, want %d", got, want)
	}
	for index, productRequest := range productRequests {
		if got, want := productRequest.URL.Host, "api.zsapi.net"; got != want {
			t.Errorf("getZIAURLCategoriesAll() request %d host = %q, want %q", index+1, got, want)
		}
		query := productRequest.URL.Query()
		for key, want := range map[string]string{
			"includeOnlyUrlKeywordCounts": "true",
			"page":                        strconv.Itoa(index + 1),
			"pageSize":                    "5000",
			"type":                        "ALL",
		} {
			if got := query.Get(key); got != want {
				t.Errorf("getZIAURLCategoriesAll() request %d query[%q] = %q, want %q", index+1, key, got, want)
			}
		}
	}
	if got, want := len(categories), 22; got != want {
		t.Fatalf("getZIAURLCategoriesAll() category count = %d, want %d", got, want)
	}
	if got, want := categories[20].Type, "URL_CATEGORY"; got != want {
		t.Errorf("getZIAURLCategoriesAll() categories[20].Type = %q, want %q", got, want)
	}
	if got, want := categories[21].Type, "TLD_CATEGORY"; got != want {
		t.Errorf("getZIAURLCategoriesAll() categories[21].Type = %q, want %q", got, want)
	}
	if got, want := categories[21].CustomUrlsCount, 1; got != want {
		t.Errorf("getZIAURLCategoriesAll() categories[21].CustomUrlsCount = %d, want %d", got, want)
	}
}

func TestGetZIALocationGroupsAllPagesFetchesMemberLocations(t *testing.T) {
	cfg := validReaderConfig()
	sdkCfg := newSDKConfiguration(context.Background(), cfg)

	var productRequests []*http.Request
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := []byte(`{"access_token":"test-token","expires_in":60}`)
		statusCode := http.StatusOK
		if request.URL.Path == "/zia/api/v1/locations/groups" {
			cloned := request.Clone(request.Context())
			clonedURL := *request.URL
			cloned.URL = &clonedURL
			productRequests = append(productRequests, cloned)
			switch request.URL.Query().Get("page") {
			case "1":
				page := make([]locationgroups.LocationGroup, 1000)
				for index := range page {
					page[index] = locationgroups.LocationGroup{
						ID:   index + 1,
						Name: "pagination-regression-group",
					}
				}
				encoded, err := json.Marshal(page)
				if err != nil {
					return nil, err
				}
				body = encoded
			case "2":
				body = []byte(`[
					{
						"id": 1001,
						"name": "Branches",
						"groupType": "STATIC_GROUP",
						"locations": [
							{"id": 601, "name": "Branch parent"},
							{"id": 602, "name": "Branch sublocation"}
						]
					}
				]`)
			default:
				statusCode = http.StatusBadRequest
				body = []byte(`{"message":"unexpected page"}`)
			}
		} else if request.URL.Path != "/oauth2/v1/token" {
			statusCode = http.StatusNotFound
			body = []byte(`{}`)
		}
		return &http.Response{
			StatusCode: statusCode,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(string(body))),
			Request:    request,
		}, nil
	})
	sdkCfg.HTTPClient.Transport = transport
	sdkCfg.ZIAHTTPClient.Transport = transport

	service, err := zsdk.NewOneAPIClient(sdkCfg)
	if err != nil {
		t.Fatalf("NewOneAPIClient() error = %v, want nil", err)
	}
	t.Cleanup(service.Client.Close)

	groups, err := getZIALocationGroupsAllPages(context.Background(), service)
	if err != nil {
		t.Fatalf("getZIALocationGroupsAllPages() error = %v, want nil", err)
	}
	if got, want := len(productRequests), 2; got != want {
		t.Fatalf("getZIALocationGroupsAllPages() request count = %d, want %d", got, want)
	}
	for index, request := range productRequests {
		if got, want := request.URL.Host, "api.zsapi.net"; got != want {
			t.Errorf("request %d host = %q, want %q", index+1, got, want)
		}
		query := request.URL.Query()
		for key, want := range map[string]string{
			"fetchLocations": "true",
			"page":           strconv.Itoa(index + 1),
			"pageSize":       "1000",
		} {
			if got := query.Get(key); got != want {
				t.Errorf("request %d query[%q] = %q, want %q", index+1, key, got, want)
			}
		}
	}
	if got, want := len(groups), 1001; got != want {
		t.Fatalf("getZIALocationGroupsAllPages() group count = %d, want %d", got, want)
	}
	memberGroup := groups[len(groups)-1]
	if got, want := len(memberGroup.Locations), 2; got != want {
		t.Fatalf("getZIALocationGroupsAllPages() member count = %d, want %d", got, want)
	}
	if got, want := memberGroup.Locations[1].ID, 602; got != want {
		t.Errorf("groups[last].Locations[1].ID = %d, want %d", got, want)
	}
	if got, want := memberGroup.Locations[1].Name, "Branch sublocation"; got != want {
		t.Errorf("groups[last].Locations[1].Name = %q, want %q", got, want)
	}
}

func TestGetZIASublocationByIDPreservesEarlyMatchAndParentTolerance(t *testing.T) {
	tests := []struct {
		name             string
		firstParentFails bool
		wantSecondParent bool
	}{
		{
			name: "returns before unrelated later parent",
		},
		{
			name:             "skips inaccessible earlier parent",
			firstParentFails: true,
			wantSecondParent: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := validReaderConfig()
			sdkCfg := newSDKConfiguration(context.Background(), cfg)

			var parentPaths []string
			transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
				body := `{"access_token":"test-token","expires_in":60}`
				statusCode := http.StatusOK
				switch request.URL.Path {
				case "/zia/api/v1/locations":
					if request.URL.Query().Get("page") == "1" {
						body = `[{"id":1,"name":"first"},{"id":2,"name":"second"}]`
					} else {
						body = `[]`
					}
				case "/zia/api/v1/locations/1/sublocations":
					parentPaths = append(parentPaths, request.URL.Path)
					if test.firstParentFails {
						statusCode = http.StatusInternalServerError
						body = `{"message":"parent unavailable"}`
					} else if request.URL.Query().Get("page") == "1" {
						body = `[{"id":99,"name":"target"}]`
					} else {
						body = `[]`
					}
				case "/zia/api/v1/locations/2/sublocations":
					parentPaths = append(parentPaths, request.URL.Path)
					if test.firstParentFails && request.URL.Query().Get("page") == "1" {
						body = `[{"id":99,"name":"target"}]`
					} else if test.firstParentFails {
						body = `[]`
					} else {
						statusCode = http.StatusInternalServerError
						body = `{"message":"later parent unavailable"}`
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
			sdkCfg.HTTPClient.Transport = transport
			sdkCfg.ZIAHTTPClient.Transport = transport

			service, err := zsdk.NewOneAPIClient(sdkCfg)
			if err != nil {
				t.Fatalf("NewOneAPIClient() error = %v, want nil", err)
			}
			t.Cleanup(service.Client.Close)

			item, err := getZIASublocationByID(context.Background(), service, 99)
			if err != nil {
				t.Fatalf("getZIASublocationByID(99) error = %v, want nil", err)
			}
			if item == nil || item.ID != 99 {
				t.Fatalf("getZIASublocationByID(99) = %#v, want ID 99", item)
			}
			firstPath := "/zia/api/v1/locations/1/sublocations"
			secondPath := "/zia/api/v1/locations/2/sublocations"
			firstCount := 0
			secondCount := 0
			for _, path := range parentPaths {
				switch path {
				case firstPath:
					firstCount++
				case secondPath:
					secondCount++
				}
			}
			if firstCount == 0 {
				t.Errorf("sublocation parent paths = %v, want at least one first-parent request", parentPaths)
			}
			if got := secondCount > 0; got != test.wantSecondParent {
				t.Errorf(
					"sublocation second-parent request presence = %t, want %t (paths %v)",
					got,
					test.wantSecondParent,
					parentPaths,
				)
			}
		})
	}
}

func TestGetZIAURLFilteringRulesAllPages(t *testing.T) {
	tests := []struct {
		name       string
		pageCounts []int
		wantCount  int
	}{
		{name: "short final page", pageCounts: []int{100, 33}, wantCount: 133},
		{name: "exact multiple requires empty terminal page", pageCounts: []int{100, 0}, wantCount: 100},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := validReaderConfig()
			sdkCfg := newSDKConfiguration(context.Background(), cfg)

			var productRequests []*http.Request
			transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
				body := []byte(`{"access_token":"test-token","expires_in":60}`)
				statusCode := http.StatusOK
				if request.URL.Path == "/zia/api/v1/urlFilteringRules" {
					cloned := request.Clone(request.Context())
					clonedURL := *request.URL
					cloned.URL = &clonedURL
					productRequests = append(productRequests, cloned)

					pageIndex := -1
					for index := range test.pageCounts {
						if request.URL.Query().Get("page") == strconv.Itoa(index+1) {
							pageIndex = index
							break
						}
					}
					if pageIndex == -1 {
						statusCode = http.StatusBadRequest
						body = []byte(`{"message":"unexpected page"}`)
					} else {
						startID := 1
						for _, priorCount := range test.pageCounts[:pageIndex] {
							startID += priorCount
						}
						rules := make([]urlfilteringpolicies.URLFilteringRule, test.pageCounts[pageIndex])
						for index := range rules {
							rules[index] = urlfilteringpolicies.URLFilteringRule{
								ID:   startID + index,
								Name: "pagination-regression-rule",
							}
						}
						var err error
						body, err = json.Marshal(rules)
						if err != nil {
							return nil, err
						}
					}
				} else if request.URL.Path != "/oauth2/v1/token" {
					statusCode = http.StatusNotFound
					body = []byte(`{}`)
				}
				return &http.Response{
					StatusCode: statusCode,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(string(body))),
					Request:    request,
				}, nil
			})
			sdkCfg.HTTPClient.Transport = transport
			sdkCfg.ZIAHTTPClient.Transport = transport

			service, err := zsdk.NewOneAPIClient(sdkCfg)
			if err != nil {
				t.Fatalf("NewOneAPIClient() error = %v, want nil", err)
			}
			t.Cleanup(service.Client.Close)

			rules, err := getZIAURLFilteringRulesAllPages(context.Background(), service)
			if err != nil {
				t.Fatalf("getZIAURLFilteringRulesAllPages() error = %v, want nil", err)
			}
			if got, want := len(productRequests), len(test.pageCounts); got != want {
				t.Fatalf("getZIAURLFilteringRulesAllPages() request count = %d, want %d", got, want)
			}
			for index, request := range productRequests {
				if got, want := request.URL.Host, "api.zsapi.net"; got != want {
					t.Errorf("request %d host = %q, want %q", index+1, got, want)
				}
				query := request.URL.Query()
				if got, want := query.Get("page"), strconv.Itoa(index+1); got != want {
					t.Errorf("request %d page = %q, want %q", index+1, got, want)
				}
				if got, want := query.Get("pageSize"), "100"; got != want {
					t.Errorf("request %d pageSize = %q, want %q", index+1, got, want)
				}
			}
			if got, want := len(rules), test.wantCount; got != want {
				t.Fatalf("getZIAURLFilteringRulesAllPages() rule count = %d, want %d", got, want)
			}
			if got, want := rules[0].ID, 1; got != want {
				t.Errorf("rules[0].ID = %d, want %d", got, want)
			}
			if got, want := rules[len(rules)-1].ID, test.wantCount; got != want {
				t.Errorf("rules[last].ID = %d, want %d", got, want)
			}
		})
	}
}

// TestZIAHighRecordEndpointsAvoidUnboundedSDKPagination guards against
// regressing the wrapped users/locations/location-groups/url-categories/
// url-filtering-rules endpoints back to unbounded or single-page SDK calls,
// mirroring the networkApplications source guard.
func TestZIAHighRecordEndpointsAvoidUnboundedSDKPagination(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile("reader_zia.go")
	if err != nil {
		t.Fatalf("ReadFile(reader_zia.go) error = %v, want nil", err)
	}
	source := string(body)

	for _, banned := range []string{
		"return ziausers.GetAllUsers(ctx, service",
		"return locationmanagement.GetAll(ctx, service)",
		"return locationgroups.GetAll(ctx, service",
		"return urlcategories.GetAll(ctx, service",
		"return urlfilteringpolicies.GetAll(ctx, service)",
	} {
		if strings.Contains(source, banned) {
			t.Errorf("reader_zia.go still calls unbounded SDK pagination: %q", banned)
		}
	}
	for _, want := range []string{
		"getZIAUsersAllPages(ctx, service)",
		"getZIALocationsAllPages(ctx, service)",
		"getZIALocationGroupsAllPages(ctx, service)",
		"getZIAURLCategoriesAll(ctx, service)",
		"getZIAURLFilteringRulesAllPages(ctx, service)",
	} {
		if !strings.Contains(source, want) {
			t.Errorf("reader_zia.go missing bounded paginator wiring: %q", want)
		}
	}
}
