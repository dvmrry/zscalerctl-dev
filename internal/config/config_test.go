package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/config"
)

func TestLoadEnvSafeConfigDoesNotExposeSecrets(t *testing.T) {
	t.Parallel()

	const clientID = "client-id-value"
	const clientSecret = "client-secret-value"
	cfg, err := config.LoadEnv([]string{
		config.EnvProfile + "=prod",
		config.EnvVanityDomain + "=acme",
		config.EnvCloud + "=zscalerthree",
		config.EnvClientID + "=" + clientID,
		config.EnvClientSecret + "=" + clientSecret,
		config.EnvProxyURL + "=http://proxy-user:proxy-secret@proxy.example.invalid:8080",
		config.EnvNoCache + "=true",
	})
	if err != nil {
		t.Fatalf("LoadEnv() error = %v, want nil", err)
	}

	body, err := json.Marshal(cfg.Safe())
	if err != nil {
		t.Fatalf("json.Marshal(Config.Safe()) error = %v, want nil", err)
	}
	got := string(body)
	for _, forbidden := range []string{clientID, clientSecret, "acme", "proxy-user", "proxy-secret", "proxy.example.invalid"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("json.Marshal(Config.Safe()) = %s, want no %q", got, forbidden)
		}
	}
	if !cfg.Safe().Credentials.ClientIDSet || !cfg.Safe().Credentials.ClientSecretSet {
		t.Errorf("Config.Safe().Credentials = %+v, want client id and secret marked set", cfg.Safe().Credentials)
	}
	if !cfg.Safe().VanityDomainSet {
		t.Errorf("Config.Safe().VanityDomainSet = false, want true")
	}
	if !cfg.Safe().Proxy.URLSet || cfg.Safe().Proxy.FromEnvironment {
		t.Errorf("Config.Safe().Proxy = %+v, want URL set without environment proxy", cfg.Safe().Proxy)
	}
}

func TestLoadEnvZIALegacySafeConfigDoesNotExposeSecrets(t *testing.T) {
	t.Parallel()

	const (
		username = "admin@example.invalid"
		password = "legacy-password-value"
		apiKey   = "legacy-api-key-value"
		cloud    = "zscalerthree"
	)
	cfg, err := config.LoadEnv([]string{
		config.EnvAuthMode + "=" + string(config.AuthModeZIALegacy),
		config.EnvZIAUsername + "=" + username,
		config.EnvZIAPassword + "=" + password,
		config.EnvZIAAPIKey + "=" + apiKey,
		config.EnvZIACloud + "=" + cloud,
	})
	if err != nil {
		t.Fatalf("LoadEnv(ZIA legacy) error = %v, want nil", err)
	}
	if cfg.EffectiveAuthMode() != config.AuthModeZIALegacy {
		t.Errorf("Config.EffectiveAuthMode() = %q, want %q", cfg.EffectiveAuthMode(), config.AuthModeZIALegacy)
	}

	body, err := json.Marshal(cfg.Safe())
	if err != nil {
		t.Fatalf("json.Marshal(Config.Safe()) error = %v, want nil", err)
	}
	got := string(body)
	for _, forbidden := range []string{username, password, apiKey, cloud} {
		if strings.Contains(got, forbidden) {
			t.Errorf("json.Marshal(Config.Safe()) = %s, want no %q", got, forbidden)
		}
	}
	if !cfg.Safe().ZIALegacy.UsernameSet || !cfg.Safe().ZIALegacy.PasswordSet || !cfg.Safe().ZIALegacy.APIKeySet || !cfg.Safe().ZIALegacy.CloudSet {
		t.Errorf("Config.Safe().ZIALegacy = %+v, want legacy fields marked set", cfg.Safe().ZIALegacy)
	}
}

func TestLoadEnvInfersZIALegacyWhenOnlyLegacyCredentialsAreSet(t *testing.T) {
	t.Parallel()

	cfg, err := config.LoadEnv([]string{
		config.EnvZIAUsername + "=admin@example.invalid",
		config.EnvZIAPassword + "=legacy-password-value",
		config.EnvZIAAPIKey + "=legacy-api-key-value",
		config.EnvZIACloud + "=zscalerthree",
	})
	if err != nil {
		t.Fatalf("LoadEnv(infer ZIA legacy) error = %v, want nil", err)
	}
	if cfg.EffectiveAuthMode() != config.AuthModeZIALegacy {
		t.Errorf("Config.EffectiveAuthMode() = %q, want %q", cfg.EffectiveAuthMode(), config.AuthModeZIALegacy)
	}
}

func TestLoadEnvRejectsInvalidAuthMode(t *testing.T) {
	t.Parallel()

	if _, err := config.LoadEnv([]string{config.EnvAuthMode + "=legacy"}); err == nil {
		t.Errorf("LoadEnv(%s=legacy) error = nil, want error", config.EnvAuthMode)
	}
}

func TestLoadEnvRejectsRedactionOff(t *testing.T) {
	t.Parallel()

	if _, err := config.LoadEnv([]string{config.EnvRedaction + "=off"}); err == nil {
		t.Errorf("LoadEnv(%s=off) error = nil, want error", config.EnvRedaction)
	}
}

func TestLoadEnvReadsExplicitProxyFromEnvironmentFlag(t *testing.T) {
	t.Parallel()

	cfg, err := config.LoadEnv([]string{
		config.EnvProxyFromEnv + "=true",
	})
	if err != nil {
		t.Fatalf("LoadEnv(proxy from env) error = %v, want nil", err)
	}
	if !cfg.Proxy.FromEnvironment {
		t.Errorf("LoadEnv(proxy from env).Proxy.FromEnvironment = false, want true")
	}
	if cfg.Safe().Proxy.URLSet {
		t.Errorf("Config.Safe().Proxy.URLSet = true, want false")
	}
	if !cfg.Safe().Proxy.FromEnvironment {
		t.Errorf("Config.Safe().Proxy.FromEnvironment = false, want true")
	}
}

func TestLoadEnvRejectsInvalidProxyFromEnvironmentFlag(t *testing.T) {
	t.Parallel()

	if _, err := config.LoadEnv([]string{config.EnvProxyFromEnv + "=sometimes"}); err == nil {
		t.Errorf("LoadEnv(%s=sometimes) error = nil, want error", config.EnvProxyFromEnv)
	}
}

func TestLoadEnvInvalidBoolErrorDoesNotEchoValue(t *testing.T) {
	t.Parallel()

	const canary = "client_secret=raw-bool-canary"
	_, err := config.LoadEnv([]string{config.EnvProxyFromEnv + "=" + canary})
	if err == nil {
		t.Fatalf("LoadEnv(%s=<canary>) error = nil, want error", config.EnvProxyFromEnv)
	}
	if !strings.Contains(err.Error(), config.EnvProxyFromEnv) {
		t.Errorf("LoadEnv invalid bool error = %q, want env var name", err.Error())
	}
	if strings.Contains(err.Error(), canary) || strings.Contains(err.Error(), "client_secret") {
		t.Errorf("LoadEnv invalid bool error = %q, want no raw value", err.Error())
	}
}

func TestLoadEnvLoadsOwnerOnlySecretFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "client-secret.txt")
	if err := os.WriteFile(path, []byte("file-secret\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}

	cfg, err := config.LoadEnv([]string{config.EnvClientSecretFile + "=" + path})
	if err != nil {
		t.Fatalf("LoadEnv(secret file) error = %v, want nil", err)
	}
	if cfg.Credentials.ClientSecret.Reveal() != "file-secret" {
		t.Errorf("LoadEnv(secret file).Credentials.ClientSecret = %q, want %q", cfg.Credentials.ClientSecret.Reveal(), "file-secret")
	}
}

func TestLoadEnvLoadsOwnerOnlyZIALegacySecretFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	passwordPath := filepath.Join(dir, "zia-password.txt")
	apiKeyPath := filepath.Join(dir, "zia-api-key.txt")
	if err := os.WriteFile(passwordPath, []byte("legacy-password\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", passwordPath, err)
	}
	if err := os.WriteFile(apiKeyPath, []byte("legacy-api-key\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", apiKeyPath, err)
	}

	cfg, err := config.LoadEnv([]string{
		config.EnvZIAPasswordFile + "=" + passwordPath,
		config.EnvZIAAPIKeyFile + "=" + apiKeyPath,
	})
	if err != nil {
		t.Fatalf("LoadEnv(ZIA legacy files) error = %v, want nil", err)
	}
	if cfg.ZIALegacy.Password.Reveal() != "legacy-password" {
		t.Errorf("LoadEnv(ZIA legacy files).ZIALegacy.Password = %q, want legacy-password", cfg.ZIALegacy.Password.Reveal())
	}
	if cfg.ZIALegacy.APIKey.Reveal() != "legacy-api-key" {
		t.Errorf("LoadEnv(ZIA legacy files).ZIALegacy.APIKey = %q, want legacy-api-key", cfg.ZIALegacy.APIKey.Reveal())
	}
}

func TestLoadEnvRejectsUnsafeSecretFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "client-secret.txt")
	if err := os.WriteFile(path, []byte("file-secret\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}

	if _, err := config.LoadEnv([]string{config.EnvClientSecretFile + "=" + path}); err == nil {
		t.Errorf("LoadEnv(unsafe secret file) error = nil, want error")
	}
}

func TestLoadEnvRejectsUnsafeZIALegacySecretFile(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "zia-api-key.txt")
	if err := os.WriteFile(path, []byte("legacy-api-key\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}

	if _, err := config.LoadEnv([]string{config.EnvZIAAPIKeyFile + "=" + path}); err == nil {
		t.Errorf("LoadEnv(unsafe ZIA legacy secret file) error = nil, want error")
	}
}
