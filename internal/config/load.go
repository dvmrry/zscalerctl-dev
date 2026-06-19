package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/keyring"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/secret"
	"github.com/dvmrry/zscalerctl/internal/secretref"
)

type SecretResolver interface {
	Resolve(context.Context, secretref.SecretRef) (secret.Secret, error)
}

type LoadOptions struct {
	Profile    string
	ConfigPath string
	Resolver   SecretResolver
}

func LoadConfig(environ []string, opts LoadOptions) (Config, error) {
	env := parseEnv(environ)
	cfg, err := LoadEnv(environ)
	if err != nil {
		return Config{}, err
	}
	disallowCmd, err := parseBoolEnv(env[EnvDisallowCmd])
	if err != nil {
		return Config{}, fmt.Errorf("%w: parse %s: %w", ErrInvalidConfig, EnvDisallowCmd, err)
	}

	requestedProfile := strings.TrimSpace(opts.Profile)
	if requestedProfile == "" {
		requestedProfile = strings.TrimSpace(env[EnvProfile])
	}
	configPath, explicitConfig := configPathFromOptions(env, opts)
	profile, loaded, err := loadProfileFile(configPath, requestedProfile, explicitConfig)
	if err != nil {
		return Config{}, err
	}
	if !loaded {
		if opts.Profile != "" {
			cfg.Profile = opts.Profile
		}
		return cfg, nil
	}

	cfg.Source = "config"
	cfg.ConfigFile = configPath
	cfg.Profile = profile.name
	if env[EnvAuthMode] == "" {
		cfg.AuthMode = ""
	}
	resolver := opts.Resolver
	if resolver == nil {
		resolver = secretref.NewResolver(secretref.ResolverOpts{
			AllowCmd: !disallowCmd,
			Keyring:  keyring.New(),
		})
	}
	if err := applyProfile(&cfg, profile.data, env, resolver); err != nil {
		return Config{}, err
	}
	if cfg.AuthMode == "" {
		cfg.AuthMode = cfg.EffectiveAuthMode()
	}
	return cfg, nil
}

func configPathFromOptions(env map[string]string, opts LoadOptions) (string, bool) {
	if strings.TrimSpace(opts.ConfigPath) != "" {
		return strings.TrimSpace(opts.ConfigPath), true
	}
	if strings.TrimSpace(env[EnvConfig]) != "" {
		return strings.TrimSpace(env[EnvConfig]), true
	}
	if xdg := strings.TrimSpace(env["XDG_CONFIG_HOME"]); xdg != "" {
		return filepath.Join(xdg, "zscalerctl", "config.yaml"), false
	}
	if home := strings.TrimSpace(env["HOME"]); home != "" {
		return filepath.Join(home, ".config", "zscalerctl", "config.yaml"), false
	}
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, "zscalerctl", "config.yaml"), false
	}
	return filepath.Join(".config", "zscalerctl", "config.yaml"), false
}

func applyProfile(cfg *Config, profile profileData, env map[string]string, resolver SecretResolver) error {
	if env[EnvAuthMode] == "" && strings.TrimSpace(profile.AuthMode) != "" {
		authMode, err := parseAuthMode(profile.AuthMode)
		if err != nil {
			return err
		}
		cfg.AuthMode = authMode
	}
	if env[EnvVanityDomain] == "" && strings.TrimSpace(profile.VanityDomain) != "" {
		cfg.VanityDomain = strings.TrimSpace(profile.VanityDomain)
	}
	if env[EnvCloud] == "" && strings.TrimSpace(profile.Cloud) != "" {
		cfg.Cloud = strings.TrimSpace(profile.Cloud)
	}
	if env[EnvClientID] == "" && strings.TrimSpace(profile.ClientID) != "" {
		cfg.Credentials.ClientID = secret.New(profile.ClientID)
	}
	if !cfg.Credentials.ClientSecret.IsConfigured() && cfg.Credentials.ClientSecretFile == "" && profile.ClientSecretRef != nil {
		cfg.Credentials.ClientSecret = secretref.Deferred(*profile.ClientSecretRef, resolver)
	}
	if env[EnvZPACustomerID] == "" && strings.TrimSpace(profile.ZPACustomerID) != "" {
		cfg.ZPA.CustomerID = strings.TrimSpace(profile.ZPACustomerID)
	}
	if env[EnvZPAMicrotenantID] == "" && strings.TrimSpace(profile.ZPAMicrotenantID) != "" {
		cfg.ZPA.MicrotenantID = strings.TrimSpace(profile.ZPAMicrotenantID)
	}
	if env[EnvZIAUsername] == "" && strings.TrimSpace(profile.ZIAUsername) != "" {
		cfg.ZIALegacy.Username = secret.New(profile.ZIAUsername)
	}
	if !cfg.ZIALegacy.Password.IsConfigured() && cfg.ZIALegacy.PasswordFile == "" && profile.ZIAPasswordRef != nil {
		cfg.ZIALegacy.Password = secretref.Deferred(*profile.ZIAPasswordRef, resolver)
	}
	if !cfg.ZIALegacy.APIKey.IsConfigured() && cfg.ZIALegacy.APIKeyFile == "" && profile.ZIAAPIKeyRef != nil {
		cfg.ZIALegacy.APIKey = secretref.Deferred(*profile.ZIAAPIKeyRef, resolver)
	}
	if env[EnvZIACloud] == "" && strings.TrimSpace(profile.ZIACloud) != "" {
		cfg.ZIALegacy.Cloud = strings.TrimSpace(profile.ZIACloud)
	}
	if env[EnvRedaction] == "" && strings.TrimSpace(profile.Redaction) != "" {
		mode, err := redact.ParseMode(profile.Redaction)
		if err != nil {
			return err
		}
		cfg.Defaults.Redaction = mode
	}
	if env[EnvNoCache] == "" && profile.NoCache != nil {
		cfg.Defaults.NoCache = *profile.NoCache
	}
	return nil
}
