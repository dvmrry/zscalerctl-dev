package runtime

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
	"unicode"

	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/secretref"
	"github.com/dvmrry/zscalerctl/internal/zscaler"
)

// StatusOptions configures shared status computations.
type StatusOptions struct {
	Timeout time.Duration
}

// Keep the existing runtime-owned names available to CLI rendering code while
// the canonical safe value types live at the typed engine boundary.
type DoctorStatus = machine.DoctorStatus
type AuthStatus = machine.AuthStatus
type ConfigStatus = machine.ConfigStatus

// StatusInspector retains only precomputed sanitized views. Raw config,
// credentials, secret references, provider commands, and proxy values are not
// retained after construction.
type StatusInspector struct {
	doctor    machine.DoctorStatus
	doctorErr error
	auth      machine.AuthStatus
	config    machine.ConfigStatus
}

// NewStatusInspector loads effective config without resolving provider-backed
// secret references or constructing an SDK reader.
func NewStatusInspector(ctx context.Context, opts Options) (*StatusInspector, error) {
	return newStatusInspector(ctx, opts, machine.OperationConfigStatus)
}

func newStatusInspector(
	ctx context.Context,
	opts Options,
	operation machine.Operation,
) (*StatusInspector, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, statusContextError(err, operation)
	}
	env := append([]string(nil), opts.Env...)
	loadConfig := opts.loadConfig
	if loadConfig == nil {
		loadConfig = config.LoadConfig
	}
	cfg, err := loadConfig(env, config.LoadOptions{
		Profile:    opts.Profile,
		ConfigPath: opts.ConfigPath,
	})
	if err != nil {
		return nil, StatusConfigError(operation, err)
	}
	return newStatusInspectorFromConfig(ctx, cfg, opts, operation)
}

// NewStatusInspectorFromConfig computes sanitized status views from an
// already-loaded config without resolving secrets or constructing a reader.
func NewStatusInspectorFromConfig(
	ctx context.Context,
	cfg config.Config,
	opts Options,
) (*StatusInspector, error) {
	return newStatusInspectorFromConfig(ctx, cfg, opts, machine.OperationConfigStatus)
}

func newStatusInspectorFromConfig(
	ctx context.Context,
	cfg config.Config,
	opts Options,
	operation machine.Operation,
) (*StatusInspector, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, statusContextError(err, operation)
	}
	cfg = normalizeStatusConfig(cfg)
	if err := applyOptions(&cfg, opts); err != nil {
		return nil, StatusConfigError(operation, err)
	}
	doctor, doctorErr := NewDoctorStatus(cfg, StatusOptions{Timeout: opts.Timeout})
	if doctorErr != nil {
		doctorErr = StatusConfigError(machine.OperationDoctor, doctorErr)
	}
	return &StatusInspector{
		doctor:    doctor,
		doctorErr: doctorErr,
		auth:      NewAuthStatus(cfg),
		config:    NewConfigStatus(cfg),
	}, nil
}

// Inspect returns one closed typed status result. It performs no filesystem,
// provider, process, SDK, or network work after construction.
func (s *StatusInspector) Inspect(
	ctx context.Context,
	req machine.StatusRequest,
) (machine.StatusResult, error) {
	if s == nil {
		return machine.StatusResult{}, errors.New("status inspector is nil")
	}
	if !isSupportedStatusOperation(req.Operation) {
		return machine.StatusResult{}, unsupportedStatusOperationError()
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return machine.StatusResult{}, statusContextError(err, req.Operation)
	}
	switch req.Operation {
	case machine.OperationDoctor:
		if s.doctorErr != nil {
			return machine.StatusResult{}, s.doctorErr
		}
		return machine.NewDoctorStatusResult(s.doctor), nil
	case machine.OperationAuthStatus:
		return machine.NewAuthStatusResult(s.auth), nil
	case machine.OperationConfigStatus:
		return machine.NewConfigStatusResult(s.config), nil
	}
	return machine.StatusResult{}, unsupportedStatusOperationError()
}

// NewDoctorStatus validates status-relevant runtime configuration and returns
// a sanitized value-only status for rendering by callers.
func NewDoctorStatus(cfg config.Config, opts StatusOptions) (DoctorStatus, error) {
	cfg = normalizeStatusConfig(cfg)
	if err := zscaler.ValidateProxyConfig(zscaler.ProxyConfig{
		URL:             cfg.Proxy.URL,
		FromEnvironment: cfg.Proxy.FromEnvironment,
	}); err != nil {
		return DoctorStatus{}, fmt.Errorf("%w: proxy settings are invalid", zscaler.ErrInvalidProxyConfig)
	}
	r := redact.New(cfg.Defaults.Redaction)
	return DoctorStatus{
		Status:      sanitizeStatusString(r, "OK"),
		Mode:        sanitizeStatusString(r, "read-only"),
		Profile:     sanitizeStatusString(r, cfg.Profile),
		Config:      sanitizeStatusString(r, ConfigSourceStatus(cfg.Safe())),
		AuthMode:    sanitizeStatusString(r, string(cfg.EffectiveAuthMode())),
		Redaction:   sanitizeStatusString(r, string(cfg.Defaults.Redaction)),
		Timeout:     sanitizeStatusString(r, opts.Timeout.String()),
		Cache:       sanitizeStatusString(r, CacheStatus(cfg.Defaults.NoCache)),
		Proxy:       sanitizeStatusString(r, ProxyStatus(cfg.Proxy)),
		Credentials: sanitizeStatusString(r, CredentialStatus(cfg)),
		LiveAPI:     sanitizeStatusString(r, LiveAPIStatus(cfg)),
	}, nil
}

// NewAuthStatus returns sanitized value-only authentication status.
func NewAuthStatus(cfg config.Config) AuthStatus {
	cfg = normalizeStatusConfig(cfg)
	r := redact.New(cfg.Defaults.Redaction)
	return AuthStatus{
		Credentials:        sanitizeStatusString(r, CredentialStatus(cfg)),
		CredentialExchange: sanitizeStatusString(r, "not requested"),
		LiveAPI:            sanitizeStatusString(r, LiveAPIStatus(cfg)),
	}
}

// NewConfigStatus returns the sanitized configuration-presence view used by
// status renderers. No raw config path, proxy URL, identifier, or secret is
// copied into the result.
func NewConfigStatus(cfg config.Config) ConfigStatus {
	cfg = normalizeStatusConfig(cfg)
	safe := cfg.Safe()
	r := redact.New(cfg.Defaults.Redaction)
	return ConfigStatus{
		Source:          sanitizeStatusString(r, safe.Source),
		ConfigFileSet:   safe.ConfigFileSet,
		Profile:         sanitizeStatusString(r, safe.Profile),
		AuthMode:        sanitizeStatusString(r, safe.AuthMode),
		VanityDomainSet: safe.VanityDomainSet,
		Cloud:           sanitizeStatusString(r, safe.Cloud),
		Credentials: machine.ConfigCredentialStatus{
			ClientIDSet:         safe.Credentials.ClientIDSet,
			ClientSecretSet:     safe.Credentials.ClientSecretSet,
			ClientSecretFileSet: safe.Credentials.ClientSecretFileSet,
			ClientSecretScheme:  sanitizeStatusString(r, safe.Credentials.ClientSecretScheme),
		},
		ZPA: machine.ConfigZPAStatus{
			CustomerIDSet:    safe.ZPA.CustomerIDSet,
			MicrotenantIDSet: safe.ZPA.MicrotenantIDSet,
		},
		ZIALegacy: machine.ConfigZIALegacyStatus{
			UsernameSet:     safe.ZIALegacy.UsernameSet,
			PasswordSet:     safe.ZIALegacy.PasswordSet,
			PasswordFileSet: safe.ZIALegacy.PasswordFileSet,
			PasswordScheme:  sanitizeStatusString(r, safe.ZIALegacy.PasswordScheme),
			APIKeySet:       safe.ZIALegacy.APIKeySet,
			APIKeyFileSet:   safe.ZIALegacy.APIKeyFileSet,
			APIKeyScheme:    sanitizeStatusString(r, safe.ZIALegacy.APIKeyScheme),
			CloudSet:        safe.ZIALegacy.CloudSet,
		},
		Proxy: machine.ConfigProxyStatus{
			URLSet:          safe.Proxy.URLSet,
			FromEnvironment: safe.Proxy.FromEnvironment,
		},
		Defaults: machine.ConfigDefaultsStatus{
			Redaction: sanitizeStatusString(r, safe.Defaults.Redaction),
			NoCache:   safe.Defaults.NoCache,
		},
	}
}

func sanitizeStatusString(r redact.Redactor, value string) string {
	value, _ = r.ScanRenderedString(value)
	return strings.Map(func(ch rune) rune {
		if unicode.IsControl(ch) || unicode.Is(unicode.Cf, ch) {
			return ' '
		}
		return ch
	}, value)
}

func statusContextError(err error, operation machine.Operation) error {
	kind := machine.ErrorKindCanceled
	message := "request canceled"
	sentinel := context.Canceled
	if errors.Is(err, context.DeadlineExceeded) {
		kind = machine.ErrorKindDeadlineExceeded
		message = "request deadline exceeded"
		sentinel = context.DeadlineExceeded
	}
	return newStatusBoundaryError(&machine.MachineError{
		Kind:      kind,
		Message:   message,
		Operation: operation,
	}, sentinel)
}

// StatusConfigError converts a status configuration failure into a static
// machine-safe error. It preserves only safe sentinel identity for in-process
// classification and never retains the original error or its details.
func StatusConfigError(operation machine.Operation, err error) error {
	if err == nil {
		return nil
	}
	if !isSupportedStatusOperation(operation) {
		operation = ""
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return statusContextError(err, operation)
	}
	if errors.Is(err, config.ErrInvalidConfig) {
		return newStatusBoundaryError(&machine.MachineError{
			Kind:      machine.ErrorKindInvalidConfig,
			Message:   "invalid configuration",
			Operation: operation,
		}, config.ErrInvalidConfig)
	}
	if errors.Is(err, zscaler.ErrInvalidProxyConfig) {
		return newStatusBoundaryError(&machine.MachineError{
			Kind:      machine.ErrorKindInvalidProxyConfig,
			Message:   "invalid proxy configuration",
			Operation: operation,
		}, zscaler.ErrInvalidProxyConfig)
	}
	return &machine.MachineError{
		Kind:      machine.ErrorKindInternal,
		Message:   "status configuration load failed",
		Operation: operation,
	}
}

type statusBoundaryError struct {
	machineErr *machine.MachineError
	sentinel   error
}

func (e *statusBoundaryError) Error() string { return e.machineErr.Error() }

func (e *statusBoundaryError) Unwrap() []error {
	return []error{e.machineErr, e.sentinel}
}

func newStatusBoundaryError(machineErr *machine.MachineError, sentinel error) error {
	if sentinel == nil {
		return machineErr
	}
	return &statusBoundaryError{machineErr: machineErr, sentinel: sentinel}
}

func isSupportedStatusOperation(operation machine.Operation) bool {
	switch operation {
	case machine.OperationDoctor, machine.OperationAuthStatus, machine.OperationConfigStatus:
		return true
	default:
		return false
	}
}

func unsupportedStatusOperationError() error {
	return &machine.MachineError{
		Kind:    machine.ErrorKindUnsupportedOperation,
		Message: "unsupported status operation",
	}
}

func normalizeStatusConfig(cfg config.Config) config.Config {
	if nilSecretSource(cfg.Credentials.ClientSecret) {
		cfg.Credentials.ClientSecret = secretref.Unset()
	}
	if nilSecretSource(cfg.ZIALegacy.Password) {
		cfg.ZIALegacy.Password = secretref.Unset()
	}
	if nilSecretSource(cfg.ZIALegacy.APIKey) {
		cfg.ZIALegacy.APIKey = secretref.Unset()
	}
	return cfg
}

func nilSecretSource(source secretref.SecretSource) bool {
	if source == nil {
		return true
	}
	value := reflect.ValueOf(source)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

// CredentialStatus reports whether the active auth mode has enough configured
// fields for live read-only access.
func CredentialStatus(cfg config.Config) string {
	cfg = normalizeStatusConfig(cfg)
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
	cfg = normalizeStatusConfig(cfg)
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
	return ConfigFileStatus(safe.ConfigFileSet)
}

// ConfigFileStatus reports the config source from the safe presence boolean.
func ConfigFileStatus(configFileSet bool) string {
	if configFileSet {
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
