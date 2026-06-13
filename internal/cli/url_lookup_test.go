package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/cli"
	"github.com/dvmrry/zscalerctl/internal/zscaler"
)

// fakeURLLookupReader satisfies ResourceReader (via embedding) plus the
// optional URLLookupReader capability, recording the URLs each call received.
type fakeURLLookupReader struct {
	fakeResourceReader
	calls   [][]string
	results []zscaler.URLClassification
	err     error
}

func (f *fakeURLLookupReader) URLLookup(_ context.Context, urls []string) ([]zscaler.URLClassification, error) {
	f.calls = append(f.calls, append([]string(nil), urls...))
	if f.err != nil {
		return nil, f.err
	}
	return f.results, nil
}

func TestURLLookupRendersStableJSONShape(t *testing.T) {
	t.Parallel()

	reader := &fakeURLLookupReader{
		results: []zscaler.URLClassification{
			{
				URL:                          "https://example.com",
				Classifications:              []string{"PROFESSIONAL_SERVICES", "TECHNOLOGY"},
				SecurityAlertClassifications: []string{"MALWARE_SITE"},
				Application:                  "EXAMPLE_APP",
			},
		},
	}
	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})

	err := app.Run(context.Background(), []string{"--format", "json", "zia", "url-lookup", "https://example.com"})
	if err != nil {
		t.Fatalf("App.Run(zia url-lookup) error = %v, want nil", err)
	}
	want := `[
  {
    "url": "https://example.com",
    "classifications": [
      "PROFESSIONAL_SERVICES",
      "TECHNOLOGY"
    ],
    "security_alert_classifications": [
      "MALWARE_SITE"
    ],
    "application": "EXAMPLE_APP"
  }
]
`
	if got := out.String(); got != want {
		t.Errorf("App.Run(zia url-lookup) stdout = %q, want %q", got, want)
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(zia url-lookup) stderr = %q, want empty", errOut.String())
	}
	if len(reader.calls) != 1 || len(reader.calls[0]) != 1 || reader.calls[0][0] != "https://example.com" {
		t.Errorf("URLLookup calls = %v, want one call with the input URL", reader.calls)
	}
}

func TestURLLookupPassesMultipleURLsAndRendersEmptyArrays(t *testing.T) {
	t.Parallel()

	reader := &fakeURLLookupReader{
		results: []zscaler.URLClassification{
			{URL: "a.example.com", Classifications: []string{"NEWS_AND_MEDIA"}},
			{URL: "b.example.com"},
		},
	}
	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})

	err := app.Run(context.Background(), []string{"--format", "json", "zia", "url-lookup", "a.example.com", "b.example.com"})
	if err != nil {
		t.Fatalf("App.Run(zia url-lookup multi) error = %v, want nil", err)
	}
	if len(reader.calls) != 1 {
		t.Fatalf("URLLookup calls = %d, want 1", len(reader.calls))
	}
	if got := strings.Join(reader.calls[0], ","); got != "a.example.com,b.example.com" {
		t.Errorf("URLLookup(urls) = %q, want %q", got, "a.example.com,b.example.com")
	}
	var decoded []struct {
		URL                          string   `json:"url"`
		Classifications              []string `json:"classifications"`
		SecurityAlertClassifications []string `json:"security_alert_classifications"`
	}
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(stdout) error = %v; body = %q", err, out.String())
	}
	if len(decoded) != 2 || decoded[0].URL != "a.example.com" || decoded[1].URL != "b.example.com" {
		t.Errorf("decoded results = %+v, want two results in input order", decoded)
	}
	// Empty classification lists must render as [], never null, so the JSON
	// shape stays stable for scripted consumers.
	if !strings.Contains(out.String(), `"classifications": []`) {
		t.Errorf("App.Run(zia url-lookup multi) stdout = %q, want empty classifications rendered as []", out.String())
	}
	if !strings.Contains(out.String(), `"security_alert_classifications": []`) {
		t.Errorf("App.Run(zia url-lookup multi) stdout = %q, want empty security alerts rendered as []", out.String())
	}
}

func TestURLLookupStripsQueryAndFragmentBeforeLookupAndOutput(t *testing.T) {
	t.Parallel()

	reader := &fakeURLLookupReader{
		results: []zscaler.URLClassification{
			{URL: "https://example.com/callback", Classifications: []string{"TECHNOLOGY"}},
		},
	}
	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})

	err := app.Run(context.Background(), []string{"--format", "json", "zia", "url-lookup", "https://example.com/callback?code=abc123#token"})
	if err != nil {
		t.Fatalf("App.Run(zia url-lookup query) error = %v, want nil", err)
	}
	if len(reader.calls) != 1 || len(reader.calls[0]) != 1 || reader.calls[0][0] != "https://example.com/callback" {
		t.Errorf("URLLookup calls = %v, want sanitized URL only", reader.calls)
	}
	for _, forbidden := range []string{"abc123", "#token", "?code"} {
		if strings.Contains(out.String(), forbidden) {
			t.Errorf("App.Run(zia url-lookup query) stdout = %q, want no %q", out.String(), forbidden)
		}
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(zia url-lookup query) stderr = %q, want empty", errOut.String())
	}
}

func TestURLLookupStripsUserinfoCredentials(t *testing.T) {
	t.Parallel()

	reader := &fakeURLLookupReader{
		results: []zscaler.URLClassification{
			{URL: "https://internal.example.com/app", Classifications: []string{"CORPORATE_MARKETING"}},
		},
	}
	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})

	err := app.Run(context.Background(), []string{"--format", "json", "zia", "url-lookup", "https://user:s3cr3t@internal.example.com/app?token=abc#frag"})
	if err != nil {
		t.Fatalf("App.Run(zia url-lookup userinfo) error = %v, want nil", err)
	}
	if len(reader.calls) != 1 || len(reader.calls[0]) != 1 || reader.calls[0][0] != "https://internal.example.com/app" {
		t.Errorf("URLLookup calls = %v, want userinfo/query/fragment stripped", reader.calls)
	}
	for _, forbidden := range []string{"user:", "s3cr3t", "token=abc", "#frag"} {
		if strings.Contains(out.String(), forbidden) {
			t.Errorf("App.Run(zia url-lookup userinfo) stdout = %q, want no %q", out.String(), forbidden)
		}
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(zia url-lookup userinfo) stderr = %q, want empty", errOut.String())
	}
}

func TestURLLookupRequiresAtLeastOneURL(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: &fakeURLLookupReader{}})

	err := app.Run(context.Background(), []string{"zia", "url-lookup"})
	if !errors.Is(err, cli.ErrUsage) {
		t.Fatalf("App.Run(zia url-lookup) error = %v, want ErrUsage", err)
	}
	if !strings.Contains(err.Error(), "usage: zscalerctl zia url-lookup <url> [url...]") {
		t.Errorf("App.Run(zia url-lookup) error = %q, want usage message", err.Error())
	}
	if out.Len() != 0 {
		t.Errorf("App.Run(zia url-lookup) stdout = %q, want empty", out.String())
	}
}

func TestURLLookupRejectsFlagShapedArgument(t *testing.T) {
	t.Parallel()

	reader := &fakeURLLookupReader{}
	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})

	err := app.Run(context.Background(), []string{"zia", "url-lookup", "--json"})
	if !errors.Is(err, cli.ErrUsage) {
		t.Fatalf("App.Run(zia url-lookup --json) error = %v, want ErrUsage", err)
	}
	if len(reader.calls) != 0 {
		t.Errorf("URLLookup calls = %v, want none for a usage error", reader.calls)
	}
}

func TestURLLookupReportsUnsupportedReader(t *testing.T) {
	t.Parallel()

	// fakeResourceReader implements ResourceReader only; the optional lookup
	// capability is absent, so the CLI must fail clean rather than panic.
	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: fakeResourceReader{}})

	err := app.Run(context.Background(), []string{"zia", "url-lookup", "example.com"})
	if !errors.Is(err, zscaler.ErrUnsupportedResource) {
		t.Fatalf("App.Run(zia url-lookup, no capability) error = %v, want ErrUnsupportedResource", err)
	}
	if !strings.Contains(err.Error(), "zia/url-lookup") {
		t.Errorf("App.Run(zia url-lookup, no capability) error = %q, want zia/url-lookup context", err.Error())
	}
	if out.Len() != 0 {
		t.Errorf("App.Run(zia url-lookup, no capability) stdout = %q, want empty", out.String())
	}
}

func TestURLLookupRedactsSecretShapedClassificationValues(t *testing.T) {
	t.Parallel()

	// A secret-shaped value in an API-returned classification must be caught by
	// the renderer's redaction pass even though the field is allow-listed.
	const canary = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJjYW5hcnkifQ.c2VjcmV0LXNpZ25hdHVyZS1jYW5hcnk"
	reader := &fakeURLLookupReader{
		results: []zscaler.URLClassification{
			{
				URL:                          "example.com",
				Classifications:              []string{canary},
				SecurityAlertClassifications: []string{"MALWARE_SITE"},
			},
		},
	}
	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})

	err := app.Run(context.Background(), []string{"--format", "json", "zia", "url-lookup", "example.com"})
	if err != nil {
		t.Fatalf("App.Run(zia url-lookup canary) error = %v, want nil", err)
	}
	if strings.Contains(out.String(), canary) {
		t.Errorf("App.Run(zia url-lookup canary) stdout = %q, want no canary", out.String())
	}
	if !strings.Contains(out.String(), "<REDACTED:JWT>") {
		t.Errorf("App.Run(zia url-lookup canary) stdout = %q, want <REDACTED:JWT> marker", out.String())
	}
}

func TestURLLookupRendersTableFormat(t *testing.T) {
	t.Parallel()

	reader := &fakeURLLookupReader{
		results: []zscaler.URLClassification{
			{
				URL:                          "example.com",
				Classifications:              []string{"NEWS_AND_MEDIA", "TECHNOLOGY"},
				SecurityAlertClassifications: []string{"MALWARE_SITE"},
				Application:                  "EXAMPLE_APP",
			},
		},
	}
	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})

	err := app.Run(context.Background(), []string{"--format", "table", "zia", "url-lookup", "example.com"})
	if err != nil {
		t.Fatalf("App.Run(zia url-lookup --format table) error = %v, want nil", err)
	}
	want := "url\tclassifications\tsecurity_alert_classifications\tapplication\n" +
		"example.com\tNEWS_AND_MEDIA,TECHNOLOGY\tMALWARE_SITE\tEXAMPLE_APP\n"
	if got := out.String(); got != want {
		t.Errorf("App.Run(zia url-lookup --format table) stdout = %q, want %q", got, want)
	}
}
