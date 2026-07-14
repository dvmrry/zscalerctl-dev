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
	"syscall"
	"testing"
	"time"

	dumpartifact "github.com/dvmrry/zscalerctl/internal/dump"
	"github.com/dvmrry/zscalerctl/internal/enginewire"
	"github.com/dvmrry/zscalerctl/internal/redact"
	"github.com/dvmrry/zscalerctl/internal/resources"
)

var (
	processTestBinary  string
	dumpHostTestBinary string
)

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
	dumpHostTestBinary = filepath.Join(directory, "zscalerctl-dump-test-engine")
	if runtime.GOOS == "windows" {
		dumpHostTestBinary += ".exe"
	}
	command = exec.Command(
		"go", "build", "-tags=zscalerctl_engine_testhooks", "-o", dumpHostTestBinary,
		"./internal/enginehost/testdata/dumpengine",
	)
	command.Dir = filepath.Join("..", "..")
	if output, err := command.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "zscalerctl-engine tests: build dump test binary: %v\n%s", err, output)
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
	return startEngineProcessWith(t, processTestBinary, nil)
}

func startEngineProcessWith(t *testing.T, binary string, extraEnv []string) *engineProcess {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	command := exec.CommandContext(ctx, binary)
	command.Env = append([]string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + t.TempDir(),
		"XDG_CONFIG_HOME=" + t.TempDir(),
		"LANG=C",
	}, extraEnv...)
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

func TestProcessDiffAcceptsSelectedResourceFailedOnOnePartialSide(t *testing.T) {
	spec, ok := resources.Catalog().FindSpec(resources.ProductZIA, "locations")
	if !ok {
		t.Fatal("catalog has no zia/locations resource")
	}
	oldDir := filepath.Join(t.TempDir(), "old")
	if err := dumpartifact.Write(oldDir, redact.ModeStandard, dumpartifact.Result{
		Errors: []dumpartifact.ResourceError{
			dumpartifact.NewResourceError(resources.ProductZIA, "locations", "list", "live_access_failed"),
		},
	}); err != nil {
		t.Fatalf("dump.Write(old partial) error = %v", err)
	}
	newDir := filepath.Join(t.TempDir(), "new")
	if err := dumpartifact.Write(newDir, redact.ModeStandard, dumpartifact.Result{
		Entries: []dumpartifact.ResourceDump{{
			Spec: spec,
			Records: resources.NewProjectedRecordsFromProjectedFields([]map[string]any{{
				"id": "1", "name": "HQ",
			}}),
		}},
	}); err != nil {
		t.Fatalf("dump.Write(new complete) error = %v", err)
	}

	process := startEngineProcess(t)
	process.initialize(t)
	request := enginewire.DiffRequest{
		Type: "request", ID: 1,
		Capability: enginewire.CapabilityDiffCompare, Operation: enginewire.OperationDiff,
		Input: enginewire.DiffInput{
			OldDir: oldDir, NewDir: newDir,
			Resources: []enginewire.ResourceSelector{{Product: enginewire.ProductZIA, Resource: "locations"}},
			Products:  []enginewire.Product{}, AllowPartial: true,
		},
	}
	if err := enginewire.WriteClientFrame(process.stdin, request); err != nil {
		t.Fatalf("WriteClientFrame(diff partial) error = %v", err)
	}
	if started, ok := process.readV1(t).(enginewire.Started); !ok || started.ID != 1 || started.Sequence != 1 {
		t.Fatalf("started = %#v", started)
	}
	if progress, ok := process.readV1(t).(enginewire.Progress); !ok || progress.Current != 1 || progress.Total != 1 {
		t.Fatalf("progress = %#v, want selected resource 1/1", progress)
	}
	completed, ok := process.readV1(t).(enginewire.Completed[enginewire.DiffSummary])
	if !ok || completed.Result.Summary.ResourcesCompared != 0 || completed.Result.StreamItemsEmitted != 0 ||
		!completed.Result.Old.Partial || completed.Result.New.Partial {
		t.Fatalf("completed = %#v, want accepted note-only partial comparison", completed)
	}
	if err := process.stdin.Close(); err != nil {
		t.Fatalf("stdin.Close() error = %v", err)
	}
	if code := process.wait(t); code != 0 {
		t.Errorf("process exit code = %d, want 0", code)
	}
}

func TestProcessDiffEmptySelectorsAccountForSameScopeSelectiveDumps(t *testing.T) {
	firstSpec := dumpHostResourceSpec()
	secondSpec := dumpHostSecondResourceSpec()
	wantProgress := []resources.ResourceSpec{firstSpec, secondSpec}
	tests := []struct {
		name      string
		collected resources.ResourceSpec
	}{
		{name: "first catalog resource", collected: firstSpec},
		{name: "second catalog resource", collected: secondSpec},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldDir := writeSelectiveProcessDump(t, tt.collected)
			newDir := writeSelectiveProcessDump(t, tt.collected)
			process := startEngineProcessWith(t, dumpHostTestBinary, nil)
			process.initialize(t)
			request := enginewire.DiffRequest{
				Type: "request", ID: 1,
				Capability: enginewire.CapabilityDiffCompare, Operation: enginewire.OperationDiff,
				Input: enginewire.DiffInput{
					OldDir: oldDir, NewDir: newDir,
					Products: []enginewire.Product{}, Resources: []enginewire.ResourceSelector{},
				},
			}
			if err := enginewire.WriteClientFrame(process.stdin, request); err != nil {
				t.Fatalf("WriteClientFrame(diff %s only) error = %v", tt.collected.Name, err)
			}
			if started, ok := process.readV1(t).(enginewire.Started); !ok || started.ID != 1 || started.Sequence != 1 {
				t.Fatalf("diff %s only started = %#v, want request 1 sequence 1", tt.collected.Name, started)
			}
			for i, spec := range wantProgress {
				progress, ok := process.readV1(t).(enginewire.Progress)
				if !ok || progress.ID != 1 || progress.Sequence != enginewire.SafeInteger(i+2) ||
					progress.Current != enginewire.SafeInteger(i+1) || progress.Total != 2 ||
					progress.Product != enginewire.Product(spec.Product) || progress.Resource != spec.Name {
					t.Fatalf(
						"diff %s only progress %d = %#v, want %s/%s %d/2",
						tt.collected.Name,
						i+1,
						progress,
						spec.Product,
						spec.Name,
						i+1,
					)
				}
			}
			item, ok := process.readV1(t).(enginewire.Item[enginewire.DiffResource])
			if !ok || item.ID != 1 || item.Sequence != 4 || item.Kind != enginewire.ItemDiffResource ||
				item.Item.Product != enginewire.Product(tt.collected.Product) ||
				item.Item.Resource != tt.collected.Name || item.Item.Added != 0 ||
				item.Item.Removed != 0 || item.Item.ChangedFields != 0 {
				t.Fatalf("diff %s only item = %#v, want one no-drift compared-resource item", tt.collected.Name, item)
			}
			completed, ok := process.readV1(t).(enginewire.Completed[enginewire.DiffSummary])
			if !ok || completed.ID != 1 || completed.Sequence != 5 ||
				completed.Result.Summary.ResourcesCompared != 1 || completed.Result.HasDrift ||
				completed.Result.StreamItemsEmitted != 1 {
				t.Fatalf("diff %s only completion = %#v, want one no-drift comparison", tt.collected.Name, completed)
			}
			closeAndWaitEngineProcess(t, process)
		})
	}
}

func TestProcessDumpCancelAfterNewDestinationCommitReturnsSuccess(t *testing.T) {
	hookDir := t.TempDir()
	outDir := filepath.Join(t.TempDir(), "dump")
	process := startEngineProcessWith(t, dumpHostTestBinary, []string{
		"ZSCALERCTL_ENGINE_TEST_HOOK_DIR=" + hookDir,
	})
	process.initialize(t)
	writeDumpProcessRequest(t, process, outDir, false)
	readDumpProcessStart(t, process)
	waitForProcessTestPath(t, filepath.Join(hookDir, "after_publish.reached"))
	if _, err := os.Stat(filepath.Join(outDir, "manifest.json")); err != nil {
		t.Fatalf("committed dump manifest before cancel error = %v", err)
	}
	if err := enginewire.WriteClientFrame(process.stdin, enginewire.Cancel{Type: "cancel", ID: 1}); err != nil {
		t.Fatalf("WriteClientFrame(cancel committed dump) error = %v", err)
	}
	releaseProcessTestHook(t, hookDir, "after_publish")
	completed, ok := process.readV1(t).(enginewire.Completed[enginewire.DumpSummary])
	if !ok || completed.Result.ResourcesWritten != 1 || completed.Result.RecordsWritten != 1 {
		t.Fatalf("terminal after committed dump cancel = %#v, want completed dump", completed)
	}
	closeAndWaitEngineProcess(t, process)
}

func TestProcessDumpCancelCannotKillCommittedForceCleanup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("existing-directory replacement intentionally fails closed on Windows")
	}
	hookDir := t.TempDir()
	parent := t.TempDir()
	outDir := filepath.Join(parent, "dump")
	spec := dumpHostResourceSpec()
	if err := dumpartifact.Write(outDir, redact.ModeStandard, dumpartifact.Result{
		Entries: []dumpartifact.ResourceDump{{
			Spec: spec,
			Records: resources.NewProjectedRecordsFromProjectedFields([]map[string]any{{
				"id": "1", "name": "old",
			}}),
		}},
	}); err != nil {
		t.Fatalf("dump.Write(previous custom artifact) error = %v", err)
	}

	process := startEngineProcessWith(t, dumpHostTestBinary, []string{
		"ZSCALERCTL_ENGINE_TEST_HOOK_DIR=" + hookDir,
	})
	process.initialize(t)
	writeDumpProcessRequest(t, process, outDir, true)
	readDumpProcessStart(t, process)
	waitForProcessTestPath(t, filepath.Join(hookDir, "after_publish.reached"))
	releaseProcessTestHook(t, hookDir, "after_publish")
	waitForProcessTestPath(t, filepath.Join(hookDir, "before_cleanup.reached"))
	if err := enginewire.WriteClientFrame(process.stdin, enginewire.Cancel{Type: "cancel", ID: 1}); err != nil {
		t.Fatalf("WriteClientFrame(cancel force cleanup) error = %v", err)
	}

	// The production host's cancel watchdog is five seconds. The process must
	// remain alive beyond it while confidential old-artifact contents are still
	// parked in the private cleanup quarantine.
	time.Sleep(5*time.Second + 250*time.Millisecond)
	if err := process.command.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("dump process exited during committed cleanup: %v", err)
	}
	quarantined, err := filepath.Glob(filepath.Join(parent, ".zscalerctl-cleanup-*", "root", "manifest.json"))
	if err != nil || len(quarantined) != 1 {
		t.Fatalf("quarantined manifests during blocked cleanup = %v, %v; want one", quarantined, err)
	}

	releaseProcessTestHook(t, hookDir, "before_cleanup")
	completed, ok := process.readV1(t).(enginewire.Completed[enginewire.DumpSummary])
	if !ok || completed.Result.ResourcesWritten != 1 || completed.Result.RecordsWritten != 1 {
		t.Fatalf("terminal after committed force cancel = %#v, want completed dump", completed)
	}
	closeAndWaitEngineProcess(t, process)
}

func dumpHostResourceSpec() resources.ResourceSpec {
	return dumpHostNamedResourceSpec("engine-test-locations")
}

func dumpHostSecondResourceSpec() resources.ResourceSpec {
	return dumpHostNamedResourceSpec("engine-test-rules")
}

func dumpHostNamedResourceSpec(name string) resources.ResourceSpec {
	return resources.ResourceSpec{
		Product:    resources.ProductZIA,
		Name:       name,
		Operations: resources.ListOperations(),
		Fields: []resources.FieldSpec{
			{
				Name:           "id",
				Classification: resources.ClassOperational,
				AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
			},
			{
				Name:           "name",
				Classification: resources.ClassTenantConfig,
				AllowedModes:   []redact.Mode{redact.ModeStandard, redact.ModeShare},
			},
		},
	}
}

func writeSelectiveProcessDump(t *testing.T, spec resources.ResourceSpec) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "dump")
	if err := dumpartifact.Write(dir, redact.ModeStandard, dumpartifact.Result{
		Entries: []dumpartifact.ResourceDump{{
			Spec: spec,
			Records: resources.NewProjectedRecordsFromProjectedFields([]map[string]any{{
				"id": "1", "name": "same",
			}}),
		}},
	}); err != nil {
		t.Fatalf("dump.Write(%s selective fixture) error = %v", spec.Name, err)
	}
	return dir
}

func writeDumpProcessRequest(t *testing.T, process *engineProcess, outDir string, force bool) {
	t.Helper()
	request := enginewire.DumpRequest{
		Type: "request", ID: 1,
		Capability: enginewire.CapabilityDumpWrite, Operation: enginewire.OperationDump,
		Input: enginewire.DumpInput{
			OutputDir: outDir,
			Products:  []enginewire.Product{},
			Resources: []enginewire.ResourceSelector{{
				Product: enginewire.ProductZIA, Resource: "engine-test-locations",
			}},
			Force: force,
		},
	}
	if err := enginewire.WriteClientFrame(process.stdin, request); err != nil {
		t.Fatalf("WriteClientFrame(dump) error = %v", err)
	}
}

func readDumpProcessStart(t *testing.T, process *engineProcess) {
	t.Helper()
	if started, ok := process.readV1(t).(enginewire.Started); !ok || started.ID != 1 || started.Sequence != 1 {
		t.Fatalf("dump started = %#v", started)
	}
	if progress, ok := process.readV1(t).(enginewire.Progress); !ok || progress.Current != 1 || progress.Total != 1 {
		t.Fatalf("dump progress = %#v", progress)
	}
}

func waitForProcessTestPath(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("os.Stat(%q) error = %v", path, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %q", path)
}

func releaseProcessTestHook(t *testing.T, dir, stage string) {
	t.Helper()
	path := filepath.Join(dir, stage+".release")
	if err := os.WriteFile(path, []byte(stage), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
}

func closeAndWaitEngineProcess(t *testing.T, process *engineProcess) {
	t.Helper()
	if err := process.stdin.Close(); err != nil {
		t.Fatalf("stdin.Close() error = %v", err)
	}
	if code := process.wait(t); code != 0 {
		t.Fatalf("process exit code = %d, want 0", code)
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
