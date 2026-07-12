package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	dumpartifact "github.com/dvmrry/zscalerctl/internal/dump"
	"github.com/dvmrry/zscalerctl/internal/enginewire"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

var processTestBinary string

func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	directory, err := os.MkdirTemp("", "zscalerctl-engine-test-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "zscalerctl-engine tests: create temporary directory: %v\n", err)
		return 1
	}
	defer os.RemoveAll(directory)

	processTestBinary = filepath.Join(directory, "zscalerctl-engine")
	if runtime.GOOS == "windows" {
		processTestBinary += ".exe"
	}
	command := exec.Command("go", "build", "-o", processTestBinary, "./cmd/zscalerctl-engine")
	command.Dir = filepath.Join("..", "..")
	if output, err := command.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "zscalerctl-engine tests: build binary: %v\n%s", err, output)
		return 1
	}
	return m.Run()
}

type engineProcess struct {
	command *exec.Cmd
	stdin   io.WriteCloser
	stdout  io.ReadCloser
	reader  *enginewire.FrameReader
	stderr  bytes.Buffer
	cancel  context.CancelFunc
	waited  bool
}

func startEngineProcess(t *testing.T) *engineProcess {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	command := exec.CommandContext(ctx, processTestBinary)
	command.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + t.TempDir(),
		"XDG_CONFIG_HOME=" + t.TempDir(),
		"LANG=C",
	}
	stdin, err := command.StdinPipe()
	if err != nil {
		cancel()
		t.Fatalf("Cmd.StdinPipe() error = %v", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		cancel()
		t.Fatalf("Cmd.StdoutPipe() error = %v", err)
	}
	process := &engineProcess{
		command: command,
		stdin:   stdin,
		stdout:  stdout,
		reader:  enginewire.NewFrameReader(stdout, enginewire.BootstrapFrameBytes),
		cancel:  cancel,
	}
	command.Stderr = &process.stderr
	if err := command.Start(); err != nil {
		cancel()
		t.Fatalf("Cmd.Start() error = %v", err)
	}
	t.Cleanup(func() {
		_ = process.stdin.Close()
		_ = process.stdout.Close()
		process.cancel()
		if !process.waited {
			_ = process.command.Wait()
		}
	})
	return process
}

func (p *engineProcess) readBootstrap(t *testing.T) enginewire.BootstrapServerFrame {
	t.Helper()
	data, err := p.reader.ReadFrameLimit(enginewire.BootstrapFrameBytes)
	if err != nil {
		t.Fatalf("FrameReader.ReadFrameLimit(bootstrap) error = %v", err)
	}
	frame, err := enginewire.DecodeBootstrapServerFrame(data)
	if err != nil {
		t.Fatalf("DecodeBootstrapServerFrame() error = %v", err)
	}
	return frame
}

func (p *engineProcess) readV1(t *testing.T) enginewire.ServerFrame {
	t.Helper()
	data, err := p.reader.ReadFrameLimit(enginewire.V1FrameBytes)
	if err != nil {
		t.Fatalf("FrameReader.ReadFrameLimit(v1) error = %v", err)
	}
	frame, err := enginewire.DecodeServerFrame(data)
	if err != nil {
		t.Fatalf("DecodeServerFrame() error = %v", err)
	}
	return frame
}

func (p *engineProcess) initialize(t *testing.T) enginewire.Ready {
	t.Helper()
	hello, ok := p.readBootstrap(t).(enginewire.Hello)
	if !ok {
		t.Fatalf("first server frame is not hello")
	}
	if hello.Protocol != enginewire.Protocol || len(hello.Versions) != 1 || hello.Versions[0] != enginewire.V1Version {
		t.Fatalf("hello = %#v", hello)
	}
	initialize := enginewire.Initialize{
		Type: "initialize", Protocol: enginewire.Protocol, Version: enginewire.V1Version,
	}
	if err := enginewire.WriteBootstrapClientFrame(p.stdin, initialize); err != nil {
		t.Fatalf("WriteBootstrapClientFrame(initialize) error = %v", err)
	}
	ready, ok := p.readV1(t).(enginewire.Ready)
	if !ok {
		t.Fatalf("post-initialize server frame is not ready")
	}
	return ready
}

func (p *engineProcess) wait(t *testing.T) int {
	t.Helper()
	err := p.command.Wait()
	p.waited = true
	p.cancel()
	if err == nil {
		return 0
	}
	if exitError, ok := err.(*exec.ExitError); ok {
		return exitError.ExitCode()
	}
	t.Fatalf("Cmd.Wait() error = %v", err)
	return -1
}

func TestProcessNegotiatesAndCompletesManifest(t *testing.T) {
	process := startEngineProcess(t)
	ready := process.initialize(t)
	if ready.Schema.ID != enginewire.V1SchemaID || ready.Schema.SHA256 != enginewire.V1SchemaSHA256 {
		t.Fatalf("ready schema = %#v", ready.Schema)
	}
	request := enginewire.ManifestRequest{
		Type: "request", ID: 1,
		Capability: enginewire.CapabilityEngineManifest, Operation: enginewire.OperationManifest,
	}
	if err := enginewire.WriteClientFrame(process.stdin, request); err != nil {
		t.Fatalf("WriteClientFrame(manifest) error = %v", err)
	}
	started, ok := process.readV1(t).(enginewire.Started)
	if !ok || started.ID != 1 || started.Sequence != 1 {
		t.Fatalf("started = %#v", started)
	}
	completed, ok := process.readV1(t).(enginewire.Completed[enginewire.EngineManifestResult])
	if !ok || completed.ID != 1 || completed.Sequence != 2 {
		t.Fatalf("completed = %#v", completed)
	}
	if err := process.stdin.Close(); err != nil {
		t.Fatalf("stdin.Close() error = %v", err)
	}
	if _, err := process.reader.ReadFrameLimit(enginewire.V1FrameBytes); !errors.Is(err, io.EOF) {
		t.Errorf("stdout after process exit error = %v, want EOF", err)
	}
	if code := process.wait(t); code != 0 {
		t.Errorf("process exit code = %d, want 0", code)
	}
	if process.stderr.Len() != 0 {
		t.Errorf("process stderr = %q, want empty", process.stderr.Bytes())
	}
}

func TestProcessDiffAcceptsSelectedResourceEmptyOnBothSides(t *testing.T) {
	spec, ok := resources.Catalog().FindSpec(resources.ProductZIA, "locations")
	if !ok {
		t.Fatal("catalog has no zia/locations resource")
	}
	writeEmptyDump := func(name string) string {
		dir := filepath.Join(t.TempDir(), name)
		if err := dumpartifact.Write(dir, redact.ModeStandard, dumpartifact.Result{
			Entries: []dumpartifact.ResourceDump{{
				Spec: spec, Records: resources.NewProjectedRecordsFromProjectedFields([]map[string]any{}),
			}},
		}); err != nil {
			t.Fatalf("dump.Write(%s) error = %v", name, err)
		}
		return dir
	}
	oldDir := writeEmptyDump("old")
	newDir := writeEmptyDump("new")

	process := startEngineProcess(t)
	process.initialize(t)
	request := enginewire.DiffRequest{
		Type: "request", ID: 1,
		Capability: enginewire.CapabilityDiffCompare, Operation: enginewire.OperationDiff,
		Input: enginewire.DiffInput{
			OldDir: oldDir, NewDir: newDir,
			Resources: []enginewire.ResourceSelector{{Product: enginewire.ProductZIA, Resource: "locations"}},
			Products:  []enginewire.Product{},
		},
	}
	if err := enginewire.WriteClientFrame(process.stdin, request); err != nil {
		t.Fatalf("WriteClientFrame(diff) error = %v", err)
	}
	started, ok := process.readV1(t).(enginewire.Started)
	if !ok || started.ID != 1 || started.Sequence != 1 {
		t.Fatalf("started = %#v", started)
	}
	progress, ok := process.readV1(t).(enginewire.Progress)
	if !ok || progress.Current != 1 || progress.Total != 1 {
		t.Fatalf("progress = %#v, want selected resource 1/1", progress)
	}
	completed, ok := process.readV1(t).(enginewire.Completed[enginewire.DiffSummary])
	if !ok || completed.Result.Summary.ResourcesCompared != 0 || completed.Result.StreamItemsEmitted != 0 {
		t.Fatalf("completed = %#v, want empty diff report", completed)
	}
	if err := process.stdin.Close(); err != nil {
		t.Fatalf("stdin.Close() error = %v", err)
	}
	if _, err := process.reader.ReadFrameLimit(enginewire.V1FrameBytes); !errors.Is(err, io.EOF) {
		t.Errorf("stdout after process exit error = %v, want EOF", err)
	}
	if code := process.wait(t); code != 0 {
		t.Errorf("process exit code = %d, want 0", code)
	}
}

func TestProcessMalformedBootstrapWritesFatalAndExitsTwo(t *testing.T) {
	process := startEngineProcess(t)
	if _, ok := process.readBootstrap(t).(enginewire.Hello); !ok {
		t.Fatal("first server frame is not hello")
	}
	if _, err := io.WriteString(process.stdin, "{}\n"); err != nil {
		t.Fatalf("WriteString(malformed bootstrap) error = %v", err)
	}
	protocolError, ok := process.readBootstrap(t).(enginewire.BootstrapProtocolError)
	if !ok || !protocolError.Fatal || protocolError.Error.Kind != enginewire.ProtocolErrorViolation {
		t.Fatalf("protocol error = %#v", protocolError)
	}
	if code := process.wait(t); code != 2 {
		t.Errorf("process exit code = %d, want 2", code)
	}
	if process.stderr.Len() != 0 {
		t.Errorf("process stderr = %q, want empty", process.stderr.Bytes())
	}
}

func TestProcessBrokenStdoutExitsOnePromptly(t *testing.T) {
	process := startEngineProcess(t)
	if _, ok := process.readBootstrap(t).(enginewire.Hello); !ok {
		t.Fatal("first server frame is not hello")
	}
	if err := process.stdout.Close(); err != nil {
		t.Fatalf("stdout.Close() error = %v", err)
	}
	initialize := enginewire.Initialize{
		Type: "initialize", Protocol: enginewire.Protocol, Version: enginewire.V1Version,
	}
	if err := enginewire.WriteBootstrapClientFrame(process.stdin, initialize); err != nil {
		t.Fatalf("WriteBootstrapClientFrame(initialize) error = %v", err)
	}
	started := time.Now()
	if code := process.wait(t); code != 1 {
		t.Errorf("process exit code = %d, want 1", code)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Errorf("broken stdout shutdown took %v, want at most 1s", elapsed)
	}
	if process.stderr.Len() != 0 {
		t.Errorf("process stderr = %q, want empty", process.stderr.Bytes())
	}
}
