package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/cli"
	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/dump"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/dvmrry/zscalerctl/internal/zscaler"
)

func TestConfigShowDoesNotExposeEnvironmentSecrets(t *testing.T) {
	t.Parallel()

	const clientID = "client-id-value"
	const clientSecret = "client-secret-value"
	const zpaCustomerID = "zpa-customer-id-value"
	const zpaMicrotenantID = "zpa-microtenant-id-value"
	const proxyURL = "http://proxy-user:proxy-secret@proxy.example.invalid:8080"
	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, []string{
		config.EnvClientID + "=" + clientID,
		config.EnvClientSecret + "=" + clientSecret,
		config.EnvZPACustomerID + "=" + zpaCustomerID,
		config.EnvZPAMicrotenantID + "=" + zpaMicrotenantID,
		config.EnvProxyURL + "=" + proxyURL,
	})

	err := app.Run(context.Background(), []string{"--format", "json", "config", "show"})
	if err != nil {
		t.Fatalf("App.Run(config show) error = %v, want nil", err)
	}
	got := out.String()
	for _, forbidden := range []string{clientID, clientSecret, zpaCustomerID, zpaMicrotenantID, "proxy-user", "proxy-secret", "proxy.example.invalid"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("App.Run(config show) output = %q, want no %q", got, forbidden)
		}
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(config show) stderr = %q, want empty", errOut.String())
	}
}

func TestConfigShowWithProfileSecretRefDoesNotResolve(t *testing.T) {
	t.Parallel()

	configPath := writeCLIConfig(t, `
default_profile: prod
profiles:
  prod:
    vanity_domain: example
    client_id: client-id
    client_secret_ref: env:PROFILE_SECRET_MUST_NOT_BE_READ
`)
	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, nil)

	err := app.Run(context.Background(), []string{"--config", configPath, "--format", "json", "config", "show"})
	if err != nil {
		t.Fatalf("App.Run(config show profile ref) error = %v, want nil", err)
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(config show profile ref) stderr = %q, want empty", errOut.String())
	}
	var safe config.SafeConfig
	if err := json.Unmarshal(out.Bytes(), &safe); err != nil {
		t.Fatalf("json.Unmarshal(config show) error = %v; body = %q", err, out.String())
	}
	if safe.Source != "config" || !safe.ConfigFileSet || safe.Profile != "prod" {
		t.Fatalf("SafeConfig source/profile = %+v, want config file prod", safe)
	}
	if !safe.Credentials.ClientSecretSet || safe.Credentials.ClientSecretScheme != "env" {
		t.Fatalf("SafeConfig credentials = %+v, want env secret source", safe.Credentials)
	}
	if strings.Contains(out.String(), "PROFILE_SECRET_MUST_NOT_BE_READ") {
		t.Errorf("App.Run(config show) stdout = %q, want no raw secret ref name", out.String())
	}
}

func TestStatusCommandsWithProfileCmdRefDoNotResolve(t *testing.T) {
	t.Parallel()

	sentinel := filepath.Join(t.TempDir(), "provider-ran")
	configPath := writeCLIConfig(t, fmt.Sprintf(`
profiles:
  default:
    vanity_domain: example
    client_id: client-id
    client_secret_ref:
      cmd:
        argv: [%q, "-test.run=^TestCLIConfigCmdHelperProcess$", "--", "touch", %q]
`, os.Args[0], sentinel))

	cases := []struct {
		name string
		args []string
	}{
		{name: "config show", args: []string{"--config", configPath, "--format", "json", "config", "show"}},
		{name: "doctor", args: []string{"--config", configPath, "--format", "json", "doctor"}},
		{name: "auth status", args: []string{"--config", configPath, "--format", "json", "auth", "status"}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			app := cli.New(&out, &errOut, nil)
			if err := app.Run(context.Background(), tc.args); err != nil {
				t.Fatalf("App.Run(%s) error = %v, want nil", tc.name, err)
			}
			if errOut.Len() != 0 {
				t.Errorf("App.Run(%s) stderr = %q, want empty", tc.name, errOut.String())
			}
			if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
				t.Fatalf("App.Run(%s) resolved cmd ref and created %q; stat err = %v", tc.name, sentinel, err)
			}
			if strings.Contains(out.String(), sentinel) || strings.Contains(out.String(), "TestCLIConfigCmdHelperProcess") {
				t.Fatalf("App.Run(%s) output = %q, want no raw cmd ref", tc.name, out.String())
			}
		})
	}

	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, nil)
	if err := app.Run(context.Background(), []string{"--config", configPath, "--format", "json", "config", "show"}); err != nil {
		t.Fatalf("App.Run(config show) error = %v, want nil", err)
	}
	var safe config.SafeConfig
	if err := json.Unmarshal(out.Bytes(), &safe); err != nil {
		t.Fatalf("json.Unmarshal(config show) error = %v; body = %q", err, out.String())
	}
	if safe.Credentials.ClientSecretScheme != "cmd" {
		t.Fatalf("SafeConfig client_secret_scheme = %q, want cmd", safe.Credentials.ClientSecretScheme)
	}
}

func TestDoctorDoesNotExposeEnvironmentSecrets(t *testing.T) {
	t.Parallel()

	const clientID = "client-id-value"
	const clientSecret = "client-secret-value"
	const zpaCustomerID = "zpa-customer-id-value"
	const zpaMicrotenantID = "zpa-microtenant-id-value"
	const proxyURL = "http://proxy-user:proxy-secret@proxy.example.invalid:8080"
	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, []string{
		config.EnvClientID + "=" + clientID,
		config.EnvClientSecret + "=" + clientSecret,
		config.EnvZPACustomerID + "=" + zpaCustomerID,
		config.EnvZPAMicrotenantID + "=" + zpaMicrotenantID,
		config.EnvProxyURL + "=" + proxyURL,
	})

	err := app.Run(context.Background(), []string{"doctor"})
	if err != nil {
		t.Fatalf("App.Run(doctor) error = %v, want nil", err)
	}
	for _, forbidden := range []string{clientID, clientSecret, zpaCustomerID, zpaMicrotenantID, "proxy-user", "proxy-secret", "proxy.example.invalid"} {
		if strings.Contains(out.String(), forbidden) {
			t.Errorf("App.Run(doctor) output = %q, want no %q", out.String(), forbidden)
		}
	}
}

func TestDoctorSupportsJSONOutput(t *testing.T) {
	t.Parallel()

	const (
		clientID     = "client-id-value"
		clientSecret = "client-secret-value"
		proxyURL     = "http://proxy-user:proxy-secret@proxy.example.invalid:8080"
	)
	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, []string{
		config.EnvClientID + "=" + clientID,
		config.EnvClientSecret + "=" + clientSecret,
		config.EnvVanityDomain + "=example",
		config.EnvProxyURL + "=" + proxyURL,
	})

	err := app.Run(context.Background(), []string{"doctor", "--format", "json"})
	if err != nil {
		t.Fatalf("App.Run(doctor --format json) error = %v, want nil", err)
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(doctor --format json) stderr = %q, want empty", errOut.String())
	}
	var got struct {
		Status      string `json:"status"`
		Mode        string `json:"mode"`
		AuthMode    string `json:"auth_mode"`
		Credentials string `json:"credentials"`
		LiveAPI     string `json:"live_api"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal(App.Run(doctor --format json) stdout) error = %v; body = %q", err, out.String())
	}
	if got.Status != "OK" || got.Mode != "read-only" || got.AuthMode == "" {
		t.Errorf("App.Run(doctor --format json) decoded = %#v, want status/mode/auth mode", got)
	}
	if got.Credentials != "configured" {
		t.Errorf("App.Run(doctor --format json) credentials = %q, want configured", got.Credentials)
	}
	if !strings.Contains(got.LiveAPI, "available") {
		t.Errorf("App.Run(doctor --format json) live_api = %q, want available status", got.LiveAPI)
	}
	for _, forbidden := range []string{clientID, clientSecret, "proxy-user", "proxy-secret", "proxy.example.invalid"} {
		if strings.Contains(out.String(), forbidden) {
			t.Errorf("App.Run(doctor --format json) stdout = %q, want no %q", out.String(), forbidden)
		}
	}
}

func TestAuthStatusDoesNotExposeEnvironmentSecrets(t *testing.T) {
	t.Parallel()

	const clientID = "client-id-value"
	const clientSecret = "client-secret-value"
	const zpaCustomerID = "zpa-customer-id-value"
	const zpaMicrotenantID = "zpa-microtenant-id-value"
	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, []string{
		config.EnvClientID + "=" + clientID,
		config.EnvClientSecret + "=" + clientSecret,
		config.EnvZPACustomerID + "=" + zpaCustomerID,
		config.EnvZPAMicrotenantID + "=" + zpaMicrotenantID,
	})

	err := app.Run(context.Background(), []string{"auth", "status"})
	if err != nil {
		t.Fatalf("App.Run(auth status) error = %v, want nil", err)
	}
	for _, forbidden := range []string{clientID, clientSecret, zpaCustomerID, zpaMicrotenantID} {
		if strings.Contains(out.String(), forbidden) {
			t.Errorf("App.Run(auth status) output = %q, want no %q", out.String(), forbidden)
		}
	}
}

func TestAuthStatusSupportsJSONOutput(t *testing.T) {
	t.Parallel()

	const (
		clientID     = "client-id-value"
		clientSecret = "client-secret-value"
	)
	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, []string{
		config.EnvClientID + "=" + clientID,
		config.EnvClientSecret + "=" + clientSecret,
		config.EnvVanityDomain + "=example",
	})

	err := app.Run(context.Background(), []string{"auth", "status", "--format", "json"})
	if err != nil {
		t.Fatalf("App.Run(auth status --format json) error = %v, want nil", err)
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(auth status --format json) stderr = %q, want empty", errOut.String())
	}
	var got struct {
		Credentials        string `json:"credentials"`
		CredentialExchange string `json:"credential_exchange"`
		LiveAPI            string `json:"live_api"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("json.Unmarshal(App.Run(auth status --format json) stdout) error = %v; body = %q", err, out.String())
	}
	if got.Credentials != "configured" {
		t.Errorf("App.Run(auth status --format json) credentials = %q, want configured", got.Credentials)
	}
	if got.CredentialExchange != "not requested" {
		t.Errorf("App.Run(auth status --format json) credential_exchange = %q, want not requested", got.CredentialExchange)
	}
	if !strings.Contains(got.LiveAPI, "available") {
		t.Errorf("App.Run(auth status --format json) live_api = %q, want available status", got.LiveAPI)
	}
	for _, forbidden := range []string{clientID, clientSecret} {
		if strings.Contains(out.String(), forbidden) {
			t.Errorf("App.Run(auth status --format json) stdout = %q, want no %q", out.String(), forbidden)
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

	err := app.Run(context.Background(), []string{"--format", "table", "doctor"})
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

	err := app.Run(context.Background(), []string{"--color", "always", "--format", "table", "doctor"})
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

	err := app.Run(context.Background(), []string{"--color", "always", "--no-color", "--format", "table", "doctor"})
	if err != nil {
		t.Fatalf("App.Run(--color always --no-color doctor) error = %v, want nil", err)
	}
	if strings.Contains(out.String(), "\x1b[") {
		t.Errorf("App.Run(--color always --no-color doctor) output = %q, want no ANSI escapes", out.String())
	}
}

func TestGlobalFlagsMayFollowCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "format after command args",
			args: []string{"schema", "list", "--format", "json"},
		},
		{
			name: "format between command args",
			args: []string{"schema", "--format", "json", "list"},
		},
		{
			name: "format before command",
			args: []string{"--format", "json", "schema", "list"},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var out, errOut bytes.Buffer
			app := cli.New(&out, &errOut, nil)

			err := app.Run(context.Background(), tt.args)
			if err != nil {
				t.Fatalf("App.Run(%v) error = %v, want nil", tt.args, err)
			}
			if !json.Valid(out.Bytes()) {
				t.Fatalf("App.Run(%v) stdout = %q, want valid JSON", tt.args, out.String())
			}
			if errOut.Len() != 0 {
				t.Errorf("App.Run(%v) stderr = %q, want empty", tt.args, errOut.String())
			}
		})
	}
}

func TestHelpFlagsReturnUsage(t *testing.T) {
	t.Parallel()

	tests := [][]string{
		{"--help"},
		{"-h"},
		// "schema list --help" is now a Cobra command; its help uses Cobra format ("Usage:\n  zscalerctl schema list")
		// and is covered by TestPerCommandHelpPrintsScopedSynopsis instead.
		// "dump --help" is now a Cobra command; its help uses Cobra format ("Usage:\n  zscalerctl dump")
		// and is covered by TestPerCommandHelpPrintsScopedSynopsis instead.
	}
	for _, args := range tests {
		args := args
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			t.Parallel()

			var out, errOut bytes.Buffer
			app := cli.New(&out, &errOut, []string{
				config.EnvClientSecretFile + "=/path/that/must/not/be/read",
			})

			err := app.Run(context.Background(), args)
			if err != nil {
				t.Fatalf("App.Run(%v) error = %v, want nil", args, err)
			}
			if !strings.Contains(out.String(), "usage: zscalerctl") {
				t.Errorf("App.Run(%v) stdout = %q, want usage text", args, out.String())
			}
			if errOut.Len() != 0 {
				t.Errorf("App.Run(%v) stderr = %q, want empty", args, errOut.String())
			}
		})
	}
}

func TestPerCommandHelpPrintsScopedSynopsis(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		args            []string
		want            string
		skipGlobalCheck bool // set for Cobra commands whose flag descriptions may contain "products:"/"commands:"
	}{
		// doctor is now a Cobra command; --help renders Cobra-formatted help
		// ("Usage:\n  zscalerctl doctor") rather than the legacy scoped synopsis.
		// Re-blessed as an intentional surface change in the Cobra migration (Task 1.5.2).
		{name: "doctor", args: []string{"doctor", "--help"}, want: "zscalerctl doctor"},
		// auth is now a Cobra command; --help renders Cobra-formatted help
		// ("Usage:\n  zscalerctl auth") rather than the legacy scoped synopsis.
		// Re-blessed as an intentional surface change in the Cobra migration (Phase 4).
		{name: "auth", args: []string{"auth", "--help"}, want: "zscalerctl auth", skipGlobalCheck: true},
		// config is now a Cobra command; --help renders Cobra-formatted help
		// ("Usage:\n  zscalerctl config") rather than the legacy scoped synopsis.
		// Re-blessed as an intentional surface change in the Cobra migration (Phase 4).
		{name: "config", args: []string{"config", "--help"}, want: "zscalerctl config", skipGlobalCheck: true},
		// schema list is now a Cobra command; --help renders Cobra-formatted help
		// ("Usage:\n  zscalerctl schema list") rather than the legacy scoped synopsis.
		// Re-blessed as an intentional surface change in the Cobra migration (Phase 4).
		{name: "schema", args: []string{"schema", "list", "--help"}, want: "zscalerctl schema list"},
		// dump is now a Cobra command; --help renders Cobra-formatted help
		// ("Usage:\n  zscalerctl dump") rather than the legacy scoped synopsis.
		// skipGlobalCheck is true because the --products flag description contains
		// "products:" which would otherwise trigger the false-positive assertion.
		// Re-blessed as an intentional surface change in the Cobra migration (Phase 3a).
		{name: "dump", args: []string{"dump", "--help"}, want: "zscalerctl dump", skipGlobalCheck: true},
		// diff is now a Cobra command; --help renders Cobra-formatted help
		// ("Usage:\n  zscalerctl diff") rather than the legacy scoped synopsis.
		// skipGlobalCheck is true because the --products flag description contains
		// "products:" which would otherwise trigger the false-positive assertion.
		// Re-blessed as an intentional surface change in the Cobra migration (Phase 3b).
		{name: "diff", args: []string{"diff", "--help"}, want: "zscalerctl diff", skipGlobalCheck: true},
		// completion is now a Cobra command group; --help renders Cobra-formatted help
		// ("Usage:\n  zscalerctl completion [command]") rather than the legacy synopsis.
		// Re-blessed as an intentional surface change in the Cobra migration (Phase 5b).
		{name: "completion", args: []string{"completion", "--help"}, want: "zscalerctl completion", skipGlobalCheck: true},
		// version is now a Cobra command; --help renders Cobra-formatted help
		// ("Usage:\n  zscalerctl version") rather than the legacy scoped synopsis.
		// Re-blessed as an intentional surface change in the Cobra migration (Task 1.5).
		{name: "version", args: []string{"version", "--help"}, want: "zscalerctl version"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var out, errOut bytes.Buffer
			app := cli.New(&out, &errOut, []string{
				config.EnvClientSecretFile + "=/path/that/must/not-be-read",
			})

			err := app.Run(context.Background(), tt.args)
			if err != nil {
				t.Fatalf("App.Run(%v) error = %v, want nil", tt.args, err)
			}
			got := out.String()
			if !strings.Contains(got, tt.want) {
				t.Errorf("App.Run(%v) stdout = %q, want %q", tt.args, got, tt.want)
			}
			if !tt.skipGlobalCheck {
				for _, globalOnly := range []string{"commands:", "products:"} {
					if strings.Contains(got, globalOnly) {
						t.Errorf("App.Run(%v) stdout = %q, want scoped help without global section %q", tt.args, got, globalOnly)
					}
				}
			}
			if errOut.Len() != 0 {
				t.Errorf("App.Run(%v) stderr = %q, want empty", tt.args, errOut.String())
			}
		})
	}
}

func TestUsageListsKnownProducts(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, nil)

	err := app.Run(context.Background(), []string{"help"})
	if err != nil {
		t.Fatalf("App.Run(help) error = %v, want nil", err)
	}
	for _, want := range []string{
		"Available Commands",
		"zia",
		"zpa",
		"ztw",
		"zcc",
		"zidentity",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("App.Run(help) stdout = %q, want %q", out.String(), want)
		}
	}
}

func TestGlobalOutputWritesSuccessfulCommandToOwnerOnlyFile(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Unix file-mode perms; Windows enforces owner-only via DACL, covered by internal/fileperm")
	}

	const clientSecret = "client-secret-value"
	var out, errOut bytes.Buffer
	outPath := filepath.Join(t.TempDir(), "config.json")
	app := cli.New(&out, &errOut, []string{
		config.EnvClientID + "=client-id-value",
		config.EnvClientSecret + "=" + clientSecret,
	})

	err := app.Run(context.Background(), []string{"config", "show", "--format", "json", "--output", outPath})
	if err != nil {
		t.Fatalf("App.Run(config show --output) error = %v, want nil", err)
	}
	if out.Len() != 0 {
		t.Errorf("App.Run(config show --output) stdout = %q, want empty", out.String())
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(config show --output) stderr = %q, want empty", errOut.String())
	}
	assertFileMode(t, outPath, 0o600)
	body := readFile(t, outPath)
	if !json.Valid([]byte(body)) {
		t.Fatalf("output file body = %q, want valid JSON", body)
	}
	if strings.Contains(body, clientSecret) {
		t.Errorf("output file body = %q, want no raw client secret", body)
	}
}

func TestGlobalOutputOverwritesAtomicallyWithoutTempLeftovers(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Unix file-mode perms; Windows enforces owner-only via DACL, covered by internal/fileperm")
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "config.json")
	// Write twice to the same path: the second run must overwrite cleanly (the
	// atomic rename replaces the file), proving --output stays re-runnable.
	for i := 0; i < 2; i++ {
		var out, errOut bytes.Buffer
		app := cli.New(&out, &errOut, []string{
			config.EnvClientID + "=client-id-value",
			config.EnvClientSecret + "=client-secret-value",
		})
		if err := app.Run(context.Background(), []string{"config", "show", "--format", "json", "--output", outPath}); err != nil {
			t.Fatalf("run %d: App.Run(config show --output) error = %v, want nil", i, err)
		}
	}
	assertFileMode(t, outPath, 0o600)
	if body := readFile(t, outPath); !json.Valid([]byte(body)) {
		t.Fatalf("output file body = %q, want valid JSON", body)
	}
	// The temp file must be renamed into place, never left behind on success.
	leftovers, err := filepath.Glob(filepath.Join(dir, ".tmp-*"))
	if err != nil {
		t.Fatalf("glob temp files error = %v", err)
	}
	if len(leftovers) != 0 {
		t.Errorf("temp leftovers = %v, want none", leftovers)
	}
}

func TestGlobalOutputTreatsDestinationAsNonTTYForColorAuto(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	outPath := filepath.Join(t.TempDir(), "doctor.txt")
	app := cli.NewWithOptions(&out, &errOut, []string{"TERM=xterm-256color"}, cli.Options{StdoutTTY: true})

	err := app.Run(context.Background(), []string{"doctor", "--output", outPath})
	if err != nil {
		t.Fatalf("App.Run(doctor --output) error = %v, want nil", err)
	}
	if out.Len() != 0 {
		t.Errorf("App.Run(doctor --output) stdout = %q, want empty", out.String())
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(doctor --output) stderr = %q, want empty", errOut.String())
	}
	body := readFile(t, outPath)
	if strings.Contains(body, "\x1b[") {
		t.Errorf("output file body = %q, want no ANSI escapes", body)
	}
}

func TestGlobalOutputDoesNotCreateFileOnError(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	outPath := filepath.Join(t.TempDir(), "doctor.txt")
	app := cli.New(&out, &errOut, nil)

	err := app.Run(context.Background(), []string{"doctor", "--timeout", "0s", "--output", outPath})
	if !errors.Is(err, cli.ErrUsage) {
		t.Fatalf("App.Run(doctor --output with usage error) error = %v, want ErrUsage", err)
	}
	if _, statErr := os.Stat(outPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("os.Stat(%q) error = %v, want os.ErrNotExist", outPath, statErr)
	}
	if out.Len() != 0 {
		t.Errorf("App.Run(doctor --output with usage error) stdout = %q, want empty", out.String())
	}
}

func TestGlobalOutputRejectsDump(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	tempDir := t.TempDir()
	outPath := filepath.Join(tempDir, "dump-status.txt")
	dumpDir := filepath.Join(tempDir, "dump")
	app := cli.New(&out, &errOut, nil)

	err := app.Run(context.Background(), []string{"dump", "--output", outPath, "--out", dumpDir})
	if !errors.Is(err, cli.ErrUsage) {
		t.Fatalf("App.Run(dump --output --out) error = %v, want ErrUsage", err)
	}
	if !strings.Contains(err.Error(), "--output cannot be used with dump") {
		t.Errorf("App.Run(dump --output --out) error = %q, want --output dump guidance", err.Error())
	}
	if _, statErr := os.Stat(outPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("os.Stat(%q) error = %v, want os.ErrNotExist", outPath, statErr)
	}
	if _, statErr := os.Stat(dumpDir); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("os.Stat(%q) error = %v, want os.ErrNotExist", dumpDir, statErr)
	}
	if out.Len() != 0 {
		t.Errorf("App.Run(dump --output --out) stdout = %q, want empty", out.String())
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(dump --output --out) stderr = %q, want empty", errOut.String())
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

			err := app.Run(context.Background(), []string{"--format", "table", "doctor"})
			if err != nil {
				t.Fatalf("App.Run(doctor) error = %v, want nil", err)
			}
			if !strings.Contains(out.String(), "Redaction") || !strings.Contains(out.String(), mode) {
				t.Errorf("App.Run(doctor) output = %q, want redaction mode %q", out.String(), mode)
			}
		})
	}
}

// TestCompletionScriptsDoNotReadCredentialFilesOrUseReader asserts that both
// the generated shell scripts (which are static shell code) and the runtime
// completion protocol (__complete) never read credentials or construct a reader.
//
// Part 1: generated scripts — run "completion <shell>" with a failing reader and
// credential env set, then check the output is a static shell script with no
// credential values embedded.
//
// Part 2: runtime completion — run "__complete zia ”" with a failing reader and
// no credentials; assert catalog resource names come back and no reader is invoked.
// This is the critical security proof: shell completion NEVER calls LoadConfig.
func TestCompletionScriptsDoNotReadCredentialFilesOrUseReader(t *testing.T) {
	t.Parallel()

	const clientID = "client-id-value"

	// Part 1: generated scripts must not embed credential values.
	for _, shell := range []string{"bash", "zsh", "fish", "powershell"} {
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
			// Cobra-generated scripts are static shell code that calls back to
			// __complete at completion-time; they must contain "zscalerctl" but
			// must NOT embed any credential values.
			if !strings.Contains(out.String(), "zscalerctl") {
				t.Errorf("App.Run(completion %s) stdout = %q, want zscalerctl", shell, out.String())
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

	// Part 2: __complete protocol must never construct a reader or load config.
	// Run with a reader that panics if constructed, and no credentials.
	t.Run("__complete_catalog_only", func(t *testing.T) {
		t.Parallel()

		var out, errOut bytes.Buffer
		app := cli.NewWithOptions(&out, &errOut, []string{
			config.EnvClientID + "=" + clientID,
			config.EnvClientSecretFile + "=/path/that/must/not/be/read",
		}, cli.Options{Reader: failingResourceReader{}})

		// __complete zia '' should return catalog resource names without any API call.
		err := app.Run(context.Background(), []string{"__complete", "zia", ""})
		if err != nil {
			t.Fatalf("App.Run(__complete zia '') error = %v, want nil", err)
		}
		// Cobra completion output is "name\n" or "name\tdesc\n" then ":directive\n".
		// Verify catalog resource names appear in the completion output.
		for _, want := range []string{"locations", "url-lookup"} {
			if !strings.Contains(out.String(), want) {
				t.Errorf("App.Run(__complete zia '') stdout = %q, want %q", out.String(), want)
			}
		}
		for _, forbidden := range []string{clientID, "ZSCALERCTL_CLIENT_SECRET_FILE"} {
			if strings.Contains(out.String(), forbidden) {
				t.Errorf("App.Run(__complete zia '') stdout = %q, want no %q", out.String(), forbidden)
			}
		}
		// Note: Cobra prints "Completion ended with directive: ..." to stderr
		// during __complete; this is expected and intentionally not checked here.
	})
}

// TestCompletionScriptsReflectCatalogProducts asserts that the Cobra completion
// tree offers each product that is actually in the catalog, and does NOT offer
// products that are absent.
//
// Cobra-generated scripts are dynamic (they call __complete at runtime), so we
// test via the __complete protocol rather than scanning the static script text.
// "__complete ”" lists all top-level subcommands; products in the catalog appear
// because execCobra adds them via newProductCmd.
func TestCompletionScriptsReflectCatalogProducts(t *testing.T) {
	t.Parallel()

	products := catalogProductsForTest()

	var out bytes.Buffer
	app := cli.New(&out, io.Discard, nil)

	// Request root-level completions: lists all top-level commands including products.
	err := app.Run(context.Background(), []string{"__complete", ""})
	if err != nil {
		t.Fatalf("App.Run(__complete '') error = %v, want nil", err)
	}
	got := out.String()
	for _, product := range []string{"zia", "zpa", "ztw", "zcc", "zidentity"} {
		if products[product] && !strings.Contains(got, product) {
			t.Errorf("App.Run(__complete '') stdout = %q, want product %q (in catalog)", got, product)
		}
		if !products[product] && strings.Contains(got, product) {
			t.Errorf("App.Run(__complete '') stdout = %q, want no product %q (not in catalog)", got, product)
		}
	}
	// Note: Cobra prints "Completion ended with directive: ..." to stderr; expected.
}

// TestCompletionScriptsUseAuthStatus asserts that completing after "auth" offers
// "status" and not "show". With Cobra-generated scripts, product commands are
// registered as Cobra subcommands, so "auth status" is a real subcommand and
// the __complete protocol returns it correctly.
func TestCompletionScriptsUseAuthStatus(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	app := cli.New(&out, io.Discard, nil)

	// __complete auth '' → should offer "status" (the auth subcommand).
	// The "show" verb belongs to "config show" / "schema show", not auth.
	// Cobra outputs "status\t<desc>\n:directive" — check the first word of each line.
	err := app.Run(context.Background(), []string{"__complete", "auth", ""})
	if err != nil {
		t.Fatalf("App.Run(__complete auth '') error = %v, want nil", err)
	}
	got := out.String()
	// Verify "status" appears as a completion candidate (first field before tab or newline).
	if !strings.Contains(got, "status") {
		t.Errorf("App.Run(__complete auth '') stdout = %q, want 'status'", got)
	}
	// Verify no "show" subcommand name appears as a leading word on its own line
	// (the description may say "show" but the subcommand name must not be "show").
	for _, line := range strings.Split(got, "\n") {
		name, _, _ := strings.Cut(line, "\t")
		if name == "show" {
			t.Errorf("App.Run(__complete auth '') stdout = %q, want no 'show' subcommand", got)
		}
	}
	// Note: Cobra prints "Completion ended with directive: ..." to stderr; expected.
}

// TestBashCompletionRegistersCommand asserts that the Cobra-generated bash
// completion script registers the completion function for "zscalerctl".
// Cobra's bash V2 script uses "__start_zscalerctl" and registers it with
// "complete -o default -F __start_zscalerctl zscalerctl".
func TestBashCompletionRegistersCommand(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, nil)

	err := app.Run(context.Background(), []string{"completion", "bash"})
	if err != nil {
		t.Fatalf("App.Run(completion bash) error = %v, want nil", err)
	}
	got := out.String()
	// Cobra registers using "__start_zscalerctl" as the entry point.
	if !strings.Contains(got, "__start_zscalerctl") {
		t.Errorf("App.Run(completion bash) stdout = %q, want __start_zscalerctl registration", got)
	}
	if !strings.Contains(got, "zscalerctl") {
		t.Errorf("App.Run(completion bash) stdout = %q, want zscalerctl", got)
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(completion bash) stderr = %q, want empty", errOut.String())
	}
}

// TestPowerShellCompletionRegistersCommand asserts that the Cobra-generated
// PowerShell completion script registers the completion scriptblock for
// "zscalerctl". Cobra uses Register-ArgumentCompleter (without -Native).
func TestPowerShellCompletionRegistersCommand(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, nil)

	err := app.Run(context.Background(), []string{"completion", "powershell"})
	if err != nil {
		t.Fatalf("App.Run(completion powershell) error = %v, want nil", err)
	}
	got := out.String()
	// Cobra's PowerShell script uses Register-ArgumentCompleter (without -Native).
	if !strings.Contains(got, "Register-ArgumentCompleter") {
		t.Errorf("App.Run(completion powershell) stdout = %q, want Register-ArgumentCompleter", got)
	}
	if !strings.Contains(got, "zscalerctl") {
		t.Errorf("App.Run(completion powershell) stdout = %q, want zscalerctl", got)
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(completion powershell) stderr = %q, want empty", errOut.String())
	}
}

// TestPowerShellCompletionParsesUnderRealPwsh runs the Cobra-generated PowerShell
// completion through an actual PowerShell parser, so a syntax regression fails
// the build — the string-level completion tests above cannot catch that. It
// runs wherever `pwsh` is on PATH; in CI (GitHub ubuntu runners ship
// PowerShell) a missing pwsh is a hard failure, so the smoke can never silently
// stop running.
func TestPowerShellCompletionParsesUnderRealPwsh(t *testing.T) {
	t.Parallel()

	pwsh, err := exec.LookPath("pwsh")
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatal("pwsh not found, but the PowerShell completion parse smoke is required in CI")
		}
		t.Skip("pwsh not installed; skipping PowerShell completion parse smoke")
	}

	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, nil)
	if err := app.Run(context.Background(), []string{"completion", "powershell"}); err != nil {
		t.Fatalf("App.Run(completion powershell) error = %v, want nil", err)
	}

	script := filepath.Join(t.TempDir(), "completion.ps1")
	if err := os.WriteFile(script, out.Bytes(), 0o600); err != nil {
		t.Fatalf("write completion script: %v", err)
	}

	// Parse-only (no execution): tokenize the file and fail on any ParseError.
	parse := fmt.Sprintf(`$tokens = $null; $errs = $null
[void][System.Management.Automation.Language.Parser]::ParseFile('%s', [ref]$tokens, [ref]$errs)
if ($errs -and $errs.Count -gt 0) { $errs | ForEach-Object { [Console]::Error.WriteLine($_.ToString()) }; exit 1 }`, script)

	var pwshErr bytes.Buffer
	cmd := exec.Command(pwsh, "-NoProfile", "-NonInteractive", "-Command", parse)
	cmd.Stderr = &pwshErr
	if err := cmd.Run(); err != nil {
		t.Fatalf("Cobra-generated PowerShell completion failed to parse under pwsh: %v\n%s", err, pwshErr.String())
	}
}

// TestCompletionRejectsUnknownShell asserts that "completion elvish" does not
// produce a shell completion script. With Cobra's built-in completion command,
// unknown shells show the completion group help (exit 0) rather than an error,
// but crucially they must NOT emit a script body that a shell could source.
//
// Surface delta: this is an intentional behavior change from the legacy
// implementation. Legacy runCompletion returned UsageError (exit 2) for any
// unknown shell name; Cobra's built-in completion command routes unknown
// subcommands to the completion group help (exit 0). See
// cmd/zscalerctl/testdata/surface/surface_changes.md:
// "completion <unknown-shell> (e.g. elvish)".
func TestCompletionRejectsUnknownShell(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, nil)

	// Documented surface delta: Cobra shows the completion group help (exit 0)
	// for unknown shell names. Legacy returned UsageError (exit 2).
	// See surface_changes.md: "completion <unknown-shell> (e.g. elvish)".
	err := app.Run(context.Background(), []string{"completion", "elvish"})
	if err != nil {
		t.Fatalf("App.Run(completion elvish) error = %v, want nil (Cobra shows group help, exit 0)", err)
	}

	got := out.String()

	// Positive assertion: the completion group help must list the supported
	// shells. If these are absent the command silently produced nothing useful.
	for _, shellName := range []string{"bash", "fish", "powershell", "zsh"} {
		if !strings.Contains(got, shellName) {
			t.Errorf("App.Run(completion elvish) stdout missing %q — expected completion group help listing supported shells; got:\n%s", shellName, got)
		}
	}

	// Negative assertion: no generated completion script must be present.
	// A real script would contain shell-specific registration markers.
	for _, scriptMarker := range []string{
		"__start_zscalerctl",         // bash V2 registration function
		"#compdef zscalerctl",        // zsh
		"complete -c zscalerctl",     // fish
		"Register-ArgumentCompleter", // powershell
	} {
		if strings.Contains(got, scriptMarker) {
			t.Errorf("App.Run(completion elvish) stdout contains script marker %q — no completion script should be emitted; got:\n%s", scriptMarker, got)
		}
	}

	// Stderr must be empty: Cobra group help goes to stdout.
	if errOut.Len() != 0 {
		t.Errorf("App.Run(completion elvish) stderr = %q, want empty", errOut.String())
	}
}

// TestCompletionScriptBashNotRedacted guards the §5a spec requirement: the
// redactor must NEVER be applied to completion script stdout. If the redactor
// were applied, its high-entropy heuristic would corrupt shell variable
// assignments such as "local shellCompDirectiveFilterFileExt=8" (treating "=8"
// as a high-entropy secret) and mangle URLs in comments, silently breaking
// file-extension filtering in the generated bash script.
//
// The guard checks two things:
//  1. No "<REDACTED" substring appears anywhere in the bash completion script.
//  2. The specific variable assignment that was historically corrupted is intact.
func TestCompletionScriptBashNotRedacted(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	app := cli.New(&out, io.Discard, nil)

	if err := app.Run(context.Background(), []string{"completion", "bash"}); err != nil {
		t.Fatalf("App.Run(completion bash) error = %v, want nil", err)
	}
	got := out.String()

	// Guard 1: no redactor tokens anywhere in the script.
	if strings.Contains(got, "<REDACTED") {
		t.Errorf("completion bash stdout contains <REDACTED — redactor must not be applied to completion output:\n%s", got)
	}

	// Guard 2: the specific variable assignment that the redactor false-positives on.
	const needle = "local shellCompDirectiveFilterFileExt=8"
	if !strings.Contains(got, needle) {
		t.Errorf("completion bash stdout does not contain %q — script may be corrupted or Cobra changed its generator:\n%s", needle, got)
	}
}

func TestSchemaListTableIncludesReadOperations(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, nil)

	err := app.Run(context.Background(), []string{"--format", "table", "schema", "list"})
	if err != nil {
		t.Fatalf("App.Run(schema list) error = %v, want nil", err)
	}
	for _, want := range []string{"zia\tlocations\tlist,get", "zia\tadvanced-settings\tshow"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("App.Run(schema list) stdout = %q, want %q", out.String(), want)
		}
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(schema list) stderr = %q, want empty", errOut.String())
	}
}

func TestSchemaListJSONIncludesGetKeyForGetResources(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, nil)

	err := app.Run(context.Background(), []string{"--format", "json", "schema", "list"})
	if err != nil {
		t.Fatalf("App.Run(schema list json) error = %v, want nil", err)
	}
	var specs []struct {
		Product string `json:"product"`
		Name    string `json:"name"`
		GetKey  string `json:"get_key,omitempty"`
	}
	if err := json.Unmarshal(out.Bytes(), &specs); err != nil {
		t.Fatalf("json.Unmarshal(schema list) error = %v, want nil", err)
	}
	seen := map[string]string{}
	for _, spec := range specs {
		seen[spec.Product+"/"+spec.Name] = spec.GetKey
	}
	if got := seen["zia/locations"]; got != "id" {
		t.Errorf("schema zia/locations get_key = %q, want id", got)
	}
	if got := seen["zia/advanced-settings"]; got != "" {
		t.Errorf("schema zia/advanced-settings get_key = %q, want omitted/empty", got)
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(schema list json) stderr = %q, want empty", errOut.String())
	}
}

// noopReader is a ResourceReader stub whose methods return empty results and
// nil errors. It is injected into an App with an empty catalog to prove the
// schema-list empty-catalog branch never reaches the reader.
type noopReader struct{}

func (noopReader) List(_ context.Context, _ resources.Product, _ string) ([]resources.SourceRecord, error) {
	return nil, nil
}
func (noopReader) Get(_ context.Context, _ resources.Product, _ string, _ string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, nil
}
func (noopReader) Show(_ context.Context, _ resources.Product, _ string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, nil
}

// TestSchemaListEmptyCatalog covers the empty-catalog branch of runSchema, a
// known coverage gap: an explicit empty (non-nil) ResourceCatalog is injected
// via NewWithOptions so resourceCatalog() returns zero entries.
//
//   - --format table: the sentinel "no resources enabled yet" text is emitted.
//   - --format json: an empty array "[]" is emitted.
//
// Neither path must error or touch the reader.
func TestSchemaListEmptyCatalog(t *testing.T) {
	t.Parallel()

	t.Run("table", func(t *testing.T) {
		t.Parallel()
		var out, errOut bytes.Buffer
		app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{
			Catalog: resources.ResourceCatalog{},
			Reader:  noopReader{},
		})
		if err := app.Run(context.Background(), []string{"--format", "table", "schema", "list"}); err != nil {
			t.Fatalf("App.Run(schema list table) empty-catalog error = %v, want nil", err)
		}
		if got := out.String(); !strings.Contains(got, "no resources enabled yet") {
			t.Errorf("schema list table empty-catalog stdout = %q, want sentinel \"no resources enabled yet\"", got)
		}
		if errOut.Len() != 0 {
			t.Errorf("schema list table empty-catalog stderr = %q, want empty", errOut.String())
		}
	})

	t.Run("json", func(t *testing.T) {
		t.Parallel()
		var out, errOut bytes.Buffer
		app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{
			Catalog: resources.ResourceCatalog{},
			Reader:  noopReader{},
		})
		if err := app.Run(context.Background(), []string{"--format", "json", "schema", "list"}); err != nil {
			t.Fatalf("App.Run(schema list json) empty-catalog error = %v, want nil", err)
		}
		var specs []json.RawMessage
		if err := json.Unmarshal(out.Bytes(), &specs); err != nil {
			t.Fatalf("json.Unmarshal(schema list json) error = %v\noutput: %s", err, out.String())
		}
		if len(specs) != 0 {
			t.Errorf("schema list json empty-catalog: got %d entries, want 0 (output: %s)", len(specs), out.String())
		}
		// The renderer must emit a literal empty array, not "null".
		if got := strings.TrimSpace(out.String()); got != "[]" {
			t.Errorf("schema list json empty-catalog stdout = %q, want \"[]\"", got)
		}
		if errOut.Len() != 0 {
			t.Errorf("schema list json empty-catalog stderr = %q, want empty", errOut.String())
		}
	})
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
	for _, want := range []string{"Available Commands", "completion"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("App.Run(help) stdout = %q, want %q", out.String(), want)
		}
	}
	forbidden := "/path/that/must/not-be-read"
	if strings.Contains(out.String(), forbidden) {
		t.Errorf("App.Run(help) stdout = %q, must not contain %q", out.String(), forbidden)
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

// TestResourceListSupportsNDJSON asserts --format ndjson emits one compact,
// redacted JSON record per line for a list, each independently parseable and in
// source order.
func TestResourceListSupportsNDJSON(t *testing.T) {
	t.Parallel()

	const psk = "ndjson-psk-canary"
	reader := fakeResourceReader{
		list: []resources.SourceRecord{
			resources.NewSourceRecord(map[string]any{"id": "1", "name": "HQ", "preSharedKey": psk}),
			resources.NewSourceRecord(map[string]any{"id": "2", "name": "Branch"}),
		},
	}
	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})
	if err := app.Run(context.Background(), []string{"--format", "ndjson", "zia", "locations", "list"}); err != nil {
		t.Fatalf("App.Run(--format ndjson list) error = %v, want nil", err)
	}
	if strings.Contains(out.String(), psk) {
		t.Errorf("ndjson output leaked secret %q: %q", psk, out.String())
	}
	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("ndjson list produced %d lines, want 2: %q", len(lines), out.String())
	}
	var names []string
	for _, ln := range lines {
		if strings.HasPrefix(ln, " ") {
			t.Errorf("ndjson line is indented, want compact: %q", ln)
		}
		var rec map[string]any
		if err := json.Unmarshal([]byte(ln), &rec); err != nil {
			t.Fatalf("ndjson line is not valid JSON: %q: %v", ln, err)
		}
		if _, ok := rec["preSharedKey"]; ok {
			t.Errorf("ndjson record retained preSharedKey: %#v", rec)
		}
		if name, ok := rec["name"].(string); ok {
			names = append(names, name)
		}
	}
	if len(names) != 2 || names[0] != "HQ" || names[1] != "Branch" {
		t.Errorf("ndjson record names = %v, want [HQ Branch] in source order", names)
	}
}

func TestResourceListWarnsUnknownFilterKeyButKeepsStdoutClean(t *testing.T) {
	t.Parallel()

	reader := fakeResourceReader{
		list: []resources.SourceRecord{
			resources.NewSourceRecord(map[string]any{"id": "1", "name": "HQ"}),
		},
	}
	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{
		Reader:  reader,
		Catalog: filterWarningCatalog(),
	})

	err := app.Run(context.Background(), []string{"--format", "json", "zia", "locations", "list", "--filter", "naem=HQ"})
	if err != nil {
		t.Fatalf("App.Run(list --filter unknown) error = %v, want nil", err)
	}
	if got := strings.TrimSpace(out.String()); got != "[]" {
		t.Fatalf("App.Run(list --filter unknown) stdout = %q, want []", out.String())
	}
	if strings.Contains(out.String(), "warning:") {
		t.Errorf("App.Run(list --filter unknown) stdout = %q, want no warning", out.String())
	}
	wantWarning := `warning: --filter key "naem" is not a field of zia/locations`
	if !strings.Contains(errOut.String(), wantWarning) {
		t.Errorf("App.Run(list --filter unknown) stderr = %q, want %q", errOut.String(), wantWarning)
	}
}

func TestResourceListDoesNotWarnForCatalogFieldDroppedByMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		filter string
	}{
		{name: "secret field", filter: "internalToken=raw-token-value"},
		{name: "standard only field in share", filter: "hostname=host-01"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reader := fakeResourceReader{
				list: []resources.SourceRecord{
					resources.NewSourceRecord(map[string]any{
						"id":            "1",
						"name":          "HQ",
						"hostname":      "host-01",
						"internalToken": "raw-token-value",
					}),
				},
			}
			var out, errOut bytes.Buffer
			app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{
				Reader:  reader,
				Catalog: filterWarningCatalog(),
			})

			err := app.Run(context.Background(), []string{
				"--format", "json",
				"--redaction", "share",
				"zia", "locations", "list",
				"--filter", tt.filter,
			})
			if err != nil {
				t.Fatalf("App.Run(list --filter catalog field dropped by mode) error = %v, want nil", err)
			}
			if got := strings.TrimSpace(out.String()); got != "[]" {
				t.Fatalf("App.Run(list --filter catalog field dropped by mode) stdout = %q, want []", out.String())
			}
			if errOut.Len() != 0 {
				t.Errorf("App.Run(list --filter catalog field dropped by mode) stderr = %q, want empty", errOut.String())
			}
		})
	}
}

// TestNonRecordCommandsRejectNDJSON asserts every non-record command rejects
// --format ndjson with a clear usage error — NDJSON is for record streams
// (list/get/show) only, so doctor/schema (own format guard) and dump/completion
// (dispatch guard) all refuse it rather than silently ignoring the flag.
func TestNonRecordCommandsRejectNDJSON(t *testing.T) {
	t.Parallel()

	cases := map[string][]string{
		"doctor":     {"doctor", "--format", "ndjson"},
		"schema":     {"--format", "ndjson", "schema", "list"},
		"dump":       {"--format", "ndjson", "dump", "--out", "/tmp/zsc-ndjson-reject"},
		"completion": {"--format", "ndjson", "completion", "bash"},
	}
	for name, args := range cases {
		name, args := name, args
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var out, errOut bytes.Buffer
			app := cli.New(&out, &errOut, []string{
				config.EnvClientID + "=id",
				config.EnvClientSecret + "=secret",
				config.EnvVanityDomain + "=example",
			})
			err := app.Run(context.Background(), args)
			if err == nil {
				t.Fatalf("App.Run(%v) error = nil, want unsupported-format error", args)
			}
			if !strings.Contains(err.Error(), "does not support ndjson") {
				t.Errorf("App.Run(%v) error = %q, want it to mention 'does not support ndjson'", args, err.Error())
			}
			// An unsupported --format is user error, so it must map to the
			// usage exit code (2), not the internal-error code (1).
			if !errors.Is(err, cli.ErrUsage) {
				t.Errorf("App.Run(%v) error = %v, want errors.Is(err, cli.ErrUsage)", args, err)
			}
		})
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

func TestUnsupportedFormatsFailBeforeReader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "list yaml",
			args: []string{"--format", "yaml", "zia", "locations", "list"},
			want: `unsupported output format "yaml"; supported: auto, table, json, ndjson, pretty`,
		},
		{
			name: "get yaml",
			args: []string{"--format", "yaml", "zia", "locations", "get", "123"},
			want: `unsupported output format "yaml"; supported: auto, table, json, ndjson, pretty`,
		},
		{
			name: "show yaml",
			args: []string{"--format", "yaml", "zia", "advanced-settings", "show"},
			want: `unsupported output format "yaml"; supported: auto, table, json, ndjson, pretty`,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var out, errOut bytes.Buffer
			app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: failingResourceReader{}})

			err := app.Run(context.Background(), tt.args)
			if err == nil {
				t.Fatalf("App.Run(%v) error = nil, want usage error", tt.args)
			}
			if !errors.Is(err, cli.ErrUsage) {
				t.Fatalf("App.Run(%v) error = %v, want ErrUsage", tt.args, err)
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

func TestResourceShowProjectsAndRedactsFixture(t *testing.T) {
	t.Parallel()

	reader := fakeResourceReader{
		show: resources.NewSourceRecord(map[string]any{
			"apiSessionTimeout": 30,
			"authBypassUrls":    []any{"admin.internal.example"},
			"ecsObject": map[string]any{
				"token": "raw-token-value",
			},
			"newSdkField": "surprise",
		}),
	}
	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})

	err := app.Run(context.Background(), []string{"--format", "json", "zia", "advanced-settings", "show"})
	if err != nil {
		t.Fatalf("App.Run(zia advanced-settings show) error = %v, want nil", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("json.Unmarshal(resource show output) error = %v, want nil; output=%q", err, out.String())
	}
	if _, ok := decoded["apiSessionTimeout"]; !ok {
		t.Errorf("decoded resource show = %#v, want apiSessionTimeout", decoded)
	}
	for _, forbidden := range []string{"ecsObject", "newSdkField"} {
		if _, ok := decoded[forbidden]; ok {
			t.Errorf("decoded resource show = %#v, want no %s", decoded, forbidden)
		}
	}
	spec, ok := resources.FindSpec(resources.ProductZIA, "advanced-settings")
	if !ok {
		t.Fatal("FindSpec(zia, advanced-settings) ok = false, want true")
	}
	if err := resources.AssertRenderedSubset(spec, "", decoded); err != nil {
		t.Errorf("AssertRenderedSubset(projected show output) error = %v, want nil", err)
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(zia advanced-settings show) stderr = %q, want empty", errOut.String())
	}
}

func TestResourceShowTableRendersVerticalKeyValues(t *testing.T) {
	t.Parallel()

	reader := fakeResourceReader{
		show: resources.NewSourceRecord(map[string]any{
			"apiSessionTimeout": 30,
			"authBypassUrls":    []any{"admin.internal.example"},
			"ecsObject": map[string]any{
				"token": "raw-token-value",
			},
			"newSdkField": "surprise",
		}),
	}
	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})

	err := app.Run(context.Background(), []string{"zia", "advanced-settings", "show"})
	if err != nil {
		t.Fatalf("App.Run(zia advanced-settings show) error = %v, want nil", err)
	}
	got := out.String()
	for _, want := range []string{"apiSessionTimeout", "30", "authBypassUrls", "admin.internal.example"} {
		if !strings.Contains(got, want) {
			t.Errorf("App.Run(zia advanced-settings show) stdout = %q, want %q", got, want)
		}
	}
	for _, forbidden := range []string{"\t", "ecsObject", "newSdkField"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("App.Run(zia advanced-settings show) stdout = %q, want no %q", got, forbidden)
		}
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(zia advanced-settings show) stderr = %q, want empty", errOut.String())
	}
}

func TestProductCommandsRejectUnsupportedOperationBeforeReader(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "list singleton",
			args: []string{"zia", "advanced-settings", "list"},
		},
		{
			name: "show list resource",
			args: []string{"zia", "locations", "show"},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var out, errOut bytes.Buffer
			app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: failingResourceReader{}})

			err := app.Run(context.Background(), tt.args)
			if !errors.Is(err, cli.ErrUsage) {
				t.Fatalf("App.Run(%v) error = %v, want ErrUsage", tt.args, err)
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
	if runtime.GOOS == "windows" {
		t.Skip("Unix file-mode perms; Windows enforces owner-only via DACL, covered by internal/fileperm")
	}

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
	if got, want := session.showCalls, len(ziaShowResourceSpecs(t)); got != want {
		t.Errorf("countingResourceSession.Show calls = %d, want %d", got, want)
	}
	if session.closeCalls != 1 {
		t.Errorf("countingResourceSession.Close calls = %d, want 1", session.closeCalls)
	}
	if !strings.Contains(errOut.String(), "dump written: "+outDir) {
		t.Errorf("App.Run(dump --products zia --out) stderr = %q, want dump written line", errOut.String())
	}
	if out.Len() != 0 {
		t.Errorf("App.Run(dump --products zia --out) stdout = %q, want empty", out.String())
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
	if !strings.Contains(errOut.String(), "dump written: "+outDir) {
		t.Errorf("App.Run(dump --resources) stderr = %q, want dump written line", errOut.String())
	}
	if out.Len() != 0 {
		t.Errorf("App.Run(dump --resources) stdout = %q, want empty", out.String())
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
	if errOut.Len() == 0 {
		t.Errorf("App.Run(dump --resources zia/locations) stderr = empty, want dump written line")
	}
	if out.Len() != 0 {
		t.Errorf("App.Run(dump --resources zia/locations) stdout = %q, want empty", out.String())
	}
}

func TestDumpResourceFilterWritesSingletonAsObject(t *testing.T) {
	t.Parallel()

	reader := fakeResourceReader{
		show: resources.NewSourceRecord(map[string]any{
			"apiSessionTimeout": 30,
			"ecsObject": map[string]any{
				"token": "raw-token-value",
			},
		}),
	}
	var out, errOut bytes.Buffer
	outDir := filepath.Join(t.TempDir(), "dump")
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})

	err := app.Run(context.Background(), []string{"dump", "--resources", "zia/advanced-settings", "--out", outDir})
	if err != nil {
		t.Fatalf("App.Run(dump --resources zia/advanced-settings) error = %v, want nil", err)
	}
	resourcePath := filepath.Join(outDir, "resources", "zia", "advanced-settings.json")
	var resourceBody map[string]any
	if err := json.Unmarshal([]byte(readFile(t, resourcePath)), &resourceBody); err != nil {
		t.Fatalf("json.Unmarshal(singleton dump resource) error = %v, want nil", err)
	}
	if _, ok := resourceBody["apiSessionTimeout"]; !ok {
		t.Errorf("singleton dump resource = %#v, want apiSessionTimeout", resourceBody)
	}
	if _, ok := resourceBody["ecsObject"]; ok {
		t.Errorf("singleton dump resource = %#v, want no ecsObject", resourceBody)
	}
	var manifest struct {
		Resources []struct {
			Product string `json:"product"`
			Name    string `json:"name"`
			Records int    `json:"records"`
		} `json:"resources"`
	}
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(outDir, "manifest.json"))), &manifest); err != nil {
		t.Fatalf("json.Unmarshal(singleton dump manifest) error = %v, want nil", err)
	}
	if len(manifest.Resources) != 1 {
		t.Fatalf("singleton dump manifest resources length = %d, want 1", len(manifest.Resources))
	}
	got := manifest.Resources[0]
	if got.Product != "zia" || got.Name != "advanced-settings" || got.Records != 1 {
		t.Errorf("singleton dump manifest resource = %#v, want zia/advanced-settings records=1", got)
	}
	if !strings.Contains(errOut.String(), "dump written: "+outDir) {
		t.Errorf("App.Run(dump singleton) stderr = %q, want dump written line", errOut.String())
	}
	if out.Len() != 0 {
		t.Errorf("App.Run(dump singleton) stdout = %q, want empty", out.String())
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

func TestResourceGetRejectsUnsupportedOperationBeforeReader(t *testing.T) {
	t.Parallel()

	catalog := resources.ResourceCatalog{{
		Product:    resources.ProductZIA,
		Name:       "list-only",
		Operations: resources.ListOperations(),
		Fields: []resources.FieldSpec{{
			Name:           "id",
			Classification: resources.ClassOperational,
		}},
	}}
	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{
		Reader:  failingResourceReader{},
		Catalog: catalog,
	})

	err := app.Run(context.Background(), []string{"zia", "list-only", "get", "123"})
	if !errors.Is(err, cli.ErrUsage) {
		t.Fatalf("App.Run(zia list-only get 123) error = %v, want ErrUsage", err)
	}
	if !strings.Contains(err.Error(), "unsupported operation get for zia/list-only") {
		t.Errorf("App.Run(zia list-only get 123) error = %q, want unsupported operation message", err.Error())
	}
	if out.Len() != 0 {
		t.Errorf("App.Run(zia list-only get 123) stdout = %q, want empty", out.String())
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(zia list-only get 123) stderr = %q, want empty", errOut.String())
	}
}

func TestDumpWritesSingletonResourceAsOneRecordWithManifestShape(t *testing.T) {
	t.Parallel()

	catalog := resources.ResourceCatalog{{
		Product:    resources.ProductZIA,
		Name:       "singleton-settings",
		Shape:      resources.ShapeSingleton,
		Operations: resources.SingletonOperations(),
		Fields: []resources.FieldSpec{
			{
				Name:           "id",
				Classification: resources.ClassOperational,
				AllowedModes:   []redact.Mode{redact.ModeStandard},
			},
			{
				Name:           "name",
				Classification: resources.ClassTenantConfig,
				AllowedModes:   []redact.Mode{redact.ModeStandard},
			},
		},
	}}
	reader := fakeResourceReader{
		list: []resources.SourceRecord{resources.NewSourceRecord(map[string]any{
			"id":   1,
			"name": "Auth settings",
		})},
	}
	var out, errOut bytes.Buffer
	outDir := filepath.Join(t.TempDir(), "dump")
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{
		Reader:  reader,
		Catalog: catalog,
	})

	err := app.Run(context.Background(), []string{"dump", "--resources", "zia/singleton-settings", "--out", outDir})
	if err != nil {
		t.Fatalf("App.Run(dump singleton-settings) error = %v, want nil", err)
	}
	var manifest struct {
		Resources []struct {
			Product string `json:"product"`
			Name    string `json:"name"`
			Shape   string `json:"shape"`
			Records int    `json:"records"`
		} `json:"resources"`
	}
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(outDir, "manifest.json"))), &manifest); err != nil {
		t.Fatalf("json.Unmarshal(singleton manifest) error = %v, want nil", err)
	}
	if len(manifest.Resources) != 1 {
		t.Fatalf("manifest resources length = %d, want 1", len(manifest.Resources))
	}
	got := manifest.Resources[0]
	if got.Product != "zia" || got.Name != "singleton-settings" || got.Shape != "singleton" || got.Records != 1 {
		t.Errorf("manifest singleton resource = %+v, want zia/singleton-settings shape singleton records 1", got)
	}
	var records []map[string]any
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(outDir, "resources", "zia", "singleton-settings.json"))), &records); err != nil {
		t.Fatalf("json.Unmarshal(singleton resource) error = %v, want nil", err)
	}
	if len(records) != 1 {
		t.Fatalf("singleton-settings records length = %d, want 1", len(records))
	}
	if records[0]["name"] != "Auth settings" {
		t.Errorf("singleton-settings record name = %v, want Auth settings", records[0]["name"])
	}
	if out.Len() != 0 {
		t.Errorf("App.Run(dump singleton-settings) stdout = %q, want empty", out.String())
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
	// The returned error now wraps the underlying cause for operator
	// diagnostics; the display layer (main.writeError) passes it through
	// standard-mode redaction before stderr or the JSON envelope. Assert that
	// redacted form drops the secret value even when a reader error embeds one.
	displayed := redact.New(redact.ModeStandard).String(err.Error())
	if strings.Contains(displayed, "raw-error-value") {
		t.Errorf("App.Run(dump resource error default) redacted error = %q, want secret value redacted", displayed)
	}
	if strings.Contains(errOut.String(), leakedErrorText) {
		t.Errorf("App.Run(dump resource error default) stderr = %q, want no raw error text", errOut.String())
	}
}

func TestDumpContinueOnErrorWritesPartialManifestAndValueFreeErrors(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("Unix file-mode perms; Windows enforces owner-only via DACL, covered by internal/fileperm")
	}

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
	if !errors.Is(err, cli.ErrPartialDump) {
		t.Fatalf("App.Run(dump --continue-on-error) error = %v, want ErrPartialDump", err)
	}
	if !strings.Contains(errOut.String(), "partial dump written: "+outDir) {
		t.Errorf("App.Run(dump --continue-on-error) stderr = %q, want partial dump written line", errOut.String())
	}
	if strings.Contains(errOut.String(), leakedErrorText) || strings.Contains(out.String(), leakedErrorText) {
		t.Errorf("App.Run(dump --continue-on-error) output = %q / %q, want no raw error text", out.String(), errOut.String())
	}
	if out.Len() != 0 {
		t.Errorf("App.Run(dump --continue-on-error) stdout = %q, want empty", out.String())
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
	if got := resourcesByName["zia/locations"]; got.Status != "ok" || got.Path != "resources/zia/locations.json" || got.Records != 1 {
		t.Errorf("partial manifest zia/locations = %#v, want ok resource entry", got)
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
	if got, want := reader.directShowCalls, len(ziaShowResourceSpecs(t)); got != want {
		t.Errorf("unsupportedSessionResourceReader.Show calls = %d, want %d", got, want)
	}
	if !strings.Contains(errOut.String(), "dump written: "+outDir) {
		t.Errorf("App.Run(dump with unsupported session --out) stderr = %q, want dump written line", errOut.String())
	}
	if out.Len() != 0 {
		t.Errorf("App.Run(dump with unsupported session --out) stdout = %q, want empty", out.String())
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

func TestDumpForceReplacesExistingDumpDirectory(t *testing.T) {
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

	err := app.Run(context.Background(), []string{"dump", "--out", outDir, "--resources", "zia/locations"})
	if err != nil {
		t.Fatalf("App.Run(initial dump) error = %v, want nil", err)
	}
	stalePath := filepath.Join(outDir, "stale.txt")
	if err := os.WriteFile(stalePath, []byte("stale"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(stale) error = %v, want nil", err)
	}
	out.Reset()
	errOut.Reset()

	err = app.Run(context.Background(), []string{"dump", "--out", outDir, "--resources", "zia/locations", "--force"})
	if err != nil {
		t.Fatalf("App.Run(dump --force) error = %v, want nil", err)
	}
	if _, err := os.Stat(stalePath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("os.Stat(stale) error = %v, want os.ErrNotExist", err)
	}
	resourcePath := filepath.Join(outDir, "resources", "zia", "locations.json")
	if _, err := os.Stat(resourcePath); err != nil {
		t.Errorf("os.Stat(%q) error = %v, want nil", resourcePath, err)
	}
	if out.Len() != 0 {
		t.Errorf("App.Run(dump --force) stdout = %q, want empty", out.String())
	}
	if !strings.Contains(errOut.String(), "dump written:") {
		t.Errorf("App.Run(dump --force) stderr = %q, want dump written", errOut.String())
	}
}

func TestDumpForceRejectsNonDumpDirectory(t *testing.T) {
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
	notesPath := filepath.Join(outDir, "notes.txt")
	if err := os.WriteFile(notesPath, []byte("not a dump"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(notes) error = %v, want nil", err)
	}
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})

	err := app.Run(context.Background(), []string{"dump", "--out", outDir, "--resources", "zia/locations", "--force"})
	if !errors.Is(err, dump.ErrUnsafePath) {
		t.Fatalf("App.Run(dump --force non-dump) error = %v, want ErrUnsafePath", err)
	}
	if readFile(t, notesPath) != "not a dump" {
		t.Errorf("notes file changed after rejected --force")
	}
	resourcePath := filepath.Join(outDir, "resources", "zia", "locations.json")
	if _, err := os.Stat(resourcePath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("os.Stat(%q) error = %v, want os.ErrNotExist", resourcePath, err)
	}
	if out.Len() != 0 {
		t.Errorf("App.Run(dump --force non-dump) stdout = %q, want empty", out.String())
	}
}

func TestDumpForceDoesNotRemovePreviousDumpWhenCollectionFails(t *testing.T) {
	t.Parallel()

	goodReader := fakeResourceReader{
		list: []resources.SourceRecord{resources.NewSourceRecord(map[string]any{
			"id":   "123",
			"name": "HQ",
		})},
	}
	var out, errOut bytes.Buffer
	outDir := filepath.Join(t.TempDir(), "dump")
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: goodReader})
	err := app.Run(context.Background(), []string{"dump", "--out", outDir, "--resources", "zia/locations"})
	if err != nil {
		t.Fatalf("App.Run(initial dump) error = %v, want nil", err)
	}
	manifestBefore := readFile(t, filepath.Join(outDir, "manifest.json"))
	out.Reset()
	errOut.Reset()

	failingReader := selectiveErrorResourceReader{
		failures: map[string]error{"zia/locations": errors.New("boom")},
	}
	app = cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: failingReader})
	err = app.Run(context.Background(), []string{"dump", "--out", outDir, "--resources", "zia/locations", "--force"})
	if err == nil {
		t.Fatalf("App.Run(dump --force collection error) error = nil, want error")
	}
	manifestAfter := readFile(t, filepath.Join(outDir, "manifest.json"))
	if manifestAfter != manifestBefore {
		t.Errorf("manifest changed after failed forced dump")
	}
}

func TestDumpForceRejectsSymlinkParentResolvingToHome(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	child := filepath.Join(home, "child")
	if err := os.MkdirAll(child, 0o700); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v, want nil", child, err)
	}
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	if err := os.WriteFile(filepath.Join(home, "manifest.json"), []byte(`{"schema":"zscalerctl.dump.manifest.v2"}`), 0o600); err != nil {
		t.Fatalf("os.WriteFile(home manifest) error = %v, want nil", err)
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(child, link); err != nil {
		t.Skipf("os.Symlink unavailable: %v", err)
	}
	reader := fakeResourceReader{
		list: []resources.SourceRecord{resources.NewSourceRecord(map[string]any{
			"id":   "123",
			"name": "HQ",
		})},
	}
	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})

	err := app.Run(context.Background(), []string{"dump", "--out", filepath.Join(link, ".."), "--resources", "zia/locations", "--force"})
	if !errors.Is(err, dump.ErrUnsafePath) {
		t.Fatalf("App.Run(dump --force symlink-parent home) error = %v, want ErrUnsafePath", err)
	}
	if _, err := os.Stat(filepath.Join(home, "manifest.json")); err != nil {
		t.Errorf("os.Stat(home manifest) error = %v, want nil", err)
	}
	if out.Len() != 0 {
		t.Errorf("App.Run(dump --force symlink-parent home) stdout = %q, want empty", out.String())
	}
}

type fakeResourceReader struct {
	list []resources.SourceRecord
	get  resources.SourceRecord
	show resources.SourceRecord
}

func (f fakeResourceReader) List(context.Context, resources.Product, string) ([]resources.SourceRecord, error) {
	return f.list, nil
}

func (f fakeResourceReader) Get(context.Context, resources.Product, string, string) (resources.SourceRecord, error) {
	return f.get, nil
}

func (f fakeResourceReader) Show(context.Context, resources.Product, string) (resources.SourceRecord, error) {
	return f.show, nil
}

func filterWarningCatalog() resources.ResourceCatalog {
	allModes := []redact.Mode{redact.ModeStandard, redact.ModeShare, redact.ModeParanoid}
	return resources.ResourceCatalog{{
		Product:    resources.ProductZIA,
		Name:       "locations",
		Operations: resources.ListOperations(),
		Fields: []resources.FieldSpec{
			{
				Name:           "id",
				Classification: resources.ClassOperational,
				AllowedModes:   allModes,
			},
			{
				Name:           "name",
				Classification: resources.ClassTenantConfig,
				AllowedModes:   allModes,
			},
			{
				Name:           "hostname",
				Classification: resources.ClassSensitiveIdentifier,
				AllowedModes:   []redact.Mode{redact.ModeStandard},
			},
			{
				Name:           "internalToken",
				Classification: resources.ClassSecret,
			},
		},
	}}
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

func (f selectiveErrorResourceReader) Show(context.Context, resources.Product, string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, errors.New("show must not be called")
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

func (f cancelingResourceReader) Show(context.Context, resources.Product, string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, errors.New("show must not be called")
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

func (f *sessionProviderResourceReader) Show(context.Context, resources.Product, string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, errors.New("direct show must not be called")
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

func (f *sessionErrorResourceReader) Show(context.Context, resources.Product, string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, errors.New("direct show must not be called")
}

type unsupportedSessionResourceReader struct {
	list            []resources.SourceRecord
	show            resources.SourceRecord
	sessionCalls    int
	directListCalls int
	directShowCalls int
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

func (f *unsupportedSessionResourceReader) Show(context.Context, resources.Product, string) (resources.SourceRecord, error) {
	f.directShowCalls++
	return f.show, nil
}

type countingResourceSession struct {
	list       []resources.SourceRecord
	show       resources.SourceRecord
	err        error
	listCalls  int
	showCalls  int
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

func (s *countingResourceSession) Show(context.Context, resources.Product, string) (resources.SourceRecord, error) {
	s.showCalls++
	if s.err != nil {
		return resources.SourceRecord{}, s.err
	}
	return s.show, nil
}

func ziaShowResourceSpecs(t *testing.T) []resources.ResourceSpec {
	t.Helper()

	var specs []resources.ResourceSpec
	for _, spec := range resources.Catalog() {
		if spec.Product != resources.ProductZIA {
			continue
		}
		if spec.SupportsReadOperation("show") {
			specs = append(specs, spec)
		}
	}
	if len(specs) == 0 {
		t.Fatal("resources.Catalog() ZIA show resources = 0, want at least 1")
	}
	return specs
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

func (nilSessionResourceReader) Show(context.Context, resources.Product, string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, errors.New("direct show must not be called")
}

type failingResourceReader struct{}

func (failingResourceReader) List(context.Context, resources.Product, string) ([]resources.SourceRecord, error) {
	return nil, errors.New("reader must not be called")
}

func (failingResourceReader) Get(context.Context, resources.Product, string, string) (resources.SourceRecord, error) {
	return resources.SourceRecord{}, errors.New("reader must not be called")
}

func (failingResourceReader) Show(context.Context, resources.Product, string) (resources.SourceRecord, error) {
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

func catalogProductsForTest() map[string]bool {
	products := map[string]bool{}
	for _, spec := range resources.Catalog() {
		products[string(spec.Product)] = true
	}
	return products
}

func TestOutputFileBadDirectoryIsUsageErrorWithoutTempName(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, []string{
		config.EnvClientID + "=client-id-value",
		config.EnvClientSecret + "=client-secret-value",
	})

	missingDir := filepath.Join(t.TempDir(), "does-not-exist", "out.json")
	err := app.Run(context.Background(), []string{"config", "show", "--format", "json", "--output", missingDir})
	if err == nil {
		t.Fatal("App.Run(--output bad dir) error = nil, want error")
	}
	// A bad --output value is a usage problem (documented exit 2), not internal.
	if !errors.Is(err, cli.ErrUsage) {
		t.Errorf("App.Run(--output bad dir) error = %v, want ErrUsage", err)
	}
	// The generated temp-file name is an implementation detail and must not leak.
	if strings.Contains(err.Error(), ".tmp-") {
		t.Errorf("App.Run(--output bad dir) error = %q, want no temp-file name", err.Error())
	}
	if !strings.Contains(err.Error(), filepath.Dir(missingDir)) {
		t.Errorf("App.Run(--output bad dir) error = %q, want the user-supplied directory", err.Error())
	}
}

func TestDiffComparesLocalDumpsWithoutCredentials(t *testing.T) {
	t.Parallel()

	catalog := resources.ResourceCatalog{cliDiffSpec()}
	oldDir := writeCLIDiffDump(t, dumpFixtureForCLI{
		spec:    cliDiffSpec(),
		payload: `[{"id":"1","name":"old"}]`,
	})
	newDir := writeCLIDiffDump(t, dumpFixtureForCLI{
		spec:    cliDiffSpec(),
		payload: `[{"id":"1","name":"new"}]`,
	})
	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Catalog: catalog})

	err := app.Run(context.Background(), []string{"--format", "json", "diff", oldDir, newDir})
	if err != nil {
		t.Fatalf("App.Run(diff) error = %v, want nil", err)
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(diff) stderr = %q, want empty", errOut.String())
	}
	var report struct {
		Schema  string `json:"schema"`
		Summary struct {
			RecordsChanged int `json:"records_changed"`
		} `json:"summary"`
		Resources []struct {
			Product  string `json:"product"`
			Resource string `json:"resource"`
			Identity struct {
				Mode  string `json:"mode"`
				Field string `json:"field"`
			} `json:"identity"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("json.Unmarshal(diff report) error = %v; output=%q", err, out.String())
	}
	if report.Schema != "zscalerctl.diff.v1" {
		t.Errorf("diff schema = %q, want zscalerctl.diff.v1", report.Schema)
	}
	if report.Summary.RecordsChanged != 1 {
		t.Errorf("records_changed = %d, want 1", report.Summary.RecordsChanged)
	}
	if len(report.Resources) != 1 || report.Resources[0].Product != "zia" || report.Resources[0].Resource != "locations" {
		t.Fatalf("diff resources = %#v, want zia/locations", report.Resources)
	}
	if report.Resources[0].Identity.Mode != "get_key" || report.Resources[0].Identity.Field != "id" {
		t.Errorf("identity = %#v, want get_key/id", report.Resources[0].Identity)
	}
}

func TestDiffFailOnDriftReturnsSentinelAndStillWritesOutputFile(t *testing.T) {
	t.Parallel()

	catalog := resources.ResourceCatalog{cliDiffSpec()}
	oldDir := writeCLIDiffDump(t, dumpFixtureForCLI{
		spec:    cliDiffSpec(),
		payload: `[{"id":"1","name":"old"}]`,
	})
	newDir := writeCLIDiffDump(t, dumpFixtureForCLI{
		spec:    cliDiffSpec(),
		payload: `[{"id":"1","name":"new"}]`,
	})
	outPath := filepath.Join(t.TempDir(), "diff.json")
	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Catalog: catalog})

	err := app.Run(context.Background(), []string{
		"--format", "json",
		"--output", outPath,
		"diff", oldDir, newDir, "--fail-on-drift",
	})
	if !errors.Is(err, cli.ErrDriftDetected) {
		t.Fatalf("App.Run(diff --fail-on-drift) error = %v, want ErrDriftDetected", err)
	}
	if out.Len() != 0 {
		t.Errorf("App.Run(diff --output) stdout = %q, want empty", out.String())
	}
	if errOut.Len() != 0 {
		t.Errorf("App.Run(diff --output) stderr = %q, want empty", errOut.String())
	}
	if got := readFile(t, outPath); !strings.Contains(got, `"schema": "zscalerctl.diff.v1"`) {
		t.Errorf("diff output file = %q, want diff schema", got)
	}
}

func TestAuthStatusTableLabelMatchesJSONKey(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, []string{
		config.EnvClientID + "=client-id-value",
		config.EnvClientSecret + "=client-secret-value",
	})

	if err := app.Run(context.Background(), []string{"--format", "table", "auth", "status"}); err != nil {
		t.Fatalf("App.Run(table auth status) error = %v, want nil", err)
	}
	got := out.String()
	// The table label must be recognizable from the JSON key credential_exchange;
	// the old bare "Token" label matched nothing in the JSON output.
	if !strings.Contains(got, "Credential Exchange") {
		t.Errorf("auth status table = %q, want Credential Exchange label", got)
	}
	if strings.Contains(got, "Token") {
		t.Errorf("auth status table = %q, want no bare Token label", got)
	}
}

func TestDoctorRejectsConflictingProxyConfig(t *testing.T) {
	t.Parallel()

	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, []string{
		config.EnvProxyURL + "=http://proxy.example.invalid:8080",
		config.EnvProxyFromEnv + "=true",
	})

	// Doctor's job is catching problems before live calls: the same proxy
	// conflict the reader rejects at request time must fail doctor (exit 2).
	err := app.Run(context.Background(), []string{"doctor"})
	if err == nil {
		t.Fatal("App.Run(doctor, conflicting proxy) error = nil, want error")
	}
	if !errors.Is(err, zscaler.ErrInvalidProxyConfig) {
		t.Errorf("App.Run(doctor, conflicting proxy) error = %v, want ErrInvalidProxyConfig", err)
	}
}

func TestDumpLogsPerResourceProgressAtInfo(t *testing.T) {
	t.Parallel()

	reader := fakeResourceReader{
		list: []resources.SourceRecord{resources.NewSourceRecord(map[string]any{
			"id":   "123",
			"name": "HQ",
		})},
	}
	var out, errOut bytes.Buffer
	app := cli.NewWithOptions(&out, &errOut, nil, cli.Options{Reader: reader})

	dir := filepath.Join(t.TempDir(), "dump")
	err := app.Run(context.Background(), []string{
		"--log-level", "info",
		"dump", "--out", dir, "--products", "zia", "--resources", "locations",
	})
	if err != nil {
		t.Fatalf("App.Run(dump --log-level info) error = %v, want nil", err)
	}
	logged := errOut.String()
	// A multi-minute dump must not be silent at info: operators get a starting
	// line with the selection size and a per-resource progress event.
	if !strings.Contains(logged, "dump starting") {
		t.Errorf("dump info logs = %q, want dump starting event", logged)
	}
	if !strings.Contains(logged, "dump reading resource") || !strings.Contains(logged, "locations") {
		t.Errorf("dump info logs = %q, want per-resource progress for locations", logged)
	}
}

func TestProductHelpListsItsResources(t *testing.T) {
	t.Parallel()

	// After the Cobra migration, `zscalerctl zia --help` shows Cobra-formatted
	// help (short description + global flags) rather than the legacy resource-list.
	// The primary discoverability path is now the error message from an unknown
	// resource (ResourceNotFoundError) and `schema list`. This test verifies the
	// Cobra help emits the product name and that the error-path discoverability
	// hints remain intact.
	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, nil)
	if err := app.Run(context.Background(), []string{"zia", "--help"}); err != nil {
		t.Fatalf("App.Run(zia --help) error = %v, want nil", err)
	}
	got := out.String()
	// Cobra help must include "zscalerctl zia" in the Usage line.
	for _, want := range []string{"zscalerctl zia"} {
		if !strings.Contains(got, want) {
			t.Errorf("zia --help = %q, want it to mention %q", got, want)
		}
	}

	// The bare-product usage error must still carry the discoverability hints:
	// "zpa --help" and "schema list" point the user (or agent) at the two
	// enumeration paths. ResourceNotFoundError.Error() still includes these.
	app2 := cli.New(&out, &errOut, nil)
	err := app2.Run(context.Background(), []string{"zpa", "bogus-resource", "list"})
	if err == nil {
		t.Fatal("App.Run(zpa bogus) error = nil, want usage error")
	}
	if !strings.Contains(err.Error(), "schema list") || !strings.Contains(err.Error(), "zpa --help") {
		t.Errorf("zpa unknown-resource error = %q, want enumeration hints", err.Error())
	}
}

type dumpFixtureForCLI struct {
	spec    resources.ResourceSpec
	payload string
}

func writeCLIConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%s) error = %v, want nil", path, err)
	}
	return path
}

func writeCLIDiffDump(t *testing.T, fixture dumpFixtureForCLI) string {
	t.Helper()
	dir := t.TempDir()
	relPath := filepath.ToSlash(filepath.Join("resources", string(fixture.spec.Product), fixture.spec.Name+".json"))
	path := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("os.MkdirAll(%s) error = %v, want nil", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(fixture.payload), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%s) error = %v, want nil", path, err)
	}
	manifest := dump.Manifest{
		Schema:      dump.ManifestSchemaID,
		CollectedAt: "2026-01-01T00:00:00Z",
		ToolVersion: "test",
		Redaction:   string(redact.ModeStandard),
		Warning:     "test fixture",
		Status:      "complete",
		Resources: []dump.ManifestResource{
			{
				Product: string(fixture.spec.Product),
				Name:    fixture.spec.Name,
				Shape:   string(fixture.spec.EffectiveShape()),
				Status:  "ok",
				Path:    relPath,
				Records: 1,
			},
		},
	}
	body, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent(manifest) error = %v, want nil", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), body, 0o600); err != nil {
		t.Fatalf("os.WriteFile(manifest) error = %v, want nil", err)
	}
	return dir
}

func cliDiffSpec() resources.ResourceSpec {
	return resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       "locations",
		Operations: resources.ReadOperations(),
		Fields: []resources.FieldSpec{
			{Name: "id", Classification: resources.ClassOperational},
			{Name: "name", Classification: resources.ClassTenantConfig},
		},
	}
}

func TestCLIConfigCmdHelperProcess(t *testing.T) {
	index := -1
	for i, arg := range os.Args {
		if arg == "--" {
			index = i
			break
		}
	}
	if index < 0 {
		return
	}
	args := os.Args[index+1:]
	if len(args) < 2 || args[0] != "touch" {
		os.Exit(2)
	}
	if err := os.WriteFile(args[1], []byte("ran"), 0o600); err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}
