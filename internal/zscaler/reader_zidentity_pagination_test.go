package zscaler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	zsdk "github.com/zscaler/zscaler-sdk-go/v3/zscaler"
)

func TestReadAllZidentityPagesCollectsShortPagesWithContinuation(t *testing.T) {
	t.Parallel()

	pages := []zidentityPage[int]{
		{
			records:      zidentityTestRecords(0, 100),
			resultsTotal: 250,
			pageOffset:   0,
			pageSize:     100,
			nextLink:     "/admin/api/v1/users?offset=100&limit=100",
		},
		{
			records:      zidentityTestRecords(100, 100),
			resultsTotal: 250,
			pageOffset:   100,
			pageSize:     100,
			nextLink:     "/admin/api/v1/users?offset=200&limit=100",
		},
		{
			records:      zidentityTestRecords(200, 50),
			resultsTotal: 250,
			pageOffset:   200,
			pageSize:     100,
		},
	}
	var offsets []int

	got, err := readAllZidentityPages(context.Background(), func(_ context.Context, offset, limit int) (zidentityPage[int], error) {
		if limit != zidentityPageLimit {
			t.Errorf("readAllZidentityPages limit = %d, want %d", limit, zidentityPageLimit)
		}
		offsets = append(offsets, offset)
		page := pages[len(offsets)-1]
		return page, nil
	})
	if err != nil {
		t.Fatalf("readAllZidentityPages(short pages with continuation) error = %v, want nil", err)
	}
	want := zidentityTestRecords(0, 250)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("readAllZidentityPages(short pages with continuation) = %v, want %v", got, want)
	}
	if wantOffsets := []int{0, 100, 200}; !reflect.DeepEqual(offsets, wantOffsets) {
		t.Errorf("readAllZidentityPages(short pages with continuation) offsets = %v, want %v", offsets, wantOffsets)
	}
}

func TestReadAllZidentityPagesUsesOwnOffsetForContinuation(t *testing.T) {
	t.Parallel()

	var offsets []int
	got, err := readAllZidentityPages(context.Background(), func(_ context.Context, offset, _ int) (zidentityPage[int], error) {
		offsets = append(offsets, offset)
		switch offset {
		case 0:
			return zidentityPage[int]{
				records:      []int{1, 2},
				resultsTotal: 4,
				pageOffset:   0,
				pageSize:     2,
				nextLink:     "https://attacker.example/admin/api/v1/users?offset=100&limit=2",
			}, nil
		case 2:
			return zidentityPage[int]{
				records:      []int{3, 4},
				resultsTotal: 4,
				pageOffset:   2,
				pageSize:     2,
			}, nil
		default:
			return zidentityPage[int]{}, errors.New("unexpected offset")
		}
	})
	if err != nil {
		t.Fatalf("readAllZidentityPages(own offset) error = %v, want nil", err)
	}
	if want := []int{1, 2, 3, 4}; !reflect.DeepEqual(got, want) {
		t.Errorf("readAllZidentityPages(own offset) = %v, want %v", got, want)
	}
	if want := []int{0, 2}; !reflect.DeepEqual(offsets, want) {
		t.Errorf("readAllZidentityPages(own offset) offsets = %v, want %v", offsets, want)
	}
}

func TestReadAllZidentityPagesRejectsMissingContinuationWithRemainingRecords(t *testing.T) {
	t.Parallel()

	fullPage := zidentityTestRecords(0, zidentityPageLimit)
	got, err := readAllZidentityPages(context.Background(), func(_ context.Context, offset, _ int) (zidentityPage[int], error) {
		return zidentityPage[int]{
			records:      fullPage,
			resultsTotal: 1500,
			pageOffset:   offset,
			pageSize:     zidentityPageLimit,
		}, nil
	})
	if err == nil {
		t.Fatal("readAllZidentityPages(missing continuation with remaining records) error = nil, want error")
	}
	if got != nil {
		t.Errorf("readAllZidentityPages(missing continuation with remaining records) result = %v, want nil", got)
	}
}

func TestReadAllZidentityPagesRejectsEmptyDeclaredRemaining(t *testing.T) {
	t.Parallel()

	calls := 0
	got, err := readAllZidentityPages(context.Background(), func(_ context.Context, offset, _ int) (zidentityPage[int], error) {
		calls++
		if calls == 1 {
			return zidentityPage[int]{
				records:      zidentityTestRecords(0, 100),
				resultsTotal: 200,
				pageOffset:   offset,
				pageSize:     100,
				nextLink:     "/admin/api/v1/users?offset=100&limit=100",
			}, nil
		}
		return zidentityPage[int]{
			resultsTotal: 200,
			pageOffset:   offset,
			pageSize:     100,
		}, nil
	})
	if err == nil {
		t.Fatal("readAllZidentityPages(empty declared remaining) error = nil, want error")
	}
	if got != nil {
		t.Errorf("readAllZidentityPages(empty declared remaining) result = %v, want nil", got)
	}
	if calls != 2 {
		t.Errorf("readAllZidentityPages(empty declared remaining) calls = %d, want 2", calls)
	}
}

func TestReadAllZidentityPagesRejectsInconsistentTotal(t *testing.T) {
	t.Parallel()

	got, err := readAllZidentityPages(context.Background(), func(_ context.Context, offset, _ int) (zidentityPage[int], error) {
		if offset == 0 {
			return zidentityPage[int]{
				records:      zidentityTestRecords(0, 100),
				resultsTotal: 250,
				pageOffset:   offset,
				pageSize:     100,
				nextLink:     "/admin/api/v1/users?offset=100&limit=100",
			}, nil
		}
		return zidentityPage[int]{
			records:      zidentityTestRecords(100, 100),
			resultsTotal: 300,
			pageOffset:   offset,
			pageSize:     100,
			nextLink:     "/admin/api/v1/users?offset=200&limit=100",
		}, nil
	})
	if err == nil {
		t.Fatal("readAllZidentityPages(inconsistent total) error = nil, want error")
	}
	if got != nil {
		t.Errorf("readAllZidentityPages(inconsistent total) result = %v, want nil", got)
	}
}

func TestReadAllZidentityPagesAcceptsNormalTerminalPages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		pages []zidentityPage[int]
		want  []int
	}{
		{
			name: "empty collection",
			pages: []zidentityPage[int]{
				{resultsTotal: 0, pageOffset: 0, pageSize: 100},
			},
		},
		{
			name: "single full page",
			pages: []zidentityPage[int]{
				{records: zidentityTestRecords(0, 1000), resultsTotal: 1000, pageOffset: 0, pageSize: 1000},
			},
			want: zidentityTestRecords(0, 1000),
		},
		{
			name: "partial last page",
			pages: []zidentityPage[int]{
				{
					records:      zidentityTestRecords(0, 100),
					resultsTotal: 150,
					pageOffset:   0,
					pageSize:     100,
					nextLink:     "/admin/api/v1/users?offset=100&limit=100",
				},
				{records: zidentityTestRecords(100, 50), resultsTotal: 150, pageOffset: 100, pageSize: 100},
			},
			want: zidentityTestRecords(0, 150),
		},
		{
			name: "full page without total",
			pages: []zidentityPage[int]{
				{records: zidentityTestRecords(0, 1000), pageOffset: 0, pageSize: 1000},
			},
			want: zidentityTestRecords(0, 1000),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			got, err := readAllZidentityPages(context.Background(), func(_ context.Context, offset, _ int) (zidentityPage[int], error) {
				if calls >= len(test.pages) {
					return zidentityPage[int]{}, errors.New("unexpected extra page")
				}
				page := test.pages[calls]
				calls++
				if page.pageOffset != offset {
					t.Errorf("readAllZidentityPages(%s) requested offset = %d, want page offset %d", test.name, offset, page.pageOffset)
				}
				return page, nil
			})
			if err != nil {
				t.Fatalf("readAllZidentityPages(%s) error = %v, want nil", test.name, err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("readAllZidentityPages(%s) = %v, want %v", test.name, got, test.want)
			}
			if calls != len(test.pages) {
				t.Errorf("readAllZidentityPages(%s) calls = %d, want %d", test.name, calls, len(test.pages))
			}
		})
	}
}

func TestReadAllZidentityPagesRejectsNoProgress(t *testing.T) {
	t.Parallel()

	fullPage := zidentityTestRecords(0, zidentityPageLimit)
	calls := 0
	got, err := readAllZidentityPages(context.Background(), func(_ context.Context, offset, _ int) (zidentityPage[int], error) {
		calls++
		return zidentityPage[int]{
			records:      fullPage,
			resultsTotal: 3000,
			pageOffset:   0,
			pageSize:     zidentityPageLimit,
			nextLink:     "/admin/api/v1/users?offset=1000&limit=1000",
		}, nil
	})
	if err == nil {
		t.Fatal("readAllZidentityPages(no progress) error = nil, want error")
	}
	if got != nil {
		t.Errorf("readAllZidentityPages(no progress) result = %v, want nil", got)
	}
	if calls != 2 {
		t.Errorf("readAllZidentityPages(no progress) calls = %d, want 2", calls)
	}
}

func TestReadAllZidentityPagesRejectsRepeatedContentWithAdvancingOffset(t *testing.T) {
	t.Parallel()

	page := zidentityTestRecords(0, zidentityPageLimit)
	calls := 0
	got, err := readAllZidentityPages(context.Background(), func(_ context.Context, offset, _ int) (zidentityPage[int], error) {
		calls++
		nextLink := ""
		if calls == 1 {
			nextLink = "/admin/api/v1/users?offset=1000&limit=1000"
		}
		return zidentityPage[int]{
			records:      page,
			resultsTotal: 2000,
			pageOffset:   offset,
			pageSize:     zidentityPageLimit,
			nextLink:     nextLink,
		}, nil
	})
	if err == nil {
		t.Fatal("readAllZidentityPages(repeated content with advancing offset) error = nil, want error")
	}
	if got != nil {
		t.Errorf("readAllZidentityPages(repeated content with advancing offset) result = %v, want nil", got)
	}
	if calls != 2 {
		t.Errorf("readAllZidentityPages(repeated content with advancing offset) calls = %d, want 2", calls)
	}
}

func TestReadAllZidentityPagesRejectsRepeatedContentWithoutDeclaredTotal(t *testing.T) {
	t.Parallel()

	page := zidentityTestRecords(0, zidentityPageLimit)
	calls := 0
	got, err := readAllZidentityPages(context.Background(), func(_ context.Context, offset, _ int) (zidentityPage[int], error) {
		calls++
		nextLink := ""
		if calls == 1 {
			nextLink = "/admin/api/v1/users?offset=1000&limit=1000"
		}
		return zidentityPage[int]{
			records:    page,
			pageOffset: offset,
			pageSize:   zidentityPageLimit,
			nextLink:   nextLink,
		}, nil
	})
	if err == nil {
		t.Fatal("readAllZidentityPages(repeated content without declared total) error = nil, want error")
	}
	if got != nil {
		t.Errorf("readAllZidentityPages(repeated content without declared total) result = %v, want nil", got)
	}
	if calls != 2 {
		t.Errorf("readAllZidentityPages(repeated content without declared total) calls = %d, want 2", calls)
	}
}

func TestReadAllZidentityPagesRejectsPageCycle(t *testing.T) {
	t.Parallel()

	pages := [][]int{{1, 2}, {3, 4}, {1, 2}}
	calls := 0
	got, err := readAllZidentityPages(context.Background(), func(_ context.Context, offset, _ int) (zidentityPage[int], error) {
		if calls >= len(pages) {
			t.Fatal("readAllZidentityPages(page cycle) requested an extra page")
		}
		records := pages[calls]
		calls++
		next := ""
		if calls < len(pages) {
			next = "continuation"
		}
		return zidentityPage[int]{
			records: records, pageOffset: offset, pageSize: 2,
			resultsTotal: 6, nextLink: next,
		}, nil
	})
	if err == nil {
		t.Fatal("readAllZidentityPages(page cycle) error = nil, want error")
	}
	if got != nil {
		t.Errorf("readAllZidentityPages(page cycle) returned %v, want no partial records", got)
	}
	if calls != 3 {
		t.Errorf("readAllZidentityPages(page cycle) calls = %d, want 3", calls)
	}
}

func TestReadAllZidentityPagesPropagatesPageFailure(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("page unavailable")
	got, err := readAllZidentityPages(context.Background(), func(_ context.Context, offset, _ int) (zidentityPage[int], error) {
		if offset > 0 {
			return zidentityPage[int]{}, wantErr
		}
		return zidentityPage[int]{
			records:      zidentityTestRecords(0, 100),
			resultsTotal: 200,
			pageOffset:   0,
			pageSize:     100,
			nextLink:     "/admin/api/v1/users?offset=100&limit=100",
		}, nil
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("readAllZidentityPages(page failure) error = %v, want %v", err, wantErr)
	}
	if got != nil {
		t.Errorf("readAllZidentityPages(page failure) result = %v, want nil", got)
	}
}

func TestReadAllZidentityPagesRejectsInvalidPageMetadata(t *testing.T) {
	t.Parallel()

	got, err := readAllZidentityPages(context.Background(), func(_ context.Context, offset, _ int) (zidentityPage[int], error) {
		return zidentityPage[int]{
			records:    []int{1, 2},
			pageOffset: offset,
			pageSize:   1,
		}, nil
	})
	if err == nil {
		t.Fatal("readAllZidentityPages(invalid page metadata) error = nil, want error")
	}
	if got != nil {
		t.Errorf("readAllZidentityPages(invalid page metadata) result = %v, want nil", got)
	}
}

func TestReadAllZidentityPagesRejectsUnmarshalableRecords(t *testing.T) {
	t.Parallel()

	type unmarshalableRecord struct {
		Callback func()
	}
	got, err := readAllZidentityPages(context.Background(), func(_ context.Context, offset, _ int) (zidentityPage[unmarshalableRecord], error) {
		return zidentityPage[unmarshalableRecord]{
			records:    []unmarshalableRecord{{Callback: func() {}}},
			pageOffset: offset,
		}, nil
	})
	if err == nil {
		t.Fatal("readAllZidentityPages(unmarshalable records) error = nil, want error")
	}
	if got != nil {
		t.Errorf("readAllZidentityPages(unmarshalable records) result = %v, want nil", got)
	}
}

func TestZidentityListAllUsesFixedEndpointForOpaqueContinuation(t *testing.T) {
	sdkCfg := newSDKConfiguration(context.Background(), validReaderConfig())
	var requests []*http.Request
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := `{"access_token":"test-token","expires_in":60}`
		if request.URL.Path == "/oauth2/v1/token" {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
				Request:    request,
			}, nil
		}
		if request.URL.Path != zidentityUsersEndpoint {
			return nil, fmt.Errorf("unexpected request path %q", request.URL.Path)
		}
		cloned := request.Clone(request.Context())
		clonedURL := *request.URL
		cloned.URL = &clonedURL
		requests = append(requests, cloned)

		switch request.URL.Query().Get("offset") {
		case "":
			body = `{"results_total":2,"pageOffset":0,"pageSize":1,"next_link":"https://attacker.example/admin/api/v1/users?offset=1&limit=1","records":[1]}`
		case "1":
			body = `{"results_total":2,"pageOffset":1,"pageSize":1,"next_link":"","records":[2]}`
		default:
			return nil, fmt.Errorf("unexpected zidentity offset %q", request.URL.Query().Get("offset"))
		}
		return &http.Response{
			StatusCode: http.StatusOK,
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

	got, err := zidentityListAll[int](context.Background(), service, zidentityUsersEndpoint)
	if err != nil {
		t.Fatalf("zidentityListAll(users) error = %v, want nil", err)
	}
	if want := []int{1, 2}; !reflect.DeepEqual(got, want) {
		t.Errorf("zidentityListAll(users) = %v, want %v", got, want)
	}
	if len(requests) != 2 {
		t.Fatalf("zidentityListAll(users) request count = %d, want 2", len(requests))
	}
	for index, request := range requests {
		if request.URL.Path != zidentityUsersEndpoint {
			t.Errorf("zidentityListAll(users) request %d path = %q, want %q", index+1, request.URL.Path, zidentityUsersEndpoint)
		}
		if got, want := request.URL.Query().Get("offset"), []string{"", "1"}[index]; got != want {
			t.Errorf("zidentityListAll(users) request %d offset = %q, want %q", index+1, got, want)
		}
		if got, want := request.URL.Query().Get("limit"), "1000"; got != want {
			t.Errorf("zidentityListAll(users) request %d limit = %q, want %q", index+1, got, want)
		}
	}
}

func zidentityTestRecords(start, count int) []int {
	records := make([]int, count)
	for i := range records {
		records[i] = start + i
	}
	return records
}
