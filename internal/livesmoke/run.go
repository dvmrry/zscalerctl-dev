package livesmoke

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
)

// Options configures a live smoke run, mirroring scripts/live-smoke.sh flags.
type Options struct {
	OutDir              string
	Bin                 string // empty -> "go run -mod=vendor ./cmd/zscalerctl"
	Resources           []string
	ManifestPath        string
	NoManifest          bool
	RequireCredentials  bool
	RequireNonempty     bool
	StrictCounts        bool
	SkipCredentialCheck bool
}

// Runner executes the zscalerctl CLI under test. Read commands return stdout;
// dump writes files under its --out directory and returns its notice on stdout.
// A real run execs the binary; tests inject a fake runner.
type Runner interface {
	Run(args ...string) (stdout, stderr []byte, exitCode int)
}

// Env resolves an environment variable. Tests inject a fake; the default is the
// process environment.
type Env func(string) string

// osEnv reads from the process environment.
func osEnv(key string) string { return os.Getenv(key) }

// NewExecRunner returns a Runner that executes the real CLI: the given binary,
// or `go run -mod=vendor ./cmd/zscalerctl` when bin is empty.
func NewExecRunner(bin string) Runner { return newExecRunner(bin) }

// execRunner runs the real CLI binary (or `go run`).
type execRunner struct {
	argv []string // command prefix, e.g. ["go","run","-mod=vendor","./cmd/zscalerctl"]
}

func newExecRunner(bin string) *execRunner {
	if bin != "" {
		return &execRunner{argv: []string{bin}}
	}
	return &execRunner{argv: []string{"go", "run", "-mod=vendor", "./cmd/zscalerctl"}}
}

func (r *execRunner) Run(args ...string) (stdout, stderr []byte, exitCode int) {
	full := append(append([]string(nil), r.argv...), args...)
	cmd := exec.Command(full[0], full[1:]...)
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			code = 1
			if errb.Len() == 0 {
				errb.WriteString(err.Error())
			}
		}
	}
	return out.Bytes(), errb.Bytes(), code
}

func isSet(env Env, key string) bool { return strings.TrimSpace(env(key)) != "" }

// credentialFamily reports the configured live credential family ("OneAPI" or
// "ZIA legacy"), or "" when no supported family is fully configured. Mirrors the
// shell credential_family.
func credentialFamily(env Env) string {
	oneapi := isSet(env, "ZSCALERCTL_CLIENT_ID") &&
		(isSet(env, "ZSCALERCTL_CLIENT_SECRET") || isSet(env, "ZSCALERCTL_CLIENT_SECRET_FILE")) &&
		isSet(env, "ZSCALERCTL_VANITY_DOMAIN")
	legacy := isSet(env, "ZSCALERCTL_ZIA_USERNAME") &&
		(isSet(env, "ZSCALERCTL_ZIA_PASSWORD") || isSet(env, "ZSCALERCTL_ZIA_PASSWORD_FILE")) &&
		(isSet(env, "ZSCALERCTL_ZIA_API_KEY") || isSet(env, "ZSCALERCTL_ZIA_API_KEY_FILE")) &&
		isSet(env, "ZSCALERCTL_ZIA_CLOUD")

	switch env("ZSCALERCTL_AUTH_MODE") {
	case "zia-legacy":
		if legacy {
			return "ZIA legacy"
		}
		return ""
	case "", "oneapi":
		if oneapi {
			return "OneAPI"
		}
		if env("ZSCALERCTL_AUTH_MODE") == "" && legacy {
			return "ZIA legacy"
		}
		return ""
	default:
		return ""
	}
}
