package config_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/secret"
	"github.com/dvmrry/zscalerctl/internal/secretref"
)

func TestLoadConfigAppliesProfileWhenEnvUnset(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, `
default_profile: prod
profiles:
  prod:
    vanity_domain: profile-vanity
    cloud: PRODUCTION
    client_id: profile-client
    client_secret_ref: env:PROFILE_SECRET
    zpa_customer_id: profile-customer
    redaction: share
    no_cache: true
`)
	cfg, err := config.LoadConfig(nil, config.LoadOptions{
		ConfigPath: path,
		Resolver:   fakeResolver{secret: secret.New("resolved")},
	})
	if err != nil {
		t.Fatalf("LoadConfig(profile) error = %v, want nil", err)
	}
	if cfg.Profile != "prod" || cfg.VanityDomain != "profile-vanity" || cfg.Cloud != "PRODUCTION" {
		t.Fatalf("profile fields = profile %q vanity %q cloud %q", cfg.Profile, cfg.VanityDomain, cfg.Cloud)
	}
	if cfg.Credentials.ClientSecret.Scheme() != "env" || !cfg.Credentials.ClientSecret.IsConfigured() {
		t.Fatalf("ClientSecret source = %q configured=%v, want env/true", cfg.Credentials.ClientSecret.Scheme(), cfg.Credentials.ClientSecret.IsConfigured())
	}
	if !cfg.Safe().ConfigFileSet || cfg.Safe().Source != "config" {
		t.Fatalf("Safe() source = %+v, want config source", cfg.Safe())
	}
}

func TestLoadConfigEnvOverridesProfile(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, `
default_profile: prod
profiles:
  prod:
    vanity_domain: profile-vanity
    client_secret_ref: env:PROFILE_SECRET
`)
	cfg, err := config.LoadConfig([]string{
		config.EnvVanityDomain + "=env-vanity",
		config.EnvClientSecret + "=env-secret",
	}, config.LoadOptions{
		ConfigPath: path,
		Resolver:   fakeResolver{secret: secret.New("profile-secret")},
	})
	if err != nil {
		t.Fatalf("LoadConfig(env overrides profile) error = %v, want nil", err)
	}
	if cfg.VanityDomain != "env-vanity" {
		t.Errorf("VanityDomain = %q, want env-vanity", cfg.VanityDomain)
	}
	if cfg.Credentials.ClientSecret.Scheme() != "env" {
		t.Errorf("ClientSecret.Scheme() = %q, want env", cfg.Credentials.ClientSecret.Scheme())
	}
	got, err := cfg.Credentials.ClientSecret.Resolve(context.Background())
	if err != nil {
		t.Fatalf("ClientSecret.Resolve() error = %v, want nil", err)
	}
	if got.Reveal() != "env-secret" {
		t.Errorf("ClientSecret.Resolve() = %q, want env-secret", got.Reveal())
	}
}

func TestLoadConfigFlagProfileOverridesEnvProfile(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, `
default_profile: prod
profiles:
  prod:
    vanity_domain: prod-vanity
  dev:
    vanity_domain: dev-vanity
`)
	cfg, err := config.LoadConfig([]string{config.EnvProfile + "=prod"}, config.LoadOptions{ConfigPath: path, Profile: "dev"})
	if err != nil {
		t.Fatalf("LoadConfig(flag profile) error = %v, want nil", err)
	}
	if cfg.Profile != "dev" || cfg.VanityDomain != "dev-vanity" {
		t.Errorf("profile = %q vanity = %q, want dev/dev-vanity", cfg.Profile, cfg.VanityDomain)
	}
}

func TestLoadConfigInfersZIALegacyFromProfileCredentials(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, `
profiles:
  default:
    zia_username: admin@example.invalid
    zia_password_ref: env:ZIA_PASSWORD_REF
    zia_api_key_ref: env:ZIA_API_KEY_REF
    zia_cloud: zscalerthree
`)
	cfg, err := config.LoadConfig(nil, config.LoadOptions{
		ConfigPath: path,
		Resolver:   fakeResolver{secret: secret.New("resolved")},
	})
	if err != nil {
		t.Fatalf("LoadConfig(ZIA legacy profile) error = %v, want nil", err)
	}
	if cfg.EffectiveAuthMode() != config.AuthModeZIALegacy {
		t.Errorf("EffectiveAuthMode() = %q, want %q", cfg.EffectiveAuthMode(), config.AuthModeZIALegacy)
	}
}

func TestLoadConfigRejectsUnsafeConfigFile(t *testing.T) {
	t.Parallel()

	path := writeConfigMode(t, 0o644, `
profiles:
  default:
    vanity_domain: profile-vanity
`)
	_, err := config.LoadConfig(nil, config.LoadOptions{ConfigPath: path})
	if err == nil {
		t.Fatal("LoadConfig(unsafe config) error = nil, want error")
	}
	if !errors.Is(err, config.ErrInvalidConfig) {
		t.Fatalf("LoadConfig(unsafe config) error = %v, want ErrInvalidConfig", err)
	}
}

func TestSafeConfigDoesNotResolveDeferredSecret(t *testing.T) {
	t.Parallel()

	resolveCalls := 0
	path := writeConfig(t, `
profiles:
  default:
    client_secret_ref: env:PROFILE_SECRET
`)
	resolver := fakeResolver{calls: &resolveCalls, panicOnResolve: true}
	cfg, err := config.LoadConfig(nil, config.LoadOptions{ConfigPath: path, Resolver: resolver})
	if err != nil {
		t.Fatalf("LoadConfig(deferred secret) error = %v, want nil", err)
	}
	safe := cfg.Safe()
	if !safe.Credentials.ClientSecretSet || safe.Credentials.ClientSecretScheme != "env" {
		t.Fatalf("Safe().Credentials = %+v, want configured env source", safe.Credentials)
	}
	if resolveCalls != 0 {
		t.Fatalf("Safe() resolved deferred secret %d time(s), want 0", resolveCalls)
	}
}

func TestLoadConfigCmdProviderKillSwitch(t *testing.T) {
	t.Parallel()

	configPath := writeConfig(t, fmt.Sprintf(`
profiles:
  default:
    client_secret_ref:
      cmd:
        argv: [%q, "-test.run=^TestConfigCmdHelperProcess$", "--", "print", "cmd-secret"]
`, os.Args[0]))

	for _, env := range [][]string{
		nil,
		{config.EnvDisallowCmd + "=false"},
	} {
		cfg, err := config.LoadConfig(env, config.LoadOptions{ConfigPath: configPath})
		if err != nil {
			t.Fatalf("LoadConfig(cmd ref, env %v) error = %v, want nil", env, err)
		}
		got, err := cfg.Credentials.ClientSecret.Resolve(context.Background())
		if err != nil {
			t.Fatalf("ClientSecret.Resolve(cmd ref, env %v) error = %v, want nil", env, err)
		}
		if got.Reveal() != "cmd-secret" {
			t.Fatalf("ClientSecret.Resolve(cmd ref, env %v) = %q, want cmd-secret", env, got.Reveal())
		}
	}

	sentinel := filepath.Join(t.TempDir(), "provider-ran")
	disabledPath := writeConfig(t, fmt.Sprintf(`
profiles:
  default:
    client_secret_ref:
      cmd:
        argv: [%q, "-test.run=^TestConfigCmdHelperProcess$", "--", "touch", %q]
`, os.Args[0], sentinel))
	cfg, err := config.LoadConfig([]string{config.EnvDisallowCmd + "=true"}, config.LoadOptions{ConfigPath: disabledPath})
	if err != nil {
		t.Fatalf("LoadConfig(disabled cmd ref) error = %v, want nil", err)
	}
	if _, err := cfg.Credentials.ClientSecret.Resolve(context.Background()); err == nil {
		t.Fatal("ClientSecret.Resolve(disabled cmd ref) error = nil, want disabled error")
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatalf("disabled cmd provider created %q; stat err = %v", sentinel, err)
	}
}

func TestLoadConfigKeyringRefIsDeferredKeyringSource(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, `
default_profile: prod
profiles:
  prod:
    vanity_domain: v
    client_id: c
    client_secret_ref: keyring:svc/key
    zpa_customer_id: z
`)
	cfg, err := config.LoadConfig(nil, config.LoadOptions{ConfigPath: path})
	if err != nil {
		t.Fatalf("LoadConfig(keyring ref) error = %v, want nil", err)
	}
	if got := cfg.Credentials.ClientSecret.Scheme(); got != "keyring" {
		t.Fatalf("ClientSecret scheme = %q, want keyring", got)
	}
}

func TestLoadConfigRejectsMalformedDisallowCmdWithoutValueLeak(t *testing.T) {
	t.Parallel()

	const badValue = "definitely-not-bool"
	_, err := config.LoadConfig([]string{config.EnvDisallowCmd + "=" + badValue}, config.LoadOptions{})
	if err == nil {
		t.Fatal("LoadConfig(malformed disallow cmd) error = nil, want error")
	}
	if !errors.Is(err, config.ErrInvalidConfig) {
		t.Fatalf("LoadConfig(malformed disallow cmd) error = %v, want ErrInvalidConfig", err)
	}
	if strings.Contains(err.Error(), badValue) {
		t.Fatalf("LoadConfig(malformed disallow cmd) error = %q, want value-free error", err.Error())
	}
	if !strings.Contains(err.Error(), config.EnvDisallowCmd) {
		t.Fatalf("LoadConfig(malformed disallow cmd) error = %q, want env var name", err.Error())
	}
}

func TestLoadConfigExplicitMissingFileFails(t *testing.T) {
	t.Parallel()

	_, err := config.LoadConfig(nil, config.LoadOptions{ConfigPath: filepath.Join(t.TempDir(), "missing.yaml")})
	if err == nil {
		t.Fatal("LoadConfig(explicit missing file) error = nil, want error")
	}
	if !errors.Is(err, config.ErrInvalidConfig) {
		t.Fatalf("LoadConfig(explicit missing file) error = %v, want ErrInvalidConfig", err)
	}
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	return writeConfigMode(t, 0o600, body)
}

func writeConfigMode(t *testing.T, mode os.FileMode, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}
	return path
}

type fakeResolver struct {
	secret         secret.Secret
	err            error
	calls          *int
	panicOnResolve bool
}

func (r fakeResolver) Resolve(context.Context, secretref.SecretRef) (secret.Secret, error) {
	if r.calls != nil {
		*r.calls++
	}
	if r.panicOnResolve {
		panic("deferred secret unexpectedly resolved")
	}
	if r.err != nil {
		return secret.Secret{}, r.err
	}
	return r.secret, nil
}

func TestConfigCmdHelperProcess(t *testing.T) {
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
	if len(args) == 0 {
		os.Exit(2)
	}
	switch args[0] {
	case "print":
		fmt.Print(strings.Join(args[1:], " "))
	case "touch":
		if len(args) < 2 {
			os.Exit(2)
		}
		if err := os.WriteFile(args[1], []byte("ran"), 0o600); err != nil {
			fmt.Fprint(os.Stderr, err)
			os.Exit(1)
		}
	default:
		os.Exit(2)
	}
	os.Exit(0)
}
