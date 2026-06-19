package secretref

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/dvmrry/zscalerctl/internal/credentials"
	"github.com/dvmrry/zscalerctl/internal/keyring"
	"github.com/dvmrry/zscalerctl/internal/secret"
)

var ErrNoResolver = errors.New("secret resolver is not configured")

type ResolverOpts struct {
	AllowCmd bool
	Keyring  keyring.Getter
}

type Resolver struct {
	opts ResolverOpts
}

func NewResolver(opts ResolverOpts) *Resolver {
	return &Resolver{opts: opts}
}

func (r *Resolver) Resolve(ctx context.Context, ref SecretRef) (secret.Secret, error) {
	select {
	case <-ctx.Done():
		return secret.Secret{}, ctx.Err()
	default:
	}

	switch ref.Scheme {
	case "env":
		value, ok := os.LookupEnv(ref.Name)
		if !ok {
			return secret.Secret{}, fmt.Errorf("%w: env ref is not set: %s", ErrInvalidRef, ref.Name)
		}
		return secret.New(value), nil
	case "file":
		return credentials.ReadOwnerOnlySecretFile(ref.Path)
	case "cmd":
		return r.resolveCmd(ctx, ref)
	case "keyring":
		return r.resolveKeyring(ctx, ref)
	default:
		return secret.Secret{}, fmt.Errorf("%w: unknown scheme %q", ErrInvalidRef, ref.Scheme)
	}
}

func (r *Resolver) resolveKeyring(ctx context.Context, ref SecretRef) (secret.Secret, error) {
	if r.opts.Keyring == nil {
		return secret.Secret{}, fmt.Errorf("%w: keyring is not available in this build", ErrInvalidRef)
	}
	value, err := r.opts.Keyring.Get(ctx, ref.Service, ref.Key)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return secret.Secret{}, fmt.Errorf("%w: keyring has no entry for service=%q key=%q; store it or use env:/file:/cmd refs", ErrInvalidRef, ref.Service, ref.Key)
		}
		if errors.Is(err, keyring.ErrUnavailable) {
			return secret.Secret{}, fmt.Errorf("%w: %s", ErrInvalidRef, err)
		}
		return secret.Secret{}, fmt.Errorf("%w: keyring lookup failed for service=%q key=%q", ErrInvalidRef, ref.Service, ref.Key)
	}
	if value == "" {
		return secret.Secret{}, fmt.Errorf("%w: keyring entry for service=%q key=%q is empty", ErrInvalidRef, ref.Service, ref.Key)
	}
	return secret.New(value), nil
}

func (r *Resolver) resolveCmd(ctx context.Context, ref SecretRef) (secret.Secret, error) {
	if !r.opts.AllowCmd {
		return secret.Secret{}, fmt.Errorf("%w: cmd refs are disabled", ErrInvalidRef)
	}
	if len(ref.Argv) == 0 || strings.TrimSpace(ref.Argv[0]) == "" {
		return secret.Secret{}, fmt.Errorf("%w: cmd.argv must be non-empty", ErrInvalidRef)
	}
	timeout := ref.Timeout
	if timeout <= 0 {
		timeout = DefaultCmdTimeout
	}
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// #nosec G204 -- owner-only profile cmd refs intentionally execute the
	// operator-specified argv directly, with no shell and a bounded timeout.
	cmd := exec.CommandContext(cmdCtx, ref.Argv[0], ref.Argv[1:]...)
	cmd.WaitDelay = 2 * time.Second
	cmd.Env = filterEnv(os.Environ())

	stdout := &cappedWriter{limit: 64 * 1024}
	stderr := &cappedWriter{limit: 16 * 1024}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(cmdCtx.Err(), context.DeadlineExceeded) {
			return secret.Secret{}, fmt.Errorf("%w: cmd provider %q timed out after %s", ErrInvalidRef, ref.Argv[0], timeout)
		}
		if cmdCtx.Err() != nil {
			return secret.Secret{}, fmt.Errorf("%w: cmd provider %q cancelled", ErrInvalidRef, ref.Argv[0])
		}
		if stdout.err != nil {
			return secret.Secret{}, fmt.Errorf("%w: cmd provider %q output too large", ErrInvalidRef, ref.Argv[0])
		}
		return secret.Secret{}, fmt.Errorf("%w: cmd provider %q failed: %s", ErrInvalidRef, ref.Argv[0], summarizeStderr(stderr.String()))
	}
	if stdout.err != nil {
		return secret.Secret{}, fmt.Errorf("%w: cmd provider %q output too large", ErrInvalidRef, ref.Argv[0])
	}
	outStr := strings.TrimRight(stdout.String(), "\r\n")
	if outStr == "" {
		return secret.Secret{}, fmt.Errorf("%w: cmd provider %q produced no output", ErrInvalidRef, ref.Argv[0])
	}
	return secret.New(outStr), nil
}

func summarizeStderr(stderr string) string {
	if stderr == "" {
		return "no stderr"
	}
	return fmt.Sprintf("stderr omitted (%d bytes)", len(stderr))
}

func filterEnv(environ []string) []string {
	var filtered []string
	for _, env := range environ {
		if !strings.HasPrefix(env, "ZSCALERCTL_") {
			filtered = append(filtered, env)
		}
	}
	return filtered
}

type cappedWriter struct {
	buf   bytes.Buffer
	limit int
	err   error
}

func (w *cappedWriter) Write(p []byte) (n int, err error) {
	if w.err != nil {
		return 0, w.err
	}
	if w.buf.Len()+len(p) > w.limit {
		w.err = errors.New("output limit exceeded")
		return 0, w.err
	}
	return w.buf.Write(p)
}

func (w *cappedWriter) String() string {
	return w.buf.String()
}
