package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/credentials"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/secret"
	"github.com/dvmrry/zscalerctl/internal/secretref"
)

const (
	EnvProfile          = "ZSCALERCTL_PROFILE"
	EnvAuthMode         = "ZSCALERCTL_AUTH_MODE"
	EnvVanityDomain     = "ZSCALERCTL_VANITY_DOMAIN"
	EnvCloud            = "ZSCALERCTL_CLOUD"
	EnvClientID         = "ZSCALERCTL_CLIENT_ID"
	EnvClientSecret     = "ZSCALERCTL_CLIENT_SECRET"      // #nosec G101 -- env var name, not a secret value
	EnvClientSecretFile = "ZSCALERCTL_CLIENT_SECRET_FILE" // #nosec G101 -- env var name, not a secret value
	EnvZPACustomerID    = "ZSCALERCTL_ZPA_CUSTOMER_ID"
	EnvZPAMicrotenantID = "ZSCALERCTL_ZPA_MICROTENANT_ID"
	EnvZIAUsername      = "ZSCALERCTL_ZIA_USERNAME"
	EnvZIAPassword      = "ZSCALERCTL_ZIA_PASSWORD"      // #nosec G101 -- env var name, not a secret value
	EnvZIAPasswordFile  = "ZSCALERCTL_ZIA_PASSWORD_FILE" // #nosec G101 -- env var name, not a secret value
	EnvZIAAPIKey        = "ZSCALERCTL_ZIA_API_KEY"       // #nosec G101 -- env var name, not a secret value
	EnvZIAAPIKeyFile    = "ZSCALERCTL_ZIA_API_KEY_FILE"  // #nosec G101 -- env var name, not a secret value
	EnvZIACloud         = "ZSCALERCTL_ZIA_CLOUD"
	EnvProxyURL         = "ZSCALERCTL_PROXY_URL"
	EnvProxyFromEnv     = "ZSCALERCTL_PROXY_FROM_ENV"
	EnvRedaction        = "ZSCALERCTL_REDACTION"
	EnvNoCache          = "ZSCALERCTL_NO_CACHE"
	EnvConfig           = "ZSCALERCTL_CONFIG"
	EnvDisallowCmd      = "ZSCALERCTL_DISALLOW_CMD"
)

type AuthMode string

const (
	AuthModeOneAPI    AuthMode = "oneapi"
	AuthModeZIALegacy AuthMode = "zia-legacy"
)

// ErrInvalidConfig classifies a malformed configuration value (e.g. an
// unparseable ZSCALERCTL_* setting). It lets the command boundary map operator
// misconfiguration to the usage exit code instead of the internal-error code.
var ErrInvalidConfig = errors.New("invalid configuration")

type Config struct {
	Source     string
	ConfigFile string
	Profile    string
	// ProfileSource and ProfileCount are set when a config file was loaded:
	// how Profile was chosen and how many profiles the file defines.
	ProfileSource ProfileSource
	ProfileCount  int
	AuthMode      AuthMode
	VanityDomain  string
	Cloud         string
	Credentials   Credentials
	ZPA           ZPAConfig
	ZIALegacy     ZIALegacyCredentials
	Proxy         Proxy
	Defaults      Defaults
}

// ProfileSource says how the active config-file profile was chosen.
type ProfileSource string

const (
	ProfileFromFlag           ProfileSource = "flag"
	ProfileFromEnv            ProfileSource = "env"
	ProfileFromDefaultProfile ProfileSource = "default_profile"
	ProfileFromBuiltinDefault ProfileSource = "builtin_default"
)

// ImplicitProfileAmongMany reports whether the profile came from the config
// file's default rather than --profile or ZSCALERCTL_PROFILE while the file
// defines more than one profile, so the tenant may not be the one intended.
func (c Config) ImplicitProfileAmongMany() bool {
	implicit := c.ProfileSource == ProfileFromDefaultProfile || c.ProfileSource == ProfileFromBuiltinDefault
	return implicit && c.ProfileCount > 1
}

type Credentials struct {
	ClientID         secret.Secret
	ClientSecret     secretref.SecretSource
	ClientSecretFile string
}

type ZPAConfig struct {
	CustomerID    string
	MicrotenantID string
}

type ZIALegacyCredentials struct {
	Username     secret.Secret
	Password     secretref.SecretSource
	PasswordFile string
	APIKey       secretref.SecretSource
	APIKeyFile   string
	Cloud        string
}

type Proxy struct {
	URL             string
	FromEnvironment bool
}

type Defaults struct {
	Redaction redact.Mode
	NoCache   bool
}

type SafeConfig struct {
	Source          string           `json:"source"`
	ConfigFileSet   bool             `json:"config_file_set"`
	Profile         string           `json:"profile"`
	AuthMode        string           `json:"auth_mode"`
	VanityDomainSet bool             `json:"vanity_domain_set"`
	Cloud           string           `json:"cloud,omitempty"`
	Credentials     CredentialStatus `json:"credentials"`
	ZPA             ZPAStatus        `json:"zpa"`
	ZIALegacy       ZIALegacyStatus  `json:"zia_legacy"`
	Proxy           ProxyStatus      `json:"proxy"`
	Defaults        DefaultsView     `json:"defaults"`
}

func (SafeConfig) OutputSafe() {}

type CredentialStatus struct {
	ClientIDSet         bool   `json:"client_id_set"`
	ClientSecretSet     bool   `json:"client_secret_set"`
	ClientSecretFileSet bool   `json:"client_secret_file_set"`
	ClientSecretScheme  string `json:"client_secret_scheme,omitempty"`
}

type ZPAStatus struct {
	CustomerIDSet    bool `json:"customer_id_set"`
	MicrotenantIDSet bool `json:"microtenant_id_set"`
}

type ZIALegacyStatus struct {
	UsernameSet     bool   `json:"username_set"`
	PasswordSet     bool   `json:"password_set"`
	PasswordFileSet bool   `json:"password_file_set"`
	PasswordScheme  string `json:"password_scheme,omitempty"`
	APIKeySet       bool   `json:"api_key_set"`
	APIKeyFileSet   bool   `json:"api_key_file_set"`
	APIKeyScheme    string `json:"api_key_scheme,omitempty"`
	CloudSet        bool   `json:"cloud_set"`
}

type ProxyStatus struct {
	URLSet          bool `json:"url_set"`
	FromEnvironment bool `json:"from_environment"`
}

type DefaultsView struct {
	Redaction string `json:"redaction"`
	NoCache   bool   `json:"no_cache"`
}

func LoadEnv(environ []string) (Config, error) {
	env := parseEnv(environ)
	mode := redact.ModeStandard
	if value := env[EnvRedaction]; value != "" {
		parsed, err := redact.ParseMode(value)
		if err != nil {
			return Config{}, fmt.Errorf("%w: parse %s: %w", ErrInvalidConfig, EnvRedaction, err)
		}
		mode = parsed
	}

	noCache, err := parseBoolEnv(env[EnvNoCache])
	if err != nil {
		return Config{}, fmt.Errorf("%w: parse %s: %w", ErrInvalidConfig, EnvNoCache, err)
	}
	proxyFromEnv, err := parseBoolEnv(env[EnvProxyFromEnv])
	if err != nil {
		return Config{}, fmt.Errorf("%w: parse %s: %w", ErrInvalidConfig, EnvProxyFromEnv, err)
	}
	authMode, err := parseAuthMode(env[EnvAuthMode])
	if err != nil {
		return Config{}, err
	}

	clientSecret := secretref.Unset()
	if env[EnvClientSecret] != "" {
		clientSecret = secretref.Resolved("env", secret.New(env[EnvClientSecret]))
	}
	if env[EnvClientSecretFile] != "" {
		fileSecret, err := credentials.ReadOwnerOnlySecretFile(env[EnvClientSecretFile])
		if err != nil {
			return Config{}, fmt.Errorf("%w: load %s: %w", ErrInvalidConfig, EnvClientSecretFile, err)
		}
		if !clientSecret.IsConfigured() {
			clientSecret = secretref.Resolved("file", fileSecret)
		}
	}
	ziaPassword := secretref.Unset()
	if env[EnvZIAPassword] != "" {
		ziaPassword = secretref.Resolved("env", secret.New(env[EnvZIAPassword]))
	}
	if env[EnvZIAPasswordFile] != "" {
		fileSecret, err := credentials.ReadOwnerOnlySecretFile(env[EnvZIAPasswordFile])
		if err != nil {
			return Config{}, fmt.Errorf("%w: load %s: %w", ErrInvalidConfig, EnvZIAPasswordFile, err)
		}
		if !ziaPassword.IsConfigured() {
			ziaPassword = secretref.Resolved("file", fileSecret)
		}
	}
	ziaAPIKey := secretref.Unset()
	if env[EnvZIAAPIKey] != "" {
		ziaAPIKey = secretref.Resolved("env", secret.New(env[EnvZIAAPIKey]))
	}
	if env[EnvZIAAPIKeyFile] != "" {
		fileSecret, err := credentials.ReadOwnerOnlySecretFile(env[EnvZIAAPIKeyFile])
		if err != nil {
			return Config{}, fmt.Errorf("%w: load %s: %w", ErrInvalidConfig, EnvZIAAPIKeyFile, err)
		}
		if !ziaAPIKey.IsConfigured() {
			ziaAPIKey = secretref.Resolved("file", fileSecret)
		}
	}

	cfg := Config{
		Source:       "env",
		Profile:      env[EnvProfile],
		AuthMode:     authMode,
		VanityDomain: strings.TrimSpace(env[EnvVanityDomain]),
		Cloud:        strings.TrimSpace(env[EnvCloud]),
		Credentials: Credentials{
			ClientID:         secret.New(env[EnvClientID]),
			ClientSecret:     clientSecret,
			ClientSecretFile: env[EnvClientSecretFile],
		},
		ZPA: ZPAConfig{
			CustomerID:    strings.TrimSpace(env[EnvZPACustomerID]),
			MicrotenantID: strings.TrimSpace(env[EnvZPAMicrotenantID]),
		},
		ZIALegacy: ZIALegacyCredentials{
			Username:     secret.New(env[EnvZIAUsername]),
			Password:     ziaPassword,
			PasswordFile: env[EnvZIAPasswordFile],
			APIKey:       ziaAPIKey,
			APIKeyFile:   env[EnvZIAAPIKeyFile],
			Cloud:        strings.TrimSpace(env[EnvZIACloud]),
		},
		Proxy: Proxy{
			URL:             strings.TrimSpace(env[EnvProxyURL]),
			FromEnvironment: proxyFromEnv,
		},
		Defaults: Defaults{
			Redaction: mode,
			NoCache:   noCache,
		},
	}
	if cfg.Profile == "" {
		cfg.Profile = "default"
	}
	if cfg.AuthMode == "" {
		cfg.AuthMode = cfg.EffectiveAuthMode()
	}
	return cfg, nil
}

func (c Config) Safe() SafeConfig {
	return SafeConfig{
		Source:          c.Source,
		ConfigFileSet:   c.ConfigFile != "",
		Profile:         c.Profile,
		AuthMode:        string(c.EffectiveAuthMode()),
		VanityDomainSet: c.VanityDomain != "",
		Cloud:           c.Cloud,
		Credentials: CredentialStatus{
			ClientIDSet:         c.Credentials.ClientID.IsSet(),
			ClientSecretSet:     c.Credentials.ClientSecret.IsConfigured(),
			ClientSecretFileSet: c.Credentials.ClientSecretFile != "",
			ClientSecretScheme:  c.Credentials.ClientSecret.Scheme(),
		},
		ZPA: ZPAStatus{
			CustomerIDSet:    c.ZPA.CustomerID != "",
			MicrotenantIDSet: c.ZPA.MicrotenantID != "",
		},
		ZIALegacy: ZIALegacyStatus{
			UsernameSet:     c.ZIALegacy.Username.IsSet(),
			PasswordSet:     c.ZIALegacy.Password.IsConfigured(),
			PasswordFileSet: c.ZIALegacy.PasswordFile != "",
			PasswordScheme:  c.ZIALegacy.Password.Scheme(),
			APIKeySet:       c.ZIALegacy.APIKey.IsConfigured(),
			APIKeyFileSet:   c.ZIALegacy.APIKeyFile != "",
			APIKeyScheme:    c.ZIALegacy.APIKey.Scheme(),
			CloudSet:        c.ZIALegacy.Cloud != "",
		},
		Proxy: ProxyStatus{
			URLSet:          c.Proxy.URL != "",
			FromEnvironment: c.Proxy.FromEnvironment,
		},
		Defaults: DefaultsView{
			Redaction: string(c.Defaults.Redaction),
			NoCache:   c.Defaults.NoCache,
		},
	}
}

func (c Config) EffectiveAuthMode() AuthMode {
	if c.AuthMode != "" {
		return c.AuthMode
	}
	if c.ZIALegacy.AnySet() && !c.Credentials.AnySet() && c.VanityDomain == "" && c.Cloud == "" {
		return AuthModeZIALegacy
	}
	return AuthModeOneAPI
}

func (c Credentials) Configured(vanityDomain string) bool {
	return c.ClientID.IsSet() && c.ClientSecret.IsConfigured() && strings.TrimSpace(vanityDomain) != ""
}

func (c Credentials) AnySet() bool {
	return c.ClientID.IsSet() || c.ClientSecret.IsConfigured() || c.ClientSecretFile != ""
}

func (c ZIALegacyCredentials) Configured() bool {
	return c.Username.IsSet() && c.Password.IsConfigured() && c.APIKey.IsConfigured() && strings.TrimSpace(c.Cloud) != ""
}

func (c ZIALegacyCredentials) AnySet() bool {
	return c.Username.IsSet() || c.Password.IsConfigured() || c.PasswordFile != "" || c.APIKey.IsConfigured() || c.APIKeyFile != "" || strings.TrimSpace(c.Cloud) != ""
}

func parseAuthMode(value string) (AuthMode, error) {
	switch mode := AuthMode(strings.TrimSpace(strings.ToLower(value))); mode {
	case "":
		return "", nil
	case AuthModeOneAPI, AuthModeZIALegacy:
		return mode, nil
	default:
		return "", fmt.Errorf("%w: parse %s: unsupported auth mode; supported: oneapi, zia-legacy", ErrInvalidConfig, EnvAuthMode)
	}
}

func parseEnv(environ []string) map[string]string {
	out := make(map[string]string, len(environ))
	for _, entry := range environ {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		out[key] = value
	}
	return out
}

func parseBoolEnv(value string) (bool, error) {
	if value == "" {
		return false, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, errors.New("must be true or false")
	}
	return parsed, nil
}
