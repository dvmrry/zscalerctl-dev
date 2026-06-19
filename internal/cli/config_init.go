package cli

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/fileperm"
)

// configInitTemplate is the commented starter config written by `config init`.
// It parses as a valid config (every uncommented field is a known profileData
// field; secret refs are commented, so credentials simply stay unconfigured)
// and contains no secret values. Update profile.go's YAML tags and this
// template together.
const configInitTemplate = `# zscalerctl configuration
#
# Owner-only file created by "zscalerctl config init". Environment variables
# (ZSCALERCTL_*) always take precedence over anything set here. Secret VALUES
# never belong in this file: reference them with a secret ref scheme instead.

default_profile: prod

profiles:
  prod:
    # Non-secret tenant metadata. Replace the placeholders below.
    vanity_domain: example          # your Zscaler vanity domain
    cloud: PRODUCTION               # OneAPI cloud, e.g. PRODUCTION
    client_id: REPLACE_WITH_CLIENT_ID
    # zpa_customer_id: REPLACE_WITH_ZPA_CUSTOMER_ID   # only for ZPA resources
    # zpa_microtenant_id: REPLACE_WITH_MICROTENANT_ID

    # Client secret: pick ONE secret ref scheme and uncomment it. Do not paste
    # the secret value here.
    #
    #   env    — read from an environment variable (highest-precedence path):
    # client_secret_ref: env:ZSCALERCTL_CLIENT_SECRET
    #
    #   file   — read from an owner-only file (absolute path):
    # client_secret_ref: file:C:\Users\you\AppData\Local\zscalerctl\client-secret
    #
    #   keyring — read from the OS keyring as service/key:
    # client_secret_ref: keyring:zscalerctl/prod-client-secret
    #
    #   cmd    — run a local command (no shell; 10s default timeout):
    # client_secret_ref:
    #   cmd:
    #     argv: ["/usr/local/bin/zscaler-secret", "prod", "client-secret"]
    #     timeout: 5s

    # ZIA legacy auth (read-only ZIA when OneAPI is unavailable). Uncomment
    # auth_mode and the matching refs to use it.
    # auth_mode: zia-legacy
    # zia_username: REPLACE_WITH_ZIA_USERNAME
    # zia_password_ref: env:ZSCALERCTL_ZIA_PASSWORD
    # zia_api_key_ref: env:ZSCALERCTL_ZIA_API_KEY
    # zia_cloud: REPLACE_WITH_ZIA_CLOUD

    # Defaults.
    # redaction: standard            # standard | share | paranoid
    # no_cache: false
`

// runConfigInit writes a starter config to the resolved default path with
// owner-only permissions so first-run on locked-down hosts "just works" —
// notably on Windows, where the default now lives under %LOCALAPPDATA% (a local
// fixed drive the fileperm volume rule accepts). It runs before LoadConfig
// because the target file is expected not to exist yet.
func (a *App) runConfigInit(opts globalOptions, args []string) error {
	flags := flag.NewFlagSet("config init", flag.ContinueOnError)
	flags.SetOutput(a.err)
	force := flags.Bool("force", false, "overwrite an existing config file")
	if err := flags.Parse(args); err != nil {
		return UsageError{Message: "usage: zscalerctl config init [--force]"}
	}
	if flags.NArg() != 0 {
		return UsageError{Message: "usage: zscalerctl config init [--force]"}
	}

	path, _ := config.ResolveConfigPath(a.env, config.LoadOptions{
		Profile:    opts.profile,
		ConfigPath: opts.configPath,
	})

	switch _, statErr := os.Lstat(path); {
	case statErr == nil:
		if !*force {
			return UsageError{Message: fmt.Sprintf("config already exists at %s; pass --force to overwrite", path)}
		}
		// WriteOwnerOnly is O_EXCL, so the caller owns the overwrite decision:
		// remove the existing file (only when --force) before re-creating it.
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("remove existing config %s: %w", path, err)
		}
	case errors.Is(statErr, fs.ErrNotExist):
		// Expected: nothing to overwrite.
	default:
		return fmt.Errorf("stat config path %s: %w", path, statErr)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config directory %s: %w", filepath.Dir(path), err)
	}
	if err := fileperm.WriteOwnerOnly(path, []byte(configInitTemplate)); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}

	// Machine-first: the path goes to stdout so it can be captured; the human
	// next-steps hint goes to stderr so it never pollutes that path.
	fmt.Fprintln(a.out, path)
	fmt.Fprintf(a.err, "Created owner-only config at %s\n", path)
	fmt.Fprintln(a.err, "Next: set a client secret — export ZSCALERCTL_CLIENT_SECRET, or uncomment a client_secret_ref in the file.")
	fmt.Fprintln(a.err, "Then run: zscalerctl doctor")
	return nil
}
