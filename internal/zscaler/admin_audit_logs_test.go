package zscaler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	sdkcache "github.com/zscaler/zscaler-sdk-go/v3/cache"
	zsdk "github.com/zscaler/zscaler-sdk-go/v3/zscaler"

	"github.com/dvmrry/zscalerctl/internal/resources"
)

type adminAuditLogsTestServiceProvider struct {
	svc *zsdk.Service
}

func (p adminAuditLogsTestServiceProvider) service(
	context.Context,
	resources.Product,
) (*zsdk.Service, func(), error) {
	return p.svc, func() {}, nil
}

func adminAuditLogsStatusStringPointer(value string) *string {
	return &value
}

func adminAuditLogsStatusIntPointer(value int) *int {
	return &value
}

func newLegacyAdminAuditLogsTestService(t *testing.T, transport roundTripFunc) *zsdk.Service {
	t.Helper()
	legacyConfig, err := newLegacyZIAConfiguration(context.Background(), validLegacyReaderConfig())
	if err != nil {
		t.Fatalf("newLegacyZIAConfiguration() error = %v, want nil", err)
	}
	legacyConfig.HTTPClient.Transport = transport
	legacyClient, err := newLegacyZIAClient(legacyConfig)
	if err != nil {
		t.Fatalf("newLegacyZIAClient() error = %v, want nil", err)
	}
	service, err := zsdk.NewOneAPIClient(&zsdk.Configuration{
		Logger:          newSDKLogger(nil),
		DefaultHeader:   make(map[string]string),
		UserAgent:       "zscalerctl zscaler-sdk-go/v3",
		Context:         effectiveContext(context.Background()),
		CacheManager:    sdkcache.NewNopCache(),
		UseLegacyClient: true,
		LegacyClient: &zsdk.LegacyClient{
			ZiaClient: legacyClient,
		},
	})
	if err != nil {
		t.Fatalf("NewOneAPIClient() for legacy service error = %v, want nil", err)
	}
	return service
}

func TestReadAdminAuditLogsStatusUsesOneGET(t *testing.T) {
	t.Parallel()

	const errorMessageCanary = "audit-log-error-message-must-not-render"
	var reportRequests []*http.Request
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := []byte(`{"access_token":"test-token","expires_in":60}`)
		statusCode := http.StatusOK
		switch request.URL.Path {
		case "/oauth2/v1/token":
		case adminAuditLogsStatusEndpoint:
			cloned := request.Clone(request.Context())
			clonedURL := *request.URL
			cloned.URL = &clonedURL
			reportRequests = append(reportRequests, cloned)
			body = []byte(`{"status":"COMPLETED","progressItemsComplete":12,"progressEndTime":1700000000000,"errorMessage":"` + errorMessageCanary + `","errorCode":"tenant-specific-code"}`)
		default:
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

	service := newZIAURLFilteringRuleTestService(t, transport)
	status, err := readAdminAuditLogsStatus(context.Background(), sdkClient{
		services: adminAuditLogsTestServiceProvider{svc: service},
	})
	if err != nil {
		t.Fatalf("readAdminAuditLogsStatus() error = %v, want nil", err)
	}
	if len(reportRequests) != 1 {
		t.Fatalf("status endpoint requests = %d, want exactly one", len(reportRequests))
	}
	if reportRequests[0].Method != http.MethodGet {
		t.Errorf("status request method = %q, want GET", reportRequests[0].Method)
	}
	if reportRequests[0].URL.Path != adminAuditLogsStatusEndpoint {
		t.Errorf("status request path = %q, want %q", reportRequests[0].URL.Path, adminAuditLogsStatusEndpoint)
	}
	if status.Status != "completed" || status.ProgressItemsComplete == nil || *status.ProgressItemsComplete != 12 {
		t.Errorf("status = %+v, want normalized completed status and count 12", status)
	}
	if status.CompletionTime != "2023-11-14T22:13:20Z" {
		t.Errorf("completion time = %q, want normalized UTC timestamp", status.CompletionTime)
	}
	if status.ErrorCode != adminAuditLogsErrorCodeUnknown {
		t.Errorf("error code = %q, want fixed allow-listed value", status.ErrorCode)
	}
	rendered, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("json.Marshal(status) error = %v", err)
	}
	if strings.Contains(string(rendered), errorMessageCanary) {
		t.Errorf("safe status JSON contains API errorMessage: %s", rendered)
	}
}

func TestAdminAuditLogsStatusPreservesProgressFieldPresence(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name         string
		body         string
		wantProgress *int
	}{
		{name: "omitted", body: `{"status":"COMPLETED"}`},
		{name: "null", body: `{"status":"COMPLETED","progressItemsComplete":null}`},
		{name: "explicit zero", body: `{"status":"COMPLETED","progressItemsComplete":0}`, wantProgress: adminAuditLogsStatusIntPointer(0)},
	} {
		t.Run(test.name, func(t *testing.T) {
			transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
				body := []byte(`{"access_token":"test-token","expires_in":60}`)
				if request.URL.Path != "/oauth2/v1/token" {
					body = []byte(test.body)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(string(body))),
					Request:    request,
				}, nil
			})
			service := newZIAURLFilteringRuleTestService(t, transport)
			status, err := readAdminAuditLogsStatus(context.Background(), sdkClient{
				services: adminAuditLogsTestServiceProvider{svc: service},
			})
			if err != nil {
				t.Fatalf("readAdminAuditLogsStatus() error = %v, want nil", err)
			}
			if test.wantProgress == nil {
				if status.ProgressItemsComplete != nil {
					t.Errorf("progress count = %d, want unavailable", *status.ProgressItemsComplete)
				}
			} else if status.ProgressItemsComplete == nil || *status.ProgressItemsComplete != *test.wantProgress {
				t.Errorf("progress count = %v, want %d", status.ProgressItemsComplete, *test.wantProgress)
			}
		})
	}
}

func TestAdminAuditLogsStatusRejectsMissingWireStatus(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		body string
	}{
		{name: "top-level null", body: `null`},
		{name: "empty body", body: ``},
		{name: "missing", body: `{"progressItemsComplete":0}`},
		{name: "null", body: `{"status":null}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
				body := []byte(`{"access_token":"test-token","expires_in":60}`)
				if request.URL.Path != "/oauth2/v1/token" {
					body = []byte(test.body)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(string(body))),
					Request:    request,
				}, nil
			})
			service := newZIAURLFilteringRuleTestService(t, transport)
			_, err := readAdminAuditLogsStatus(context.Background(), sdkClient{
				services: adminAuditLogsTestServiceProvider{svc: service},
			})
			if !errors.Is(err, ErrLiveAccessFailed) {
				t.Fatalf("readAdminAuditLogsStatus() error = %v, want ErrLiveAccessFailed", err)
			}
		})
	}
}

func TestAdminAuditLogsStatusRejectsEmptyLegacyResponse(t *testing.T) {
	var statusRequests int
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := []byte(`{}`)
		header := make(http.Header)
		switch request.URL.Path {
		case "/api/v1/authenticatedSession":
			header.Add("Set-Cookie", "JSESSIONID=test-session; Path=/")
		case strings.TrimPrefix(adminAuditLogsStatusEndpoint, "/zia"):
			// The legacy ZIA client strips the OneAPI "/zia" prefix.
			statusRequests++
			body = nil
		default:
			t.Errorf("unexpected legacy request path %q", request.URL.Path)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     header,
			Body:       io.NopCloser(strings.NewReader(string(body))),
			Request:    request,
		}, nil
	})
	service := newLegacyAdminAuditLogsTestService(t, transport)
	_, err := readAdminAuditLogsStatus(context.Background(), sdkClient{
		services: adminAuditLogsTestServiceProvider{svc: service},
	})
	if !errors.Is(err, ErrLiveAccessFailed) {
		t.Fatalf("readAdminAuditLogsStatus(empty legacy response) error = %v, want ErrLiveAccessFailed", err)
	}
	if statusRequests != 1 {
		t.Errorf("status endpoint requests = %d, want 1", statusRequests)
	}
}

func TestAdminAuditLogsStatusErrorsAreValueFree(t *testing.T) {
	t.Parallel()

	const errorMessageCanary = "api-error-message-must-not-render"
	for _, test := range []struct {
		name       string
		statusCode int
		want       error
	}{
		{name: "not found", statusCode: http.StatusNotFound, want: ErrResourceNotFound},
		{name: "live failure", statusCode: http.StatusInternalServerError, want: ErrLiveAccessFailed},
	} {
		t.Run(test.name, func(t *testing.T) {
			transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
				body := []byte(`{"access_token":"test-token","expires_in":60}`)
				statusCode := http.StatusOK
				if request.URL.Path != "/oauth2/v1/token" {
					statusCode = test.statusCode
					body = []byte(`{"errorMessage":"` + errorMessageCanary + `"}`)
				}
				return &http.Response{
					StatusCode: statusCode,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(string(body))),
					Request:    request,
				}, nil
			})
			service := newZIAURLFilteringRuleTestService(t, transport)
			_, err := readAdminAuditLogsStatus(context.Background(), sdkClient{
				services: adminAuditLogsTestServiceProvider{svc: service},
			})
			if !errors.Is(err, test.want) {
				t.Fatalf("readAdminAuditLogsStatus() error = %v, want errors.Is(err, %v)", err, test.want)
			}
			if strings.Contains(err.Error(), errorMessageCanary) {
				t.Errorf("status error = %q, want no API error message", err)
			}
			var contextual ErrorContexter
			if !errors.As(err, &contextual) {
				t.Fatalf("status error %T does not carry safe error context", err)
			}
			if got := contextual.ErrorContext(); got.Product != string(resources.ProductZIA) ||
				got.Resource != adminAuditLogsResourceName || got.Operation != "status" {
				t.Errorf("status error context = %+v, want zia/admin-audit-logs status", got)
			}
		})
	}
}

// cancelOnReadBody cancels a context when the SDK reads the response body.
type cancelOnReadBody struct {
	cancel context.CancelFunc
}

func (b cancelOnReadBody) Read([]byte) (int, error) {
	b.cancel()
	return 0, context.Canceled
}

func (cancelOnReadBody) Close() error { return nil }

func TestAdminAuditLogsStatusCancellationDuringNotFoundBody(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/oauth2/v1/token" {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"access_token":"test-token","expires_in":60}`)),
				Request:    request,
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusNotFound,
			Header:     make(http.Header),
			Body:       cancelOnReadBody{cancel: cancel},
			Request:    request,
		}, nil
	})
	service := newZIAURLFilteringRuleTestService(t, transport)
	_, err := readAdminAuditLogsStatus(ctx, sdkClient{
		services: adminAuditLogsTestServiceProvider{svc: service},
	})
	if errors.Is(err, ErrResourceNotFound) {
		t.Fatalf("readAdminAuditLogsStatus(canceled during 404 body) error = %v, want cancellation, not not-found", err)
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("readAdminAuditLogsStatus(canceled during 404 body) error = %v, want context.Canceled", err)
	}
}

func TestAdminAuditLogsStatusNormalizesUnknownStatus(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		raw  string
		want string
	}{
		{name: "in progress", raw: "IN_PROGRESS", want: "in_progress"},
		{name: "completed", raw: "COMPLETED", want: "completed"},
		{name: "failed", raw: "FAILED", want: "failed"},
		{name: "unknown", raw: "TENANT_STATUS", want: adminAuditLogsStatusUnknown},
	} {
		t.Run(test.name, func(t *testing.T) {
			status := adminAuditLogsStatusFromResponse(adminAuditLogsStatusResponse{
				Status:                adminAuditLogsStatusStringPointer(test.raw),
				ProgressItemsComplete: adminAuditLogsStatusIntPointer(-1),
				ProgressEndTime:       0,
				ErrorCode:             "tenant-specific-code",
			})
			if status.Status != test.want {
				t.Errorf("normalized status = %q, want %q", status.Status, test.want)
			}
			if status.ProgressItemsComplete != nil {
				t.Errorf("progress count = %v, want negative count omitted", *status.ProgressItemsComplete)
			}
			if status.CompletionTime != "" {
				t.Errorf("completion time = %q, want omitted", status.CompletionTime)
			}
			if status.ErrorCode != adminAuditLogsErrorCodeUnknown {
				t.Errorf("error code = %q, want fixed allow-listed value", status.ErrorCode)
			}
		})
	}
}
