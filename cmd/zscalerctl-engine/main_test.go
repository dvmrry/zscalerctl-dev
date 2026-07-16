package main

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/dvmrry/zscalerctl/internal/enginewire"
	"github.com/dvmrry/zscalerctl/internal/redact"
)

func TestParseSettingsUsesOnlyProcessStartPolicy(t *testing.T) {
	t.Parallel()

	settings, ok := parseSettings([]string{
		"--profile", "ops", "--config", "/tmp/config.yaml", "--timeout", "17s",
		"--redaction", "share", "--no-cache",
	})
	if !ok {
		t.Fatal("parseSettings() ok = false")
	}
	if settings.Profile != "ops" || settings.ConfigPath != "/tmp/config.yaml" || settings.Timeout != 17*time.Second ||
		settings.Redaction != redact.ModeShare || !settings.RedactionSet || !settings.NoCache {
		t.Fatalf("parseSettings() = %#v", settings)
	}
	for _, args := range [][]string{
		{"positional"}, {"--timeout", "0s"}, {"--redaction", "future"}, {"--unknown"},
	} {
		if _, ok := parseSettings(args); ok {
			t.Errorf("parseSettings(%q) ok = true", args)
		}
	}
}

func TestRunWritesHelloAndTreatsBootstrapEOFAsClean(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	if code := run(context.Background(), nil, bytes.NewReader(nil), &output, []string{"UNRELATED=value"}); code != 0 {
		t.Fatalf("run() = %d, want 0", code)
	}
	reader := enginewire.NewFrameReader(bytes.NewReader(output.Bytes()), enginewire.BootstrapFrameBytes)
	data, err := reader.ReadFrame()
	if err != nil {
		t.Fatalf("ReadFrame(hello) error = %v", err)
	}
	frame, err := enginewire.DecodeBootstrapServerFrame(data)
	if err != nil {
		t.Fatalf("DecodeBootstrapServerFrame() error = %v", err)
	}
	if _, ok := frame.(enginewire.Hello); !ok {
		t.Fatalf("bootstrap frame = %T, want Hello", frame)
	}
}

func TestRunRejectsInvalidProcessArgumentsBeforeProtocolOutput(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	if code := run(context.Background(), []string{"--timeout", "0s"}, bytes.NewReader(nil), &output, nil); code != 2 {
		t.Fatalf("run(invalid args) = %d, want 2", code)
	}
	if output.Len() != 0 {
		t.Fatalf("run(invalid args) output = %q, want empty", output.Bytes())
	}
}

func TestSignalControllerCancelsThenForcesWithSignalExitCode(t *testing.T) {
	forced := make(chan int, 1)
	controller := newSignalController(context.Background(), func(code int) { forced <- code })
	t.Cleanup(controller.Stop)
	controller.signals <- os.Interrupt
	select {
	case <-controller.Context().Done():
	case <-time.After(time.Second):
		t.Fatal("first signal did not cancel context")
	}
	if controller.ExitCode() != 130 {
		t.Fatalf("first signal exit code = %d, want 130", controller.ExitCode())
	}
	select {
	case code := <-forced:
		t.Fatalf("first signal forced exit %d", code)
	default:
	}
	controller.signals <- os.Interrupt
	select {
	case code := <-forced:
		if code != 130 {
			t.Fatalf("forced exit code = %d, want 130", code)
		}
	case <-time.After(time.Second):
		t.Fatal("second signal did not force exit")
	}
}
