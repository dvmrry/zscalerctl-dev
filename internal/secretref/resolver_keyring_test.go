package secretref

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/keyring"
)

type fakeKeyringGetter struct {
	val string
	err error
}

func (f fakeKeyringGetter) Get(context.Context, string, string) (string, error) {
	return f.val, f.err
}

func TestResolveKeyringReturnsSecret(t *testing.T) {
	t.Parallel()

	r := NewResolver(ResolverOpts{Keyring: fakeKeyringGetter{val: "s3cr3t"}})
	got, err := r.Resolve(context.Background(), SecretRef{Scheme: "keyring", Service: "svc", Key: "k"})
	if err != nil {
		t.Fatalf("Resolve(keyring) error = %v, want nil", err)
	}
	if got.Reveal() != "s3cr3t" {
		t.Fatalf("Resolve(keyring) = %q, want s3cr3t", got.Reveal())
	}
}

func TestResolveKeyringNotFound(t *testing.T) {
	t.Parallel()

	r := NewResolver(ResolverOpts{Keyring: fakeKeyringGetter{err: keyring.ErrNotFound}})
	_, err := r.Resolve(context.Background(), SecretRef{Scheme: "keyring", Service: "svc", Key: "k"})
	if err == nil || !strings.Contains(err.Error(), "env:/file:/cmd") {
		t.Fatalf("Resolve(keyring not found) error = %v, want alternatives hint", err)
	}
}

func TestResolveKeyringBackendErrorIsValueFree(t *testing.T) {
	t.Parallel()

	r := NewResolver(ResolverOpts{Keyring: fakeKeyringGetter{err: errors.New("D-Bus said s3cr3t")}})
	_, err := r.Resolve(context.Background(), SecretRef{Scheme: "keyring", Service: "svc", Key: "k"})
	if err == nil {
		t.Fatal("Resolve(keyring backend error) error = nil, want error")
	}
	if strings.Contains(err.Error(), "s3cr3t") || strings.Contains(err.Error(), "D-Bus") {
		t.Fatalf("Resolve(keyring backend error) = %q, want value-free error", err.Error())
	}
}

func TestResolveKeyringNilGetter(t *testing.T) {
	t.Parallel()

	r := NewResolver(ResolverOpts{})
	_, err := r.Resolve(context.Background(), SecretRef{Scheme: "keyring", Service: "svc", Key: "k"})
	if err == nil {
		t.Fatal("Resolve(keyring nil getter) error = nil, want error")
	}
}

func TestResolveKeyringUnavailableSurfacesHint(t *testing.T) {
	t.Parallel()

	hint := fmt.Errorf("keyring: secret-tool not found; install libsecret-tools or use env:/file:/cmd: (%w)", keyring.ErrUnavailable)
	r := NewResolver(ResolverOpts{Keyring: fakeKeyringGetter{err: hint}})
	_, err := r.Resolve(context.Background(), SecretRef{Scheme: "keyring", Service: "svc", Key: "k"})
	if err == nil || !strings.Contains(err.Error(), "libsecret-tools") {
		t.Fatalf("Resolve(keyring unavailable) error = %v, want surfaced install hint", err)
	}
}
