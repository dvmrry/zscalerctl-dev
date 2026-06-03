package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/cli"
	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/dump"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/dvmrry/zscalerctl/internal/zscaler"
)

func TestConfigShowDoesNotExposeEnvironmentSecrets(t *testing.T) {
	t.Parallel()

	const clientID = "client-id-value"
	const clientSecret = "client-secret-value"
	const proxyURL = "http://proxy-user:proxy-secret@proxy.example.invalid:8080"
	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, []string{
		config.EnvClientID + "=" + clientID,
		config.EnvClientSecret + "=" + clientSecret,
		config.EnvProxyURL + "=" + proxyURL,
	})

	err := app.Run(context.Background(), []string{"--format", "json", "config", "show"})
	if err != nil {
		t.Fatalf("App.Run(config show) error = %v, want nil", err)
	}
	got := out.String()
	for _, forbidden := range []string{clientID, clientSecret, "proxy-user", "proxy-secret", "proxy.example.invalid"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("App.Run(config show) output = %q, want no %q", got, forbidden)
		}
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(config show) stderr = %q, want empty", errOut.String())
	}
}

func TestDoctorDoesNotExposeEnvironmentSecrets(t *testing.T) {
	t.Parallel()

	const clientID = "client-id-value"
	const clientSecret = "client-secret-value"
	const proxyURL = "http://proxy-user:proxy-secret@proxy.example.invalid:8080"
	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, []string{
		config.EnvClientID + "=" + clientID,
		config.EnvClientSecret + "=" + clientSecret,
		config.EnvProxyURL + "=" + proxyURL,
	})

	err := app.Run(context.Background(), []string{"doctor"})
	if err != nil {
		t.Fatalf("App.Run(doctor) error = %v, want nil", err)
	}
	for _, forbidden := range []string{clientID, clientSecret, "proxy-user", "proxy-secret", "proxy.example.invalid"} {
		if strings.Contains(out.String(), forbidden) {
			t.Errorf("App.Run(doctor) output = %q, want no %q", out.String(), forbidden)
		}
	}
}

func TestAuthStatusDoesNotExposeEnvironmentSecrets(t *testing.T) {
	t.Parallel()

	const clientID = "client-id-value"
	const clientSecret = "client-secret-value"
	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, []string{
		config.EnvClientID + "=" + clientID,
		config.EnvClientSecret + "=" + clientSecret,
	})

	err := app.Run(context.Background(), []string{"auth", "status"})
	if err != nil {
		t.Fatalf("App.Run(auth status) error = %v, want nil", err)
	}
	for _, forbidden := range []string{clientID, clientSecret} {
		if strings.Contains(out.String(), forbidden) {
			t.Errorf("App.Run(auth status) output = %q, want no %q", out.String(), forbidden)
		}
	}
}

func TestAuthStatusReportsZIALegacyWithoutExposingSecrets(t *testing.T) {
	t.Parallel()

	const (
		username = "admin@example.invalid"
		password = "legacy-password-value"
		apiKey   = "legacy-api-key-value"
		cloud    = "zscalerthree"
	)
	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, []string{
		config.EnvAuthMode + "=" + string(config.AuthModeZIALegacy),
		config.EnvZIAUsername + "=" + username,
		config.EnvZIAPassword + "=" + password,
		config.EnvZIAAPIKey + "=" + apiKey,
		config.EnvZIACloud + "=" + cloud,
	})

	err := app.Run(context.Background(), []string{"auth", "status"})
	if err != nil {
		t.Fatalf("App.Run(auth status ZIA legacy) error = %v, want nil", err)
	}
	if !strings.Contains(out.String(), "available for read-only commands") {
		t.Errorf("App.Run(auth status ZIA legacy) output = %q, want live API available", out.String())
	}
	for _, forbidden := range []string{username, password, apiKey, cloud} {
		if strings.Contains(out.String(), forbidden) {
			t.Errorf("App.Run(auth status ZIA legacy) output = %q, want no %q", out.String(), forbidden)
		}
		if strings.Contains(errOut.String(), forbidden) {
			t.Errorf("App.Run(auth status ZIA legacy) stderr = %q, want no %q", errOut.String(), forbidden)
		}
	}
}

func TestDoctorReportsReadOnlyMode(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, nil)

	err := app.Run(context.Background(), []string{"doctor"})
	if err != nil {
		t.Fatalf("App.Run(doctor) error = %v, want nil", err)
	}
	got := out.String()
	if !strings.Contains(got, "Mode") || !strings.Contains(got, "read-only") {
		t.Errorf("App.Run(doctor) output = %q, want read-only mode", got)
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(doctor) stderr = %q, want empty", errOut.String())
	}
}

func TestDoctorColorAlwaysUsesANSI(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, []string{"TERM=xterm-256color"})

	err := app.Run(context.Background(), []string{"--color", "always", "doctor"})
	if err != nil {
		t.Fatalf("App.Run(--color always doctor) error = %v, want nil", err)
	}
	if !strings.Contains(out.String(), "\x1b[38;5;") {
		t.Errorf("App.Run(--color always doctor) output = %q, want 256-color ANSI escape", out.String())
	}
}

func TestDoctorNoColorOverridesAlways(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, []string{"TERM=xterm-256color"})

	err := app.Run(context.Background(), []string{"--color", "always", "--no-color", "doctor"})
	if err != nil {
		t.Fatalf("App.Run(--color always --no-color doctor) error = %v, want nil", err)
	}
	if strings.Contains(out.String(), "\x1b[") {
		t.Errorf("App.Run(--color always --no-color doctor) output = %q, want no ANSI escapes", out.String())
	}
}

func TestRedactionOffIsUsageError(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, nil)

	err := app.Run(context.Background(), []string{"--redaction", "off", "doctor"})
	if !errors.Is(err, cli.ErrUsage) {
		t.Fatalf("App.Run(--redaction off doctor) error = %v, want ErrUsage", err)
	}
}

func TestEnvRedactionIsNotDowngradedByAbsentFlag(t *testing.T) {
	t.Parallel()

	for _, mode := range []string{"share", "paranoid"} {
		mode := mode
		t.Run(mode, func(t *testing.T) {
			t.Parallel()

			var out, errOut bytes.Buffer
			app := cli.New(&out, &errOut, []string{config.EnvRedaction + "=" + mode})

			err := app.Run(context.Background(), []string{"doctor"})
			if err != nil {
				t.Fatalf("App.Run(doctor) error = %v, want nil", err)
			}
			if !strings.Contains(out.String(), "Redaction") || !strings.Contains(out.String(), mode) {
				t.Errorf("App.Run(doctor) output = %q, want redaction mode %q", out.String(), mode)
			}
		})
	}
}

func TestCompletionScriptsDoNotReadCredentialFilesOrUseReader(t *testing.T) {
	t.Parallel()

	const clientID = "client-id-value"
	for _, shell := range []string{"bash", "zsh", "fish"} {
		shell := shell
		t.Run(shell, func(t *testing.T) {
			t.Parallel()

			var out, errOut bytes.Buffer
			app := cli.NewWithOptions(&out, &errOut, []string{
				config.EnvClientID + "=" + clientID,
				config.EnvClientSecretFile + "=/path/that/must/not/be/read",
			}, cli.Options{Reader: failingResourceReader{}})

			err := app.Run(context.Background(), []string{"completion", shell})
			if err != nil {
				t.Fatalf("App.Run(completion %s) error = %v, want nil", shell, err)
			}
			for _, want := range []string{"zscalerctl", "locations", "location-groups", "rule-labels", "static-ips", "gre-tunnels", "--resources", "--continue-on-error", "list get"} {
				if !strings.Contains(out.String(), want) {
					t.Errorf("App.Run(completion %s) stdout = %q, want %q", shell, out.String(), want)
				}
			}
			for _, forbidden := range []string{clientID, "ZSCALERCTL_CLIENT_SECRET_FILE"} {
				if strings.Contains(out.String(), forbidden) {
					t.Errorf("App.Run(completion %s) stdout = %q, want no %q", shell, out.String(), forbidden)
				}
			}
			if errOut.Len() != 0 {
				t.Errorf("App.Run(completion %s) stderr = %q, want empty", shell, errOut.String())
			}
		})
	}
}

func TestBashCompletionRegistersCommand(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, nil)

	err := app.Run(context.Background(), []string{"completion", "bash"})
	if err != nil {
		t.Fatalf("App.Run(completion bash) error = %v, want nil", err)
	}
	if !strings.Contains(out.String(), "complete -F _zscalerctl zscalerctl") {
		t.Errorf("App.Run(completion bash) stdout = %q, want bash registration", out.String())
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(completion bash) stderr = %q, want empty", errOut.String())
	}
}

func TestCompletionRejectsUnknownShell(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, nil)

	err := app.Run(context.Background(), []string{"completion", "powershell"})
	if !errors.Is(err, cli.ErrUsage) {
		t.Fatalf("App.Run(completion powershell) error = %v, want ErrUsage", err)
	}
	if out.Len() != 0 {
		t.Errorf("App.Run(completion powershell) stdout = %q, want empty", out.String())
	}
}

func TestHelpDoesNotReadCredentialFile(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, []string{
		config.EnvClientSecretFile + "=/path/that/must/not-be-read",
	})

	err := app.Run(context.Background(), []string{"help"})
	if err != nil {
		t.Fatalf("App.Run(help) error = %v, want nil", err)
	}
	if !strings.Contains(out.String(), "completion bash|zsh|fish") {
		t.Errorf("App.Run(help) stdout = %q, want completion usage", out.String())
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(help) stderr = %q, want empty", errOut.String())
	}
}

func TestVersionDoesNotReadCredentialFileOrUseReader(t *testing.T) {
	t.Parallel()

	const clientID = "client-id-value"
	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, []string{
		config.EnvClientID + "=" + clientID,
		config.EnvClientSecretFile + "=/path/that/must-not-be-read",
	}, cli.Options{Reader: failingResourceReader{}})

	err := app.Run(context.Background(), []string{"--format", "json", "version"})
	if err != nil {
		t.Fatalf("App.Run(version) error = %v, want nil", err)
	}
	if !strings.Contains(out.String(), `"version"`) {
		t.Errorf("App.Run(version) stdout = %q, want version JSON", out.String())
	}
	for _, forbidden := range []string{clientID, "ZSCALERCTL_CLIENT_SECRET_FILE"} {
		if strings.Contains(out.String(), forbidden) {
			t.Errorf("App.Run(version) stdout = %q, want no %q", out.String(), forbidden)
		}
		if strings.Contains(errOut.String(), forbidden) {
			t.Errorf("App.Run(version) stderr = %q, want no %q", errOut.String(), forbidden)
		}
	}
}

func TestResourceListDefaultReaderRequiresExplicitCredentials(t *testing.T) {
	t.Parallel()

	for _, spec := range ziaListResourceSpecs(t) {
		spec := spec
		t.Run(string(spec.Product)+"/"+spec.Name, func(t *testing.T) {
			t.Parallel()

			var out, errOut bytes.Buffer
			app := cli.New(&out, &errOut, nil)

			err := app.Run(context.Background(), []string{string(spec.Product), spec.Name, "list"})
			if !errors.Is(err, zscaler.ErrMissingCredentials) {
				t.Fatalf("App.Run(%s %s list) error = %v, want ErrMissingCredentials", spec.Product, spec.Name, err)
			}
			if out.Len() != 0 {
				t.Errorf("App.Run(%s %s list) stdout = %q, want empty", spec.Product, spec.Name, out.String())
			}
			if errOut.Len() != 0 {
				t.Errorf("App.Run(%s %s list) stderr = %q, want empty", spec.Product, spec.Name, errOut.String())
			}
		})
	}
}

func TestResourceListDoesNotUseSDKEnvironmentNames(t *testing.T) {
	t.Setenv("ZSCALER_CLIENT_ID", "sdk-client-id")
	t.Setenv("ZSCALER_CLIENT_SECRET", "sdk-client-secret")
	t.Setenv("ZSCALER_VANITY_DOMAIN", "sdk-vanity")
	t.Setenv("ZSCALER_SDK_LOG", "true")
	t.Setenv("ZSCALER_SDK_VERBOSE", "true")
	t.Setenv("ZIA_USERNAME", "legacy-admin@example.invalid")
	t.Setenv("ZIA_PASSWORD", "legacy-password-value")
	t.Setenv("ZIA_API_KEY", "legacy-api-key-value")
	t.Setenv("ZIA_CLOUD", "legacy-cloud")

	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, nil)

	err := app.Run(context.Background(), []string{"zia", "locations", "list"})
	if !errors.Is(err, zscaler.ErrMissingCredentials) {
		t.Fatalf("App.Run(zia locations list with SDK env) error = %v, want ErrMissingCredentials", err)
	}
	for _, forbidden := range []string{"sdk-client-id", "sdk-client-secret", "sdk-vanity", "legacy-admin@example.invalid", "legacy-password-value", "legacy-api-key-value", "legacy-cloud"} {
		if strings.Contains(out.String(), forbidden) {
			t.Errorf("App.Run(zia locations list with SDK env) stdout = %q, want no %q", out.String(), forbidden)
		}
		if strings.Contains(errOut.String(), forbidden) {
			t.Errorf("App.Run(zia locations list with SDK env) stderr = %q, want no %q", errOut.String(), forbidden)
		}
	}
}

func TestResourceListProjectsAndRedactsFixture(t *testing.T) {
	t.Parallel()

	const (
		topLevelPSK       = "top-level-psk-canary"
		nestedPSK         = "plain-raw-nested-psk-canary"
		freeTextPSK       = "free-text-psk-canary"
		bareFreeTextToken = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
	)
	reader := fakeResourceReader{
		list: []resources.SourceRecord{resources.NewSourceRecord(map[string]any{
			"id":           "123",
			"name":         "HQ",
			"ipAddresses":  []any{"192.0.2.10"},
			"description":  "temporary psk=" + freeTextPSK + " " + bareFreeTextToken,
			"preSharedKey": topLevelPSK,
			"vpnCredentials": map[string]any{
				"preSharedKey": nestedPSK,
			},
			"newSdkField": "surprise",
		})},
	}
	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})

	err := app.Run(context.Background(), []string{"--format", "json", "zia", "locations", "list"})
	if err != nil {
		t.Fatalf("App.Run(zia locations list) error = %v, want nil", err)
	}
	for _, forbidden := range []string{topLevelPSK, nestedPSK, freeTextPSK, bareFreeTextToken} {
		if strings.Contains(out.String(), forbidden) {
			t.Errorf("App.Run(zia locations list) stdout = %q, want no %q", out.String(), forbidden)
		}
		if strings.Contains(errOut.String(), forbidden) {
			t.Errorf("App.Run(zia locations list) stderr = %q, want no %q", errOut.String(), forbidden)
		}
	}
	var decoded []map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(resource list output) error = %v, want nil; output=%q", err, out.String())
	}
	if len(decoded) != 1 {
		t.Fatalf("decoded resource list length = %d, want 1", len(decoded))
	}
	if _, ok := decoded[0]["preSharedKey"]; ok {
		t.Errorf("decoded resource list = %#v, want no preSharedKey", decoded[0])
	}
	if _, ok := decoded[0]["vpnCredentials"]; ok {
		t.Errorf("decoded resource list = %#v, want no unmodeled nested vpnCredentials", decoded[0])
	}
	if _, ok := decoded[0]["newSdkField"]; ok {
		t.Errorf("decoded resource list = %#v, want no unknown newSdkField", decoded[0])
	}
	description, ok := decoded[0]["description"].(string)
	if !ok || !strings.Contains(description, "<REDACTED:SECRET>") {
		t.Errorf("decoded resource list description = %v, %t, want typed redaction marker", decoded[0]["description"], ok)
	}
	spec, ok := resources.FindSpec(resources.ProductZIA, "locations")
	if !ok {
		t.Fatal("FindSpec(zia, locations) ok = false, want true")
	}
	if err := resources.AssertRenderedSubset(spec, "", decoded[0]); err != nil {
		t.Errorf("AssertRenderedSubset(projected output) error = %v, want nil", err)
	}
}

func TestResourceCommandsRejectAdvertisedButUnsupportedFormats(t *testing.T) {
	t.Parallel()

	reader := fakeResourceReader{
		list: []resources.SourceRecord{resources.NewSourceRecord(map[string]any{
			"id":   "123",
			"name": "HQ",
		})},
		get: resources.NewSourceRecord(map[string]any{
			"id":   "123",
			"name": "HQ",
		}),
	}
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "list yaml",
			args: []string{"--format", "yaml", "zia", "locations", "list"},
			want: "yaml output is not supported for resource list yet",
		},
		{
			name: "get ndjson",
			args: []string{"--format", "ndjson", "zia", "locations", "get", "123"},
			want: "ndjson output is not supported for resource get yet",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var out, errOut bytes.Buffer
			app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})

			err := app.Run(context.Background(), tt.args)
			if err == nil {
				t.Fatalf("App.Run(%v) error = nil, want unsupported format error", tt.args)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("App.Run(%v) error = %q, want %q", tt.args, err.Error(), tt.want)
			}
			if out.Len() != 0 {
				t.Errorf("App.Run(%v) stdout = %q, want empty", tt.args, out.String())
			}
			if errOut.Len() != 0 {
				t.Errorf("App.Run(%v) stderr = %q, want empty", tt.args, errOut.String())
			}
		})
	}
}

func TestResourceListRuleLabelsUsesCatalogProjection(t *testing.T) {
	t.Parallel()

	const (
		canary            = "rule-label-cli-psk-canary"
		bareFreeTextToken = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
	)
	reader := fakeResourceReader{
		list: []resources.SourceRecord{resources.NewSourceRecord(map[string]any{
			"id":                  789,
			"name":                "Outbound psk=" + canary,
			"description":         "temporary psk=" + canary + " " + bareFreeTextToken,
			"lastModifiedTime":    1712345678,
			"referencedRuleCount": 3,
			"createdBy": map[string]any{
				"name": "admin@example.invalid",
			},
		})},
	}
	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})

	err := app.Run(context.Background(), []string{"--format", "json", "zia", "rule-labels", "list"})
	if err != nil {
		t.Fatalf("App.Run(zia rule-labels list) error = %v, want nil", err)
	}
	if strings.Contains(out.String(), canary) {
		t.Errorf("App.Run(zia rule-labels list) stdout = %q, want no %q", out.String(), canary)
	}
	if strings.Contains(out.String(), bareFreeTextToken) {
		t.Errorf("App.Run(zia rule-labels list) stdout = %q, want no bare token", out.String())
	}
	if strings.Contains(errOut.String(), canary) {
		t.Errorf("App.Run(zia rule-labels list) stderr = %q, want no %q", errOut.String(), canary)
	}
	if strings.Contains(errOut.String(), bareFreeTextToken) {
		t.Errorf("App.Run(zia rule-labels list) stderr = %q, want no bare token", errOut.String())
	}
	var decoded []map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(rule-labels list output) error = %v, want nil; output=%q", err, out.String())
	}
	if len(decoded) != 1 {
		t.Fatalf("decoded rule-labels list length = %d, want 1", len(decoded))
	}
	if _, ok := decoded[0]["createdBy"]; ok {
		t.Errorf("decoded rule-labels list = %#v, want no unmodeled createdBy", decoded[0])
	}
	for _, field := range []string{"name", "description"} {
		value, ok := decoded[0][field].(string)
		if !ok || !strings.Contains(value, "<REDACTED:SECRET>") {
			t.Errorf("decoded rule-labels %s = %v, %t, want typed redaction marker", field, decoded[0][field], ok)
		}
	}
	spec, ok := resources.FindSpec(resources.ProductZIA, "rule-labels")
	if !ok {
		t.Fatal("FindSpec(zia, rule-labels) ok = false, want true")
	}
	if err := resources.AssertRenderedSubset(spec, "", decoded[0]); err != nil {
		t.Errorf("AssertRenderedSubset(rule-labels output) error = %v, want nil", err)
	}
}

func TestDumpWritesRestrictedFilesAndReportsWithoutCanaries(t *testing.T) {
	t.Parallel()

	const (
		topLevelPSK       = "top-level-psk-canary"
		nestedPSK         = "plain-raw-nested-psk-canary"
		freeTextPSK       = "free-text-psk-canary"
		bareFreeTextToken = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
	)
	reader := fakeResourceReader{
		list: []resources.SourceRecord{resources.NewSourceRecord(map[string]any{
			"id":           "123",
			"name":         "HQ",
			"ipAddresses":  []any{"192.0.2.10"},
			"description":  "temporary psk=" + freeTextPSK + " " + bareFreeTextToken,
			"preSharedKey": topLevelPSK,
			"vpnCredentials": map[string]any{
				"preSharedKey": nestedPSK,
			},
			"newSdkField": "surprise",
		})},
	}
	var out, errOut bytes.Buffer
	outDir := filepath.Join(t.TempDir(), "dump")
	resourcesDir := filepath.Join(outDir, "resources")
	if err := os.MkdirAll(resourcesDir, 0o777); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", resourcesDir, err)
	}
	if err := os.Chmod(outDir, 0o777); err != nil {
		t.Fatalf("os.Chmod(%q, 0777) error = %v, want nil", outDir, err)
	}
	if err := os.Chmod(resourcesDir, 0o777); err != nil {
		t.Fatalf("os.Chmod(%q, 0777) error = %v, want nil", resourcesDir, err)
	}
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})

	err := app.Run(context.Background(), []string{"dump", "--out", outDir})
	if err != nil {
		t.Fatalf("App.Run(dump --out) error = %v, want nil", err)
	}
	for _, forbidden := range []string{topLevelPSK, nestedPSK, freeTextPSK, bareFreeTextToken} {
		if strings.Contains(out.String(), forbidden) {
			t.Errorf("App.Run(dump --out) stdout = %q, want no %q", out.String(), forbidden)
		}
		if strings.Contains(errOut.String(), forbidden) {
			t.Errorf("App.Run(dump --out) stderr = %q, want no %q", errOut.String(), forbidden)
		}
	}

	resourcePath := filepath.Join(outDir, "resources", "zia", "locations.json")
	ruleLabelsPath := filepath.Join(outDir, "resources", "zia", "rule-labels.json")
	manifestPath := filepath.Join(outDir, "manifest.json")
	reportPath := filepath.Join(outDir, "redaction_report.json")
	resourceBody := readFile(t, resourcePath)
	ruleLabelsBody := readFile(t, ruleLabelsPath)
	manifestBody := readFile(t, manifestPath)
	reportBody := readFile(t, reportPath)
	for _, body := range []string{resourceBody, ruleLabelsBody, manifestBody, reportBody} {
		for _, forbidden := range []string{topLevelPSK, nestedPSK, freeTextPSK, bareFreeTextToken} {
			if strings.Contains(body, forbidden) {
				t.Errorf("dump file body = %q, want no %q", body, forbidden)
			}
		}
	}
	if !strings.Contains(resourceBody, "<REDACTED:SECRET>") {
		t.Errorf("resource dump body = %q, want typed redaction marker", resourceBody)
	}
	if !strings.Contains(manifestBody, "sanitized dumps remain confidential operational data") {
		t.Errorf("manifest body = %q, want confidentiality warning", manifestBody)
	}
	for _, forbidden := range []string{"<REDACTED:", "top-level", "nested", "free-text"} {
		if strings.Contains(reportBody, forbidden) {
			t.Errorf("redaction report = %q, want no redacted snippets or markers containing %q", reportBody, forbidden)
		}
	}
	for _, want := range []string{"description", "preSharedKey", "vpnCredentials", "newSdkField"} {
		if !strings.Contains(reportBody, want) {
			t.Errorf("redaction report = %q, want field name %q", reportBody, want)
		}
	}
	assertFileMode(t, resourcePath, 0o600)
	assertFileMode(t, ruleLabelsPath, 0o600)
	assertFileMode(t, manifestPath, 0o600)
	assertFileMode(t, reportPath, 0o600)
	assertFileMode(t, outDir, 0o700)
	assertFileMode(t, filepath.Join(outDir, "resources"), 0o700)
	assertFileMode(t, filepath.Join(outDir, "resources", "zia"), 0o700)
}

func TestDumpUsesSingleReaderSessionForAllZIAResources(t *testing.T) {
	t.Parallel()

	session := &countingResourceSession{
		list: []resources.SourceRecord{resources.NewSourceRecord(map[string]any{
			"id":               "123",
			"name":             "HQ",
			"description":      "",
			"comments":         "",
			"comment":          "",
			"groupType":        "STATIC_GROUP",
			"ipAddresses":      []any{"192.0.2.10"},
			"lastModTime":      1712345678,
			"lastModifiedTime": 1712345678,
		})},
	}
	reader := &sessionProviderResourceReader{session: session}
	var out, errOut bytes.Buffer
	outDir := filepath.Join(t.TempDir(), "dump")
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})

	err := app.Run(context.Background(), []string{"dump", "--products", "zia", "--out", outDir})
	if err != nil {
		t.Fatalf("App.Run(dump --products zia --out) error = %v, want nil", err)
	}
	if reader.sessionCalls != 1 {
		t.Errorf("sessionProviderResourceReader.Session calls = %d, want 1", reader.sessionCalls)
	}
	if reader.directListCalls != 0 {
		t.Errorf("sessionProviderResourceReader.List calls = %d, want 0", reader.directListCalls)
	}
	if got, want := session.listCalls, len(ziaListResourceSpecs(t)); got != want {
		t.Errorf("countingResourceSession.List calls = %d, want %d", got, want)
	}
	if session.closeCalls != 1 {
		t.Errorf("countingResourceSession.Close calls = %d, want 1", session.closeCalls)
	}
	if !strings.Contains(out.String(), "dump written: "+outDir) {
		t.Errorf("App.Run(dump --products zia --out) stdout = %q, want dump written line", out.String())
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(dump --products zia --out) stderr = %q, want empty", errOut.String())
	}
}

func TestDumpResourceFilterWritesOnlySelectedResources(t *testing.T) {
	t.Parallel()

	reader := fakeResourceReader{
		list: []resources.SourceRecord{resources.NewSourceRecord(map[string]any{
			"id":          "123",
			"name":        "HQ",
			"description": "",
		})},
	}
	var out, errOut bytes.Buffer
	outDir := filepath.Join(t.TempDir(), "dump")
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})

	err := app.Run(context.Background(), []string{
		"dump",
		"--products", "zia",
		"--resources", "locations,rule-labels",
		"--out", outDir,
	})
	if err != nil {
		t.Fatalf("App.Run(dump --resources locations,rule-labels) error = %v, want nil", err)
	}

	for _, name := range []string{"locations", "rule-labels"} {
		path := filepath.Join(outDir, "resources", "zia", name+".json")
		if _, err := os.Stat(path); err != nil {
			t.Errorf("os.Stat(%q) error = %v, want nil", path, err)
		}
	}
	for _, spec := range ziaListResourceSpecs(t) {
		if spec.Name == "locations" || spec.Name == "rule-labels" {
			continue
		}
		path := filepath.Join(outDir, "resources", string(spec.Product), spec.Name+".json")
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("os.Stat(%q) error = %v, want os.ErrNotExist", path, err)
		}
	}

	var manifest struct {
		Resources []struct {
			Product string `json:"product"`
			Name    string `json:"name"`
		} `json:"resources"`
	}
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(outDir, "manifest.json"))), &manifest); err != nil {
		t.Fatalf("json.Unmarshal(filtered dump manifest) error = %v, want nil", err)
	}
	if got, want := len(manifest.Resources), 2; got != want {
		t.Fatalf("filtered dump manifest resources length = %d, want %d", got, want)
	}
	gotNames := map[string]bool{}
	for _, resource := range manifest.Resources {
		gotNames[resource.Product+"/"+resource.Name] = true
	}
	for _, want := range []string{"zia/locations", "zia/rule-labels"} {
		if !gotNames[want] {
			t.Errorf("filtered dump manifest resources = %#v, want %s", gotNames, want)
		}
	}
	if !strings.Contains(out.String(), "dump written: "+outDir) {
		t.Errorf("App.Run(dump --resources) stdout = %q, want dump written line", out.String())
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(dump --resources) stderr = %q, want empty", errOut.String())
	}
}

func TestDumpResourceFilterSupportsQualifiedResourceName(t *testing.T) {
	t.Parallel()

	reader := fakeResourceReader{
		list: []resources.SourceRecord{resources.NewSourceRecord(map[string]any{
			"id":   "123",
			"name": "HQ",
		})},
	}
	var out, errOut bytes.Buffer
	outDir := filepath.Join(t.TempDir(), "dump")
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})

	err := app.Run(context.Background(), []string{"dump", "--resources", "zia/locations", "--out", outDir})
	if err != nil {
		t.Fatalf("App.Run(dump --resources zia/locations) error = %v, want nil", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "resources", "zia", "locations.json")); err != nil {
		t.Errorf("os.Stat(filtered locations dump) error = %v, want nil", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "resources", "zia", "rule-labels.json")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("os.Stat(filtered rule-labels dump) error = %v, want os.ErrNotExist", err)
	}
	if out.Len() == 0 {
		t.Errorf("App.Run(dump --resources zia/locations) stdout = empty, want dump written line")
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(dump --resources zia/locations) stderr = %q, want empty", errOut.String())
	}
}

func TestDumpResourceFilterRejectsUnknownResourceBeforeReader(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	outDir := filepath.Join(t.TempDir(), "dump")
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: failingResourceReader{}})

	err := app.Run(context.Background(), []string{"dump", "--resources", "not-a-resource", "--out", outDir})
	if !errors.Is(err, cli.ErrUsage) {
		t.Fatalf("App.Run(dump --resources not-a-resource) error = %v, want ErrUsage", err)
	}
	if _, statErr := os.Stat(outDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("os.Stat(%q) error = %v, want os.ErrNotExist", outDir, statErr)
	}
	if out.Len() != 0 {
		t.Errorf("App.Run(dump --resources not-a-resource) stdout = %q, want empty", out.String())
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(dump --resources not-a-resource) stderr = %q, want empty", errOut.String())
	}
}

func TestDumpResourceFilterRejectsResourceOutsideSelectedProductsBeforeReader(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	outDir := filepath.Join(t.TempDir(), "dump")
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: failingResourceReader{}})

	err := app.Run(context.Background(), []string{"dump", "--products", "zpa", "--resources", "locations", "--out", outDir})
	if !errors.Is(err, cli.ErrUsage) {
		t.Fatalf("App.Run(dump --products zpa --resources locations) error = %v, want ErrUsage", err)
	}
	if _, statErr := os.Stat(outDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("os.Stat(%q) error = %v, want os.ErrNotExist", outDir, statErr)
	}
	if out.Len() != 0 {
		t.Errorf("App.Run(dump --products zpa --resources locations) stdout = %q, want empty", out.String())
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(dump --products zpa --resources locations) stderr = %q, want empty", errOut.String())
	}
}

func TestDumpAbortsWithoutWritingOnResourceErrorByDefault(t *testing.T) {
	t.Parallel()

	const leakedErrorText = "client_secret=raw-error-value"
	reader := selectiveErrorResourceReader{
		list: []resources.SourceRecord{resources.NewSourceRecord(map[string]any{
			"id":          "123",
			"name":        "HQ",
			"description": "",
		})},
		failures: map[string]error{
			"zia/rule-labels": errors.New(leakedErrorText),
		},
	}
	var out, errOut bytes.Buffer
	outDir := filepath.Join(t.TempDir(), "dump")
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})

	err := app.Run(context.Background(), []string{
		"dump",
		"--products", "zia",
		"--resources", "locations,rule-labels",
		"--out", outDir,
	})
	if err == nil {
		t.Fatal("App.Run(dump resource error default) error = nil, want non-nil")
	}
	if _, statErr := os.Stat(outDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("os.Stat(%q) error = %v, want os.ErrNotExist", outDir, statErr)
	}
	if out.Len() != 0 {
		t.Errorf("App.Run(dump resource error default) stdout = %q, want empty", out.String())
	}
	if strings.Contains(err.Error(), leakedErrorText) || strings.Contains(err.Error(), "client_secret") {
		t.Errorf("App.Run(dump resource error default) error = %q, want no raw error text", err.Error())
	}
	if strings.Contains(errOut.String(), leakedErrorText) {
		t.Errorf("App.Run(dump resource error default) stderr = %q, want no raw error text", errOut.String())
	}
}

func TestDumpContinueOnErrorWritesPartialManifestAndValueFreeErrors(t *testing.T) {
	t.Parallel()

	const leakedErrorText = "client_secret=raw-error-value"
	reader := selectiveErrorResourceReader{
		list: []resources.SourceRecord{resources.NewSourceRecord(map[string]any{
			"id":          "123",
			"name":        "HQ",
			"description": "",
		})},
		failures: map[string]error{
			"zia/rule-labels": errors.New(leakedErrorText),
		},
	}
	var out, errOut bytes.Buffer
	outDir := filepath.Join(t.TempDir(), "dump")
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})

	err := app.Run(context.Background(), []string{
		"dump",
		"--products", "zia",
		"--resources", "locations,rule-labels",
		"--continue-on-error",
		"--out", outDir,
	})
	if err != nil {
		t.Fatalf("App.Run(dump --continue-on-error) error = %v, want nil", err)
	}
	if !strings.Contains(out.String(), "partial dump written: "+outDir) {
		t.Errorf("App.Run(dump --continue-on-error) stdout = %q, want partial dump written line", out.String())
	}
	if strings.Contains(out.String(), leakedErrorText) {
		t.Errorf("App.Run(dump --continue-on-error) stdout = %q, want no raw error text", out.String())
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(dump --continue-on-error) stderr = %q, want empty", errOut.String())
	}

	locationPath := filepath.Join(outDir, "resources", "zia", "locations.json")
	ruleLabelsPath := filepath.Join(outDir, "resources", "zia", "rule-labels.json")
	errorsPath := filepath.Join(outDir, "errors.ndjson")
	manifestPath := filepath.Join(outDir, "manifest.json")
	reportPath := filepath.Join(outDir, "redaction_report.json")
	if _, err := os.Stat(locationPath); err != nil {
		t.Errorf("os.Stat(%q) error = %v, want nil", locationPath, err)
	}
	if _, err := os.Stat(ruleLabelsPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("os.Stat(%q) error = %v, want os.ErrNotExist", ruleLabelsPath, err)
	}
	assertFileMode(t, errorsPath, 0o600)
	assertFileMode(t, manifestPath, 0o600)
	assertFileMode(t, reportPath, 0o600)

	manifestBody := readFile(t, manifestPath)
	errorsBody := readFile(t, errorsPath)
	for _, body := range []string{manifestBody, errorsBody} {
		if strings.Contains(body, leakedErrorText) || strings.Contains(body, "client_secret") {
			t.Errorf("partial dump metadata body = %q, want no raw error text", body)
		}
	}

	var manifest struct {
		Status     string `json:"status"`
		Errors     int    `json:"errors"`
		ErrorsPath string `json:"errors_path"`
		Resources  []struct {
			Product   string `json:"product"`
			Name      string `json:"name"`
			Status    string `json:"status"`
			Path      string `json:"path"`
			Records   int    `json:"records"`
			Operation string `json:"operation"`
			ErrorKind string `json:"error_kind"`
		} `json:"resources"`
	}
	if err := json.Unmarshal([]byte(manifestBody), &manifest); err != nil {
		t.Fatalf("json.Unmarshal(partial manifest) error = %v, want nil", err)
	}
	if manifest.Status != "partial" {
		t.Errorf("partial manifest status = %q, want partial", manifest.Status)
	}
	if manifest.Errors != 1 {
		t.Errorf("partial manifest errors = %d, want 1", manifest.Errors)
	}
	if manifest.ErrorsPath != "errors.ndjson" {
		t.Errorf("partial manifest errors_path = %q, want errors.ndjson", manifest.ErrorsPath)
	}
	resourcesByName := map[string]struct {
		Status    string
		Path      string
		Records   int
		Operation string
		ErrorKind string
	}{}
	for _, resource := range manifest.Resources {
		resourcesByName[resource.Product+"/"+resource.Name] = struct {
			Status    string
			Path      string
			Records   int
			Operation string
			ErrorKind string
		}{
			Status:    resource.Status,
			Path:      resource.Path,
			Records:   resource.Records,
			Operation: resource.Operation,
			ErrorKind: resource.ErrorKind,
		}
	}
	if got := resourcesByName["zia/locations"]; got.Status != "complete" || got.Path != "resources/zia/locations.json" || got.Records != 1 {
		t.Errorf("partial manifest zia/locations = %#v, want complete resource entry", got)
	}
	if got := resourcesByName["zia/rule-labels"]; got.Status != "error" || got.Operation != "list" || got.ErrorKind != "list_failed" || got.Path != "" {
		t.Errorf("partial manifest zia/rule-labels = %#v, want value-free error entry", got)
	}

	var errorRecord struct {
		Schema    string `json:"schema"`
		Product   string `json:"product"`
		Name      string `json:"name"`
		Operation string `json:"operation"`
		Kind      string `json:"kind"`
	}
	lines := strings.Split(strings.TrimSpace(errorsBody), "\n")
	if got, want := len(lines), 1; got != want {
		t.Fatalf("errors.ndjson lines = %d, want %d; body=%q", got, want, errorsBody)
	}
	if err := json.Unmarshal([]byte(lines[0]), &errorRecord); err != nil {
		t.Fatalf("json.Unmarshal(errors.ndjson line) error = %v, want nil", err)
	}
	if errorRecord.Schema != "zscalerctl.dump.error.v1" ||
		errorRecord.Product != "zia" ||
		errorRecord.Name != "rule-labels" ||
		errorRecord.Operation != "list" ||
		errorRecord.Kind != "list_failed" {
		t.Errorf("errors.ndjson record = %#v, want value-free list failure", errorRecord)
	}
}

func TestDumpContinueOnErrorTreatsContextCancellationAsFatal(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	reader := cancelingResourceReader{cancel: cancel}
	var out, errOut bytes.Buffer
	outDir := filepath.Join(t.TempDir(), "dump")
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})

	err := app.Run(ctx, []string{
		"dump",
		"--products", "zia",
		"--resources", "locations",
		"--continue-on-error",
		"--out", outDir,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("App.Run(dump cancelled --continue-on-error) error = %v, want context.Canceled", err)
	}
	if _, statErr := os.Stat(outDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("os.Stat(%q) error = %v, want os.ErrNotExist", outDir, statErr)
	}
	if out.Len() != 0 {
		t.Errorf("App.Run(dump cancelled --continue-on-error) stdout = %q, want empty", out.String())
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(dump cancelled --continue-on-error) stderr = %q, want empty", errOut.String())
	}
}

func TestDumpContinueOnErrorTreatsSessionFailureAsFatal(t *testing.T) {
	t.Parallel()

	reader := &sessionErrorResourceReader{err: zscaler.ErrMissingCredentials}
	var out, errOut bytes.Buffer
	outDir := filepath.Join(t.TempDir(), "dump")
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})

	err := app.Run(context.Background(), []string{
		"dump",
		"--products", "zia",
		"--resources", "locations",
		"--continue-on-error",
		"--out", outDir,
	})
	if !errors.Is(err, zscaler.ErrMissingCredentials) {
		t.Fatalf("App.Run(dump session failure --continue-on-error) error = %v, want ErrMissingCredentials", err)
	}
	if _, statErr := os.Stat(outDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("os.Stat(%q) error = %v, want os.ErrNotExist", outDir, statErr)
	}
	if reader.sessionCalls != 1 {
		t.Errorf("sessionErrorResourceReader.Session calls = %d, want 1", reader.sessionCalls)
	}
	if reader.directListCalls != 0 {
		t.Errorf("sessionErrorResourceReader.List calls = %d, want 0", reader.directListCalls)
	}
	if out.Len() != 0 {
		t.Errorf("App.Run(dump session failure --continue-on-error) stdout = %q, want empty", out.String())
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(dump session failure --continue-on-error) stderr = %q, want empty", errOut.String())
	}
}

func TestDumpFallsBackWhenReaderDoesNotSupportProductSession(t *testing.T) {
	t.Parallel()

	reader := &unsupportedSessionResourceReader{
		list: []resources.SourceRecord{resources.NewSourceRecord(map[string]any{
			"id":   "123",
			"name": "HQ",
		})},
	}
	var out, errOut bytes.Buffer
	outDir := filepath.Join(t.TempDir(), "dump")
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})

	err := app.Run(context.Background(), []string{"dump", "--products", "zia", "--out", outDir})
	if err != nil {
		t.Fatalf("App.Run(dump with unsupported session --out) error = %v, want nil", err)
	}
	if reader.sessionCalls != 1 {
		t.Errorf("unsupportedSessionResourceReader.Session calls = %d, want 1", reader.sessionCalls)
	}
	if got, want := reader.directListCalls, len(ziaListResourceSpecs(t)); got != want {
		t.Errorf("unsupportedSessionResourceReader.List calls = %d, want %d", got, want)
	}
	if !strings.Contains(out.String(), "dump written: "+outDir) {
		t.Errorf("App.Run(dump with unsupported session --out) stdout = %q, want dump written line", out.String())
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(dump with unsupported session --out) stderr = %q, want empty", errOut.String())
	}
}

func TestDumpClosesReaderSessionOnListError(t *testing.T) {
	t.Parallel()

	session := &countingResourceSession{err: errors.New("session list failed")}
	reader := &sessionProviderResourceReader{session: session}
	var out, errOut bytes.Buffer
	outDir := filepath.Join(t.TempDir(), "dump")
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})

	err := app.Run(context.Background(), []string{"dump", "--products", "zia", "--out", outDir})
	if err == nil {
		t.Fatal("App.Run(dump with session list error) error = nil, want non-nil")
	}
	if reader.sessionCalls != 1 {
		t.Errorf("sessionProviderResourceReader.Session calls = %d, want 1", reader.sessionCalls)
	}
	if session.closeCalls != 1 {
		t.Errorf("countingResourceSession.Close calls = %d, want 1", session.closeCalls)
	}
	if reader.directListCalls != 0 {
		t.Errorf("sessionProviderResourceReader.List calls = %d, want 0", reader.directListCalls)
	}
}

func TestDumpRejectsNilReaderSession(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	outDir := filepath.Join(t.TempDir(), "dump")
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: nilSessionResourceReader{}})

	err := app.Run(context.Background(), []string{"dump", "--products", "zia", "--out", outDir})
	if err == nil {
		t.Fatal("App.Run(dump with nil session) error = nil, want non-nil")
	}
	if out.Len() != 0 {
		t.Errorf("App.Run(dump with nil session) stdout = %q, want empty", out.String())
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(dump with nil session) stderr = %q, want empty", errOut.String())
	}
}

func TestDumpWithNoSelectedResourcesDoesNotOpenReader(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	outDir := filepath.Join(t.TempDir(), "dump")
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: failingResourceReader{}})

	err := app.Run(context.Background(), []string{"dump", "--products", "zpa", "--out", outDir})
	if err != nil {
		t.Fatalf("App.Run(dump --products zpa --out) error = %v, want nil", err)
	}
	if !strings.Contains(out.String(), "dump written: "+outDir) {
		t.Errorf("App.Run(dump --products zpa --out) stdout = %q, want dump written line", out.String())
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(dump --products zpa --out) stderr = %q, want empty", errOut.String())
	}
}

func TestDumpRefusesOverwriteBeforeWritingNewFiles(t *testing.T) {
	t.Parallel()

	reader := fakeResourceReader{
		list: []resources.SourceRecord{resources.NewSourceRecord(map[string]any{
			"id":   "123",
			"name": "HQ",
		})},
	}
	var out, errOut bytes.Buffer
	outDir := filepath.Join(t.TempDir(), "dump")
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", outDir, err)
	}
	if err := os.Chmod(outDir, 0o777); err != nil {
		t.Fatalf("os.Chmod(%q, 0777) error = %v, want nil", outDir, err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "manifest.json"), []byte("existing"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(manifest) error = %v, want nil", err)
	}
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})

	err := app.Run(context.Background(), []string{"dump", "--out", outDir})
	if !errors.Is(err, dump.ErrUnsafeOverwrite) {
		t.Fatalf("App.Run(dump overwrite) error = %v, want ErrUnsafeOverwrite", err)
	}
	resourcePath := filepath.Join(outDir, "resources", "zia", "locations.json")
	if _, err := os.Stat(resourcePath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("os.Stat(%q) error = %v, want os.ErrNotExist", resourcePath, err)
	}
	ruleLabelsPath := filepath.Join(outDir, "resources", "zia", "rule-labels.json")
	if _, err := os.Stat(ruleLabelsPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("os.Stat(%q) error = %v, want os.ErrNotExist", ruleLabelsPath, err)
	}
	if out.Len() != 0 {
		t.Errorf("App.Run(dump overwrite) stdout = %q, want empty", out.String())
	}
	assertFileMode(t, outDir, 0o777)
}

type fakeResourceReader struct {
	list []resources.SourceRecord
	get  resources.SourceRecord
}

func (f fakeResourceReader) List(context.Context, resources.Product, string) ([]resources.SourceRecord, error) {
	return f.list, nil
}

func (f fakeResourceReader) Get(context.Context, resources.Product, string, string) (resources.SourceRecord, error) {
	return f.get, nil
}

type selectiveErrorResourceReader struct {
	list     []resources.SourceRecord
	failures map[string]error
}

func (f selectiveErrorResourceReader) List(_ context.Context, product resources.Product, name string) ([]resources.SourceRecord, error) {
	if err := f.failures[string(product)+"/"+name]; err != nil {
		return nil, err
	}
	return f.list, nil
}

func (f selectiveErrorResourceReader) Get(context.Context, resources.Product, string, string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, errors.New("get must not be called")
}

type cancelingResourceReader struct {
	cancel context.CancelFunc
}

func (f cancelingResourceReader) List(context.Context, resources.Product, string) ([]resources.SourceRecord, error) {
	f.cancel()
	return nil, context.Canceled
}

func (f cancelingResourceReader) Get(context.Context, resources.Product, string, string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, errors.New("get must not be called")
}

type sessionProviderResourceReader struct {
	session         zscaler.ResourceSession
	sessionCalls    int
	directListCalls int
}

func (f *sessionProviderResourceReader) Session(context.Context, resources.Product) (zscaler.ResourceSession, error) {
	f.sessionCalls++
	return f.session, nil
}

func (f *sessionProviderResourceReader) List(context.Context, resources.Product, string) ([]resources.SourceRecord, error) {
	f.directListCalls++
	return nil, errors.New("direct list must not be called")
}

func (f *sessionProviderResourceReader) Get(context.Context, resources.Product, string, string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, errors.New("direct get must not be called")
}

type sessionErrorResourceReader struct {
	err             error
	sessionCalls    int
	directListCalls int
}

func (f *sessionErrorResourceReader) Session(context.Context, resources.Product) (zscaler.ResourceSession, error) {
	f.sessionCalls++
	return nil, f.err
}

func (f *sessionErrorResourceReader) List(context.Context, resources.Product, string) ([]resources.SourceRecord, error) {
	f.directListCalls++
	return nil, errors.New("direct list must not be called")
}

func (f *sessionErrorResourceReader) Get(context.Context, resources.Product, string, string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, errors.New("direct get must not be called")
}

type unsupportedSessionResourceReader struct {
	list            []resources.SourceRecord
	sessionCalls    int
	directListCalls int
}

func (f *unsupportedSessionResourceReader) Session(_ context.Context, product resources.Product) (zscaler.ResourceSession, error) {
	f.sessionCalls++
	return nil, fmt.Errorf("%w: %s/session", zscaler.ErrUnsupportedResource, product)
}

func (f *unsupportedSessionResourceReader) List(context.Context, resources.Product, string) ([]resources.SourceRecord, error) {
	f.directListCalls++
	return f.list, nil
}

func (f *unsupportedSessionResourceReader) Get(context.Context, resources.Product, string, string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, errors.New("direct get must not be called")
}

type countingResourceSession struct {
	list       []resources.SourceRecord
	err        error
	listCalls  int
	closeCalls int
}

func (s *countingResourceSession) List(context.Context, resources.Product, string) ([]resources.SourceRecord, error) {
	s.listCalls++
	if s.err != nil {
		return nil, s.err
	}
	return s.list, nil
}

func (s *countingResourceSession) Get(context.Context, resources.Product, string, string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, errors.New("session get must not be called")
}

func (s *countingResourceSession) Close() {
	s.closeCalls++
}

type nilSessionResourceReader struct{}

func (nilSessionResourceReader) Session(context.Context, resources.Product) (zscaler.ResourceSession, error) {
	return nil, nil
}

func (nilSessionResourceReader) List(context.Context, resources.Product, string) ([]resources.SourceRecord, error) {
	return nil, errors.New("direct list must not be called")
}

func (nilSessionResourceReader) Get(context.Context, resources.Product, string, string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, errors.New("direct get must not be called")
}

type failingResourceReader struct{}

func (failingResourceReader) List(context.Context, resources.Product, string) ([]resources.SourceRecord, error) {
	return nil, errors.New("reader must not be called")
}

func (failingResourceReader) Get(context.Context, resources.Product, string, string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, errors.New("reader must not be called")
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", path, err)
	}
	return string(body)
}

func assertFileMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v, want nil", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("os.Stat(%q).Mode().Perm() = %04o, want %04o", path, got, want)
	}
}

func ziaListResourceSpecs(t *testing.T) []resources.ResourceSpec {
	t.Helper()

	var specs []resources.ResourceSpec
	for _, spec := range resources.Catalog() {
		if spec.Product != resources.ProductZIA {
			continue
		}
		if hasReadListOperation(spec) {
			specs = append(specs, spec)
		}
	}
	if len(specs) == 0 {
		t.Fatal("resources.Catalog() ZIA list resources = 0, want at least 1")
	}
	return specs
}

func hasReadListOperation(spec resources.ResourceSpec) bool {
	for _, op := range spec.Operations {
		if op.Name == "list" && op.Capability == resources.CapabilityRead {
			return true
		}
	}
	return false
}
