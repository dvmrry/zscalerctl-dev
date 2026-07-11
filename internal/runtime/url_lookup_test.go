package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/dvmrry/zscalerctl/internal/browser"
	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
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

	result, err := lookup.Lookup(context.Background(), machine.URLLookupRequest{
		URLs: []string{"example.com"},
	})
	if err != nil {
		t.Fatalf("URLLookup.Lookup(example.com) error = %v, want nil", err)
	}
	want := []machine.URLClassification{{
		URL:                          "example.com",
		Classifications:              []string{"NEWS_AND_MEDIA"},
		SecurityAlertClassifications: []string{"MALWARE_SITE"},
		Application:                  "EXAMPLE_APP",
	}}
	if got := result.Classifications(); !reflect.DeepEqual(got, want) {
		t.Fatalf("URLLookup.Lookup(example.com) = %#v, want %#v", got, want)
	}
	wantCalls := [][]string{{"example.com"}}
	if !reflect.DeepEqual(reader.calls, wantCalls) {
		t.Fatalf("URLLookup.Lookup(example.com) calls = %#v, want %#v", reader.calls, wantCalls)
	}
}

func TestURLLookupNormalizesInputAndSanitizesResponse(t *testing.T) {
	t.Parallel()

	const (
		queryCanary    = "response-query-canary"
		userinfoCanary = "response-userinfo-canary"
		jwtCanary      = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJjYW5hcnkifQ.c2VjcmV0LXNpZ25hdHVyZS1jYW5hcnk"
	)
	reader := &runtimeURLLookupReader{results: []zscaler.URLClassification{{
		URL: "https://user:" + userinfoCanary + "@echo.example/path?token=" + queryCanary + "#fragment",
		Classifications: []string{
			jwtCanary,
			"safe\nFORGED\x1b[31m\u0085\u202e",
		},
		SecurityAlertClassifications: nil,
		Application:                  "Authorization: Bearer " + jwtCanary,
	}}}
	lookup, err := NewURLLookupFromReader(reader, redact.ModeStandard)
	if err != nil {
		t.Fatalf("NewURLLookupFromReader() error = %v, want nil", err)
	}
	result, err := lookup.Lookup(context.Background(), machine.URLLookupRequest{
		URLs: []string{"https://input:secret@input.example/path?token=input-canary#fragment"},
	})
	if err != nil {
		t.Fatalf("URLLookup.Lookup(sensitive URLs) error = %v, want nil", err)
	}
	if want := [][]string{{"https://input.example/path"}}; !reflect.DeepEqual(reader.calls, want) {
		t.Fatalf("URLLookup reader calls = %#v, want %#v", reader.calls, want)
	}
	got := result.Classifications()
	if len(got) != 1 || got[0].URL != "https://echo.example/path" {
		t.Fatalf("URLLookup sanitized result = %#v, want sanitized echoed URL", got)
	}
	if got[0].SecurityAlertClassifications == nil {
		t.Fatalf("URLLookup empty security classifications = nil, want non-nil empty")
	}
	body, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("json.Marshal(URL classifications) error = %v", err)
	}
	for _, forbidden := range []string{queryCanary, userinfoCanary, jwtCanary, "input-canary"} {
		if strings.Contains(string(body), forbidden) {
			t.Fatalf("URLLookup sanitized result = %s, want no %q", body, forbidden)
		}
	}
	if got[0].Classifications[0] != "<REDACTED:JWT>" {
		t.Fatalf("URLLookup sanitized classifications = %#v, want JWT marker", got[0].Classifications)
	}
	assertNoUnsafeURLLookupRunes(t, got)
}

func TestURLLookupRejectsInvalidRequestsWithoutCallingReader(t *testing.T) {
	t.Parallel()

	tests := []machine.URLLookupRequest{
		{},
		{URLs: []string{""}},
		{URLs: []string{"\nhttps://example.com"}},
		{URLs: []string{"https://example.com\t"}},
		{URLs: []string{"\u0085https://example.com"}},
		{URLs: []string{"https://example.com\u0085"}},
		{URLs: []string{"/path"}},
		{URLs: []string{"//example.com/path"}},
		{URLs: []string{"http:"}},
		{URLs: []string{"http:/path"}},
		{URLs: []string{"https://example.com/%"}},
		{URLs: []string{"mailto:user@example.com?token=secret"}},
		{URLs: []string{"https://example.com/\nFORGED"}},
		{URLs: []string{"https://example.com/\u202eFORGED"}},
	}
	for _, req := range tests {
		reader := &runtimeURLLookupReader{}
		lookup, err := NewURLLookupFromReader(reader, redact.ModeStandard)
		if err != nil {
			t.Fatalf("NewURLLookupFromReader() error = %v, want nil", err)
		}
		_, err = lookup.Lookup(context.Background(), req)
		var machineErr *machine.MachineError
		if !errors.As(err, &machineErr) || machineErr.Kind != machine.ErrorKindUsage {
			t.Fatalf("URLLookup.Lookup(%#v) error = %v, want usage MachineError", req, err)
		}
		if len(reader.calls) != 0 {
			t.Fatalf("URLLookup.Lookup(%#v) reader calls = %#v, want none", req, reader.calls)
		}
	}
}

func TestURLLookupAcceptsAbsoluteAndBareHostForms(t *testing.T) {
	t.Parallel()

	reader := &runtimeURLLookupReader{}
	lookup, err := NewURLLookupFromReader(reader, redact.ModeStandard)
	if err != nil {
		t.Fatalf("NewURLLookupFromReader() error = %v, want nil", err)
	}
	_, err = lookup.Lookup(context.Background(), machine.URLLookupRequest{URLs: []string{
		"  https://example.com/path  ",
		"example.net/path",
	}})
	if err != nil {
		t.Fatalf("URLLookup.Lookup(supported URL forms) error = %v, want nil", err)
	}
	want := [][]string{{"https://example.com/path", "example.net/path"}}
	if !reflect.DeepEqual(reader.calls, want) {
		t.Fatalf("URLLookup.Lookup(supported URL forms) calls = %#v, want %#v", reader.calls, want)
	}
}

func TestURLLookupRejectsMalformedSDKResponseWithoutLeaking(t *testing.T) {
	t.Parallel()

	const canary = "malformed-response-query-canary"
	tests := []struct {
		name string
		url  string
	}{
		{name: "embedded C0", url: "https://example.com/path?token=" + canary + "\x7f"},
		{name: "leading C0", url: "\nhttps://example.com/path?token=" + canary},
		{name: "trailing C0", url: "https://example.com/path?token=" + canary + "\t"},
		{name: "leading C1", url: "\u0085https://example.com/path?token=" + canary},
		{name: "trailing C1", url: "https://example.com/path?token=" + canary + "\u0085"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reader := &runtimeURLLookupReader{results: []zscaler.URLClassification{{URL: tt.url}}}
			lookup, err := NewURLLookupFromReader(reader, redact.ModeStandard)
			if err != nil {
				t.Fatalf("NewURLLookupFromReader() error = %v, want nil", err)
			}
			_, err = lookup.Lookup(context.Background(), machine.URLLookupRequest{URLs: []string{"example.com"}})
			var machineErr *machine.MachineError
			if !errors.As(err, &machineErr) || machineErr.Kind != machine.ErrorKindLiveAccessFailed {
				t.Fatalf("URLLookup.Lookup(malformed response) error = %v, want live-access MachineError", err)
			}
			if !errors.Is(err, zscaler.ErrLiveAccessFailed) || strings.Contains(err.Error(), canary) {
				t.Fatalf("URLLookup.Lookup(malformed response) error = %q, want safe live sentinel", err)
			}
		})
	}
}

func TestEngineURLLookupValidatesBeforeConfigAndPreservesContext(t *testing.T) {
	t.Parallel()

	configLoads := 0
	engine, err := NewEngine(Options{
		Catalog: resources.ResourceCatalog{},
		loadConfig: func([]string, config.LoadOptions) (config.Config, error) {
			configLoads++
			return config.Config{}, errors.New("config loader must not run")
		},
	})
	if err != nil {
		t.Fatalf("NewEngine() error = %v, want nil", err)
	}
	for _, rawURL := range []string{
		"https://example.com/%",
		"\nhttps://example.com",
		"https://example.com\t",
		"\u0085https://example.com",
		"https://example.com\u0085",
	} {
		_, err = engine.LookupURL(context.Background(), machine.URLLookupRequest{
			URLs: []string{rawURL},
		})
		var machineErr *machine.MachineError
		if !errors.As(err, &machineErr) || machineErr.Kind != machine.ErrorKindUsage {
			t.Fatalf("Engine.LookupURL(invalid request) error = %v, want usage MachineError", err)
		}
		if configLoads != 0 {
			t.Fatalf("Engine.LookupURL(invalid request) config loads = %d, want 0", configLoads)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = engine.LookupURL(ctx, machine.URLLookupRequest{URLs: []string{"example.com"}})
	var machineErr *machine.MachineError
	if !errors.As(err, &machineErr) || machineErr.Kind != machine.ErrorKindCanceled ||
		!errors.Is(err, context.Canceled) {
		t.Fatalf("Engine.LookupURL(canceled) error = %v, want canceled MachineError", err)
	}
	if configLoads != 0 {
		t.Fatalf("Engine.LookupURL(canceled) config loads = %d, want 0", configLoads)
	}
}

func TestEngineURLLookupSanitizesConfigErrors(t *testing.T) {
	t.Parallel()

	const canary = "/private/url-lookup-config-canary.yaml"
	rawErr := fmt.Errorf("%w: %s", config.ErrInvalidConfig, canary)
	engine, err := NewEngine(Options{
		Catalog: resources.ResourceCatalog{},
		loadConfig: func([]string, config.LoadOptions) (config.Config, error) {
			return config.Config{}, rawErr
		},
	})
	if err != nil {
		t.Fatalf("NewEngine() error = %v, want nil", err)
	}
	_, err = engine.LookupURL(context.Background(), machine.URLLookupRequest{URLs: []string{"example.com"}})
	var machineErr *machine.MachineError
	if !errors.As(err, &machineErr) || machineErr.Kind != machine.ErrorKindInvalidConfig {
		t.Fatalf("Engine.LookupURL(config error) = %v, want invalid-config MachineError", err)
	}
	if !errors.Is(err, config.ErrInvalidConfig) || errors.Is(err, rawErr) || strings.Contains(err.Error(), canary) {
		t.Fatalf("Engine.LookupURL(config error) = %q, want only safe config sentinel", err)
	}
}

func TestEngineURLLookupExecutesAdvertisedCapability(t *testing.T) {
	t.Parallel()

	cfg, err := config.LoadConfig([]string{
		config.EnvClientID + "=client-id",
		config.EnvClientSecret + "=client-secret",
		config.EnvVanityDomain + "=example",
	}, config.LoadOptions{})
	if err != nil {
		t.Fatalf("config.LoadConfig(engine URL fixture) error = %v, want nil", err)
	}
	reader := &runtimeURLLookupReader{results: []zscaler.URLClassification{{
		URL:         "https://answer.example/path?response=secret",
		Application: "0123456789abcdef0123456789abcdef01234567",
	}}}
	configLoads := 0
	engine, err := NewEngine(Options{
		Catalog: resources.ResourceCatalog{},
		loadConfig: func([]string, config.LoadOptions) (config.Config, error) {
			configLoads++
			return cfg, nil
		},
		newReader: func(zscaler.ReaderConfig) (browser.RecordReader, error) {
			return reader, nil
		},
		Redaction:    redact.ModeShare,
		RedactionSet: true,
	})
	if err != nil {
		t.Fatalf("NewEngine() error = %v, want nil", err)
	}
	result, err := engine.LookupURL(context.Background(), machine.URLLookupRequest{
		URLs: []string{"https://input.example/path?request=secret"},
	})
	if err != nil {
		t.Fatalf("Engine.LookupURL() error = %v, want nil", err)
	}
	if configLoads != 1 {
		t.Fatalf("Engine.LookupURL() config loads = %d, want 1", configLoads)
	}
	if want := [][]string{{"https://input.example/path"}}; !reflect.DeepEqual(reader.calls, want) {
		t.Fatalf("Engine.LookupURL() calls = %#v, want %#v", reader.calls, want)
	}
	got := result.Classifications()
	if len(got) != 1 || got[0].URL != "https://answer.example/path" {
		t.Fatalf("Engine.LookupURL() result = %#v, want sanitized answer", got)
	}
	if got[0].Application != "<REDACTED:SECRET>" {
		t.Fatalf("Engine.LookupURL() application = %q, want effective share redaction", got[0].Application)
	}
}

func TestEngineURLLookupClassifiesMissingCredentialsSafely(t *testing.T) {
	t.Parallel()

	engine, err := NewEngine(Options{
		Env:     []string{"XDG_CONFIG_HOME=" + t.TempDir()},
		Catalog: resources.ResourceCatalog{},
	})
	if err != nil {
		t.Fatalf("NewEngine() error = %v, want nil", err)
	}
	_, err = engine.LookupURL(context.Background(), machine.URLLookupRequest{URLs: []string{"example.com"}})
	var machineErr *machine.MachineError
	if !errors.As(err, &machineErr) || machineErr.Kind != machine.ErrorKindMissingCredentials {
		t.Fatalf("Engine.LookupURL(no credentials) error = %v, want missing-credentials MachineError", err)
	}
	if !errors.Is(err, zscaler.ErrMissingCredentials) {
		t.Fatalf("Engine.LookupURL(no credentials) error = %v, want ErrMissingCredentials", err)
	}
	var missingErr *zscaler.MissingCredentialsError
	if !errors.As(err, &missingErr) || len(missingErr.Missing) == 0 {
		t.Fatalf("Engine.LookupURL(no credentials) error = %v, want safe missing names", err)
	}
	if !reflect.DeepEqual(machineErr.Missing, missingErr.Missing) {
		t.Fatalf("Engine.LookupURL(no credentials) MachineError missing = %#v, want %#v", machineErr.Missing, missingErr.Missing)
	}
	for _, name := range missingErr.Missing {
		if !strings.HasPrefix(name, "ZSCALERCTL_") {
			t.Fatalf("Engine.LookupURL(no credentials) missing name = %q, want canonical env name", name)
		}
	}
}

func TestURLLookupNormalizesReaderErrorsWithoutCanary(t *testing.T) {
	t.Parallel()

	const canary = "reader-error-client_secret-canary"
	reader := &runtimeURLLookupReader{err: fmt.Errorf("%s: %w", canary, zscaler.ErrLiveAccessFailed)}
	lookup, err := NewURLLookupFromReader(reader, redact.ModeStandard)
	if err != nil {
		t.Fatalf("NewURLLookupFromReader() error = %v, want nil", err)
	}
	_, err = lookup.Lookup(context.Background(), machine.URLLookupRequest{URLs: []string{"example.com"}})
	var machineErr *machine.MachineError
	if !errors.As(err, &machineErr) || machineErr.Kind != machine.ErrorKindLiveAccessFailed {
		t.Fatalf("URLLookup.Lookup(reader error) = %v, want live-access MachineError", err)
	}
	if !errors.Is(err, zscaler.ErrLiveAccessFailed) || strings.Contains(err.Error(), canary) {
		t.Fatalf("URLLookup.Lookup(reader error) = %q, want safe live sentinel", err)
	}
}

func TestURLLookupDefensivelyCopiesRequestBeforeReader(t *testing.T) {
	t.Parallel()

	reader := &runtimeURLLookupReader{
		mutateInput: true,
		results:     []zscaler.URLClassification{{URL: "example.com"}},
	}
	lookup, err := NewURLLookupFromReader(reader, redact.ModeStandard)
	if err != nil {
		t.Fatalf("NewURLLookupFromReader() error = %v, want nil", err)
	}
	requestURLs := []string{"example.com"}
	if _, err := lookup.Lookup(context.Background(), machine.URLLookupRequest{URLs: requestURLs}); err != nil {
		t.Fatalf("URLLookup.Lookup() error = %v, want nil", err)
	}
	if requestURLs[0] != "example.com" {
		t.Fatalf("URLLookup.Lookup() mutated request URLs = %#v", requestURLs)
	}
}

func TestNewURLLookupFromReaderReportsUnsupportedCapability(t *testing.T) {
	t.Parallel()

	_, err := NewURLLookupFromReader(&runtimeFakeReader{}, redact.ModeStandard)
	if !errors.Is(err, zscaler.ErrUnsupportedResource) {
		t.Fatalf("NewURLLookupFromReader(record-only reader) error = %v, want ErrUnsupportedResource", err)
	}
	var machineErr *machine.MachineError
	if !errors.As(err, &machineErr) || machineErr.Kind != machine.ErrorKindUnsupportedResource {
		t.Fatalf("NewURLLookupFromReader(record-only reader) error = %v, want unsupported-resource MachineError", err)
	}
	if !strings.Contains(err.Error(), "zia/url-lookup") {
		t.Fatalf("NewURLLookupFromReader(record-only reader) error = %q, want zia/url-lookup context", err.Error())
	}
}

type runtimeURLLookupReader struct {
	runtimeFakeReader
	calls       [][]string
	results     []zscaler.URLClassification
	err         error
	mutateInput bool
}

func (r *runtimeURLLookupReader) URLLookup(_ context.Context, urls []string) ([]zscaler.URLClassification, error) {
	r.calls = append(r.calls, append([]string(nil), urls...))
	if r.mutateInput && len(urls) > 0 {
		urls[0] = "reader-mutated"
	}
	if r.err != nil {
		return nil, r.err
	}
	return r.results, nil
}

func assertNoUnsafeURLLookupRunes(t *testing.T, values []machine.URLClassification) {
	t.Helper()

	check := func(value string) {
		for _, ch := range value {
			if unicode.IsControl(ch) || unicode.Is(unicode.Cf, ch) {
				t.Fatalf("URL lookup value %q contains unsafe rune U+%04X", value, ch)
			}
		}
	}
	for _, value := range values {
		check(value.URL)
		check(value.Application)
		for _, classification := range value.Classifications {
			check(classification)
		}
		for _, classification := range value.SecurityAlertClassifications {
			check(classification)
		}
	}
}
