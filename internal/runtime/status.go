package runtime

import (
	"strings"
	"time"

	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/zscaler"
)

// StatusOptions configures shared status computations.
type StatusOptions struct {
	Timeout time.Duration
}

// DoctorStatus is the structured diagnostic status for the active runtime
// configuration.
type DoctorStatus struct {
	Status      string `json:"status"`
	Mode        string `json:"mode"`
	Profile     string `json:"profile"`
	Config      string `json:"config"`
	AuthMode    string `json:"auth_mode"`
	Redaction   string `json:"redaction"`
	Timeout     string `json:"timeout"`
	Cache       string `json:"cache"`
	Proxy       string `json:"proxy"`
	Credentials string `json:"credentials"`
	LiveAPI     string `json:"live_api"`
}

func (DoctorStatus) OutputSafe() {}

// AuthStatus is the structured authentication status for the active runtime
// configuration.
type AuthStatus struct {
	Credentials        string `json:"credentials"`
	CredentialExchange string `json:"credential_exchange"`
	LiveAPI            string `json:"live_api"`
}

func (AuthStatus) OutputSafe() {}

// NewDoctorStatus validates status-relevant runtime configuration and returns
// value-only diagnostic status for rendering by callers.
func NewDoctorStatus(cfg config.Config, opts StatusOptions) (DoctorStatus, error) {
	if err := zscaler.ValidateProxyConfig(zscaler.ProxyConfig{
		URL:             cfg.Proxy.URL,
		FromEnvironment: cfg.Proxy.FromEnvironment,
	}); err != nil {
		return DoctorStatus{}, err
	}
	return DoctorStatus{
		Status:      "OK",
		Mode:        "read-only",
		Profile:     cfg.Profile,
		Config:      ConfigSourceStatus(cfg.Safe()),
		AuthMode:    string(cfg.EffectiveAuthMode()),
		Redaction:   string(cfg.Defaults.Redaction),
		Timeout:     opts.Timeout.String(),
		Cache:       CacheStatus(cfg.Defaults.NoCache),
		Proxy:       ProxyStatus(cfg.Proxy),
		Credentials: CredentialStatus(cfg),
		LiveAPI:     LiveAPIStatus(cfg),
	}, nil
}

// NewAuthStatus returns value-only authentication status for rendering by
// callers.
func NewAuthStatus(cfg config.Config) AuthStatus {
	return AuthStatus{
		Credentials:        CredentialStatus(cfg),
		CredentialExchange: "not requested",
		LiveAPI:            LiveAPIStatus(cfg),
	}
}

// NewConfigStatus returns the redacted/safe configuration view used by status
// renderers.
func NewConfigStatus(cfg config.Config) config.SafeConfig {
	return cfg.Safe()
}

// CredentialStatus reports whether the active auth mode has enough configured
// fields for live read-only access.
func CredentialStatus(cfg config.Config) string {
	switch cfg.EffectiveAuthMode() {
	case config.AuthModeZIALegacy:
		if cfg.ZIALegacy.Configured() {
			return "configured"
		}
		if cfg.ZIALegacy.AnySet() {
			return "partial"
		}
		return "not configured"
	default:
		if cfg.Credentials.Configured(cfg.VanityDomain) {
			return "configured"
		}
		if cfg.Credentials.AnySet() || cfg.VanityDomain != "" {
			return "partial"
		}
		return "not configured"
	}
}

// LiveAPIStatus reports the live read-only availability implied by the active
// credential status.
func LiveAPIStatus(cfg config.Config) string {
	if CredentialStatus(cfg) == "configured" {
		if cfg.EffectiveAuthMode() != config.AuthModeZIALegacy && strings.TrimSpace(cfg.ZPA.CustomerID) == "" {
			return "available for ZIA read-only commands; ZPA resources require ZSCALERCTL_ZPA_CUSTOMER_ID"
		}
		return "available for read-only commands"
	}
	if cfg.EffectiveAuthMode() == config.AuthModeZIALegacy {
		return "requires ZSCALERCTL_ZIA_USERNAME, ZSCALERCTL_ZIA_PASSWORD, ZSCALERCTL_ZIA_API_KEY, and ZSCALERCTL_ZIA_CLOUD"
	}
	return "requires ZSCALERCTL_CLIENT_ID, ZSCALERCTL_CLIENT_SECRET, and ZSCALERCTL_VANITY_DOMAIN; ZPA resources also require ZSCALERCTL_ZPA_CUSTOMER_ID"
}

// SetStatus renders the common set/unset status for configuration booleans.
func SetStatus(set bool) string {
	if set {
		return "set"
	}
	return "unset"
}

// SecretSourceStatus renders a secret source without exposing its value.
func SecretSourceStatus(set bool, scheme string) string {
	if !set {
		return "unset"
	}
	if scheme == "" {
		return "set"
	}
	return "set (" + scheme + ")"
}

// ConfigSourceStatus reports whether the active config came from a file or the
// environment.
func ConfigSourceStatus(safe config.SafeConfig) string {
	if safe.ConfigFileSet {
		return "config file"
	}
	return "environment"
}

// CacheStatus reports whether SDK cache bypass is configured.
func CacheStatus(noCache bool) string {
	if noCache {
		return "bypass"
	}
	return "default"
}

// ProxyStatus reports the active proxy configuration source without exposing
// proxy values.
func ProxyStatus(proxy config.Proxy) string {
	switch {
	case proxy.FromEnvironment:
		return "environment"
	case strings.TrimSpace(proxy.URL) != "":
		return "explicit"
	default:
		return "direct"
	}
}

// ValueOrUnset renders trimmed configuration values that are safe to disclose.
func ValueOrUnset(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unset"
	}
	return value
}
