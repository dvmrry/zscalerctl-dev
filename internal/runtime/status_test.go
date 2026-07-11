package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/dvmrry/zscalerctl/internal/browser"
	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/machine"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/secret"
	"github.com/dvmrry/zscalerctl/internal/zscaler"
)

func TestNewDoctorStatusComputesConfiguredRuntimeStatus(t *testing.T) {
	t.Parallel()

	cfg, err := config.LoadConfig([]string{
		config.EnvClientID + "=client-id",
		config.EnvClientSecret + "=client-secret",
		config.EnvVanityDomain + "=example",
		config.EnvCloud + "=PRODUCTION",
		config.EnvRedaction + "=share",
		config.EnvNoCache + "=true",
		config.EnvProxyFromEnv + "=true",
	}, config.LoadOptions{})
	if err != nil {
		t.Fatalf("config.LoadConfig(configured env) error = %v, want nil", err)
	}

	status, err := NewDoctorStatus(cfg, StatusOptions{Timeout: 11 * time.Second})
	if err != nil {
		t.Fatalf("NewDoctorStatus(configured env) error = %v, want nil", err)
	}

	if status.Status != "OK" ||
		status.Mode != "read-only" ||
		status.Profile != "default" ||
		status.Config != "environment" ||
		status.AuthMode != string(config.AuthModeOneAPI) ||
		status.Redaction != "share" ||
		status.Timeout != "11s" ||
		status.Cache != "bypass" ||
		status.Proxy != "environment" ||
		status.Credentials != "configured" {
		t.Fatalf("NewDoctorStatus(configured env) = %#v, want configured read-only status", status)
	}
	if got, want := status.LiveAPI, "available for ZIA read-only commands; ZPA resources require ZSCALERCTL_ZPA_CUSTOMER_ID"; got != want {
		t.Fatalf("NewDoctorStatus(configured env).LiveAPI = %q, want %q", got, want)
	}
}

func TestNewAuthStatusComputesLegacyStatus(t *testing.T) {
	t.Parallel()

	cfg, err := config.LoadConfig([]string{
		config.EnvAuthMode + "=" + string(config.AuthModeZIALegacy),
		config.EnvZIAUsername + "=admin@example.invalid",
		config.EnvZIAPassword + "=legacy-password",
		config.EnvZIAAPIKey + "=legacy-api-key",
		config.EnvZIACloud + "=zscalerthree",
	}, config.LoadOptions{})
	if err != nil {
		t.Fatalf("config.LoadConfig(legacy env) error = %v, want nil", err)
	}

	status := NewAuthStatus(cfg)
	if status.Credentials != "configured" ||
		status.CredentialExchange != "not requested" ||
		status.LiveAPI != "available for read-only commands" {
		t.Fatalf("NewAuthStatus(legacy env) = %#v, want configured legacy status", status)
	}
}

func TestNewConfigStatusReturnsSafeConfig(t *testing.T) {
	t.Parallel()

	cfg, err := config.LoadConfig([]string{
		config.EnvClientID + "=client-id",
		config.EnvClientSecret + "=client-secret",
		config.EnvVanityDomain + "=example",
		config.EnvZPACustomerID + "=customer-id",
		config.EnvZPAMicrotenantID + "=microtenant-id",
	}, config.LoadOptions{})
	if err != nil {
		t.Fatalf("config.LoadConfig(config env) error = %v, want nil", err)
	}

	gotBody, err := json.Marshal(NewConfigStatus(cfg))
	if err != nil {
		t.Fatalf("json.Marshal(NewConfigStatus(config env)) error = %v, want nil", err)
	}
	wantBody, err := json.Marshal(cfg.Safe())
	if err != nil {
		t.Fatalf("json.Marshal(Config.Safe(config env)) error = %v, want nil", err)
	}
	if string(gotBody) != string(wantBody) {
		t.Fatalf("NewConfigStatus(config env) JSON = %s, want %s", gotBody, wantBody)
	}
}

func TestNewDoctorStatusRejectsConflictingProxyConfig(t *testing.T) {
	t.Parallel()

	cfg, err := config.LoadConfig([]string{
		config.EnvProxyURL + "=http://proxy.example.invalid:8080",
		config.EnvProxyFromEnv + "=true",
	}, config.LoadOptions{})
	if err != nil {
		t.Fatalf("config.LoadConfig(conflicting proxy env) error = %v, want nil", err)
	}

	_, err = NewDoctorStatus(cfg, StatusOptions{})
	if !errors.Is(err, zscaler.ErrInvalidProxyConfig) {
		t.Fatalf("NewDoctorStatus(conflicting proxy env) error = %v, want ErrInvalidProxyConfig", err)
	}
}

func TestNewDoctorStatusSanitizesInvalidProxyError(t *testing.T) {
	t.Parallel()

	const canary = "proxy-error-secret-canary"
	cfg, err := config.LoadConfig([]string{
		config.EnvProxyURL + "=://" + canary,
	}, config.LoadOptions{})
	if err != nil {
		t.Fatalf("config.LoadConfig(invalid proxy env) error = %v, want nil", err)
	}
	_, err = NewDoctorStatus(cfg, StatusOptions{})
	if !errors.Is(err, zscaler.ErrInvalidProxyConfig) {
		t.Fatalf("NewDoctorStatus(invalid proxy env) error = %v, want ErrInvalidProxyConfig", err)
	}
	if strings.Contains(err.Error(), canary) {
		t.Fatalf("NewDoctorStatus(invalid proxy env) error = %q, want no canary", err)
	}
}

func TestStatusInspectorReturnsClosedViewsWithoutConstructingReader(t *testing.T) {
	t.Parallel()

	cfg, err := config.LoadConfig([]string{
		config.EnvClientID + "=client-id",
		config.EnvClientSecret + "=client-secret",
		config.EnvVanityDomain + "=example",
	}, config.LoadOptions{})
	if err != nil {
		t.Fatalf("config.LoadConfig(status inspector fixture) error = %v, want nil", err)
	}
	provider := &statusProviderSecret{}
	cfg.Credentials.ClientSecret = provider
	readerConstructed := false
	inspector, err := NewStatusInspectorFromConfig(context.Background(), cfg, Options{
		Timeout: 13 * time.Second,
		newReader: func(zscaler.ReaderConfig) (browser.RecordReader, error) {
			readerConstructed = true
			return nil, errors.New("reader must not be constructed")
		},
	})
	if err != nil {
		t.Fatalf("NewStatusInspectorFromConfig() error = %v, want nil", err)
	}
	if readerConstructed {
		t.Fatal("NewStatusInspectorFromConfig() constructed reader, want SDK-free status")
	}
	if provider.resolveCalls != 0 {
		t.Fatalf("NewStatusInspectorFromConfig() provider resolve calls = %d, want 0", provider.resolveCalls)
	}

	operations := []machine.Operation{
		machine.OperationDoctor,
		machine.OperationAuthStatus,
		machine.OperationConfigStatus,
	}
	for _, operation := range operations {
		result, err := inspector.Inspect(context.Background(), machine.StatusRequest{Operation: operation})
		if err != nil {
			t.Fatalf("StatusInspector.Inspect(%s) error = %v, want nil", operation, err)
		}
		if result.Operation() != operation {
			t.Fatalf("StatusInspector.Inspect(%s).Operation() = %q", operation, result.Operation())
		}
		switch operation {
		case machine.OperationDoctor:
			status, ok := result.Doctor()
			if !ok || status.Timeout != "13s" || status.Credentials != "configured" {
				t.Fatalf("StatusInspector.Inspect(doctor) = %#v/%t, want configured 13s status", status, ok)
			}
		case machine.OperationAuthStatus:
			status, ok := result.Auth()
			if !ok || status.Credentials != "configured" {
				t.Fatalf("StatusInspector.Inspect(auth) = %#v/%t, want configured", status, ok)
			}
		case machine.OperationConfigStatus:
			status, ok := result.Config()
			if !ok || !status.Credentials.ClientIDSet || !status.Credentials.ClientSecretSet {
				t.Fatalf("StatusInspector.Inspect(config) = %#v/%t, want configured presence", status, ok)
			}
		}
	}

	const unknownCanary = "unknown-status-operation-canary"
	_, err = inspector.Inspect(context.Background(), machine.StatusRequest{Operation: unknownCanary})
	var machineErr *machine.MachineError
	if !errors.As(err, &machineErr) || machineErr.Kind != machine.ErrorKindUnsupportedOperation {
		t.Fatalf("StatusInspector.Inspect(unknown) error = %v, want unsupported-operation MachineError", err)
	}
	if machineErr.Operation != "" || strings.Contains(err.Error(), unknownCanary) {
		t.Fatalf("StatusInspector.Inspect(unknown) error = %#v, want no echoed operation", machineErr)
	}
}

func TestStatusInspectorPreservesOperationSpecificProxyValidation(t *testing.T) {
	t.Parallel()

	cfg, err := config.LoadConfig([]string{
		config.EnvProxyURL + "=http://proxy.example.invalid:8080",
		config.EnvProxyFromEnv + "=true",
	}, config.LoadOptions{})
	if err != nil {
		t.Fatalf("config.LoadConfig(conflicting proxy env) error = %v, want nil", err)
	}
	inspector, err := NewStatusInspectorFromConfig(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("NewStatusInspectorFromConfig(conflicting proxy) error = %v, want nil", err)
	}
	if _, err := inspector.Inspect(context.Background(), machine.StatusRequest{Operation: machine.OperationAuthStatus}); err != nil {
		t.Fatalf("StatusInspector.Inspect(auth conflicting proxy) error = %v, want nil", err)
	}
	_, err = inspector.Inspect(context.Background(), machine.StatusRequest{Operation: machine.OperationDoctor})
	if !errors.Is(err, zscaler.ErrInvalidProxyConfig) {
		t.Fatalf("StatusInspector.Inspect(doctor conflicting proxy) error = %v, want ErrInvalidProxyConfig", err)
	}
}

func TestStatusInspectorSanitizesStringsBeforeReturning(t *testing.T) {
	t.Parallel()

	const canary = "status-provider-secret-canary"
	cfg := config.Config{
		Profile: "psk=" + canary,
		Cloud:   "Authorization: Bearer " + canary,
		Defaults: config.Defaults{
			Redaction: redact.ModeStandard,
		},
	}
	inspector, err := NewStatusInspectorFromConfig(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("NewStatusInspectorFromConfig(secret-like status strings) error = %v, want nil", err)
	}
	for _, operation := range []machine.Operation{machine.OperationDoctor, machine.OperationConfigStatus} {
		result, err := inspector.Inspect(context.Background(), machine.StatusRequest{Operation: operation})
		if err != nil {
			t.Fatalf("StatusInspector.Inspect(%s) error = %v, want nil", operation, err)
		}
		var value any
		switch operation {
		case machine.OperationDoctor:
			value, _ = result.Doctor()
		case machine.OperationConfigStatus:
			value, _ = result.Config()
		}
		body, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("json.Marshal(StatusInspector.Inspect(%s)) error = %v", operation, err)
		}
		if strings.Contains(string(body), canary) {
			t.Fatalf("StatusInspector.Inspect(%s) JSON = %s, want no canary", operation, body)
		}
	}
}

func TestStatusInspectorNormalizesTerminalControlAndFormatRunes(t *testing.T) {
	t.Parallel()

	const injected = "safe\nFORGED\x1b[31m\u0085\u202e"
	provider := &statusProviderSecret{scheme: "cmd:" + injected}
	cfg := config.Config{
		Profile: "profile:" + injected,
		Cloud:   "cloud:" + injected,
		Credentials: config.Credentials{
			ClientSecret: provider,
		},
		Defaults: config.Defaults{Redaction: redact.ModeStandard},
	}
	inspector, err := NewStatusInspectorFromConfig(context.Background(), cfg, Options{})
	if err != nil {
		t.Fatalf("NewStatusInspectorFromConfig(control strings) error = %v, want nil", err)
	}
	for _, operation := range []machine.Operation{
		machine.OperationDoctor,
		machine.OperationAuthStatus,
		machine.OperationConfigStatus,
	} {
		result, err := inspector.Inspect(context.Background(), machine.StatusRequest{Operation: operation})
		if err != nil {
			t.Fatalf("StatusInspector.Inspect(%s) error = %v, want nil", operation, err)
		}
		var value any
		switch operation {
		case machine.OperationDoctor:
			value, _ = result.Doctor()
		case machine.OperationAuthStatus:
			value, _ = result.Auth()
		case machine.OperationConfigStatus:
			value, _ = result.Config()
		}
		assertNoUnsafeStatusRunes(t, operation, value)
	}
	if provider.resolveCalls != 0 {
		t.Fatalf("status control normalization provider resolves = %d, want 0", provider.resolveCalls)
	}
}

func TestNewStatusInspectorFromConfigSanitizesOptionErrors(t *testing.T) {
	t.Parallel()

	_, err := NewStatusInspectorFromConfig(context.Background(), config.Config{}, Options{
		Redaction:    redact.Mode("private-option-canary"),
		RedactionSet: true,
	})
	var machineErr *machine.MachineError
	if !errors.As(err, &machineErr) || machineErr.Kind != machine.ErrorKindInvalidConfig {
		t.Fatalf("NewStatusInspectorFromConfig(invalid option) error = %v, want invalid-config MachineError", err)
	}
	if !errors.Is(err, config.ErrInvalidConfig) {
		t.Fatalf("NewStatusInspectorFromConfig(invalid option) error = %v, want ErrInvalidConfig", err)
	}
	if strings.Contains(err.Error(), "canary") {
		t.Fatalf("NewStatusInspectorFromConfig(invalid option) error = %q, want no raw option", err)
	}
}

func TestStatusConfigErrorDoesNotEchoUnknownOperation(t *testing.T) {
	t.Parallel()

	const canary = "unknown-status-error-operation-canary"
	err := StatusConfigError(machine.Operation(canary), config.ErrInvalidConfig)
	var machineErr *machine.MachineError
	if !errors.As(err, &machineErr) || machineErr.Kind != machine.ErrorKindInvalidConfig {
		t.Fatalf("StatusConfigError(unknown operation) = %v, want invalid-config MachineError", err)
	}
	if machineErr.Operation != "" || strings.Contains(err.Error(), canary) {
		t.Fatalf("StatusConfigError(unknown operation) = %#v, want no echoed operation", machineErr)
	}
}

func TestStatusInspectorMapsCanceledContext(t *testing.T) {
	t.Parallel()

	inspector, err := NewStatusInspectorFromConfig(context.Background(), config.Config{}, Options{})
	if err != nil {
		t.Fatalf("NewStatusInspectorFromConfig(empty config) error = %v, want nil", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = inspector.Inspect(ctx, machine.StatusRequest{Operation: machine.OperationDoctor})
	var machineErr *machine.MachineError
	if !errors.As(err, &machineErr) || machineErr.Kind != machine.ErrorKindCanceled {
		t.Fatalf("StatusInspector.Inspect(canceled) error = %v, want canceled MachineError", err)
	}
	if machineErr.Operation != machine.OperationDoctor || !errors.Is(err, context.Canceled) {
		t.Fatalf("StatusInspector.Inspect(canceled) error = %#v, want doctor operation and context.Canceled", machineErr)
	}
}

func TestStatusInspectorMapsDeadlineContext(t *testing.T) {
	t.Parallel()

	inspector, err := NewStatusInspectorFromConfig(context.Background(), config.Config{}, Options{})
	if err != nil {
		t.Fatalf("NewStatusInspectorFromConfig(empty config) error = %v, want nil", err)
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	_, err = inspector.Inspect(ctx, machine.StatusRequest{Operation: machine.OperationAuthStatus})
	var machineErr *machine.MachineError
	if !errors.As(err, &machineErr) || machineErr.Kind != machine.ErrorKindDeadlineExceeded {
		t.Fatalf("StatusInspector.Inspect(deadline) error = %v, want deadline-exceeded MachineError", err)
	}
	if machineErr.Operation != machine.OperationAuthStatus || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("StatusInspector.Inspect(deadline) error = %#v, want auth operation and context.DeadlineExceeded", machineErr)
	}
}

func TestStatusHelpersTreatTypedNilSecretSourceAsUnset(t *testing.T) {
	t.Parallel()

	var provider *statusProviderSecret
	cfg := config.Config{}
	cfg.Credentials.ClientSecret = provider
	status := NewConfigStatus(cfg)
	if status.Credentials.ClientSecretSet || status.Credentials.ClientSecretScheme != "" {
		t.Fatalf("NewConfigStatus(typed nil provider) credentials = %#v, want unset", status.Credentials)
	}
}

type statusProviderSecret struct {
	resolveCalls int
	scheme       string
}

func (s *statusProviderSecret) Scheme() string {
	if s.scheme == "" {
		return "cmd"
	}
	return s.scheme
}

func (*statusProviderSecret) IsConfigured() bool { return true }

func (s *statusProviderSecret) Resolve(context.Context) (secret.Secret, error) {
	s.resolveCalls++
	return secret.Secret{}, errors.New("status must not resolve providers")
}

func assertNoUnsafeStatusRunes(t *testing.T, operation machine.Operation, value any) {
	t.Helper()

	body, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal(status %s) error = %v", operation, err)
	}
	var decoded any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(status %s) error = %v; body = %s", operation, err, body)
	}
	var visit func(any)
	visit = func(current any) {
		switch current := current.(type) {
		case string:
			for _, ch := range current {
				if unicode.IsControl(ch) || unicode.Is(unicode.Cf, ch) {
					t.Fatalf("status %s value %q contains unsafe rune U+%04X", operation, current, ch)
				}
			}
		case []any:
			for _, item := range current {
				visit(item)
			}
		case map[string]any:
			for _, item := range current {
				visit(item)
			}
		}
	}
	visit(decoded)
}
