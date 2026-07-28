package zscaler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	zsdk "github.com/zscaler/zscaler-sdk-go/v3/zscaler"
	zpaappsegmentba "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zpa/services/applicationsegmentbrowseraccess"
)

func TestSDKV3843GovernmentCloudRouting(t *testing.T) {
	tests := []struct {
		name           string
		cloud          string
		wantOAuthURL   string
		wantProductURL string
	}{
		{
			name:           "GOV",
			cloud:          "GOV",
			wantOAuthURL:   "https://zscalerctl-vanity.zidentitygov.net/oauth2/v1/token",
			wantProductURL: "https://api.zscalergov.net/zia/api/v1/status",
		},
		{
			name:           "GOVUS lowercase",
			cloud:          "govus",
			wantOAuthURL:   "https://zscalerctl-vanity.zidentitygov.us/oauth2/v1/token",
			wantProductURL: "https://api.zscalergov.us/zia/api/v1/status",
		},
		{
			name:           "GOVUS uppercase",
			cloud:          "GOVUS",
			wantOAuthURL:   "https://zscalerctl-vanity.zidentitygov.us/oauth2/v1/token",
			wantProductURL: "https://api.zscalergov.us/zia/api/v1/status",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := validReaderConfig()
			cfg.Cloud = test.cloud
			sdkCfg := newSDKConfiguration(context.Background(), cfg)

			var urls []string
			transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
				urls = append(urls, request.URL.String())
				body := `{}`
				if request.URL.Path == "/oauth2/v1/token" {
					body = `{"access_token":"test-token","expires_in":60}`
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
				t.Fatalf("NewOneAPIClient(%s) error = %v, want nil", test.cloud, err)
			}
			t.Cleanup(service.Client.Close)

			var status map[string]any
			if err := service.Client.Read(context.Background(), "/zia/api/v1/status", &status); err != nil {
				t.Fatalf("Client.Read(%s) error = %v, want nil", test.cloud, err)
			}
			if len(urls) != 2 {
				t.Fatalf("%s request URLs = %v, want OAuth and product requests", test.cloud, urls)
			}
			if urls[0] != test.wantOAuthURL || urls[1] != test.wantProductURL {
				t.Errorf("%s request URLs = %v, want [%s %s]", test.cloud, urls, test.wantOAuthURL, test.wantProductURL)
			}
		})
	}
}

func TestSDKV3843DeterministicServerErrorIsNotRetriedOrExposed(t *testing.T) {
	const responseCanary = "client_secret=raw-sdk-response"

	cfg := validReaderConfig()
	sdkCfg := newSDKConfiguration(context.Background(), cfg)
	productRequests := 0
	transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		statusCode := http.StatusOK
		body := `{"access_token":"test-token","expires_in":60}`
		if request.URL.Path != "/oauth2/v1/token" {
			productRequests++
			statusCode = http.StatusInternalServerError
			body = `{"code":"UNEXPECTED_ERROR","message":"` + responseCanary + `"}`
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

	var status map[string]any
	rawErr := service.Client.Read(context.Background(), "/zia/api/v1/status", &status)
	if rawErr == nil {
		t.Fatal("Client.Read(deterministic 500) error = nil, want SDK API error")
	}
	if productRequests != 1 {
		t.Fatalf("Client.Read(deterministic 500) product requests = %d, want 1", productRequests)
	}

	normalized := normalizeLiveError(
		context.Background(),
		"list",
		resources.ProductZIA,
		"locations",
		rawErr,
	)
	if !errors.Is(normalized, ErrLiveAccessFailed) {
		t.Errorf("normalizeLiveError(deterministic 500) error = %v, want ErrLiveAccessFailed", normalized)
	}
	if !strings.Contains(normalized.Error(), "status 500") {
		t.Errorf("normalizeLiveError(deterministic 500) error = %q, want safe status code", normalized.Error())
	}
	if strings.Contains(normalized.Error(), responseCanary) {
		t.Errorf("normalizeLiveError(deterministic 500) error = %q, want no SDK response content", normalized.Error())
	}
}

func TestSDKV3843BrowserAccessFalseBypassOnReauthSurvivesProjectionInput(t *testing.T) {
	record := jsonSourceRecord(zpaappsegmentba.BrowserAccess{
		ID:             "browser-app-1",
		BypassOnReauth: false,
	})
	spec, ok := resources.FindSpec(resources.ProductZPA, resourceZPABrowserAccess)
	if !ok {
		t.Fatalf("FindSpec(zpa, %s) ok = false, want true", resourceZPABrowserAccess)
	}
	projected, _, err := resources.ProjectRecords(spec, redact.ModeStandard, []resources.SourceRecord{record})
	if err != nil {
		t.Fatalf("ProjectRecords(browser access) error = %v, want nil", err)
	}
	records := projected.Records()
	if len(records) != 1 {
		t.Fatalf("ProjectRecords(browser access) records length = %d, want 1", len(records))
	}
	fields := records[0].Fields()
	value, ok := fields["bypassOnReauth"]
	if !ok {
		t.Fatal("jsonSourceRecord(browser access) omitted false bypassOnReauth")
	}
	if value != false {
		t.Errorf("jsonSourceRecord(browser access) bypassOnReauth = %#v, want false", value)
	}
}
