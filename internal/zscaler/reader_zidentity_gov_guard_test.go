package zscaler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	zsdk "github.com/zscaler/zscaler-sdk-go/v3/zscaler"

	"github.com/dvmrry/zscalerctl/internal/resources"
)

func TestUnsupportedOneAPIZidentityCloud(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     ReaderConfig
		product resources.Product
		wantErr bool
	}{
		{
			name: "government",
			cfg: func() ReaderConfig {
				cfg := validReaderConfig()
				cfg.Cloud = "gov"
				return cfg
			}(),
			product: resources.ProductZidentity,
			wantErr: true,
		},
		{
			name: "government case insensitive and trimmed",
			cfg: func() ReaderConfig {
				cfg := validReaderConfig()
				cfg.Cloud = " GoVuS "
				return cfg
			}(),
			product: resources.ProductZidentity,
			wantErr: true,
		},
		{
			name: "commercial",
			cfg: func() ReaderConfig {
				cfg := validReaderConfig()
				cfg.Cloud = "production"
				return cfg
			}(),
			product: resources.ProductZidentity,
		},
		{
			name: "ZPATWO",
			cfg: func() ReaderConfig {
				cfg := validReaderConfig()
				cfg.Cloud = "ZPATWO"
				return cfg
			}(),
			product: resources.ProductZidentity,
		},
		{
			name: "government ZIA remains supported",
			cfg: func() ReaderConfig {
				cfg := validReaderConfig()
				cfg.Cloud = "gov"
				return cfg
			}(),
			product: resources.ProductZIA,
		},
		{
			name: "government ZPA remains supported",
			cfg: func() ReaderConfig {
				cfg := validReaderConfig()
				cfg.Cloud = "govus"
				return cfg
			}(),
			product: resources.ProductZPA,
		},
		{
			name: "legacy ZIA auth keeps its existing product gate",
			cfg: func() ReaderConfig {
				cfg := validLegacyReaderConfig()
				cfg.ZIALegacy.Cloud = "zscalergov"
				return cfg
			}(),
			product: resources.ProductZidentity,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := unsupportedOneAPIZidentityCloud(test.cfg, test.product)
			if test.wantErr {
				if !errors.Is(err, ErrUnsupportedResource) {
					t.Fatalf("unsupportedOneAPIZidentityCloud(%q, %s) error = %v, want ErrUnsupportedResource", test.cfg.Cloud, test.product, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unsupportedOneAPIZidentityCloud(%q, %s) error = %v, want nil", test.cfg.Cloud, test.product, err)
			}
		})
	}
}

func TestReaderGovZidentityOperationsRejectBeforeAuthentication(t *testing.T) {
	t.Parallel()

	cfg := validReaderConfig()
	cfg.Cloud = "GOVUS"
	var requests atomic.Int64
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(proxy.Close)
	cfg.Proxy.URL = proxy.URL
	reader, err := NewReader(cfg)
	if err != nil {
		t.Fatalf("NewReader(government OneAPI) error = %v, want nil", err)
	}

	tests := []struct {
		name string
		call func(context.Context) error
	}{
		{
			name: "list",
			call: func(ctx context.Context) error {
				_, err := reader.List(ctx, resources.ProductZidentity, resourceZidentityGroups)
				return err
			},
		},
		{
			name: "get",
			call: func(ctx context.Context) error {
				_, err := reader.Get(ctx, resources.ProductZidentity, resourceZidentityGroups, "1")
				return err
			},
		},
		{
			name: "show",
			call: func(ctx context.Context) error {
				_, err := reader.Show(ctx, resources.ProductZidentity, resourceZidentityGroups)
				return err
			},
		},
		{
			name: "session",
			call: func(ctx context.Context) error {
				_, err := reader.Session(ctx, resources.ProductZidentity)
				return err
			},
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			err := test.call(ctx)
			if !errors.Is(err, ErrUnsupportedResource) {
				t.Fatalf("government Zidentity %s error = %v, want ErrUnsupportedResource", test.name, err)
			}
			if errors.Is(err, ErrLiveAccessFailed) {
				t.Fatalf("government Zidentity %s error = %v, want unsupported classification", test.name, err)
			}
		})
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("government Zidentity operations sent %d auth/API requests, want 0", got)
	}
}

func TestFixedServiceGovZidentityRejectsCrossProductSession(t *testing.T) {
	t.Parallel()

	cfg := validReaderConfig()
	cfg.Cloud = "gov"
	service := fixedService{cfg: cfg, sdkService: &zsdk.Service{}}

	got, cleanup, err := service.service(context.Background(), resources.ProductZidentity)
	if got != nil {
		t.Fatalf("fixedService.service(government, zidentity) service = %v, want nil", got)
	}
	if cleanup != nil {
		t.Fatal("fixedService.service(government, zidentity) cleanup is non-nil, want nil")
	}
	if !errors.Is(err, ErrUnsupportedResource) {
		t.Fatalf("fixedService.service(government, zidentity) error = %v, want ErrUnsupportedResource", err)
	}

	// A dump session authenticates once for its initial product and then shares
	// this fixed provider across product handlers. Exercise the later
	// Zidentity operation directly to ensure the provider still enforces the
	// government restriction before dereferencing the SDK client.
	session := &SDKSession{handlers: newResourceHandlers(sdkClient{services: service})}
	_, err = session.List(context.Background(), resources.ProductZidentity, resourceZidentityGroups)
	if !errors.Is(err, ErrUnsupportedResource) {
		t.Fatalf("fixed government session Zidentity list error = %v, want ErrUnsupportedResource", err)
	}
	_, err = session.Get(context.Background(), resources.ProductZidentity, resourceZidentityGroups, "1")
	if !errors.Is(err, ErrUnsupportedResource) {
		t.Fatalf("fixed government session Zidentity get error = %v, want ErrUnsupportedResource", err)
	}
}
