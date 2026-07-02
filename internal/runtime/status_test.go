package runtime

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/dvmrry/zscalerctl/internal/config"
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

	if got, want := NewConfigStatus(cfg), cfg.Safe(); !reflect.DeepEqual(got, want) {
		t.Fatalf("NewConfigStatus(config env) = %#v, want %#v", got, want)
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
