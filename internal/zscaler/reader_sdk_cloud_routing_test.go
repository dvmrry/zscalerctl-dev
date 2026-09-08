package zscaler

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	zsdk "github.com/zscaler/zscaler-sdk-go/v3/zscaler"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/location/locationmanagement"
)

// Exercise the configured SDK transports, rather than only its URL helpers,
// so a future client or adapter change cannot silently undo cloud routing.
func TestSDKCloudRoutesOAuthAndZIA(t *testing.T) {
	tests := []struct {
		cloud    string
		authHost string
		apiHost  string
	}{
		{cloud: "PRODUCTION", authHost: "zscalerctl-vanity.zslogin.net", apiHost: "api.zsapi.net"},
		{cloud: "gov", authHost: "zscalerctl-vanity.zidentitygov.net", apiHost: "api.zscalergov.net"},
		{cloud: "GOVUS", authHost: "zscalerctl-vanity.zidentitygov.us", apiHost: "api.zscalergov.us"},
	}
	for _, test := range tests {
		t.Run(test.cloud, func(t *testing.T) {
			cfg := validReaderConfig()
			cfg.Cloud = test.cloud
			sdkCfg := newSDKConfiguration(context.Background(), cfg)
			var gotURLs []string
			transport := roundTripFunc(func(request *http.Request) (*http.Response, error) {
				gotURLs = append(gotURLs, request.Method+" "+request.URL.String())
				body := `{"id":1,"name":"fixture"}`
				if request.URL.Path == "/oauth2/v1/token" {
					// Avoid starting the SDK's background renewal goroutine.
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
				t.Fatalf("NewOneAPIClient(%q) error = %v, want nil", test.cloud, err)
			}
			t.Cleanup(service.Client.Close)
			if _, err := locationmanagement.GetLocation(context.Background(), service, 1); err != nil {
				t.Fatalf("GetLocation(%q, 1) error = %v, want nil", test.cloud, err)
			}
			wantURLs := []string{
				"POST https://" + test.authHost + "/oauth2/v1/token",
				"GET https://" + test.apiHost + "/zia/api/v1/locations/1",
			}
			if !reflect.DeepEqual(gotURLs, wantURLs) {
				t.Errorf("SDK requests for cloud %q = %v, want %v", test.cloud, gotURLs, wantURLs)
			}
		})
	}
}
