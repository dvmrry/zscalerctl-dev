# Config Profiles + Pluggable Secret Providers — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. This plan is also written to be handed to an external coding agent (Codex); each phase is a self-contained, mergeable PR.

**Goal:** Add opt-in YAML config profiles and a pluggable secret-provider layer (`env`/`file`/`cmd`/`keyring`) to `zscalerctl`, without regressing the env-only security posture.

**Architecture:** Additive layer over the existing `config.LoadEnv(environ) → Config` seam. Credential fields on `Config` become a lazy-capable `SecretSource`. Precedence is `flag > env > profile > default`; env-inline/env-file resolve eagerly (today's behavior), profile `*_ref`s resolve only at live-reader construction. Providers live behind small interfaces; keyring is cgo-free.

**Tech Stack:** Go (existing), `gopkg.in/yaml.v3` (config parse), `golang.org/x/sys/windows` (DACL validation, cgo-free), `os/exec` (cmd provider), `godbus/dbus` + `x/sys/windows` + `security` CLI (keyring backends).

**Frozen spec:** [docs/superpowers/specs/2026-06-17-config-profiles-secret-providers-design.md](../specs/2026-06-17-config-profiles-secret-providers-design.md). Read it before starting; this plan implements it.

---

## File Structure (decomposition)

**New:**
- `internal/secretref/ref.go` — `SecretRef` type + `UnmarshalYAML` (string-or-structured), scheme parsing/validation.
- `internal/secretref/source.go` — `SecretSource` interface + `resolved`/`deferred`/`unset` impls.
- `internal/secretref/resolver.go` — `Resolver` dispatching a `SecretRef` to a provider.
- `internal/secretref/provider_env.go`, `provider_file.go`, `provider_cmd.go`, `provider_keyring.go` — one provider each.
- `internal/fileperm/fileperm.go` + `fileperm_posix.go` + `fileperm_windows.go` — owner-only (POSIX) and DACL (Windows) validation, shared by config loader and `file:` provider.
- `internal/keyring/keyring.go` (interface) + `keyring_linux.go` / `keyring_windows.go` / `keyring_darwin.go` — cgo-free backends behind a mockable `Client`.
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

**Outcome:** `keyring:` refs resolve from the OS keychain, cgo-free, static binary intact, behind a mockable interface. Mergeable PR.

### Task 3.1: `keyring.Client` interface + mock; wire `resolveKeyring`

**Files:** `internal/keyring/keyring.go`, `internal/secretref/provider_keyring.go`, tests.

- [ ] **Step 1: Failing test** — `resolveKeyring` calls `Client.Get(service,key)` and returns the secret; a not-found error from the client surfaces a clear "use env:/file:/cmd:" message; nil client (no backend) errors clearly.
- [ ] **Step 2–4:** Define `type Getter interface { Get(service, key string) (string, error) }`; `resolveKeyring` calls `r.opts.Keyring.Get(...)`. Commit.

### Task 3.2: cgo-free backends (Linux D-Bus / Windows wincred / macOS `security`)

**Files:** `internal/keyring/keyring_linux.go` / `_windows.go` / `_darwin.go` (build-tagged), tests gated behind an env flag so CI never hits a real keychain.

- [ ] **Decision step:** evaluate `zalando/go-keyring` (cgo-free: D-Bus/syscall/`security`) vs hand-rolling. If using the dependency: `go get` + `go mod vendor` + pass `verify-licenses.sh` + pin/vet; wrap it behind our `Getter` so it stays swappable. If hand-rolling: macOS = `exec security find-generic-password -s <service> -a <key> -w`; Windows = `x/sys/windows` `CredRead`; Linux = `godbus/dbus` Secret Service `GetSecret`. **Record the choice + rationale in the PR.**
- [ ] **Steps:** implement the chosen backend behind `Getter`; unit-test the macOS exec path by stubbing the command runner; keep real-keychain tests behind `ZSCALERCTL_KEYRING_LIVE=1` (off in CI). Build all three via build tags; confirm `CGO_ENABLED=0` cross-compile still works in the release matrix (dry-run `go build`).
- [ ] **Commit + phase-3 PR** (`semver:minor`); confirm the release `CGO_ENABLED=0` darwin/linux/windows builds still pass.

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
