package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/dvmrry/zscalerctl/internal/credentials"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/secret"
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
	Profile      string
	AuthMode     AuthMode
	VanityDomain string
	Cloud        string
	Credentials  Credentials
	ZPA          ZPAConfig
	ZIALegacy    ZIALegacyCredentials
	Proxy        Proxy
	Defaults     Defaults
}

type Credentials struct {
	ClientID         secret.Secret
	ClientSecret     secret.Secret
	ClientSecretFile string
}

type ZPAConfig struct {
	CustomerID    string
	MicrotenantID string
}

type ZIALegacyCredentials struct {
	Username     secret.Secret
	Password     secret.Secret
	PasswordFile string
	APIKey       secret.Secret
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
	ClientIDSet         bool `json:"client_id_set"`
	ClientSecretSet     bool `json:"client_secret_set"`
	ClientSecretFileSet bool `json:"client_secret_file_set"`
}

type ZPAStatus struct {
	CustomerIDSet    bool `json:"customer_id_set"`
	MicrotenantIDSet bool `json:"microtenant_id_set"`
}

type ZIALegacyStatus struct {
	UsernameSet     bool `json:"username_set"`
	PasswordSet     bool `json:"password_set"`
	PasswordFileSet bool `json:"password_file_set"`
	APIKeySet       bool `json:"api_key_set"`
	APIKeyFileSet   bool `json:"api_key_file_set"`
	CloudSet        bool `json:"cloud_set"`
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

	clientSecret := secret.New(env[EnvClientSecret])
	if env[EnvClientSecretFile] != "" {
		fileSecret, err := credentials.ReadOwnerOnlySecretFile(env[EnvClientSecretFile])
		if err != nil {
			return Config{}, fmt.Errorf("%w: load %s: %w", ErrInvalidConfig, EnvClientSecretFile, err)
		}
		if !clientSecret.IsSet() {
			clientSecret = fileSecret
		}
	}
	ziaPassword := secret.New(env[EnvZIAPassword])
	if env[EnvZIAPasswordFile] != "" {
		fileSecret, err := credentials.ReadOwnerOnlySecretFile(env[EnvZIAPasswordFile])
		if err != nil {
			return Config{}, fmt.Errorf("%w: load %s: %w", ErrInvalidConfig, EnvZIAPasswordFile, err)
		}
		if !ziaPassword.IsSet() {
			ziaPassword = fileSecret
		}
	}
	ziaAPIKey := secret.New(env[EnvZIAAPIKey])
	if env[EnvZIAAPIKeyFile] != "" {
		fileSecret, err := credentials.ReadOwnerOnlySecretFile(env[EnvZIAAPIKeyFile])
		if err != nil {
			return Config{}, fmt.Errorf("%w: load %s: %w", ErrInvalidConfig, EnvZIAAPIKeyFile, err)
		}
		if !ziaAPIKey.IsSet() {
			ziaAPIKey = fileSecret
		}
	}

	cfg := Config{
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
		Profile:         c.Profile,
		AuthMode:        string(c.EffectiveAuthMode()),
		VanityDomainSet: c.VanityDomain != "",
		Cloud:           c.Cloud,
		Credentials: CredentialStatus{
			ClientIDSet:         c.Credentials.ClientID.IsSet(),
			ClientSecretSet:     c.Credentials.ClientSecret.IsSet(),
			ClientSecretFileSet: c.Credentials.ClientSecretFile != "",
		},
		ZPA: ZPAStatus{
			CustomerIDSet:    c.ZPA.CustomerID != "",
			MicrotenantIDSet: c.ZPA.MicrotenantID != "",
		},
		ZIALegacy: ZIALegacyStatus{
			UsernameSet:     c.ZIALegacy.Username.IsSet(),
			PasswordSet:     c.ZIALegacy.Password.IsSet(),
			PasswordFileSet: c.ZIALegacy.PasswordFile != "",
			APIKeySet:       c.ZIALegacy.APIKey.IsSet(),
			APIKeyFileSet:   c.ZIALegacy.APIKeyFile != "",
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
	return c.ClientID.IsSet() && c.ClientSecret.IsSet() && strings.TrimSpace(vanityDomain) != ""
}

func (c Credentials) AnySet() bool {
	return c.ClientID.IsSet() || c.ClientSecret.IsSet() || c.ClientSecretFile != ""
}

func (c ZIALegacyCredentials) Configured() bool {
	return c.Username.IsSet() && c.Password.IsSet() && c.APIKey.IsSet() && strings.TrimSpace(c.Cloud) != ""
}

func (c ZIALegacyCredentials) AnySet() bool {
	return c.Username.IsSet() || c.Password.IsSet() || c.PasswordFile != "" || c.APIKey.IsSet() || c.APIKeyFile != "" || strings.TrimSpace(c.Cloud) != ""
}

func parseAuthMode(value string) (AuthMode, error) {
	switch mode := AuthMode(strings.TrimSpace(strings.ToLower(value))); mode {
	case "":
		return "", nil
	case AuthModeOneAPI, AuthModeZIALegacy:
		return mode, nil
	default:
		return "", fmt.Errorf("%w: parse %s: unsupported auth mode %q", ErrInvalidConfig, EnvAuthMode, value)
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
