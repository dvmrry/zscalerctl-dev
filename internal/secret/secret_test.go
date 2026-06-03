package secret_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/secret"
	"gopkg.in/yaml.v3"
)

func TestSecretDoesNotRevealThroughFormatting(t *testing.T) {
	t.Parallel()

	const raw = "super-secret-value"
	s := secret.New(raw)

	tests := []struct {
		name string
		got  string
	}{
		{name: "String", got: s.String()},
		//lint:ignore S1025 this intentionally exercises fmt's %s path.
		{name: "fmt_s", got: fmt.Sprintf("%s", s)},
		{name: "fmt_q", got: fmt.Sprintf("%q", s)},
		{name: "fmt_v", got: fmt.Sprintf("%v", s)},
		{name: "fmt_plus_v", got: fmt.Sprintf("%+v", s)},
		{name: "fmt_sharp_v", got: fmt.Sprintf("%#v", s)},
		{name: "LogValue", got: s.LogValue().String()},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if strings.Contains(tt.got, raw) {
				t.Errorf("%s leaked secret: got %q, want no %q", tt.name, tt.got, raw)
			}
		})
	}
}

func TestSecretDoesNotRevealThroughTextMarshal(t *testing.T) {
	t.Parallel()

	const raw = "text-secret-value"
	got, err := secret.New(raw).MarshalText()
	if err != nil {
		t.Fatalf("Secret.MarshalText() error = %v, want nil", err)
	}
	if strings.Contains(string(got), raw) {
		t.Errorf("Secret.MarshalText() = %q, want no %q", got, raw)
	}
	if !strings.Contains(string(got), "REDACTED") {
		t.Errorf("Secret.MarshalText() = %q, want redacted marker", got)
	}
}

func TestSecretDoesNotRevealThroughJSON(t *testing.T) {
	t.Parallel()

	const raw = "client-secret-value"
	got, err := json.Marshal(struct {
		Secret secret.Secret `json:"secret"`
	}{Secret: secret.New(raw)})
	if err != nil {
		t.Fatalf("json.Marshal(secret) error = %v, want nil", err)
	}
	if strings.Contains(string(got), raw) {
		t.Errorf("json.Marshal(secret) = %s, want no %q", got, raw)
	}
	if !strings.Contains(string(got), "REDACTED") {
		t.Errorf("json.Marshal(secret) = %s, want redacted marker", got)
	}
}

func TestSecretDoesNotRevealThroughYAML(t *testing.T) {
	t.Parallel()

	const raw = "yaml-secret-value"
	got, err := yaml.Marshal(struct {
		Secret secret.Secret `yaml:"secret"`
	}{Secret: secret.New(raw)})
	if err != nil {
		t.Fatalf("yaml.Marshal(secret) error = %v, want nil", err)
	}
	if strings.Contains(string(got), raw) {
		t.Errorf("yaml.Marshal(secret) = %s, want no %q", got, raw)
	}
	if !strings.Contains(string(got), "REDACTED") {
		t.Errorf("yaml.Marshal(secret) = %s, want redacted marker", got)
	}
}

func TestSecretDoesNotRevealThroughSlogHandler(t *testing.T) {
	t.Parallel()

	const raw = "slog-secret-value"
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	logger.Info("test", "secret", secret.New(raw))

	got := buf.String()
	if strings.Contains(got, raw) {
		t.Errorf("slog output = %q, want no %q", got, raw)
	}
	if !strings.Contains(got, "REDACTED") {
		t.Errorf("slog output = %q, want redacted marker", got)
	}
}

func TestSecretLogValuerInterface(t *testing.T) {
	t.Parallel()

	var _ slog.LogValuer = secret.New("value")
}
