package zscaler

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	zsdk "github.com/zscaler/zscaler-sdk-go/v3/zscaler"
)

func TestSDKV3841GovernmentCloudRouting(t *testing.T) {
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
			name:           "GOVUS case insensitive",
			cloud:          "govus",
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
