package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/cli"
	"github.com/dvmrry/zscalerctl/internal/config"
	"github.com/dvmrry/zscalerctl/internal/dump"
	"github.com/dvmrry/zscalerctl/internal/output"
	"github.com/dvmrry/zscalerctl/internal/resources"
	"github.com/dvmrry/zscalerctl/internal/zscaler"
)

func TestExitCodeForError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want int
	}{
		{"usage", cli.ErrUsage, exitUsageError},
		{"partial_dump", cli.ErrPartialDump, exitPartialDump},
		{"wrapped_partial_dump", fmt.Errorf("dump zia: %w", cli.ErrPartialDump), exitPartialDump},
		{"drift_detected", cli.ErrDriftDetected, exitDriftDetected},
		{"not_found", cli.ErrNotFound, exitNotFound},
		{"resource_not_found", zscaler.ErrResourceNotFound, exitNotFound},
		{"wrapped_resource_not_found", fmt.Errorf("zia get: %w", zscaler.ErrResourceNotFound), exitNotFound},
		{"missing_credentials", zscaler.ErrMissingCredentials, exitCredentialError},
		{"invalid_resource_id", zscaler.ErrInvalidResourceID, exitUsageError},
		{"unsupported_resource", zscaler.ErrUnsupportedResource, exitNotFound},
		{"live_access_failed", zscaler.ErrLiveAccessFailed, exitLiveAccessFailure},
		{"invalid_proxy_config", zscaler.ErrInvalidProxyConfig, exitUsageError},
		{"invalid_config", config.ErrInvalidConfig, exitUsageError},
		{"wrapped_invalid_config", fmt.Errorf("load env: %w", config.ErrInvalidConfig), exitUsageError},
		{"unknown", errors.New("boom"), exitInternalError},
	}
	for _, tc := range cases {
		if got := exitCodeForError(tc.err); got != tc.want {
			t.Errorf("exitCodeForError(%s) = %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestErrorEnvelopeJSONIncludesResourceContext(t *testing.T) {
	t.Parallel()

	err := cli.ResourceNotFoundError{Product: resources.ProductZIA, Resource: "locations"}
	var buf bytes.Buffer
	writeError(&buf, output.FormatJSON, err)

	var env errorEnvelope
	if e := json.Unmarshal(buf.Bytes(), &env); e != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v, want nil", buf.String(), e)
	}
	if env.Error.Kind != "not_found" {
		t.Errorf("error.kind = %q, want not_found", env.Error.Kind)
	}
	if env.Error.Product != "zia" {
		t.Errorf("error.product = %q, want zia", env.Error.Product)
	}
	if env.Error.Resource != "locations" {
		t.Errorf("error.resource = %q, want locations", env.Error.Resource)
	}
}

func TestRunHelpReturnsSuccess(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"help"}, &stdout, &stderr, nil)
	if code != 0 {
		t.Fatalf("run(help) exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "zscalerctl") {
		t.Errorf("run(help) stdout = %q, want usage text", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("run(help) stderr = %q, want empty", stderr.String())
	}
}

func TestRunHelpFlagReturnsSuccess(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"--help"}, &stdout, &stderr, nil)
	if code != 0 {
		t.Fatalf("run(--help) exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "usage: zscalerctl") {
		t.Errorf("run(--help) stdout = %q, want usage text", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("run(--help) stderr = %q, want empty", stderr.String())
	}
}

func TestRunUsageErrorReturnsTwo(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"--timeout", "0s", "doctor"}, &stdout, &stderr, nil)
	if code != 2 {
		t.Fatalf("run(usage error) exit code = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("run(usage error) stdout = %q, want empty", stdout.String())
	}
	if !strings.Contains(stderr.String(), "timeout must be positive") {
		t.Errorf("run(usage error) stderr = %q, want usage error", stderr.String())
	}
}

func TestRunJSONUsageErrorEnvelope(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"--format", "json", "--timeout", "0s", "doctor"}, &stdout, &stderr, nil)
	if code != exitUsageError {
		t.Fatalf("run(json usage error) exit code = %d, want %d", code, exitUsageError)
	}
	if stdout.Len() != 0 {
		t.Errorf("run(json usage error) stdout = %q, want empty", stdout.String())
	}
	got := decodeErrorEnvelope(t, stderr.Bytes())
	if got.Error.Kind != "usage" || !strings.Contains(got.Error.Message, "timeout must be positive") {
		t.Errorf("run(json usage error) envelope = %#v, want usage timeout error", got)
	}
	if strings.Contains(stderr.String(), "zscalerctl:") {
		t.Errorf("run(json usage error) stderr = %q, want JSON without text prefix", stderr.String())
	}
}

func TestRunNoCommandJSONStderrIsPureEnvelope(t *testing.T) {
	t.Parallel()

	// The whole stderr stream must be a single parseable JSON document — no
	// plain-text usage block prepended (the double-output bug from the
	// pre-1.0 sweep).
	for _, args := range [][]string{
		{"--format", "json"},
		{"--format", "json", "frobnicate"},
	} {
		args := args
		var stdout, stderr bytes.Buffer
		code := run(context.Background(), args, &stdout, &stderr, nil)
		if code != exitUsageError {
			t.Fatalf("run(%v) exit code = %d, want %d", args, code, exitUsageError)
		}
		var env errorEnvelope
		if err := json.Unmarshal(stderr.Bytes(), &env); err != nil {
			t.Errorf("run(%v) stderr = %q, want pure JSON envelope; unmarshal err = %v", args, stderr.String(), err)
		} else if env.Error.Kind != "usage" {
			t.Errorf("run(%v) envelope kind = %q, want usage", args, env.Error.Kind)
		}
	}
}

func TestRunJSONCredentialErrorEnvelope(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"--format", "json", "zia", "locations", "list"}, &stdout, &stderr, nil)
	if code != exitCredentialError {
		t.Fatalf("run(json missing credentials) exit code = %d, want %d", code, exitCredentialError)
	}
	if stdout.Len() != 0 {
		t.Errorf("run(json missing credentials) stdout = %q, want empty", stdout.String())
	}
	got := decodeErrorEnvelope(t, stderr.Bytes())
	if got.Error.Kind != "missing_credentials" {
		t.Errorf("run(json missing credentials) kind = %q, want missing_credentials", got.Error.Kind)
	}
	if !strings.Contains(got.Error.Message, "missing zscaler API credentials") {
		t.Errorf("run(json missing credentials) message = %q, want missing credentials text", got.Error.Message)
	}
}

func TestRunDeferredSecretResolveFailureIsCredentialError(t *testing.T) {
	t.Parallel()

	sentinel := filepath.Join(t.TempDir(), "provider-ran")
	configPath := writeMainConfig(t, fmt.Sprintf(`
profiles:
  default:
    vanity_domain: example
    client_id: client-id
    client_secret_ref:
      cmd:
        argv: [%q, "-test.run=^TestRunConfigCmdHelperProcess$", "--", "touch", %q]
`, os.Args[0], sentinel))

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"--config", configPath, "--format", "json", "zia", "locations", "list"}, &stdout, &stderr, []string{config.EnvDisallowCmd + "=true"})
	if code != exitCredentialError {
		t.Fatalf("run(disabled deferred secret) exit code = %d, want %d; stderr = %q", code, exitCredentialError, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("run(disabled deferred secret) stdout = %q, want empty", stdout.String())
	}
	got := decodeErrorEnvelope(t, stderr.Bytes())
	if got.Error.Kind != "missing_credentials" {
		t.Fatalf("run(disabled deferred secret) error kind = %q, want missing_credentials", got.Error.Kind)
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatalf("disabled cmd provider created %q; stat err = %v", sentinel, err)
	}
}

func TestRunLiveCmdProviderFailureIsCredentialError(t *testing.T) {
	t.Parallel()

	configPath := writeMainConfig(t, fmt.Sprintf(`
profiles:
  default:
    vanity_domain: example
    client_id: client-id
    client_secret_ref:
      cmd:
        argv: [%q, "-test.run=^TestRunConfigCmdHelperProcess$", "--", "fail"]
`, os.Args[0]))

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"--config", configPath, "--format", "json", "zia", "locations", "list"}, &stdout, &stderr, nil)
	if code != exitCredentialError {
		t.Fatalf("run(live failing cmd) exit code = %d, want %d; stderr = %q", code, exitCredentialError, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Errorf("run(live failing cmd) stdout = %q, want empty", stdout.String())
	}
	got := decodeErrorEnvelope(t, stderr.Bytes())
	if got.Error.Kind != "missing_credentials" {
		t.Fatalf("run(live failing cmd) error kind = %q, want missing_credentials", got.Error.Kind)
	}
}

// TestRunJSONCredentialErrorEnvelopeMissingArray verifies that the JSON error
// envelope for a missing-credentials failure includes a non-empty "missing"
// array of variable NAMES and that no secret values appear anywhere in the
// rendered JSON.
func TestRunJSONCredentialErrorEnvelopeMissingArray(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"--format", "json", "zia", "locations", "list"}, &stdout, &stderr, nil)
	if code != exitCredentialError {
		t.Fatalf("run(json missing credentials) exit code = %d, want %d", code, exitCredentialError)
	}
	got := decodeErrorEnvelope(t, stderr.Bytes())
	if len(got.Error.Missing) == 0 {
		t.Fatalf("run(json missing credentials) missing array is empty, want at least one entry")
	}
	// All elements must be variable names (uppercase, underscore) — never values.
	for _, name := range got.Error.Missing {
		if !strings.HasPrefix(name, "ZSCALERCTL_") {
			t.Errorf("missing entry %q does not look like an env-var name", name)
		}
	}
	// The raw JSON must not contain any credential values (values are empty in
	// this test because no env vars are set, but guard the field names).
	raw := stderr.String()
	for _, forbidden := range []string{"zscalerctl-client-secret", "zscalerctl-client-id"} {
		if strings.Contains(raw, forbidden) {
			t.Errorf("stderr JSON contains forbidden value %q", forbidden)
		}
	}
}

func TestRunJSONNotFoundErrorEnvelope(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"--format", "json", "zia", "not-a-resource", "list"}, &stdout, &stderr, nil)
	if code != exitNotFound {
		t.Fatalf("run(json resource not found) exit code = %d, want %d", code, exitNotFound)
	}
	if stdout.Len() != 0 {
		t.Errorf("run(json resource not found) stdout = %q, want empty", stdout.String())
	}
	got := decodeErrorEnvelope(t, stderr.Bytes())
	if got.Error.Kind != "not_found" || got.Error.Product != "zia" || got.Error.Resource != "not-a-resource" {
		t.Errorf("run(json resource not found) envelope = %#v, want zia/not-a-resource not_found", got)
	}
}

func TestExitCodeForPartialDump(t *testing.T) {
	t.Parallel()

	err := cli.PartialDumpError{Dir: "/tmp/dump", Errors: 2}
	if got := exitCodeForError(err); got != exitPartialDump {
		t.Fatalf("exitCodeForError(PartialDumpError) = %d, want %d", got, exitPartialDump)
	}
}

// writeDumpDirForMain creates a minimal valid dump directory for integration tests
// in the main package. It mirrors the writeDiffDump helper in cobra_diff_test.go
// (internal/cli package) using only exported dump package types.
func writeDumpDirForMain(t *testing.T, product, resource, payload string) string {
	t.Helper()
	dir := t.TempDir()
	relPath := filepath.ToSlash(filepath.Join("resources", product, resource+".json"))
	path := filepath.Join(dir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("os.MkdirAll(%s) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%s) error = %v", path, err)
	}
	m := dump.Manifest{
		Schema:      dump.ManifestSchemaID,
		CollectedAt: "2026-01-01T00:00:00Z",
		ToolVersion: "test",
		Redaction:   "standard",
		Warning:     "test fixture",
		Status:      "complete",
		Resources: []dump.ManifestResource{
			{Product: product, Name: resource, Shape: "list", Status: "ok", Path: relPath, Records: 1},
		},
	}
	body, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		t.Fatalf("json.MarshalIndent(manifest) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), body, 0o600); err != nil {
		t.Fatalf("os.WriteFile(manifest) error = %v", err)
	}
	return dir
}

// TestRunDiffFailOnDriftExitsSeven is an end-to-end integration test verifying
// that run() returns exitDriftDetected (7) when diff detects a difference
// between two dump directories and --fail-on-drift is set. This closes L-2 from
// the adversarial review: exit-7 had no test through the cmd/zscalerctl boundary.
func TestRunDiffFailOnDriftExitsSeven(t *testing.T) {
	t.Parallel()

	// Build two minimal dump dirs with a synthetic difference (name field changed).
	oldDir := writeDumpDirForMain(t, "zia", "locations", `[{"id":"1","name":"old"}]`)
	newDir := writeDumpDirForMain(t, "zia", "locations", `[{"id":"1","name":"new"}]`)

	var stdout, stderr bytes.Buffer
	code := run(context.Background(),
		[]string{"--format", "json", "diff", "--fail-on-drift", oldDir, newDir},
		&stdout, &stderr, nil)
	if code != exitDriftDetected {
		t.Fatalf("run(diff --fail-on-drift, drift present) exit code = %d, want %d (exitDriftDetected)\nstdout: %s\nstderr: %s",
			code, exitDriftDetected, stdout.String(), stderr.String())
	}
}

func TestRunVersionReturnsSuccess(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"--format", "table", "version"}, &stdout, &stderr, nil)
	if code != 0 {
		t.Fatalf("run(version) exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Version") {
		t.Errorf("run(version) stdout = %q, want version text", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("run(version) stderr = %q, want empty", stderr.String())
	}
}

func TestRunRecoversPanicWithoutTracebackOrRawSecret(t *testing.T) {
	const raw = "panic client_secret=raw-panic-canary"

	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"help"}, panicWriter{value: raw}, &stderr, nil)
	if code != 1 {
		t.Fatalf("run(help with panicking stdout) exit code = %d, want 1", code)
	}
	if stdout.Len() != 0 {
		t.Errorf("run(help with panicking stdout) captured stdout = %q, want empty", stdout.String())
	}
	got := stderr.String()
	// stdout is a non-terminal sink, so the default (auto) format resolves to
	// JSON and the panic is reported as the structured envelope, not text.
	env := decodeErrorEnvelope(t, stderr.Bytes())
	if env.Error.Kind != "internal" || !strings.Contains(env.Error.Message, "internal error:") {
		t.Errorf("run panic envelope = %#v, want internal error", env)
	}
	if strings.Contains(got, raw) || strings.Contains(got, "raw-panic-canary") {
		t.Errorf("run panic stderr = %q, want no raw panic value", got)
	}
	if strings.Contains(got, "goroutine ") || strings.Contains(got, ".go:") {
		t.Errorf("run panic stderr = %q, want no traceback", got)
	}
	if !strings.Contains(got, "<REDACTED:SECRET>") {
		t.Errorf("run panic stderr = %q, want redaction marker", got)
	}
}

func TestMuteProcessOutputSuppressesGlobalStdoutStderrAndLogger(t *testing.T) {
	processOutputMu.Lock()
	defer processOutputMu.Unlock()

	previousStdout := os.Stdout
	previousStderr := os.Stderr
	previousLogWriter := log.Writer()
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout os.Pipe() error = %v, want nil", err)
	}
	defer stdoutReader.Close()
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr os.Pipe() error = %v, want nil", err)
	}
	defer stderrReader.Close()
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter
	var logBuffer bytes.Buffer
	log.SetOutput(&logBuffer)
	defer func() {
		os.Stdout = previousStdout
		os.Stderr = previousStderr
		log.SetOutput(previousLogWriter)
		_ = stdoutWriter.Close()
		_ = stderrWriter.Close()
	}()

	restore, err := muteProcessOutput()
	if err != nil {
		t.Fatalf("muteProcessOutput() error = %v, want nil", err)
	}
	fmt.Fprint(os.Stdout, "stdout-canary")
	fmt.Fprint(os.Stderr, "stderr-canary")
	log.Print("log-canary")
	restore()

	fmt.Fprint(os.Stdout, "visible")
	fmt.Fprint(os.Stderr, "err-visible")
	if err := stdoutWriter.Close(); err != nil {
		t.Fatalf("stdout pipe Close() error = %v, want nil", err)
	}
	if err := stderrWriter.Close(); err != nil {
		t.Fatalf("stderr pipe Close() error = %v, want nil", err)
	}
	body, err := io.ReadAll(stdoutReader)
	if err != nil {
		t.Fatalf("io.ReadAll(stdout pipe) error = %v, want nil", err)
	}
	if strings.Contains(string(body), "stdout-canary") {
		t.Errorf("captured stdout = %q, want no muted canary", body)
	}
	if !strings.Contains(string(body), "visible") {
		t.Errorf("captured stdout = %q, want post-restore output", body)
	}
	errBody, err := io.ReadAll(stderrReader)
	if err != nil {
		t.Fatalf("io.ReadAll(stderr pipe) error = %v, want nil", err)
	}
	if strings.Contains(string(errBody), "stderr-canary") {
		t.Errorf("captured stderr = %q, want no muted canary", errBody)
	}
	if !strings.Contains(string(errBody), "err-visible") {
		t.Errorf("captured stderr = %q, want post-restore output", errBody)
	}
	if strings.Contains(logBuffer.String(), "log-canary") {
		t.Errorf("captured log = %q, want no muted canary", logBuffer.String())
	}
}

type panicWriter struct {
	value string
}

func (w panicWriter) Write([]byte) (int, error) {
	panic(w.value)
}

func decodeErrorEnvelope(t *testing.T, body []byte) errorEnvelope {
	t.Helper()
	var got errorEnvelope
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("json.Unmarshal(error envelope %q) error = %v, want nil", body, err)
	}
	return got
}

// TestErrorEnvelopeSchemaMatchesStruct guards docs/schema/error.schema.json
// against drift: the published stderr error-envelope schema must list exactly
// the JSON fields the errorEnvelope/errorBody structs emit, mirroring the dump
// artifact drift guard so the contract cannot diverge silently.
func TestErrorEnvelopeSchemaMatchesStruct(t *testing.T) {
	t.Parallel()

	body, err := os.ReadFile(filepath.Join("..", "..", "docs", "schema", "error.schema.json"))
	if err != nil {
		t.Fatalf("read error.schema.json: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("parse error.schema.json: %v", err)
	}

	topProps := schemaProps(t, doc, "properties")
	if want := jsonTagNames(reflect.TypeOf(errorEnvelope{})); !reflect.DeepEqual(topProps, want) {
		t.Errorf("error.schema.json top properties = %v, want struct fields %v", topProps, want)
	}
	errProps := schemaProps(t, doc, "properties", "error", "properties")
	if want := jsonTagNames(reflect.TypeOf(errorBody{})); !reflect.DeepEqual(errProps, want) {
		t.Errorf("error.schema.json error properties = %v, want struct fields %v", errProps, want)
	}
}

func schemaProps(t *testing.T, doc map[string]any, path ...string) []string {
	t.Helper()
	var cur any = doc
	for _, seg := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			t.Fatalf("error.schema.json: %q is not an object while walking %v", seg, path)
		}
		cur, ok = m[seg]
		if !ok {
			t.Fatalf("error.schema.json: missing %q while walking %v", seg, path)
		}
	}
	props, ok := cur.(map[string]any)
	if !ok {
		t.Fatalf("error.schema.json: node at %v is not an object", path)
	}
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func jsonTagNames(typ reflect.Type) []string {
	var names []string
	for i := 0; i < typ.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		if idx := strings.IndexByte(tag, ','); idx >= 0 {
			tag = tag[:idx]
		}
		if tag != "" {
			names = append(names, tag)
		}
	}
	sort.Strings(names)
	return names
}

func TestRunMapsInvalidConfigToUsageExit(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	const canary = "plainredactioncanary"
	code := run(context.Background(), []string{"--format", "json", "doctor"}, &stdout, &stderr,
		[]string{"ZSCALERCTL_REDACTION=" + canary})
	if code != exitUsageError {
		t.Fatalf("run(bad ZSCALERCTL_REDACTION) exit code = %d, want %d", code, exitUsageError)
	}
	var env errorEnvelope
	if err := json.Unmarshal(stderr.Bytes(), &env); err != nil {
		t.Fatalf("stderr = %q, want JSON error envelope; err = %v", stderr.String(), err)
	}
	if env.Error.Kind != "invalid_config" {
		t.Errorf("error kind = %q, want invalid_config", env.Error.Kind)
	}
	if strings.Contains(stderr.String(), canary) {
		t.Errorf("stderr = %q, want no raw invalid config value", stderr.String())
	}
}

func TestRunDoesNotEchoInvalidAuthModeValue(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	const canary = "plainauthmodecanary"
	code := run(context.Background(), []string{"--format", "json", "doctor"}, &stdout, &stderr,
		[]string{"ZSCALERCTL_AUTH_MODE=" + canary})
	if code != exitUsageError {
		t.Fatalf("run(bad ZSCALERCTL_AUTH_MODE) exit code = %d, want %d", code, exitUsageError)
	}
	var env errorEnvelope
	if err := json.Unmarshal(stderr.Bytes(), &env); err != nil {
		t.Fatalf("stderr = %q, want JSON error envelope; err = %v", stderr.String(), err)
	}
	if env.Error.Kind != "invalid_config" {
		t.Errorf("error kind = %q, want invalid_config", env.Error.Kind)
	}
	if strings.Contains(stderr.String(), canary) {
		t.Errorf("stderr = %q, want no raw invalid config value", stderr.String())
	}
}

func TestErrorDetailsRedactsBareHighEntropyTokenInJSONMessage(t *testing.T) {
	t.Parallel()

	// A bare high-entropy token is caught by the text path's ScanRenderedString
	// entropy pass but not by the JSON path's Renderer.Bytes (ScanString), so the
	// envelope message must be pre-scanned. (Token is gitleaks-allowlisted.)
	const canary = "A7b9C2d4E6f8G1h3J5k7L9m2N4p6Q8r0S2t4U6v"
	body := errorDetails(errors.New("zscaler request failed: " + canary))
	if strings.Contains(body.Message, canary) {
		t.Errorf("errorDetails Message = %q, want bare high-entropy token redacted", body.Message)
	}
	if !strings.Contains(body.Message, "<REDACTED:SECRET>") {
		t.Errorf("errorDetails Message = %q, want secret marker", body.Message)
	}
}

func TestErrorFormatFollowsDataPathForAuto(t *testing.T) {
	t.Parallel()

	// Default (auto) format with a non-terminal stdout (bytes.Buffer) → the data
	// path is JSON, so errors must be JSON too, not plain text.
	if got := errorFormat([]string{"doctor"}, &bytes.Buffer{}); got != output.FormatJSON {
		t.Errorf("errorFormat(auto, non-TTY stdout) = %q, want json", got)
	}
	// Explicit table → text errors regardless of sink.
	if got := errorFormat([]string{"--format", "table", "doctor"}, &bytes.Buffer{}); got != output.FormatTable {
		t.Errorf("errorFormat(table) = %q, want table", got)
	}
	// Explicit json → JSON errors.
	if got := errorFormat([]string{"--format", "json", "doctor"}, &bytes.Buffer{}); got != output.FormatJSON {
		t.Errorf("errorFormat(json) = %q, want json", got)
	}
}

// stubContextError implements zscaler.ErrorContexter and unwraps to a sentinel,
// exercising errorDetails' envelope enrichment without the unexported zscaler
// error types.
type stubContextError struct{ ctx zscaler.ErrorContext }

func (e stubContextError) Error() string                      { return "stub live failure" }
func (e stubContextError) Unwrap() error                      { return zscaler.ErrLiveAccessFailed }
func (e stubContextError) ErrorContext() zscaler.ErrorContext { return e.ctx }

func TestErrorDetailsPopulatesOperationContext(t *testing.T) {
	t.Parallel()

	body := errorDetails(stubContextError{ctx: zscaler.ErrorContext{Product: "zia", Resource: "locations", Operation: "list"}})
	if body.Kind != "live_access_failed" {
		t.Errorf("errorDetails kind = %q, want live_access_failed", body.Kind)
	}
	if body.Product != "zia" || body.Resource != "locations" || body.Operation != "list" {
		t.Errorf("errorDetails context = %q/%q/%q, want zia/locations/list", body.Product, body.Resource, body.Operation)
	}
}

func writeMainConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", path, err)
	}
	return path
}

func TestRunConfigCmdHelperProcess(t *testing.T) {
	index := -1
	for i, arg := range os.Args {
		if arg == "--" {
			index = i
			break
		}
	}
	if index < 0 {
		return
	}
	args := os.Args[index+1:]
	if len(args) < 2 || args[0] != "touch" {
		os.Exit(2)
	}
	if err := os.WriteFile(args[1], []byte("ran"), 0o600); err != nil {
		fmt.Fprint(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}
