package runtime

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/dvmrry/zscalerctl/internal/browser"
	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/zscaler"
)

func TestNewURLLookupFromConfigAssemblesReaderConfigAndLooksUp(t *testing.T) {
	t.Parallel()

	cfg, err := config.LoadConfig([]string{
		config.EnvClientID + "=client-id",
		config.EnvClientSecret + "=client-secret",
		config.EnvVanityDomain + "=example",
		config.EnvCloud + "=PRODUCTION",
	}, config.LoadOptions{})
	if err != nil {
		t.Fatalf("LoadConfig(runtime URL lookup fixture) error = %v, want nil", err)
	}

	reader := &runtimeURLLookupReader{
		results: []zscaler.URLClassification{{
			URL:                          "example.com",
			Classifications:              []string{"NEWS_AND_MEDIA"},
			SecurityAlertClassifications: []string{"MALWARE_SITE"},
			Application:                  "EXAMPLE_APP",
		}},
	}
	var gotReaderConfig zscaler.ReaderConfig
	lookup, err := NewURLLookupFromConfig(context.Background(), cfg, Options{
		Timeout: 5 * time.Second,
		newReader: func(cfg zscaler.ReaderConfig) (browser.RecordReader, error) {
			gotReaderConfig = cfg
			return reader, nil
		},
	})
	if err != nil {
		t.Fatalf("NewURLLookupFromConfig(effective config) error = %v, want nil", err)
	}
	if got := gotReaderConfig.ClientID.Reveal(); got != "client-id" {
		t.Errorf("NewURLLookupFromConfig(effective config) ClientID = %q, want client-id", got)
	}
	if got := gotReaderConfig.ClientSecret.Reveal(); got != "client-secret" {
		t.Errorf("NewURLLookupFromConfig(effective config) ClientSecret = %q, want client-secret", got)
	}
	if gotReaderConfig.Timeout != 5*time.Second {
		t.Errorf("NewURLLookupFromConfig(effective config) Timeout = %s, want 5s", gotReaderConfig.Timeout)
	}

	got, err := lookup.Lookup(context.Background(), []string{"example.com"})
	if err != nil {
		t.Fatalf("URLLookup.Lookup(example.com) error = %v, want nil", err)
	}
	want := []URLClassification{{
		URL:                          "example.com",
		Classifications:              []string{"NEWS_AND_MEDIA"},
		SecurityAlertClassifications: []string{"MALWARE_SITE"},
		Application:                  "EXAMPLE_APP",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("URLLookup.Lookup(example.com) = %#v, want %#v", got, want)
	}
	wantCalls := [][]string{{"example.com"}}
	if !reflect.DeepEqual(reader.calls, wantCalls) {
		t.Fatalf("URLLookup.Lookup(example.com) calls = %#v, want %#v", reader.calls, wantCalls)
	}
}

func TestNewURLLookupFromReaderReportsUnsupportedCapability(t *testing.T) {
	t.Parallel()

	_, err := NewURLLookupFromReader(&runtimeFakeReader{})
	if !errors.Is(err, zscaler.ErrUnsupportedResource) {
		t.Fatalf("NewURLLookupFromReader(record-only reader) error = %v, want ErrUnsupportedResource", err)
	}
	if !strings.Contains(err.Error(), "zia/url-lookup") {
		t.Fatalf("NewURLLookupFromReader(record-only reader) error = %q, want zia/url-lookup context", err.Error())
	}
}

type runtimeURLLookupReader struct {
	runtimeFakeReader
	calls   [][]string
	results []zscaler.URLClassification
	err     error
}

func (r *runtimeURLLookupReader) URLLookup(_ context.Context, urls []string) ([]zscaler.URLClassification, error) {
	r.calls = append(r.calls, append([]string(nil), urls...))
	if r.err != nil {
		return nil, r.err
	}
	return r.results, nil
}
