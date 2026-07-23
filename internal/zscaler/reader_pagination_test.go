package zscaler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	zsdk "github.com/zscaler/zscaler-sdk-go/v3/zscaler"
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

// TestZIAPaginateCeilingFailsClosed drives the ziaPaginate page ceiling.
func TestZIAPaginateCeilingFailsClosed(t *testing.T) {
	t.Parallel()

	const pageSize = 10000
	calls := 0
	full := make([]int, pageSize)
	_, err := ziaPaginate(context.Background(), pageSize, func(_ context.Context, page, size int) ([]int, error) {
		calls++
		return full, nil
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

func TestGetZIAURLCategoriesAllRequestsAllCategoryTypes(t *testing.T) {
	cfg := validReaderConfig()
	sdkCfg := newSDKConfiguration(context.Background(), cfg)

	var productRequest *http.Request
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := `{"access_token":"test-token","expires_in":60}`
		statusCode := http.StatusOK
		if request.URL.Path == "/zia/api/v1/urlCategories" {
			cloned := request.Clone(request.Context())
			clonedURL := *request.URL
			cloned.URL = &clonedURL
			productRequest = cloned
			body = `[
				{"id":"CUSTOM_URL","type":"URL_CATEGORY"},
				{"id":"CUSTOM_TLD","type":"TLD_CATEGORY","customUrlsCount":1}
			]`
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

	categories, err := getZIAURLCategoriesAll(context.Background(), service)
	if err != nil {
		t.Fatalf("getZIAURLCategoriesAll() error = %v, want nil", err)
	}
	if productRequest == nil {
		t.Fatal("getZIAURLCategoriesAll() product request = nil, want URL-category request")
	}
	if got, want := productRequest.URL.Host, "api.zsapi.net"; got != want {
		t.Errorf("getZIAURLCategoriesAll() host = %q, want %q", got, want)
	}
	query := productRequest.URL.Query()
	for key, want := range map[string]string{
		"includeOnlyUrlKeywordCounts": "true",
		"page":                        "1",
		"pageSize":                    "5000",
		"type":                        "ALL",
	} {
		if got := query.Get(key); got != want {
			t.Errorf("getZIAURLCategoriesAll() query[%q] = %q, want %q", key, got, want)
		}
	}
	if got, want := len(categories), 2; got != want {
		t.Fatalf("getZIAURLCategoriesAll() category count = %d, want %d", got, want)
	}
	if got, want := categories[0].Type, "URL_CATEGORY"; got != want {
		t.Errorf("getZIAURLCategoriesAll() categories[0].Type = %q, want %q", got, want)
	}
	if got, want := categories[1].Type, "TLD_CATEGORY"; got != want {
		t.Errorf("getZIAURLCategoriesAll() categories[1].Type = %q, want %q", got, want)
	}
	if got, want := categories[1].CustomUrlsCount, 1; got != want {
		t.Errorf("getZIAURLCategoriesAll() categories[1].CustomUrlsCount = %d, want %d", got, want)
	}
}

func TestGetZIAURLFilteringRulesAllPages(t *testing.T) {
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

			startID, count := 0, 0
			switch request.URL.Query().Get("page") {
			case "1":
				startID, count = 1, 100
			case "2":
				startID, count = 101, 33
			default:
				statusCode = http.StatusBadRequest
				body = []byte(`{"message":"unexpected page"}`)
			}
			if statusCode == http.StatusOK {
				rules := make([]urlfilteringpolicies.URLFilteringRule, count)
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
	if got, want := len(productRequests), 2; got != want {
		t.Fatalf("getZIAURLFilteringRulesAllPages() request count = %d, want %d", got, want)
	}
	for index, request := range productRequests {
		if got, want := request.URL.Host, "api.zsapi.net"; got != want {
			t.Errorf("request %d host = %q, want %q", index+1, got, want)
		}
		query := request.URL.Query()
		if got, want := query.Get("page"), []string{"1", "2"}[index]; got != want {
			t.Errorf("request %d page = %q, want %q", index+1, got, want)
		}
		if got, want := query.Get("pageSize"), "100"; got != want {
			t.Errorf("request %d pageSize = %q, want %q", index+1, got, want)
		}
	}
	if got, want := len(rules), 133; got != want {
		t.Fatalf("getZIAURLFilteringRulesAllPages() rule count = %d, want %d", got, want)
	}
	if got, want := rules[0].ID, 1; got != want {
		t.Errorf("rules[0].ID = %d, want %d", got, want)
	}
	if got, want := rules[len(rules)-1].ID, 133; got != want {
		t.Errorf("rules[last].ID = %d, want %d", got, want)
	}
}

// TestZIAHighRecordEndpointsAvoidUnboundedSDKPagination guards against
// regressing the wrapped users/locations/url-categories/url-filtering-rules
// endpoints back to unbounded or single-page SDK calls, mirroring the
// networkApplications source guard.
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
		"getZIAURLCategoriesAll(ctx, service)",
		"getZIAURLFilteringRulesAllPages(ctx, service)",
	} {
		if !strings.Contains(source, want) {
			t.Errorf("reader_zia.go missing bounded paginator wiring: %q", want)
		}
	}
}
