package zscaler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	sdkerrorx "github.com/zscaler/zscaler-sdk-go/v3/zscaler/errorx"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zia/services/urlcategories"
)

func TestURLClassificationsFromSDKCopiesAllFields(t *testing.T) {
	t.Parallel()

	got := urlClassificationsFromSDK([]urlcategories.URLClassification{
		{
			URL:                                 "https://example.com",
			URLClassifications:                  []string{"PROFESSIONAL_SERVICES", "TECHNOLOGY"},
			URLClassificationsWithSecurityAlert: []string{"MALWARE_SITE"},
			Application:                         "EXAMPLE_APP",
		},
		{URL: "b.example.com"},
	})

	if len(got) != 2 {
		t.Fatalf("urlClassificationsFromSDK(2 results) len = %d, want 2", len(got))
	}
	first := got[0]
	if first.URL != "https://example.com" || first.Application != "EXAMPLE_APP" {
		t.Errorf("urlClassificationsFromSDK()[0] = %+v, want url and application copied", first)
	}
	if strings.Join(first.Classifications, ",") != "PROFESSIONAL_SERVICES,TECHNOLOGY" {
		t.Errorf("urlClassificationsFromSDK()[0].Classifications = %v, want both categories in order", first.Classifications)
	}
	if strings.Join(first.SecurityAlertClassifications, ",") != "MALWARE_SITE" {
		t.Errorf("urlClassificationsFromSDK()[0].SecurityAlertClassifications = %v, want MALWARE_SITE", first.SecurityAlertClassifications)
	}
	second := got[1]
	if second.URL != "b.example.com" || len(second.Classifications) != 0 || len(second.SecurityAlertClassifications) != 0 || second.Application != "" {
		t.Errorf("urlClassificationsFromSDK()[1] = %+v, want empty classification fields", second)
	}
}

func TestLookupURLClassificationsNormalizesErrorsWithoutLeaking(t *testing.T) {
	t.Parallel()

	// The SDK error carries response-body content (the canary); the normalized
	// error must keep only the value-free operation context and status code.
	const canary = "sdk-body-client_secret=canary-value"
	sdkErr := &sdkerrorx.ErrorResponse{
		Response: &http.Response{StatusCode: http.StatusForbidden, Status: "403 Forbidden"},
		Message:  canary,
	}

	_, err := lookupURLClassifications(context.Background(), func(context.Context) ([]urlcategories.URLClassification, error) {
		return []urlcategories.URLClassification{{URL: canary, Application: canary}}, sdkErr
	})
	if !errors.Is(err, ErrLiveAccessFailed) {
		t.Fatalf("lookupURLClassifications(sdk error) error = %v, want ErrLiveAccessFailed", err)
	}
	if strings.Contains(err.Error(), canary) {
		t.Errorf("lookupURLClassifications(sdk error) error = %q, want no leaked SDK content", err.Error())
	}
	if !strings.Contains(err.Error(), "lookup zia/url-lookup") {
		t.Errorf("lookupURLClassifications(sdk error) error = %q, want lookup zia/url-lookup context", err.Error())
	}
	if !strings.Contains(err.Error(), "status 403") {
		t.Errorf("lookupURLClassifications(sdk error) error = %q, want safe status code", err.Error())
	}
}

func TestLookupURLClassificationsPassesThroughMissingCredentials(t *testing.T) {
	t.Parallel()

	wrapped := fmt.Errorf("zia service: %w", ErrMissingCredentials)
	_, err := lookupURLClassifications(context.Background(), func(context.Context) ([]urlcategories.URLClassification, error) {
		return nil, wrapped
	})
	if !errors.Is(err, ErrMissingCredentials) {
		t.Errorf("lookupURLClassifications(missing credentials) error = %v, want ErrMissingCredentials", err)
	}
	if errors.Is(err, ErrLiveAccessFailed) {
		t.Errorf("lookupURLClassifications(missing credentials) error = %v, want NOT ErrLiveAccessFailed", err)
	}
}

func TestNilSDKReaderURLLookupIsUnsupported(t *testing.T) {
	t.Parallel()

	var reader *SDKReader
	_, err := reader.URLLookup(context.Background(), []string{"example.com"})
	if !errors.Is(err, ErrUnsupportedResource) {
		t.Errorf("(*SDKReader)(nil).URLLookup() error = %v, want ErrUnsupportedResource", err)
	}
}
