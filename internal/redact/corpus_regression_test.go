package redact_test

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/redact"
)

// The corpora below were generated as realistic accidental pastes and
// realistic tenant text, then measured against main and this scanner. They are
// stored gzip+base64 so vendor-shaped synthetic tokens never appear on disk
// for secret scanners; they are decoded only in memory here.
//
// testdata/corpus/pasted_keys.json.gz.b64: each pasted key lists the scanners
// that redact it; a scanner that stops redacting one is a regression.
// testdata/corpus/tenant_text.json.gz.b64: tenant text that main (or, for
// later additions, the previous branch head) leaves unchanged in every
// scanner; any change is a new false positive.

type corpusPastedKey struct {
	Input      string   `json:"input"`
	Secret     string   `json:"secret"`
	Category   string   `json:"category"`
	RedactedBy []string `json:"redacted_by"`
}

type corpusTenantText struct {
	Input    string `json:"input"`
	Category string `json:"category"`
}

func loadCorpus(t *testing.T, name string, into any) {
	t.Helper()
	encoded, err := os.ReadFile("testdata/corpus/" + name)
	if err != nil {
		t.Fatalf("read corpus %s: %v", name, err)
	}
	compressed, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(string(encoded)), ""))
	if err != nil {
		t.Fatalf("decode corpus %s: %v", name, err)
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("gunzip corpus %s: %v", name, err)
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("gunzip corpus %s: %v", name, err)
	}
	if err := json.Unmarshal(body, into); err != nil {
		t.Fatalf("parse corpus %s: %v", name, err)
	}
}

func corpusScanners() map[string]func(redact.Redactor, string) (string, redact.Report) {
	return map[string]func(redact.Redactor, string) (string, redact.Report){
		"ScanString":         redact.Redactor.ScanString,
		"ScanRenderedString": redact.Redactor.ScanRenderedString,
		"ScanFreeText":       redact.Redactor.ScanFreeText,
		"ScanDisplayName":    redact.Redactor.ScanDisplayName,
	}
}

func TestPastedKeyCorpusStaysRedacted(t *testing.T) {
	t.Parallel()

	var cases []corpusPastedKey
	loadCorpus(t, "pasted_keys.json.gz.b64", &cases)
	if len(cases) < 100 {
		t.Fatalf("pasted key corpus has %d cases, want at least 100", len(cases))
	}
	scanners := corpusScanners()
	r := redact.New(redact.ModeStandard)
	for _, c := range cases {
		if !strings.Contains(c.Input, c.Secret) {
			t.Fatalf("corpus case %q does not contain its secret", c.Category)
		}
		for _, name := range c.RedactedBy {
			scan, ok := scanners[name]
			if !ok {
				t.Fatalf("corpus names unknown scanner %q", name)
			}
			if got, _ := scan(r, c.Input); strings.Contains(got, c.Secret) {
				t.Errorf("%s no longer redacts a %s corpus key", name, c.Category)
			}
		}
	}
}

func TestTenantTextCorpusIsPreserved(t *testing.T) {
	t.Parallel()

	var cases []corpusTenantText
	loadCorpus(t, "tenant_text.json.gz.b64", &cases)
	if len(cases) < 200 {
		t.Fatalf("tenant text corpus has %d cases, want at least 200", len(cases))
	}
	r := redact.New(redact.ModeStandard)
	for _, c := range cases {
		for name, scan := range corpusScanners() {
			if got, report := scan(r, c.Input); got != c.Input {
				t.Errorf("%s changed %s tenant text %q to %q (report %#v)", name, c.Category, c.Input, got, report)
			}
		}
	}
}
