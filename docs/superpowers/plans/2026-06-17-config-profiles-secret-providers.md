# Config Profiles + Pluggable Secret Providers — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. This plan is also written to be handed to an external coding agent (Codex); each phase is a self-contained, mergeable PR.

**Goal:** Add opt-in YAML config profiles and a pluggable secret-provider layer (`env`/`file`/`cmd`/`keyring`) to `zscalerctl`, without regressing the env-only security posture.

**Architecture:** Additive layer over the existing `config.LoadEnv(environ) → Config` seam. Credential fields on `Config` become a lazy-capable `SecretSource`. Precedence is `flag > env > profile > default`; env-inline/env-file resolve eagerly (today's behavior), profile `*_ref`s resolve only at live-reader construction. Providers live behind small interfaces; keyring is cgo-free.

**Tech Stack:** Go (existing), `gopkg.in/yaml.v3` (config parse), `golang.org/x/sys/windows` (DACL validation + Windows `CredReadW`, cgo-free), `os/exec` (cmd provider + macOS `security` / Linux `secret-tool` keyring backends). **Zero new module dependencies** — the keyring is hand-rolled per OS (see Phase 3's "Backend approach — DECIDED").

**Frozen spec:** [docs/superpowers/specs/2026-06-17-config-profiles-secret-providers-design.md](../specs/2026-06-17-config-profiles-secret-providers-design.md). Read it before starting; this plan implements it.

---

## File Structure (decomposition)

**New:**
- `internal/secretref/ref.go` — `SecretRef` type + `UnmarshalYAML` (string-or-structured), scheme parsing/validation.
- `internal/secretref/source.go` — `SecretSource` interface + `resolved`/`deferred`/`unset` impls.
- `internal/secretref/resolver.go` — `Resolver` dispatching a `SecretRef` to a provider.
- `internal/secretref/resolver.go` — all provider dispatch (`env`/`file`/`cmd`/`keyring`) as `resolve*` methods. (The per-`provider_*.go` split in this draft was not used; phases 1–3 keep dispatch in `resolver.go`.)
- `internal/fileperm/fileperm.go` + `fileperm_posix.go` + `fileperm_windows.go` — owner-only (POSIX) and DACL (Windows) validation, shared by config loader and `file:` provider.
- `internal/keyring/keyring.go` (`Getter` interface, `ErrNotFound`, `decodeUTF16LE`, `New()`) + `exec.go` (shared leak-safe `runKeyringCmd`) + build-tagged `keyring_darwin.go` / `keyring_linux.go` / `keyring_windows.go` / `keyring_other.go` — cgo-free, hand-rolled backends behind the mockable `Getter`. (`resolveKeyring` lives in `resolver.go`, not a separate provider file.)
- `internal/config/profile.go` — config-file model + YAML load + profile selection.
- `internal/config/load.go` — `LoadConfig(...)` that layers flag/env/profile/default into `Config` (wraps existing `LoadEnv`).
- `docs/schema/config.schema.json` — drift-gated config schema.

**Modified:**
- `internal/config/config.go` — credential fields change `secret.Secret` → `secretref.SecretSource`; extend `SafeConfig` with profile/source metadata.
- `internal/cli/app.go` — add `--profile`/`--config` global flags; call `LoadConfig`; resolve secrets at reader build; surface metadata in `config show`/`doctor`.
- `docs/INSTALL.md`, `THREAT_MODEL.md`, `AGENTS.md`, `skills/zscalerctl/SKILL.md` — docs.

**Pattern notes for the implementer (this codebase):**
- Secrets use the `secret.Secret` wrapper (never log/print raw). Build via `secret.New(value)`; `IsSet()` reports presence.
- Config errors wrap `config.ErrInvalidConfig` (maps to exit 2). Never echo a secret or an offending value into an error.
- Tests are standard `go test`; run `go test -mod=vendor ./...` and `make check` before each phase PR. New deps require `go mod tidy && go mod vendor` and must pass `verify-licenses.sh` (allow-list) + be SHA/pin-clean.
- Owner-only POSIX file reads already exist in `internal/credentials/files.go` (`ReadOwnerOnlySecretFile`); reuse it.

---

## PHASE 1 — `SecretSource`, config loader, permission validation, `env`/`file` providers, `SafeConfig` metadata

**Outcome:** profiles work end-to-end with `env:`/`file:` secrets; env-only behavior unchanged; Windows DACL enforced; `config show`/`doctor` prove no-resolution. Mergeable PR.

### Task 1.1: `SecretSource` interface + `resolved`/`unset` sources

**Files:**
- Create: `internal/secretref/source.go`
- Test: `internal/secretref/source_test.go`

- [ ] **Step 1: Write the failing test**

```go
package secretref

import (
	"context"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/secret"
)

func TestResolvedSourceReturnsCapturedSecret(t *testing.T) {
	src := Resolved("env", secret.New("hunter2"))
	if src.Scheme() != "env" || !src.IsConfigured() {
		t.Fatalf("scheme=%q configured=%v", src.Scheme(), src.IsConfigured())
	}
	got, err := src.Resolve(context.Background())
	if err != nil || got.Reveal() != "hunter2" {
		t.Fatalf("resolve = %q, %v", got.Reveal(), err)
	}
}

func TestUnsetSourceIsNotConfigured(t *testing.T) {
	src := Unset()
	if src.IsConfigured() || src.Scheme() != "" {
		t.Fatalf("unset reported configured=%v scheme=%q", src.IsConfigured(), src.Scheme())
	}
}
```

> Note: confirm the reveal accessor name on `secret.Secret` (`Reveal()`/`Value()`); use whatever the existing type exposes for tests. If none is exported, add a test-only accessor in the secret package's `_test` or compare via the existing reader path.

- [ ] **Step 2: Run test, verify it fails**

Run: `go test -mod=vendor ./internal/secretref/ -run TestResolvedSource -v`
Expected: FAIL (undefined `Resolved`/`Unset`).

- [ ] **Step 3: Implement**

```go
package secretref

import (
	"context"

	"github.com/dvmrry/zscalerctl/internal/secret"
)

// SecretSource carries provenance metadata and resolves to a secret on demand.
type SecretSource interface {
	Scheme() string                              // "env"|"file"|"cmd"|"keyring"|"" (unset)
	IsConfigured() bool                          // a source exists
	Resolve(ctx context.Context) (secret.Secret, error)
}

type resolved struct {
	scheme string
	sec    secret.Secret
}

func Resolved(scheme string, sec secret.Secret) SecretSource { return resolved{scheme, sec} }
func (r resolved) Scheme() string                            { return r.scheme }
func (r resolved) IsConfigured() bool                        { return r.sec.IsSet() }
func (r resolved) Resolve(context.Context) (secret.Secret, error) { return r.sec, nil }

type unset struct{}

func Unset() SecretSource                                       { return unset{} }
func (unset) Scheme() string                                    { return "" }
func (unset) IsConfigured() bool                                { return false }
func (unset) Resolve(context.Context) (secret.Secret, error)    { return secret.Secret{}, nil }
```

- [ ] **Step 4: Run test, verify pass.** `go test -mod=vendor ./internal/secretref/ -run 'TestResolvedSource|TestUnsetSource' -v` → PASS.
- [ ] **Step 5: Commit.** `git add internal/secretref/ && git commit -m "Add SecretSource (resolved/unset)"`

### Task 1.2: `SecretRef` parsing + `UnmarshalYAML` (string + structured cmd)

**Files:**
- Create: `internal/secretref/ref.go`
- Test: `internal/secretref/ref_test.go`

- [ ] **Step 1: Write the failing test** — cover: `env:NAME`, `file:/p`, `keyring:svc/key`, structured `cmd`, and every rejection (no scheme, unknown scheme, empty keyring segment, keyring segment with extra `/`, empty cmd argv).

```go
func TestParseStringRefs(t *testing.T) {
	cases := map[string]SecretRef{
		"env:ZS_SECRET":         {Scheme: "env", Name: "ZS_SECRET"},
		"file:/etc/zs/secret":   {Scheme: "file", Path: "/etc/zs/secret"},
		"keyring:zscalerctl/ k": {Scheme: "keyring", Service: "zscalerctl", Key: " k"}, // spaces ok, '/' not
	}
	for in, want := range cases {
		var r SecretRef
		if err := r.UnmarshalYAML(yamlScalar(t, in)); err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if r != want {
			t.Fatalf("%q -> %+v want %+v", in, r, want)
		}
	}
}

func TestParseRejects(t *testing.T) {
	for _, in := range []string{"", "noscheme", "bogus:x", "keyring:onlyservice", "keyring:a/b/c", "keyring:/k", "keyring:svc/"} {
		var r SecretRef
		if err := r.UnmarshalYAML(yamlScalar(t, in)); err == nil {
			t.Fatalf("expected error for %q", in)
		}
	}
}

func TestParseStructuredCmd(t *testing.T) {
	var r SecretRef
	if err := r.UnmarshalYAML(yamlNode(t, "cmd:\n  argv: [\"/bin/get\", \"--p\", \"prod\"]\n  timeout: 5s")); err != nil {
		t.Fatal(err)
	}
	if r.Scheme != "cmd" || len(r.Argv) != 3 || r.Timeout != 5*time.Second {
		t.Fatalf("got %+v", r)
	}
}

func TestStructuredCmdRejectsEmptyArgv(t *testing.T) {
	var r SecretRef
	if err := r.UnmarshalYAML(yamlNode(t, "cmd:\n  argv: []")); err == nil {
		t.Fatal("expected error for empty argv")
	}
}
```

> Helpers `yamlScalar`/`yamlNode` build a `*yaml.Node` from a string (use `yaml.Unmarshal` into a `yaml.Node`). Include them in the test file.

- [ ] **Step 2: Run, verify fail.**
- [ ] **Step 3: Implement** `SecretRef` + `UnmarshalYAML`:

```go
package secretref

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const DefaultCmdTimeout = 10 * time.Second

type SecretRef struct {
	Scheme  string
	Name    string        // env
	Path    string        // file
	Service string        // keyring
	Key     string        // keyring
	Argv    []string      // cmd
	Timeout time.Duration // cmd (0 => DefaultCmdTimeout at resolve time)
}

var ErrInvalidRef = errors.New("invalid secret reference")

func (r *SecretRef) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		return r.parseString(node.Value)
	case yaml.MappingNode:
		return r.parseStructured(node)
	default:
		return fmt.Errorf("%w: must be a string or a {cmd: {...}} mapping", ErrInvalidRef)
	}
}

func (r *SecretRef) parseString(s string) error {
	scheme, val, ok := strings.Cut(s, ":")
	if !ok || scheme == "" {
		return fmt.Errorf("%w: missing provider scheme (env:/file:/keyring:)", ErrInvalidRef)
	}
	switch scheme {
	case "env":
		if val == "" {
			return fmt.Errorf("%w: env: requires a variable name", ErrInvalidRef)
		}
		r.Scheme, r.Name = "env", val
	case "file":
		if val == "" {
			return fmt.Errorf("%w: file: requires a path", ErrInvalidRef)
		}
		r.Scheme, r.Path = "file", val
	case "keyring":
		svc, key, ok := strings.Cut(val, "/")
		if !ok || svc == "" || key == "" || strings.Contains(key, "/") {
			return fmt.Errorf("%w: keyring: must be service/key with one '/' and non-empty segments", ErrInvalidRef)
		}
		r.Scheme, r.Service, r.Key = "keyring", svc, key
	default:
		return fmt.Errorf("%w: unknown scheme %q", ErrInvalidRef, scheme)
	}
	return nil
}

func (r *SecretRef) parseStructured(node *yaml.Node) error {
	var m struct {
		Cmd *struct {
			Argv    []string `yaml:"argv"`
			Timeout string   `yaml:"timeout"`
		} `yaml:"cmd"`
	}
	if err := node.Decode(&m); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidRef, err)
	}
	if m.Cmd == nil {
		return fmt.Errorf("%w: structured ref must contain a 'cmd' key", ErrInvalidRef)
	}
	if len(m.Cmd.Argv) == 0 || m.Cmd.Argv[0] == "" {
		return fmt.Errorf("%w: cmd.argv must be a non-empty list", ErrInvalidRef)
	}
	r.Scheme, r.Argv = "cmd", m.Cmd.Argv
	if m.Cmd.Timeout != "" {
		d, err := time.ParseDuration(m.Cmd.Timeout)
		if err != nil || d <= 0 {
			return fmt.Errorf("%w: cmd.timeout must be a positive duration", ErrInvalidRef)
		}
		r.Timeout = d
	}
	return nil
}
```

- [ ] **Step 4: Run, verify pass.**
- [ ] **Step 5: Commit.** `git commit -m "Add SecretRef parsing (string + structured cmd)"`

### Task 1.3: `fileperm` — POSIX owner-only + Windows DACL validation

**Files:**
- Create: `internal/fileperm/fileperm.go`, `fileperm_posix.go` (`//go:build !windows`), `fileperm_windows.go` (`//go:build windows`)
- Test: `internal/fileperm/fileperm_posix_test.go`, `internal/fileperm/fileperm_windows_test.go`

- [ ] **Step 1: POSIX failing test** — a 0600 file passes; 0644/0640 reject.

```go
//go:build !windows
func TestPOSIXOwnerOnly(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.yaml")
	os.WriteFile(p, []byte("x"), 0o600)
	if err := Validate(p); err != nil { t.Fatalf("0600 rejected: %v", err) }
	os.Chmod(p, 0o644)
	if err := Validate(p); err == nil { t.Fatal("0644 accepted") }
}
```

- [ ] **Step 2: Run, verify fail.**
- [ ] **Step 3: Implement** `Validate(path) error` (POSIX): stat, reject if `mode & 0o077 != 0` (group/other bits), wrap a sentinel `ErrInsecurePermissions`. Define `ErrInsecurePermissions` in `fileperm.go`. Mirror `internal/credentials/files.go` logic; factor the shared check so `ReadOwnerOnlySecretFile` can later call it.
- [ ] **Step 4: Run, verify pass. Commit.** `git commit -m "Add POSIX fileperm.Validate"`

- [ ] **Step 5: Windows DACL failing test** — construct security descriptors and assert accept/reject per the spec (owner/SYSTEM/Administrators accepted; Everyone/Users/Authenticated Users/Domain Users or broad inherited grants rejected; admin access alone is acceptable but not sufficient if a broad principal is also granted).

```go
//go:build windows
func TestWindowsDACL(t *testing.T) {
	// table: build *windows.SECURITY_DESCRIPTOR with explicit ACEs (owner SID,
	// SYSTEM, Administrators) => accept; add Everyone/Users/Authenticated Users
	// or an inherited ACE granting an interactive group => reject.
}
```

- [ ] **Step 6: Implement** Windows `Validate(path)` using `golang.org/x/sys/windows`: `GetNamedSecurityInfo(path, SE_FILE_OBJECT, DACL_SECURITY_INFORMATION|OWNER_SECURITY_INFORMATION)`, walk the DACL ACEs, resolve well-known SIDs (`WinWorldSid`=Everyone, `WinBuiltinUsersSid`=Users, `WinAuthenticatedUserSid`, `WinAccountDomainUsersSid`), and **reject** if any ACE grants read/write to a broad/interactive principal (including inherited ACEs with such trustees); **accept** owner / `WinLocalSystemSid` / `WinBuiltinAdministratorsSid`. Return `ErrInsecurePermissions` on reject. Keep the accept/reject set exactly as the spec's Windows bullet states.
- [ ] **Step 7: Run on a Windows runner** (CI matrix; see Task 1.9). Locally on non-Windows the file builds via build tags. **Commit.** `git commit -m "Add Windows DACL fileperm.Validate"`

### Task 1.4: `env` and `file` providers + `Resolver` skeleton

**Files:**
- Create: `internal/secretref/provider_env.go`, `provider_file.go`, `resolver.go`
- Test: `internal/secretref/resolver_test.go`

- [ ] **Step 1: Failing test** — `env:` reads a set var (error if unset); `file:` reads an owner-only file (error on bad perms via `fileperm`); unknown scheme errors.

```go
func TestResolveEnv(t *testing.T) {
	t.Setenv("ZS_X", "abc")
	r := NewResolver(ResolverOpts{})
	got, err := r.Resolve(context.Background(), SecretRef{Scheme: "env", Name: "ZS_X"})
	if err != nil || got.Reveal() != "abc" { t.Fatalf("%q %v", got.Reveal(), err) }
	if _, err := r.Resolve(context.Background(), SecretRef{Scheme: "env", Name: "ZS_MISSING"}); err == nil {
		t.Fatal("missing env accepted")
	}
}
```

- [ ] **Step 2: Run, fail.**
- [ ] **Step 3: Implement** `Resolver` + env/file providers:

```go
type ResolverOpts struct {
	Keyring  KeyringGetter // nil until phase 3
	AllowCmd bool          // false disables cmd: (DISALLOW_CMD) — phase 2 wires it
}
type Resolver struct{ opts ResolverOpts }
func NewResolver(o ResolverOpts) *Resolver { return &Resolver{o} }

func (r *Resolver) Resolve(ctx context.Context, ref SecretRef) (secret.Secret, error) {
	switch ref.Scheme {
	case "env":
		v, ok := os.LookupEnv(ref.Name)
		if !ok { return secret.Secret{}, fmt.Errorf("%w: env %s is not set", ErrInvalidRef, ref.Name) }
		return secret.New(v), nil
	case "file":
		return credentials.ReadOwnerOnlySecretFile(ref.Path) // already returns secret.Secret + perm check
	case "cmd":
		return r.resolveCmd(ctx, ref) // phase 2
	case "keyring":
		return r.resolveKeyring(ref)  // phase 3
	default:
		return secret.Secret{}, fmt.Errorf("%w: unknown scheme %q", ErrInvalidRef, ref.Scheme)
	}
}
```

Add stub `resolveCmd`/`resolveKeyring` returning a "not yet enabled" error so the package builds; phases 2/3 fill them.

- [ ] **Step 4: Run, pass. Commit.** `git commit -m "Add env/file secret providers + Resolver"`

### Task 1.5: `Config` credential fields → `SecretSource`; `SafeConfig` metadata

**Files:**
- Modify: `internal/config/config.go` (Credentials/ZIALegacy secret fields → `secretref.SecretSource`; `SafeConfig`/status structs gain `Scheme` + `Configured`)
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Failing test** — `SafeConfig` reports `Scheme`/`Configured` without resolving (use a `deferred` source backed by a resolver that panics if `Resolve` is called).
- [ ] **Step 2: Run, fail.**
- [ ] **Step 3: Implement** — change `Credentials.ClientSecret`, `ZIALegacyCredentials.Password`/`APIKey` from `secret.Secret` to `secretref.SecretSource`. Add a `deferred` source in `internal/secretref/source.go`:

```go
type deferred struct {
	ref      SecretRef
	resolver *Resolver
}
func Deferred(ref SecretRef, r *Resolver) SecretSource { return deferred{ref, r} }
func (d deferred) Scheme() string     { return d.ref.Scheme }
func (d deferred) IsConfigured() bool  { return true }
func (d deferred) Resolve(ctx context.Context) (secret.Secret, error) { return d.resolver.Resolve(ctx, d.ref) }
```

Extend `CredentialStatus`/`ZIALegacyStatus` with `ClientSecretScheme string` + keep the `*Set` bools driven by `IsConfigured()`. **Update every existing reference** to the changed fields (the reader build path now calls `.Resolve(ctx)`; `config show`/`doctor` read `.Scheme()`/`.IsConfigured()`).

- [ ] **Step 4: Run the full `internal/config` + `internal/cli` suites; fix all call sites.** `go test -mod=vendor ./internal/config ./internal/cli` → PASS.
- [ ] **Step 5: Commit.** `git commit -m "Make Config credentials SecretSource; SafeConfig surfaces scheme"`

### Task 1.6: Update `LoadEnv` to emit `SecretSource` (backward-compat, eager)

**Files:**
- Modify: `internal/config/config.go` (`LoadEnv`)
- Test: existing `LoadEnv` tests must pass unchanged + a new test asserting env-inline/env-file produce a `resolved` source with the right scheme.

- [ ] **Step 1:** Add a test: `ZSCALERCTL_CLIENT_SECRET` set → `ClientSecret.Scheme()=="env"` and resolves eagerly; `..._FILE` (0600) → `Scheme()=="file"`.
- [ ] **Step 2: Run, fail.**
- [ ] **Step 3: Implement** — wrap the existing eager resolution: `Credentials.ClientSecret = secretref.Resolved("env", secret.New(env[EnvClientSecret]))` when inline set, else `secretref.Resolved("file", fileSecret)` when the `_FILE` path resolves, else `secretref.Unset()`. Preserve the existing precedence (inline wins over file). Do the same for the ZIA legacy password/api-key.
- [ ] **Step 4: Run the existing `LoadEnv` tests — they must pass unchanged. Commit.** `git commit -m "LoadEnv emits SecretSource (eager, backward-compatible)"`

### Task 1.7: Config-file model + load + profile selection

**Files:**
- Create: `internal/config/profile.go`
- Test: `internal/config/profile_test.go`

- [ ] **Step 1: Failing test** — parse a YAML config with two profiles; select by name; `fileperm.Validate` is called (reject a 0644 config); unknown profile errors; `default_profile` honored.
- [ ] **Step 2: Run, fail.**
- [ ] **Step 3: Implement** the YAML model + loader:

```go
type fileModel struct {
	DefaultProfile string                  `yaml:"default_profile"`
	Profiles       map[string]profileModel `yaml:"profiles"`
}
type profileModel struct {
	AuthMode        string              `yaml:"auth_mode"`
	VanityDomain    string              `yaml:"vanity_domain"`
	Cloud           string              `yaml:"cloud"`
	ClientID        string              `yaml:"client_id"`
	ClientSecretRef *secretref.SecretRef `yaml:"client_secret_ref"`
	ZPACustomerID   string              `yaml:"zpa_customer_id"`
	ZPAMicrotenantID string             `yaml:"zpa_microtenant_id"`
	ZIAUsername     string              `yaml:"zia_username"`
	ZIAPasswordRef  *secretref.SecretRef `yaml:"zia_password_ref"`
	ZIAAPIKeyRef    *secretref.SecretRef `yaml:"zia_api_key_ref"`
	ZIACloud        string              `yaml:"zia_cloud"`
	Redaction       string              `yaml:"redaction"`
	NoCache         *bool               `yaml:"no_cache"`
}

// LoadProfile reads+validates the config file and returns the selected profile.
func LoadProfile(path, name string) (profileModel, bool, error) {
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return profileModel{}, false, nil // no file => additive no-op
	}
	if err := fileperm.Validate(path); err != nil {
		return profileModel{}, false, fmt.Errorf("%w: config %s: %w", ErrInvalidConfig, path, err)
	}
	// read (bounded), yaml.Unmarshal into fileModel, pick name (or DefaultProfile),
	// error if name unknown.
}
```

Bound the read (reuse the `1<<20`-style guard pattern). Reject unknown YAML keys is optional (yaml.v3 `KnownFields` on a decoder — recommended for typo safety).

- [ ] **Step 4: Run, pass. Commit.** `git commit -m "Add config-file model, load, profile selection"`

### Task 1.8: `LoadConfig` — layer flag/env/profile/default into `Config`

**Files:**
- Create: `internal/config/load.go`
- Modify: `internal/cli/app.go` (call `LoadConfig` with `--profile`/`--config`; add the flags)
- Test: `internal/config/load_test.go` (precedence matrix) + `internal/cli/app_test.go` (flag wiring)

- [ ] **Step 1: Failing test — precedence matrix.** With a profile providing `vanity_domain=p` and a `client_secret_ref`, and env `ZSCALERCTL_VANITY_DOMAIN=e`: resulting `Config.VanityDomain=="e"` (env wins); secret source scheme comes from the profile ref when env secret unset; env secret (inline) overrides the profile ref.
- [ ] **Step 2: Run, fail.**
- [ ] **Step 3: Implement** `LoadConfig(environ, flags, resolver)`:
  - Resolve profile name: flag `--profile` > `ZSCALERCTL_PROFILE` > file `default_profile`.
  - Resolve config path: flag `--config` > `ZSCALERCTL_CONFIG` > default XDG path.
  - `base := LoadEnv(environ)` (eager env).
  - Load profile (Task 1.7). For each non-secret field, `base` value if set else profile value.
  - For each secret field: if `base` source `IsConfigured()` (env provided), keep it; else if the profile has a `*_ref`, set `secretref.Deferred(ref, resolver)`; else `Unset()`.
  - Operational defaults (`redaction`, `no_cache`): env > profile > default.
  - Return the merged `Config`.
- [ ] **Step 4: Add `--profile`/`--config` to the global flag set in `internal/cli`** (mirror existing global-flag plumbing + `completionFlags` + the agent-docs/man drift gates — add them to AGENTS.md/man or the exemption list to satisfy `TestAgentDocsDocumentEveryFlag`/`TestManPageDocumentsFlagsAndCommands`).
- [ ] **Step 5: Run `./internal/config ./internal/cli`; pass. Commit.** `git commit -m "Add LoadConfig precedence layering + --profile/--config flags"`

### Task 1.9: `config show`/`doctor` metadata + Windows CI + phase-1 PR

**Files:**
- Modify: `internal/cli/app.go` (`config show`/`doctor` render `SafeConfig` profile + per-secret scheme), `.github/workflows/ci.yml` (add a `windows-latest` job running the `fileperm` + config tests)
- Test: `internal/cli/app_test.go` (config show output includes profile + the client-secret source scheme; **asserts no Resolve happens** — inject a resolver that fails if called)

- [ ] **Step 1: Failing test** — `config show --format json` with a profile + deferred keyring ref reports `"client_secret": {"scheme":"keyring","configured":true}` and the injected resolver's `Resolve` is never called.
- [ ] **Step 2: Run, fail.**
- [ ] **Step 3: Implement** the surfacing (read `.Scheme()`/`.IsConfigured()`), add the `windows-latest` CI job (`go test -mod=vendor ./internal/fileperm/... ./internal/config/...`).
- [ ] **Step 4: `make check` green; Windows job green.**
- [ ] **Step 5: Open phase-1 PR** (`semver:minor`), squash-merge after review.

---

## PHASE 2 — `cmd:` provider (structured argv, no-shell, timeout, kill-switch) + threat-model docs

**Outcome:** `cmd:` refs resolve by exec'ing a structured argv **directly (no shell)**, bounded by a per-ref-or-default **10s** timeout, gated by `ZSCALERCTL_DISALLOW_CMD`. This provider executes code from the config file, so it is **security-critical**: PR review = Opus + a **GPT-5.5 cross-check of the exec path**. Mergeable PR.

**Reality alignment (what phase 1 ALREADY shipped — build on it, don't recreate):**
- `SecretRef` already parses structured `cmd` into `.Argv []string` (parse guarantees non-empty with a non-blank `argv[0]`) and `.Timeout time.Duration`; `DefaultCmdTimeout = 10 * time.Second` exists — all in `internal/secretref/ref.go`.
- `Resolver.Resolve` already has `case "cmd":` returning `"cmd refs are not enabled in this build phase"` — this phase **replaces that stub** (`internal/secretref/resolver.go`).
- `ResolverOpts` is currently `struct{}` (empty) — this phase **adds the `AllowCmd` field**.
- The resolver is built at **`internal/config/load.go:55`** as `secretref.NewResolver(secretref.ResolverOpts{})` — this phase threads `AllowCmd` in there.
- Resolve already early-returns on `ctx.Done()`; the config gate (owner-only POSIX + Windows DACL) it sits on was hardened in phase 1 and is the trust anchor for executing `cmd:`.

### Task 2.1: Add `AllowCmd` to `ResolverOpts` + implement `resolveCmd`

**Files:** Modify `internal/secretref/resolver.go`; add `internal/secretref/resolver_cmd_test.go` (keep `resolveCmd` in `resolver.go` to match the package layout).

- [ ] **Step 1: Add the opt.** Change `type ResolverOpts struct{}` to:

```go
type ResolverOpts struct {
	AllowCmd bool // false disables cmd: refs (set from ZSCALERCTL_DISALLOW_CMD)
}
```

- [ ] **Step 2: Write failing tests.** Build resolvers via `NewResolver(ResolverOpts{AllowCmd: true})` unless noted. Use a real interpreter argv where needed (e.g. `{"/bin/sh","/path/to/fixture.sh"}` written into `t.TempDir()`), NOT a shell string in one argv element.
  - **echo:** `{"/bin/echo","-n","s3cr3t"}` → `Resolve` returns a secret revealing `s3cr3t`.
  - **no-shell (literal argv):** `{"/bin/echo", "$(touch SENTINEL)", ";", "rm -rf x", "|", "cat"}` where `SENTINEL` is an absolute temp path → output contains the literal strings AND `SENTINEL` is **not** created (assert `os.Stat` is `ErrNotExist`). Proves no shell expansion.
  - **trailing newline trimmed:** a fixture that prints `secret\n` → revealed secret is `secret` (no trailing `\n`).
  - **timeout:** `{"/bin/sleep","5"}` with `ref.Timeout = 200*time.Millisecond` → returns a timeout error within ~timeout (process killed by `CommandContext`); assert the error is the timeout branch, not a generic failure.
  - **non-zero exit is value-free:** a fixture that writes a known token (e.g. `LEAK-<rand>`) to BOTH stdout and stderr and `exit 1` → `Resolve` errors, and the error string contains **neither** the token (stdout never surfaced; stderr summarized+bounded so the token isn't echoed in full — see `summarizeStderr`).
  - **disabled does not exec:** `NewResolver(ResolverOpts{})` (AllowCmd=false) with a cmd ref whose `argv` would `touch SENTINEL` → returns the disabled error AND `SENTINEL` is **not** created (the command must never run when disabled).
- [ ] **Step 3: Run, fail.**
- [ ] **Step 4: Implement.** Replace the `case "cmd":` stub body in `Resolve` with `return r.resolveCmd(ctx, ref)`, and add (imports: `bytes`, `context`, `os/exec`, `strings`):

```go
// resolveCmd execs ref.Argv directly (no shell), bounded by a timeout, and
// returns trimmed stdout as the secret. Never logs stdout (the secret); stderr
// is summarized and bounded so a misbehaving provider cannot leak via the error.
func (r *Resolver) resolveCmd(ctx context.Context, ref SecretRef) (secret.Secret, error) {
	if !r.opts.AllowCmd {
		return secret.Secret{}, fmt.Errorf("%w: cmd refs are disabled (ZSCALERCTL_DISALLOW_CMD)", ErrInvalidRef)
	}
	timeout := ref.Timeout
	if timeout <= 0 {
		timeout = DefaultCmdTimeout
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	// ref.Argv is guaranteed non-empty with a non-blank argv[0] by SecretRef parsing.
	cmd := exec.CommandContext(cctx, ref.Argv[0], ref.Argv[1:]...) // no shell: direct argv exec
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if cctx.Err() == context.DeadlineExceeded {
			return secret.Secret{}, fmt.Errorf("%w: cmd provider %q timed out after %s", ErrInvalidRef, ref.Argv[0], timeout)
		}
		return secret.Secret{}, fmt.Errorf("%w: cmd provider %q failed: %s", ErrInvalidRef, ref.Argv[0], summarizeStderr(stderr.String()))
	}
	return secret.New(strings.TrimRight(stdout.String(), "\r\n")), nil
}

// summarizeStderr returns a short, single-line, length-bounded view of stderr
// so a provider that dumps a secret to stderr cannot leak it through the error.
func summarizeStderr(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	const max = 200
	if len(s) > max {
		s = s[:max] + "…"
	}
	if s == "" {
		s = "(no stderr)"
	}
	return s
}
```

- [ ] **Step 5: Run, pass. Commit.** `git commit -m "Enable cmd: secret provider (no-shell argv, timeout, AllowCmd gate)"`

### Task 2.2: Wire `ZSCALERCTL_DISALLOW_CMD` (existing bool parser, value-free errors)

**Files:** `internal/config/config.go` (env constant), `internal/config/load.go:55` (thread `AllowCmd`), test.

- [ ] **Step 1: Failing test** — (a) `ZSCALERCTL_DISALLOW_CMD=true` → a profile `cmd:` ref resolves to the **disabled** error (resolver built with `AllowCmd=false`); (b) unset / `false` → `cmd:` enabled; (c) garbage (`"maybe"`) → `ErrInvalidConfig` whose message does **NOT** contain `"maybe"` (value-free).
- [ ] **Step 2: Run, fail.**
- [ ] **Step 3: Implement.** Add `EnvDisallowCmd = "ZSCALERCTL_DISALLOW_CMD"` to the env constants in `config.go`. In `LoadConfig`, parse it with the existing `parseBoolEnv`; on parse error return `fmt.Errorf("%w: invalid %s", ErrInvalidConfig, EnvDisallowCmd)` (no value echoed). Change `load.go:55` to `secretref.NewResolver(secretref.ResolverOpts{AllowCmd: !disallowCmd})`.
- [ ] **Step 4: Run, pass. Commit.** `git commit -m "Wire ZSCALERCTL_DISALLOW_CMD kill-switch"`

### Task 2.3: Surfacing test + docs + phase-2 PR

- [ ] **Surfacing (no exec):** add a test that a profile with a `cmd:` ref makes `config show`/`doctor` report `scheme=cmd, configured=true` WITHOUT executing the command (reuse the panic-on-resolve `fakeResolver` pattern from `internal/config/load_test.go`).
- [ ] **THREAT_MODEL.md:** add the `cmd:` section per the frozen spec — the code-execution surface; the gate is the **phase-1-hardened** owner-only POSIX + Windows-DACL config validation; `cmd:` execs a structured argv directly (no shell); bounded by the 10s-default timeout; `ZSCALERCTL_DISALLOW_CMD` opt-out for hardened fleets; the "owner-only config ⇒ no new privilege (you could run it yourself)" reasoning; secret/stdout never logged, stderr summarized + bounded.
- [ ] **INSTALL.md:** add a `cmd:` profile example (structured `argv` + optional `timeout`), state no-shell (wrap pipelines/SOPS in a script the `argv` points at), and document `ZSCALERCTL_DISALLOW_CMD`.
- [ ] **Validate:** `go test -mod=vendor ./...`; `make check` (incl. `verify-docs`, `sync-agents-skill --check`). Apply `semver:minor`.
- [ ] **Open phase-2 PR (draft).** Review = Opus + **GPT-5.5 cross-check of the exec path** (no-shell argv, timeout actually killing a hung provider, value-free errors, the `AllowCmd` gate). Merge after clean.

**Open decision for the implementer/reviewer:** whether to require `argv[0]` to be an absolute path (reject relative / PATH-resolved binaries) to harden against PATH hijacking. Lean: allow PATH (the config is owner-only and PATH is the operator's own) but **document** that `argv[0]` resolves via the operator's `PATH` and recommend absolute paths. Decide in the PR.

### Resolve-error exit-code note (applies to all deferred providers)

A deferred secret that fails to `Resolve` at reader-build time (bad `cmd:`, disabled, timeout, missing `env:`, etc.) must map to a sensible exit code — **exit 3 (missing/invalid credentials)**, since the effect is "the credential could not be obtained" — not the internal-error code. Confirm the reader-build path classifies a `Resolve` error (wrapping `secretref.ErrInvalidRef`) to the credentials exit code, and add a test. (This wasn't needed in phase 1 because only eager env/file sources existed; it becomes reachable the moment a `cmd:` ref resolves.)

---

## PHASE 3 — `keyring:` cgo-free backend (read)

**Outcome:** `keyring:` refs resolve from the OS keychain — macOS Keychain via `/usr/bin/security`, Linux Secret Service via `secret-tool`, Windows Credential Manager via the `CredReadW` syscall — all cgo-free, **zero new Go dependencies**, static binary intact, behind a mockable `Getter` interface. Mergeable `semver:minor` PR.

### Backend approach — DECIDED: zero new dependencies (hand-roll all three)

Each backend uses only the stdlib (`os/exec` for macOS/Linux) or an already-vendored dependency (`golang.org/x/sys/windows`, present since the phase-1 DACL work). Rationale:

- The project posture is aggressively supply-chain-minimal (Scorecard, pinned deps, `verify-licenses.sh`, govulncheck) and `keyring:` is an **optional convenience** — `env:`/`file:`/`cmd:` already cover every credential, including headless/CI use. Adding `zalando/go-keyring` (pulls `godbus` + `danieljoos/wincred`) or `godbus/dbus` for an optional read path is not worth the dependency surface. A strategy panel (three independent lenses, including a pro-library steelman) unanimously chose hand-roll.
- macOS + Linux reuse the **exact phase-2 `cmd:` leak-safe exec discipline** (no shell, bounded output, bounded timeout + `WaitDelay`, `ZSCALERCTL_*`-scrubbed env, value-free errors).
- Windows: `golang.org/x/sys/windows` does **not** export `CredReadW`/`CREDENTIALW` (confirmed — only DACL/security APIs are vendored). The backend declares its own `advapi32.dll` `LazyProc` and `credentialW` struct — ~80 lines of unsafe syscall code. This is the highest-risk surface; it carries a **mandatory live acceptance test on a real Windows host** before merge (an operator-provided host is available), plus cross-family review.
- **Trade-off (documented):** Linux requires `secret-tool` (libsecret-tools) on `PATH`; if absent, the backend returns a clear install-hint error (not a crash, not `ErrNotFound`). Native D-Bus via `godbus` remains a documented, reversible future upgrade.

**Storage conventions (read invocation + how the operator provisions the credential):**

| OS | Read invocation | Not-found signal | Secret encoding | How operator stores the credential |
|----|-----------------|------------------|-----------------|-------------------------------------|
| **macOS** | `/usr/bin/security find-generic-password -s <service> -a <key> -w` (absolute path, anti-hijack; `-w` routes **only** the raw password to stdout — never `-g`, which leaks it to stderr) | Exit code **44** (low byte of `errSecItemNotFound` OSStatus `0xFFFF9D2C`) | Raw bytes + trailing LF; `strings.TrimRight(stdout, "\r\n")` | `security add-generic-password -s <service> -a <key> -w` or Keychain Access.app → File ▸ New Password Item ("Keychain Item Name" = service, "Account Name" = key) |
| **Linux** | `secret-tool lookup service <service> account <key>` (PATH-resolved; `exec.LookPath` first for a clear install-hint if absent) | Exit **1** AND trimmed stdout empty. Exit 1 + non-empty stdout = hard error; D-Bus language in stderr + empty stdout = "Secret Service unavailable" hard error | UTF-8, no trailing newline in libsecret ≥ 0.18 (trim `\r\n` defensively) | `secret-tool store --label="zscalerctl: <service>/<key>" service <service> account <key>` (type secret at prompt) |
| **Windows** | `CredReadW` via `advapi32.dll` `NewLazySystemDLL` + `NewProc` (System32-only load; no subprocess, no PATH) | `r1 == 0` and `lastErr == windows.ERROR_NOT_FOUND` (`syscall.Errno(1168)`) | UTF-16LE blob, no guaranteed NUL terminator; decode via pure-Go `decodeUTF16LE` (Task 3.1) | `cmdkey /generic:<service>/<key> /user:<service>/<key> /pass:<secret>` or Credential Manager → Windows Credentials ▸ Add a generic credential (address = `<service>/<key>`) |
| **Other** | N/A — `Get` returns a hard "not supported on this platform" error | N/A | N/A | N/A |

The config ref is always `keyring:<service>/<key>`, e.g. `client_secret_ref: "keyring:zscalerctl/prod-client-secret"`.

### Reality check — shipped shapes this phase builds on (verified against `main`)

- `SecretRef{Service, Key}` is **already** parsed + validated in `internal/secretref/ref.go` (`keyring:service/key`, both segments non-empty, no extra `/`). Phase 3 only *consumes* `ref.Service`/`ref.Key` — no new parsing.
- `Resolve`'s `case "keyring"` is a live stub at `internal/secretref/resolver.go:49-50` returning `ErrInvalidRef`. Phase 3 replaces it.
- `ResolverOpts` is `{AllowCmd bool}` at `resolver.go:19-21`. Phase 3 adds a `Keyring keyring.Getter` field.
- `resolveCmd` lives in `resolver.go` (not a separate `provider_cmd.go`). Put `resolveKeyring` there too. The original "File Structure" names `provider_keyring.go` / `keyring.Client` / `godbus` are **superseded** by this section.
- `internal/config/load.go:58-61` is a **nil-guard branch**: `if resolver == nil { resolver = secretref.NewResolver(secretref.ResolverOpts{AllowCmd: !disallowCmd}) }`. `keyring.New()` is only evaluated when `opts.Resolver == nil`; add the `Keyring:` field there.

### Architecture — one exec primitive, exit-code-driven, `ctx`-threaded

macOS + Linux both shell out and **share one leak-safe primitive** `runKeyringCmd(ctx, timeout, argv) (stdout, stderr string, exitCode int, err error)` in `internal/keyring/exec.go`. It returns the process **exit code directly** so backends never string-match an error message (a regression that silently breaks not-found detection). `err` is non-nil only for exec-level failures (start failure, timeout, output overflow); a non-zero exit is reported via `exitCode`, not `err`. `Getter.Get(ctx, service, key)` takes a context so the caller's deadline/cancel is honoured (the backend layers its own `defaultKeyringTimeout = 10s` on top). Windows uses no subprocess — a direct `CredReadW` call.

**File layout (`internal/keyring/`):** `keyring.go` (interface, `ErrNotFound`, `decodeUTF16LE`, `New()`, `runnerFunc`), `exec.go` (`runKeyringCmd`, `cappedWriter`, `summarizeStderr`, `filterEnv`), build-tagged `keyring_darwin.go` / `keyring_linux.go` / `keyring_windows.go` / `keyring_other.go`, and matching `_test.go` files.

### Task 3.1: `Getter` interface (`ctx`-threaded) + `ErrNotFound` + `decodeUTF16LE` + un-stub `resolveKeyring`

**Files:**
- Create: `internal/keyring/keyring.go`
- Modify: `internal/secretref/resolver.go` (add `Keyring` to `ResolverOpts`; replace the `keyring` stub at `:49-50`)
- Test: `internal/keyring/keyring_test.go`, `internal/secretref/resolver_keyring_test.go`

- [ ] **Step 1: Failing test — `decodeUTF16LE`.** In `keyring_test.go` (package `keyring`, cross-platform):

```go
func TestDecodeUTF16LE(t *testing.T) {
	// "hi" little-endian: 0x68 0x00 0x69 0x00
	got, err := decodeUTF16LE([]byte{0x68, 0x00, 0x69, 0x00})
	if err != nil || got != "hi" {
		t.Fatalf("got %q, %v; want \"hi\", nil", got, err)
	}
}
func TestDecodeUTF16LEOddBytes(t *testing.T) {
	if _, err := decodeUTF16LE([]byte{0x68}); err == nil {
		t.Fatal("odd byte count must error")
	}
}
func TestDecodeUTF16LEAllNul(t *testing.T) {
	got, err := decodeUTF16LE([]byte{0x00, 0x00, 0x00, 0x00})
	if err != nil || got != "" {
		t.Fatalf("all-NUL blob must decode to empty, got %q, %v", got, err)
	}
}
func TestDecodeUTF16LEEmbeddedNul(t *testing.T) {
	// "hi" NUL "x": must truncate at the first NUL (Windows blob convention).
	// Pin this so a future "optimization" can't silently change it.
	got, err := decodeUTF16LE([]byte{0x68, 0x00, 0x69, 0x00, 0x00, 0x00, 0x78, 0x00})
	if err != nil || got != "hi" {
		t.Fatalf("embedded NUL must truncate to %q, got %q, %v", "hi", got, err)
	}
}
```

- [ ] **Step 2: Run — `go test ./internal/keyring/` → FAIL** (undefined: `decodeUTF16LE`).
- [ ] **Step 3: Write `keyring.go`.**

```go
// Package keyring provides a cgo-free, read-only OS keychain backend behind a
// mockable Getter. macOS/Linux shell out to the OS tool; Windows calls CredReadW.
package keyring

import (
	"context"
	"errors"
	"time"
	"unicode/utf16"
)

// ErrNotFound is returned by Getter.Get when no credential exists for service/key.
var ErrNotFound = errors.New("keyring item not found")

// ErrUnavailable marks a backend that is present but cannot service the request
// right now — a locked keychain, a stopped Secret Service, or a missing helper
// (e.g. secret-tool not installed). Errors wrapping it carry a value-free,
// actionable message BY CONTRACT, so resolveKeyring is allowed to surface that
// message to the user (unlike unknown backend errors, which it suppresses).
var ErrUnavailable = errors.New("keyring backend unavailable")

const defaultKeyringTimeout = 10 * time.Second

// Getter reads a single credential from the OS keychain. Implementations are
// cgo-free and build-tagged per OS. Get returns ErrNotFound when the item is
// absent; every other error is value-free (it never contains the secret).
type Getter interface {
	Get(ctx context.Context, service, key string) (string, error)
}

// New returns the production Getter for the current platform.
func New() Getter { return newBackend() }

// runnerFunc runs argv (no shell) and reports trimmed stdout, captured stderr,
// the process exit code (-1 if it never ran), and a value-free error for
// exec-level failures only (start failure, timeout, overflow). A non-zero exit
// is NOT an error here — the caller interprets exitCode.
type runnerFunc func(ctx context.Context, timeout time.Duration, argv []string) (stdout, stderr string, exitCode int, err error)

// decodeUTF16LE decodes a little-endian UTF-16 byte blob (as stored by Windows
// Credential Manager) into a Go string, trimming at the first NUL. It is pure
// Go so it is unit-testable on every platform, not just Windows.
func decodeUTF16LE(b []byte) (string, error) {
	if len(b)%2 != 0 {
		return "", errors.New("keyring: UTF-16LE blob has odd byte count")
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
	}
	for i, c := range u { // cmdkey may or may not NUL-terminate
		if c == 0 {
			u = u[:i]
			break
		}
	}
	return string(utf16.Decode(u)), nil
}
```

- [ ] **Step 4: Failing test — `resolveKeyring`.** In `internal/secretref/resolver_keyring_test.go` (package `secretref`), with a fake Getter. Coordinate names (service/key) MAY appear in errors; resolved values MUST NOT:

```go
type fakeGetter struct {
	val string
	err error
}

func (f fakeGetter) Get(_ context.Context, _, _ string) (string, error) { return f.val, f.err }

func TestResolveKeyringReturnsSecret(t *testing.T) {
	r := NewResolver(ResolverOpts{Keyring: fakeGetter{val: "s3cr3t"}})
	got, err := r.Resolve(context.Background(), SecretRef{Scheme: "keyring", Service: "svc", Key: "k"})
	if err != nil || got.Reveal() != "s3cr3t" {
		t.Fatalf("got %q, %v", got.Reveal(), err)
	}
}

func TestResolveKeyringNotFound(t *testing.T) {
	r := NewResolver(ResolverOpts{Keyring: fakeGetter{err: keyring.ErrNotFound}})
	_, err := r.Resolve(context.Background(), SecretRef{Scheme: "keyring", Service: "svc", Key: "k"})
	if err == nil || !strings.Contains(err.Error(), "env:/file:/cmd:") {
		t.Fatalf("not-found must hint at alternatives: %v", err)
	}
}

func TestResolveKeyringBackendErrorIsValueFree(t *testing.T) {
	// A buggy backend error must never be echoed (it could embed token material).
	r := NewResolver(ResolverOpts{Keyring: fakeGetter{err: errors.New("D-Bus said s3cr3t")}})
	_, err := r.Resolve(context.Background(), SecretRef{Scheme: "keyring", Service: "svc", Key: "k"})
	if err == nil || strings.Contains(err.Error(), "s3cr3t") || strings.Contains(err.Error(), "D-Bus") {
		t.Fatalf("backend error must not leak: %v", err)
	}
}

func TestResolveKeyringNilGetter(t *testing.T) {
	r := NewResolver(ResolverOpts{}) // no Keyring
	_, err := r.Resolve(context.Background(), SecretRef{Scheme: "keyring", Service: "svc", Key: "k"})
	if err == nil {
		t.Fatal("nil Getter must error clearly")
	}
}

func TestResolveKeyringUnavailableSurfacesHint(t *testing.T) {
	// ErrUnavailable-wrapped errors carry a value-free, actionable message by
	// contract, so the resolver DOES surface it (e.g. the install/start hint).
	hint := fmt.Errorf("keyring: secret-tool not found; install libsecret-tools or use env:/file:/cmd: (%w)", keyring.ErrUnavailable)
	r := NewResolver(ResolverOpts{Keyring: fakeGetter{err: hint}})
	_, err := r.Resolve(context.Background(), SecretRef{Scheme: "keyring", Service: "svc", Key: "k"})
	if err == nil || !strings.Contains(err.Error(), "libsecret-tools") {
		t.Fatalf("ErrUnavailable hint must reach the user: %v", err)
	}
}
```

> Test imports for this file: `context`, `errors`, `fmt`, `strings`, `testing`, plus `internal/keyring`.

- [ ] **Step 5: Run → FAIL** (`ResolverOpts` has no `Keyring`; `keyring` import unused).
- [ ] **Step 6: Implement in `resolver.go`.** Add the import `"github.com/dvmrry/zscalerctl/internal/keyring"`, extend `ResolverOpts`, and replace the stub:

```go
type ResolverOpts struct {
	AllowCmd bool
	Keyring  keyring.Getter
}
```

```go
	case "keyring":
		return r.resolveKeyring(ctx, ref)
```

```go
func (r *Resolver) resolveKeyring(ctx context.Context, ref SecretRef) (secret.Secret, error) {
	if r.opts.Keyring == nil {
		return secret.Secret{}, fmt.Errorf("%w: keyring is not available in this build", ErrInvalidRef)
	}
	value, err := r.opts.Keyring.Get(ctx, ref.Service, ref.Key)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return secret.Secret{}, fmt.Errorf("%w: keyring has no entry for service=%q key=%q; store it or use env:/file:/cmd:", ErrInvalidRef, ref.Service, ref.Key)
		}
		if errors.Is(err, keyring.ErrUnavailable) {
			// ErrUnavailable carries a value-free, actionable message by contract
			// (locked keychain / Secret Service down / helper missing) — safe to surface.
			return secret.Secret{}, fmt.Errorf("%w: %s", ErrInvalidRef, err)
		}
		// Unknown backend error — never echo it (a buggy backend could embed output).
		return secret.Secret{}, fmt.Errorf("%w: keyring lookup failed for service=%q key=%q", ErrInvalidRef, ref.Service, ref.Key)
	}
	if value == "" {
		return secret.Secret{}, fmt.Errorf("%w: keyring entry for service=%q key=%q is empty", ErrInvalidRef, ref.Service, ref.Key)
	}
	return secret.New(value), nil
}
```

- [ ] **Step 7: Run → PASS** (`go test ./internal/keyring/ ./internal/secretref/`).
- [ ] **Step 8: Commit** — `feat(keyring): Getter interface, decodeUTF16LE, resolveKeyring dispatch`.

### Task 3.2: `runKeyringCmd` leak-safe exec primitive (returns exit code)

**Files:** Create `internal/keyring/exec.go`; Test `internal/keyring/exec_test.go`.

- [ ] **Step 1: Failing tests.** In `exec_test.go`, using the **cross-platform test-binary helper pattern** (phase 2's `TestResolverCmdHelperProcess` style). `/usr/bin/env` and `/bin/sh` do NOT exist on the `windows-config` CI runner — this package is untagged and runs there — so re-exec the test binary itself as the stand-in command:

```go
// helperCmd builds an argv that re-execs THIS test binary as a stand-in command.
func helperCmd(args ...string) []string {
	return append([]string{os.Args[0], "-test.run=TestKeyringHelperProcess", "--"}, args...)
}

// TestKeyringHelperProcess is not a real test: when GO_KEYRING_HELPER=1 it acts
// as the external command. Cross-platform — no POSIX paths.
func TestKeyringHelperProcess(t *testing.T) {
	if os.Getenv("GO_KEYRING_HELPER") != "1" {
		return
	}
	args := os.Args
	for i, a := range args {
		if a == "--" {
			args = args[i+1:]
			break
		}
	}
	if len(args) == 0 {
		os.Exit(2) // malformed helper invocation — exit cleanly, don't panic
	}
	switch args[0] {
	case "echo":
		fmt.Fprint(os.Stdout, args[1])
	case "stderr":
		fmt.Fprint(os.Stderr, args[1])
		os.Exit(1)
	case "exit":
		n, _ := strconv.Atoi(args[1])
		os.Exit(n)
	case "sleep":
		time.Sleep(5 * time.Second)
	case "bigout":
		fmt.Fprint(os.Stdout, strings.Repeat("x", 70*1024))
	case "printenv":
		for _, e := range os.Environ() {
			fmt.Fprintln(os.Stdout, e)
		}
	}
	os.Exit(0)
}

func TestRunKeyringCmdScrubsZscalerctlEnv(t *testing.T) {
	t.Setenv("GO_KEYRING_HELPER", "1")
	t.Setenv("ZSCALERCTL_CLIENT_SECRET", "leak-me")
	out, _, code, err := runKeyringCmd(context.Background(), 5*time.Second, helperCmd("printenv"))
	if err != nil || code != 0 {
		t.Fatalf("code=%d err=%v", code, err)
	}
	if strings.Contains(out, "leak-me") || strings.Contains(out, "ZSCALERCTL_") {
		t.Fatal("ZSCALERCTL_* must be scrubbed from the child env")
	}
	if !strings.Contains(out, "GO_KEYRING_HELPER=1") {
		t.Fatal("non-ZSCALERCTL_ env must pass through")
	}
}

func TestRunKeyringCmdNonZeroExit(t *testing.T) {
	t.Setenv("GO_KEYRING_HELPER", "1")
	_, _, code, err := runKeyringCmd(context.Background(), 5*time.Second, helperCmd("exit", "7"))
	if err != nil || code != 7 {
		t.Fatalf("non-zero exit is reported via code, not err: code=%d err=%v", code, err)
	}
}

func TestRunKeyringCmdStderrStaysValueFree(t *testing.T) {
	t.Setenv("GO_KEYRING_HELPER", "1")
	_, stderr, code, err := runKeyringCmd(context.Background(), 5*time.Second, helperCmd("stderr", "TOKEN"))
	if err != nil || code != 1 {
		t.Fatalf("code=%d err=%v", code, err)
	}
	if strings.Contains(summarizeStderr(stderr), "TOKEN") {
		t.Fatal("summarizeStderr must omit stderr content")
	}
}

func TestRunKeyringCmdTimeoutKillsProcess(t *testing.T) {
	t.Setenv("GO_KEYRING_HELPER", "1")
	start := time.Now()
	_, _, _, err := runKeyringCmd(context.Background(), 50*time.Millisecond, helperCmd("sleep"))
	if err == nil || time.Since(start) > 3*time.Second {
		t.Fatalf("timeout must bound the run, took %s err=%v", time.Since(start), err)
	}
}

func TestRunKeyringCmdStdoutOverflow(t *testing.T) { // bounded-output path (Kimi)
	t.Setenv("GO_KEYRING_HELPER", "1")
	_, _, _, err := runKeyringCmd(context.Background(), 5*time.Second, helperCmd("bigout"))
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("oversized stdout must be a clean value-free error: %v", err)
	}
}

func TestRunKeyringCmdEmptyArgv(t *testing.T) { // must not panic on argv[0]
	if _, _, _, err := runKeyringCmd(context.Background(), time.Second, nil); err == nil {
		t.Fatal("empty argv must return an error, not panic")
	}
}

func TestRunKeyringCmdStartFailureIsUnavailable(t *testing.T) {
	// An ABSOLUTE path that doesn't exist yields *fs.PathError (os.ErrNotExist),
	// NOT exec.ErrNotFound — that sentinel is only for bare names via LookPath.
	// runKeyringCmd wraps any start failure with ErrUnavailable, which is what
	// resolveKeyring keys on. (The Linux missing-tool case is caught earlier by
	// exec.LookPath in the backend, not here.)
	_, _, _, err := runKeyringCmd(context.Background(), time.Second, []string{"/nonexistent-zscalerctl-bin"})
	if err == nil || !errors.Is(err, ErrUnavailable) || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("start failure must wrap ErrUnavailable and os.ErrNotExist: %v", err)
	}
}
```

> Test imports: `context`, `errors`, `fmt`, `os`, `strconv`, `strings`, `testing`, `time`.

- [ ] **Step 2: Run → FAIL** (undefined: `runKeyringCmd`).
- [ ] **Step 3: Implement `exec.go`** (mirrors `resolveCmd`'s leak-safety; `cappedWriter`/`summarizeStderr` duplicated here intentionally — keyring is a *lower* layer than `secretref` and cannot import it; a shared `execcapture` extraction is a deferred YAGNI cleanup):

```go
package keyring

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// runKeyringCmd runs argv with no shell, a bounded timeout (+WaitDelay grace),
// a ZSCALERCTL_*-scrubbed env, and bounded output. It returns trimmed stdout,
// captured stderr, the exit code (-1 if the process never ran), and a
// value-free error for exec-level failures only. A non-zero exit is reported
// via exitCode, not err. It never includes stdout/stderr CONTENT in its error.
func runKeyringCmd(ctx context.Context, timeout time.Duration, argv []string) (string, string, int, error) {
	if len(argv) == 0 || strings.TrimSpace(argv[0]) == "" {
		return "", "", -1, errors.New("keyring: empty command")
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// #nosec G204 -- argv[0] is a fixed OS keychain tool; service/key are
	// validated and passed as separate argv elements with no shell. Mirrors resolveCmd.
	cmd := exec.CommandContext(cctx, argv[0], argv[1:]...)
	cmd.WaitDelay = 2 * time.Second
	cmd.Env = filterEnv(os.Environ()) // strip ZSCALERCTL_* so a child never sees our other secrets

	stdout := &cappedWriter{limit: 64 * 1024}
	stderr := &cappedWriter{limit: 16 * 1024, truncate: true} // diagnostic-only: truncate, don't fail a good lookup
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	runErr := cmd.Run()
	// stdout overflow takes precedence: a >64KB "secret" is bogus, and the capped
	// writer makes cmd.Run() return the write error — don't misread it as a start
	// failure. stderr truncates silently, so a noisy-but-successful command (exit 0
	// with >16KB of warnings) is never failed over diagnostic output.
	if stdout.err != nil {
		return "", "", -1, fmt.Errorf("keyring: %q stdout too large", argv[0])
	}
	out := strings.TrimRight(stdout.String(), "\r\n")
	errOut := stderr.String()

	if runErr != nil {
		if errors.Is(cctx.Err(), context.DeadlineExceeded) {
			// Locked/hung keychain: actionable + value-free, so resolveKeyring surfaces it.
			return "", "", -1, fmt.Errorf("keyring: %q timed out after %s (keychain may be locked or require interaction); use env:/file:/cmd: (%w)", argv[0], timeout, ErrUnavailable)
		}
		if cctx.Err() != nil {
			return "", "", -1, cctx.Err() // caller cancelled — propagate the context error
		}
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			// Process ran and exited non-zero — hand the code back to the caller.
			return out, errOut, exitErr.ExitCode(), nil
		}
		// Exec never started (binary missing / not executable). Double-wrap so the
		// underlying cause stays inspectable (exec.ErrNotFound for a bare name,
		// fs.ErrNotExist for an absolute path) alongside ErrUnavailable. Value-free.
		return "", "", -1, fmt.Errorf("keyring: %q could not run: %w (%w)", argv[0], runErr, ErrUnavailable)
	}
	return out, errOut, 0, nil
}

func filterEnv(environ []string) []string {
	out := environ[:0:0]
	for _, e := range environ {
		if !strings.HasPrefix(e, "ZSCALERCTL_") {
			out = append(out, e)
		}
	}
	return out
}

func summarizeStderr(s string) string {
	if s == "" {
		return "no stderr"
	}
	return fmt.Sprintf("stderr omitted (%d bytes)", len(s))
}

// cappedWriter bounds an in-memory buffer at limit bytes. With truncate=false
// (stdout) it errors on overflow so an oversized "secret" is rejected. With
// truncate=true (stderr) it silently drops the overflow and reports success, so
// a noisy-but-successful command is never failed over diagnostic output.
type cappedWriter struct {
	buf      bytes.Buffer
	limit    int
	truncate bool
	err      error
}

func (w *cappedWriter) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	if w.buf.Len()+len(p) > w.limit {
		if w.truncate {
			if room := w.limit - w.buf.Len(); room > 0 {
				w.buf.Write(p[:room])
			}
			return len(p), nil // pretend full success; overflow dropped
		}
		w.err = errors.New("output limit exceeded")
		return 0, w.err
	}
	return w.buf.Write(p)
}

func (w *cappedWriter) String() string { return w.buf.String() }
```

- [ ] **Step 4: Run → PASS.** **Step 5: Commit** — `feat(keyring): bounded leak-safe runKeyringCmd exec primitive`.

### Task 3.3: macOS backend (`/usr/bin/security`)

**Files:** Create `internal/keyring/keyring_darwin.go`; Test `internal/keyring/keyring_darwin_test.go`.

- [ ] **Step 1: Failing test** with an injected runner seam (no real keychain; the stub returns what the *real* `runKeyringCmd` would — `(stdout, stderr, exitCode, nil)` — so it cannot mask the exit-code path):

```go
//go:build darwin

package keyring

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"
)

func newDarwinWithRunner(r runnerFunc) *macOSGetter { return &macOSGetter{run: r, timeout: time.Second} }

func TestDarwinGetNotFoundExit44(t *testing.T) {
	g := newDarwinWithRunner(func(context.Context, time.Duration, []string) (string, string, int, error) {
		return "", "security: ... item could not be found", 44, nil
	})
	if _, err := g.Get(context.Background(), "svc", "k"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("exit 44 must map to ErrNotFound, got %v", err)
	}
}

func TestDarwinGetSuccess(t *testing.T) {
	g := newDarwinWithRunner(func(context.Context, time.Duration, []string) (string, string, int, error) {
		return "s3cr3t", "", 0, nil
	})
	if got, err := g.Get(context.Background(), "svc", "k"); err != nil || got != "s3cr3t" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestDarwinGetEmptyValueErrors(t *testing.T) {
	g := newDarwinWithRunner(func(context.Context, time.Duration, []string) (string, string, int, error) {
		return "", "", 0, nil
	})
	if _, err := g.Get(context.Background(), "svc", "k"); err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("empty stored value is a hard error, got %v", err)
	}
}

// Live, off in CI: ZSCALERCTL_KEYRING_LIVE=1 adds/reads/deletes a real item.
func TestDarwinGetLive(t *testing.T) {
	if os.Getenv("ZSCALERCTL_KEYRING_LIVE") == "" {
		t.Skip("set ZSCALERCTL_KEYRING_LIVE=1 to run against the real Keychain")
	}
	const svc, acct, want = "zscalerctl-livetest", "k", "naïve-pä$$wörd" // non-ASCII exercises the decode
	if out, err := exec.Command("/usr/bin/security", "add-generic-password", "-s", svc, "-a", acct, "-w", want, "-U").CombinedOutput(); err != nil {
		t.Fatalf("seed failed: %v: %s", err, out)
	}
	// Always clean up, even if an assertion below fails (no leaked test credential).
	t.Cleanup(func() {
		_ = exec.Command("/usr/bin/security", "delete-generic-password", "-s", svc, "-a", acct).Run()
	})
	got, err := New().Get(context.Background(), svc, acct)
	if err != nil || got != want {
		t.Fatalf("Get = %q, %v; want %q", got, err, want)
	}
}
```

- [ ] **Step 2: Run → FAIL.** **Step 3: Implement `keyring_darwin.go`:**

```go
//go:build darwin

package keyring

import (
	"context"
	"fmt"
	"time"
)

const (
	// errSecItemNotFoundExit is the low byte of errSecItemNotFound (OSStatus
	// 0xFFFF9D2C & 0xFF = 0x2C = 44). Verified live on Darwin 24.6.0.
	errSecItemNotFoundExit = 44
	// errSecInteractionNotAllowedExit is the low byte of errSecInteractionNotAllowed
	// (-25308 = 0xFFFF9D24 & 0xFF = 0x24 = 36): keychain locked / no interaction allowed.
	errSecInteractionNotAllowedExit = 36
)

type macOSGetter struct {
	run     runnerFunc
	timeout time.Duration
}

func newBackend() Getter { return &macOSGetter{run: runKeyringCmd, timeout: defaultKeyringTimeout} }

func (g *macOSGetter) Get(ctx context.Context, service, key string) (string, error) {
	// -w writes ONLY the raw password to stdout. NEVER use -g (routes it to stderr).
	// Absolute path: a rogue `security` in PATH must not intercept the lookup.
	argv := []string{"/usr/bin/security", "find-generic-password", "-s", service, "-a", key, "-w"}
	stdout, _, code, err := g.run(ctx, g.timeout, argv)
	if err != nil {
		return "", err // already value-free (timeout / overflow / exec failure)
	}
	if code == errSecItemNotFoundExit {
		return "", ErrNotFound
	}
	if code == errSecInteractionNotAllowedExit {
		// Locked keychain / interaction disallowed (headless): actionable hint.
		return "", fmt.Errorf("keyring: macOS Keychain is locked or requires interaction; unlock it or use env:/file:/cmd: (%w)", ErrUnavailable)
	}
	if code != 0 {
		return "", fmt.Errorf("keyring: /usr/bin/security failed (exit %d)", code)
	}
	if stdout == "" {
		return "", fmt.Errorf("keyring: item has no value")
	}
	return stdout, nil
}
```

- [ ] **Step 4: Run → PASS** (`go test ./internal/keyring/` on macOS). **Step 5: Commit** — `feat(keyring): macOS security backend`.

### Task 3.4: Linux backend (`secret-tool`)

**Files:** Create `internal/keyring/keyring_linux.go`; Test `internal/keyring/keyring_linux_test.go`.

- [ ] **Step 1: Failing test** (runner seam; covers found / not-found / D-Bus-unavailable). `secret-tool` exits 1 for *both* not-found and service-unavailable — empty trimmed stdout is the discriminator; stderr language separates unavailable from absent:

```go
//go:build linux

func newLinuxWithRunner(r runnerFunc) *linuxGetter { return &linuxGetter{run: r, timeout: time.Second, lookPath: func(string) (string, error) { return "/usr/bin/secret-tool", nil }} }

func TestLinuxGetFound(t *testing.T) {
	g := newLinuxWithRunner(func(context.Context, time.Duration, []string) (string, string, int, error) {
		return "s3cr3t", "", 0, nil
	})
	if got, err := g.Get(context.Background(), "svc", "k"); err != nil || got != "s3cr3t" {
		t.Fatalf("got %q, %v", got, err)
	}
}
func TestLinuxGetNotFound(t *testing.T) {
	g := newLinuxWithRunner(func(context.Context, time.Duration, []string) (string, string, int, error) {
		return "", "No matching secrets", 1, nil // exit 1 + empty stdout = not found
	})
	if _, err := g.Get(context.Background(), "svc", "k"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}
func TestLinuxGetServiceUnavailable(t *testing.T) {
	g := newLinuxWithRunner(func(context.Context, time.Duration, []string) (string, string, int, error) {
		return "", "Failed to connect to the D-Bus session bus", 1, nil
	})
	_, err := g.Get(context.Background(), "svc", "k")
	if err == nil || errors.Is(err, ErrNotFound) || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("D-Bus failure must be ErrUnavailable, not ErrNotFound: %v", err)
	}
}
func TestLinuxGetToolMissing(t *testing.T) {
	g := &linuxGetter{run: nil, timeout: time.Second, lookPath: func(string) (string, error) { return "", exec.ErrNotFound }}
	_, err := g.Get(context.Background(), "svc", "k")
	if err == nil || errors.Is(err, ErrNotFound) || !errors.Is(err, ErrUnavailable) || !strings.Contains(err.Error(), "libsecret-tools") {
		t.Fatalf("missing tool must be ErrUnavailable with an install hint: %v", err)
	}
}
```

- [ ] **Step 2: Run → FAIL.** **Step 3: Implement `keyring_linux.go`** (`lookPath` is a struct field so the missing-tool path is testable without mutating a global):

```go
//go:build linux

package keyring

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type linuxGetter struct {
	run      runnerFunc
	timeout  time.Duration
	lookPath func(string) (string, error)
}

func newBackend() Getter {
	return &linuxGetter{run: runKeyringCmd, timeout: defaultKeyringTimeout, lookPath: exec.LookPath}
}

func (g *linuxGetter) Get(ctx context.Context, service, key string) (string, error) {
	if _, err := g.lookPath("secret-tool"); err != nil {
		return "", fmt.Errorf("keyring: secret-tool not found; install libsecret-tools (apt install libsecret-tools / dnf install libsecret) or use env:/file:/cmd: (%w)", ErrUnavailable)
	}
	argv := []string{"secret-tool", "lookup", "service", service, "account", key}
	stdout, stderr, code, err := g.run(ctx, g.timeout, argv)
	if err != nil {
		return "", err
	}
	if code == 0 {
		if stdout == "" {
			return "", ErrNotFound
		}
		return stdout, nil
	}
	if code == 1 && stdout == "" {
		if looksLikeServiceUnavailable(stderr) {
			return "", fmt.Errorf("keyring: Secret Service unavailable; ensure gnome-keyring or kwallet is running, or use env:/file:/cmd: for headless use (%w)", ErrUnavailable)
		}
		return "", ErrNotFound
	}
	return "", fmt.Errorf("keyring: secret-tool failed (exit %d)", code)
}

// looksLikeServiceUnavailable is a conservative best-effort heuristic over
// stderr (never echoed). False-negatives (treating unavailable as not-found)
// are preferred to false-positives; "gdbus"/D-Bus markers are intentionally broad.
func looksLikeServiceUnavailable(stderr string) bool {
	s := strings.ToLower(stderr)
	for _, m := range []string{"failed to connect", "could not connect", "no such interface", "org.freedesktop.dbus", "gdbus"} {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}
```

- [ ] **Step 4: Run → PASS** (`GOOS=linux go test` or on a Linux host). **Step 5: Commit** — `feat(keyring): Linux secret-tool backend`.

### Task 3.5: Windows backend (`CredReadW`) — highest-risk; live Windows-host validation required

**Files:** Create `internal/keyring/keyring_windows.go`; Test `internal/keyring/keyring_windows_test.go`. **`x/sys/windows` does not export `CredReadW`** — declare the `LazyProc` and `credentialW` struct here.

- [ ] **Step 1: Failing struct-layout test** (amd64-specific; the 4-byte pad is the single highest-risk detail):

```go
//go:build windows && amd64

package keyring

import (
	"testing"
	"unsafe"
)

func TestCredentialWLayout(t *testing.T) {
	var c credentialW
	if off := unsafe.Offsetof(c.CredentialBlob); off != 40 {
		t.Fatalf("CredentialBlob must be at offset 40 on amd64 (pad missing?), got %d", off)
	}
	if sz := unsafe.Sizeof(c); sz != 80 {
		t.Fatalf("credentialW must be 80 bytes on amd64, got %d", sz)
	}
}
```

- [ ] **Step 2: Run → FAIL** (`GOOS=windows GOARCH=amd64 go test ./internal/keyring/`). **Step 3: Implement `keyring_windows.go`:**

```go
//go:build windows

package keyring

import (
	"context"
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// credentialW mirrors CREDENTIALW on amd64. The 4-byte pad between
// CredentialBlobSize and CredentialBlob is MANDATORY on 64-bit Windows — the C
// compiler inserts it to 8-byte-align the pointer. Omitting it silently misreads
// the blob. Verify: unsafe.Offsetof(credentialW{}.CredentialBlob) == 40 (Task 3.5 test).
type credentialW struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	_                  [4]byte // alignment pad — do NOT remove
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

const credTypeGeneric uint32 = 1 // CRED_TYPE_GENERIC

var (
	// NewLazySystemDLL forces System32-only load (DLL-hijack-proof).
	modAdvapi32   = windows.NewLazySystemDLL("advapi32.dll")
	procCredReadW = modAdvapi32.NewProc("CredReadW")
	procCredFree  = modAdvapi32.NewProc("CredFree")
)

type windowsGetter struct{}

func newBackend() Getter { return windowsGetter{} }

// Get reads the credential stored under TargetName = service+"/"+key.
// Errors are value-free: they name the service only — never the key, the
// target name, the errno text, or any blob bytes.
func (windowsGetter) Get(ctx context.Context, service, key string) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	targetPtr, err := windows.UTF16PtrFromString(service + "/" + key)
	if err != nil {
		return "", fmt.Errorf("keyring: invalid target name for service %q", service)
	}

	var cred *credentialW
	r1, _, lastErr := procCredReadW.Call(
		uintptr(unsafe.Pointer(targetPtr)),
		uintptr(credTypeGeneric),
		0,
		uintptr(unsafe.Pointer(&cred)),
	)
	if r1 == 0 {
		if errors.Is(lastErr, windows.ERROR_NOT_FOUND) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("keyring: read failed for service %q (windows errno %d)", service, lastErr)
	}
	// cred points to OS-allocated memory; deref at call time, not at defer-registration.
	defer func() { procCredFree.Call(uintptr(unsafe.Pointer(cred))) }() //nolint:errcheck // CredFree has no return

	blobSize := cred.CredentialBlobSize
	if blobSize == 0 {
		return "", fmt.Errorf("keyring: credential for service %q has empty blob", service)
	}
	// Copy the blob into Go memory BEFORE CredFree fires, then decode UTF-16LE.
	blob := make([]byte, blobSize)
	copy(blob, unsafe.Slice(cred.CredentialBlob, blobSize)) // CredentialBlob is already *byte
	value, derr := decodeUTF16LE(blob)
	if derr != nil {
		return "", fmt.Errorf("keyring: credential for service %q is corrupted", service)
	}
	if value == "" { // all-NUL blob decodes to empty
		return "", fmt.Errorf("keyring: credential for service %q is empty", service)
	}
	return value, nil
}
```

- [ ] **Step 4: Live-gated test** in `keyring_windows_test.go` (imports `os`, `testing`; `ZSCALERCTL_KEYRING_LIVE=1`, off in CI): pre-seed via `cmdkey /generic:zscalerctl-test/k /user:x /pass:hunter2`, assert `Get` returns `hunter2`; **also seed a non-ASCII secret** (e.g. `näïve-pä$$wörd-🔑` — `cmdkey /pass` mangles non-ASCII, so seed this one via the Credential Manager GUI or PowerShell) and assert it round-trips byte-exact — that path, not ASCII, is the real UTF-16LE decode risk. Assert a missing target → `ErrNotFound`.
- [ ] **Step 5: Run → PASS** (struct-layout test on the cross-compile; unit/live on the host). **Step 6: Commit** — `feat(keyring): Windows CredReadW backend`.
- [ ] **Step 7: MANDATORY live acceptance on the operator's Windows host** (the merge gate for this task — see "Required before merge"). Empirically confirm the blob encoding: store via `cmdkey` AND via the Credential Manager GUI, read both back, confirm `decodeUTF16LE` yields the exact secret. **Include at least one non-ASCII secret** (the actual UTF-16LE risk) and confirm a byte-exact round-trip; confirm not-found → `ErrNotFound`. Record the result in the PR.

### Task 3.6: unsupported-platform backend + wire `keyring.New()` into `load.go` (test-first)

**Files:** Create `internal/keyring/keyring_other.go`; Modify `internal/config/load.go:58-61`; Test `internal/config/load_test.go`.

- [ ] **Step 1: `keyring_other.go`** (so non-darwin/linux/windows still builds; `New()` is always non-nil):

```go
//go:build !darwin && !linux && !windows

package keyring

import (
	"context"
	"fmt"
)

type unsupportedGetter struct{}

func newBackend() Getter { return unsupportedGetter{} }

func (unsupportedGetter) Get(context.Context, string, string) (string, error) {
	return "", fmt.Errorf("keyring: not supported on this platform; use env:/file:/cmd:")
}
```

- [ ] **Step 2: Wire `load.go:58-61`** — add the `Keyring` field to the nil-guard branch (only evaluated when the caller passes no `Resolver`):

```go
	if resolver == nil {
		resolver = secretref.NewResolver(secretref.ResolverOpts{
			AllowCmd: !disallowCmd,
			Keyring:  keyring.New(),
		})
	}
```

Add the import `"github.com/dvmrry/zscalerctl/internal/keyring"`.

- [ ] **Step 3: Coverage test** in `internal/config/load_test.go`, using the existing `writeConfig` + `LoadConfig` pattern. A `keyring:` ref is **deferred** — it does NOT resolve at load time, so this passes *without* a keyring getter; it pins that a keyring profile ref flows through `LoadConfig` into a keyring-scheme deferred source. The `keyring.New()` injection above is plumbing verified by the build + the `resolveKeyring` tests in Task 3.1 (there is no genuine red-before-green for a one-line wiring change).

```go
func TestLoadConfigKeyringRefIsDeferredKeyringSource(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `
default_profile: prod
profiles:
  prod:
    vanity_domain: v
    client_id: c
    client_secret_ref: keyring:svc/key
    zpa_customer_id: z
`)
	cfg, err := config.LoadConfig(nil, config.LoadOptions{ConfigPath: path})
	if err != nil {
		t.Fatalf("LoadConfig(keyring ref) error = %v, want nil", err)
	}
	if got := cfg.Credentials.ClientSecret.Scheme(); got != "keyring" {
		t.Fatalf("ClientSecret scheme = %q, want keyring", got)
	}
}
```

- [ ] **Step 4: Run → PASS** (`go test ./internal/config/`). **Step 5: Commit** — `feat(keyring): wire keyring.New() into the resolver`.

### Task 3.7: `CGO_ENABLED=0` matrix + zero-new-deps gate + docs + draft PR

**Files:** `docs/THREAT_MODEL.md`, `docs/INSTALL.md`; no code.

- [ ] **Step 1: Static-binary gate** — confirm cgo-free cross-compiles for all three OSes:

```
CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build ./...
CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build ./...
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...
```
Expected: all succeed (no cgo). Run `GOOS=windows GOARCH=amd64 go vet ./internal/keyring/` to type-check the unsafe Windows path.

- [ ] **Step 2: Zero-new-deps gate** — `git diff main -- go.mod go.sum` MUST be empty. (`golang.org/x/sys` is already present at its vendored version and the `windows` sub-package is already vendored — this gate forbids *new* `go.mod`/`go.sum` entries, not `x/sys` usage. Do not remove the `x/sys/windows` import to "satisfy" it.)
- [ ] **Step 3: `make check`** fully green (`go test`, `-race`, vet, staticcheck, govulncheck, gitleaks, verify scripts).
- [ ] **Step 4: `THREAT_MODEL.md`** — after the `cmd:`-provider paragraph (ends "... `ZSCALERCTL_DISALLOW_CMD=true`."), add a `keyring:` subsection: read-only; no shell (macOS/Linux exec absolute-path/PATH-resolved, bounded, env-scrubbed, value-free errors); Windows direct `CredReadW` via System32-only DLL load (no PATH/DLL hijack), UTF-16LE blob; headless caveat (locked/absent keychain → bounded-timeout error, use env:/file:/cmd:); per-OS storage conventions reference the table above.
- [ ] **Step 5: `INSTALL.md`** — create a `### Secret providers` subsection if absent (current headers: Switching Tenants, Bash/Zsh/Fish/PowerShell); document the `keyring:<service>/<key>` ref form and the per-OS **store** commands (macOS `security add-generic-password`; Linux `secret-tool store` + the `libsecret-tools` requirement and the `service`/`account` attribute names; Windows `cmdkey` / Credential Manager). Note keyring is a desktop convenience — agents/CI keep using env.
- [ ] **Step 6: Commit docs** (`docs(keyring): threat model + install conventions`) — **before** opening the PR so all commits are present at creation.
- [ ] **Step 7: Open the draft PR** (`semver:minor`). PR body: the zero-dep decision + rationale, the per-OS conventions, and an explicit **"Required before merge"** checklist (below). Request the cross-family security review.

### Required before merge

- [ ] **Cross-family security review** of `keyring_windows.go` (the unsafe `CredReadW` syscall: struct layout/pad, `CredFree` lifetime, UTF-16LE decode, `ERROR_NOT_FOUND` mapping) and the shared exec leak-safety (`runKeyringCmd`, value-free errors, env scrub) — the established discipline that caught real bugs in the phase-1 DACL and phase-2 `cmd:` paths.
- [ ] **Live acceptance on the operator's Windows host** (Task 3.5 Step 7): empirically pin the blob encoding (store via `cmdkey` and the GUI, read both back), confirm a real read and a not-found → `ErrNotFound`. This is the gate that justifies hand-rolling Windows rather than taking a library.
- [ ] **Live keychain smoke** on macOS (`ZSCALERCTL_KEYRING_LIVE=1`): add → read → delete → `ErrNotFound`.
- [ ] `CGO_ENABLED=0` matrix green; `git diff main -- go.mod go.sum` empty; `make check` green.

---

## PHASE 4 — Surfacing polish, config JSON schema, remaining docs

**Files:** `docs/schema/config.schema.json` + drift test (mirror `internal/diff/published_schema_test.go`), `docs/INSTALL.md` (full profiles+providers section), `AGENTS.md` + `skills/zscalerctl/SKILL.md` (note: agents keep using env), fuller `config show`/`doctor` output.

- [ ] **Task 4.1:** Publish `config.schema.json`; add a drift-gated test asserting the schema matches the `fileModel`/`profileModel` shape (every field present; provider-ref forms documented). Pattern: `internal/diff/published_schema_test.go`.
- [ ] **Task 4.2:** INSTALL.md "Profiles & Secret Providers" section — env/`*_FILE` remains the primary/CI path; show a profile example with each provider; document precedence + no-fallback + the `cmd:`/keyring desktop-vs-headless guidance.
- [ ] **Task 4.3:** AGENTS.md/skill one-liner: agents use env; profiles are operator ergonomics. Run `scripts/sync-agents-skill.sh` after editing the skill.
- [ ] **Task 4.4:** `make check` green; open phase-4 PR (`semver:patch`), review, merge.

---

## Self-Review (against the frozen spec)

- **Spec coverage:** SecretSource (1.1,1.5,1.6) ✓; precedence (1.8) ✓; config file + perms POSIX+Windows (1.3,1.7) ✓; SafeConfig metadata + no-resolution proof (1.5,1.9) ✓; env/file (1.4,1.6) ✓; cmd structured+timeout+killswitch (2.1–2.3) ✓; keyring cgo-free (3.1–3.2) ✓; surfacing/schema/docs (4.x) ✓; backward-compat (1.6) ✓; no-fallback + unknown-scheme (1.2,1.4) ✓; keyring segment rules (1.2) ✓; value-free errors (2.2) ✓.
- **Placeholders:** none — each task has concrete code or a concrete acceptance criterion + signatures; the one open *decision* (keyring lib vs hand-roll, Task 3.2) is an explicit decision step with both branches specified, not a TODO.
- **Type consistency:** `SecretSource`/`SecretRef`/`Resolver`/`ResolverOpts`/`Getter`/`fileperm.Validate`/`Deferred`/`Resolved`/`Unset`/`DefaultCmdTimeout` are named consistently across tasks.
- **Verify before each PR:** `go test -mod=vendor ./...` and `make check` (gofmt, vet, staticcheck, govulncheck, semgrep, gitleaks, verify-docs, verify-actions-pinned, sync-agents-skill --check, verify-release-artifacts). New deps: `go mod tidy && go mod vendor` + `verify-licenses.sh`.
