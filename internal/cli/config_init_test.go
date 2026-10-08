package cli_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/cli"
	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/fileperm"
	"github.com/dvmrry/zscalerctl/internal/secret"
	"github.com/dvmrry/zscalerctl/internal/secretref"
)

// TestConfigInitCreatesLoadableOwnerOnlyConfig is the core round-trip: config
// init writes a file at the --config path, prints that path on stdout, the file
// passes the owner-only validator, and LoadConfig accepts it as a valid config
// whose credentials are simply unset (no secrets in the template).
func TestConfigInitCreatesLoadableOwnerOnlyConfig(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")
	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, nil)

	if err := app.Run(context.Background(), []string{"--config", path, "config", "init"}); err != nil {
		t.Fatalf("App.Run(config init) error = %v, want nil", err)
	}
	if strings.TrimSpace(out.String()) != path {
		t.Fatalf("config init stdout = %q, want the created path %q", out.String(), path)
	}
	if errOut.Len() == 0 {
		t.Fatalf("config init stderr = empty, want a next-steps hint")
	}

	// Self-verify: the written file passes the same validator the loader uses.
	file, err := fileperm.OpenOwnerOnly(path)
	if err != nil {
		t.Fatalf("OpenOwnerOnly(created config) error = %v, want nil", err)
	}
	_ = file.Close()

	if runtime.GOOS != "windows" {
		info, statErr := os.Stat(path)
		if statErr != nil {
			t.Fatalf("os.Stat(%q) error = %v, want nil", path, statErr)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("config init mode = %03o, want 600", info.Mode().Perm())
		}
	}

	// The template must round-trip through LoadConfig with no error and yield
	// the expected unset-credential state.
	cfg, err := config.LoadConfig(nil, config.LoadOptions{
		ConfigPath: path,
		Resolver:   stubResolver{},
	})
	if err != nil {
		t.Fatalf("LoadConfig(template) error = %v, want nil", err)
	}
	if cfg.Source != "config" || cfg.Profile != "prod" {
		t.Fatalf("LoadConfig(template) source/profile = %q/%q, want config/prod", cfg.Source, cfg.Profile)
	}
	if cfg.Credentials.ClientSecret.IsConfigured() {
		t.Fatalf("LoadConfig(template) client secret = configured, want unset (template carries no secret)")
	}
}

// TestConfigInitRefusesOverwriteWithoutForce asserts a usage error (exit 2)
// when the target already exists and --force is absent, and that the existing
// file is left untouched.
func TestConfigInitRefusesOverwriteWithoutForce(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")
	sentinel := "default_profile: keep-me\nprofiles:\n  keep-me:\n    vanity_domain: keep\n"
	if err := os.WriteFile(path, []byte(sentinel), 0o600); err != nil {
		t.Fatalf("seed os.WriteFile error = %v, want nil", err)
	}

	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, nil)

	err := app.Run(context.Background(), []string{"--config", path, "config", "init"})
	if !errors.Is(err, cli.ErrUsage) {
		t.Fatalf("config init (existing, no --force) error = %v, want errors.Is(err, cli.ErrUsage)", err)
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Fatalf("config init error = %q, want it to mention --force", err.Error())
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", path, readErr)
	}
	if string(got) != sentinel {
		t.Fatalf("config init (no --force) overwrote existing file: got %q", got)
	}
}

// TestConfigInitForceOverwrites asserts --force replaces an existing file with
// the loadable template.
func TestConfigInitRejectsTrailingSeparator(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "export")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatalf("os.Mkdir(%q) error = %v, want nil", dir, err)
	}
	nested := filepath.Join(dir, "export")
	const sentinel = "nested config must remain unchanged"
	if err := os.WriteFile(nested, []byte(sentinel), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", nested, err)
	}

	destination := dir + string(filepath.Separator)
	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, nil)
	err := app.Run(context.Background(), []string{"--config", destination, "config", "init", "--force"})
	if !errors.Is(err, cli.ErrUsage) {
		t.Fatalf("config init --force (trailing separator) error = %v, want ErrUsage", err)
	}
	if got, err := os.ReadFile(nested); err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want sentinel unchanged", nested, err)
	} else if string(got) != sentinel {
		t.Errorf("nested config body = %q, want %q", got, sentinel)
	}

	missingDir := filepath.Join(t.TempDir(), "empty-export")
	if err := os.Mkdir(missingDir, 0o700); err != nil {
		t.Fatalf("os.Mkdir(%q) error = %v, want nil", missingDir, err)
	}
	destination = missingDir + string(filepath.Separator)
	app = cli.New(&out, &errOut, nil)
	err = app.Run(context.Background(), []string{"--config", destination, "config", "init"})
	if !errors.Is(err, cli.ErrUsage) {
		t.Fatalf("config init (missing trailing-separator destination) error = %v, want ErrUsage", err)
	}
	missingNested := filepath.Join(missingDir, filepath.Base(missingDir))
	if _, err := os.Lstat(missingNested); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Lstat(%q) error = %v, want ErrNotExist", missingNested, err)
	}

	// An absent directory spelled with a trailing separator must not be
	// created by the config directory MkdirAll either.
	absentDir := filepath.Join(t.TempDir(), "absent")
	destination = absentDir + string(filepath.Separator)
	app = cli.New(&out, &errOut, nil)
	err = app.Run(context.Background(), []string{"--config", destination, "config", "init"})
	if !errors.Is(err, cli.ErrUsage) {
		t.Fatalf("config init (absent trailing-separator destination) error = %v, want ErrUsage", err)
	}
	if _, err := os.Lstat(absentDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Lstat(%q) error = %v, want ErrNotExist", absentDir, err)
	}
}

func TestConfigInitForceOverwrites(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("default_profile: old\nprofiles:\n  old: {}\n"), 0o600); err != nil {
		t.Fatalf("seed os.WriteFile error = %v, want nil", err)
	}

	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, nil)

	if err := app.Run(context.Background(), []string{"--config", path, "config", "init", "--force"}); err != nil {
		t.Fatalf("App.Run(config init --force) error = %v, want nil", err)
	}
	cfg, err := config.LoadConfig(nil, config.LoadOptions{ConfigPath: path, Resolver: stubResolver{}})
	if err != nil {
		t.Fatalf("LoadConfig(overwritten) error = %v, want nil", err)
	}
	if cfg.Profile != "prod" {
		t.Fatalf("config init --force profile = %q, want prod (template), file not overwritten", cfg.Profile)
	}
}

// TestConfigInitRejectsExtraArgs asserts unexpected positional args produce a
// usage error rather than silently succeeding.
func TestConfigInitRejectsExtraArgs(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "config.yaml")
	var out, errOut bytes.Buffer
	app := cli.New(&out, &errOut, nil)

	err := app.Run(context.Background(), []string{"--config", path, "config", "init", "extra"})
	if !errors.Is(err, cli.ErrUsage) {
		t.Fatalf("config init extra-arg error = %v, want errors.Is(err, cli.ErrUsage)", err)
	}
	if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("config init (bad args) created a file; stat err = %v, want ErrNotExist", statErr)
	}
}

// stubResolver satisfies config.SecretResolver without resolving anything; the
// template carries no secret refs, so it is never consulted.
type stubResolver struct{}

func (stubResolver) Resolve(context.Context, secretref.SecretRef) (secret.Secret, error) {
	return secret.Secret{}, nil
}
