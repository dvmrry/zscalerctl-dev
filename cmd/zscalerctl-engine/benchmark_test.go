package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/dvmrry/zscalerctl/internal/enginewire"
)

func BenchmarkProcessStartupToHello(b *testing.B) {
	benchmarkProcessLifecycle(b, false)
}

func BenchmarkProcessManifestLifecycle(b *testing.B) {
	benchmarkProcessLifecycle(b, true)
}

func benchmarkProcessLifecycle(b *testing.B, manifest bool) {
	b.Helper()
	home := b.TempDir()
	b.ReportAllocs()
	for b.Loop() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		command := exec.CommandContext(ctx, processTestBinary)
		command.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + home, "XDG_CONFIG_HOME=" + home, "LANG=C"}
		stdin, err := command.StdinPipe()
		if err != nil {
			cancel()
			b.Fatal(err)
		}
		stdout, err := command.StdoutPipe()
		if err != nil {
			cancel()
			b.Fatal(err)
		}
		var stderr bytes.Buffer
		command.Stderr = &stderr
		if err := command.Start(); err != nil {
			cancel()
			b.Fatal(err)
		}
		reader := enginewire.NewFrameReader(stdout, enginewire.BootstrapFrameBytes)
		data, err := reader.ReadFrameLimit(enginewire.BootstrapFrameBytes)
		if err != nil {
			cancel()
			b.Fatal(err)
		}
		if _, err := enginewire.DecodeBootstrapServerFrame(data); err != nil {
			cancel()
			b.Fatal(err)
		}
		if manifest {
			if err := enginewire.WriteBootstrapClientFrame(stdin, enginewire.Initialize{
				Type: "initialize", Protocol: enginewire.Protocol, Version: enginewire.V1Version,
			}); err != nil {
				cancel()
				b.Fatal(err)
			}
			if _, err := reader.ReadFrameLimit(enginewire.V1FrameBytes); err != nil {
				cancel()
				b.Fatal(err)
			}
			if err := enginewire.WriteClientFrame(stdin, enginewire.ManifestRequest{
				Type: "request", ID: 1,
				Capability: enginewire.CapabilityEngineManifest, Operation: enginewire.OperationManifest,
			}); err != nil {
				cancel()
				b.Fatal(err)
			}
			if _, err := reader.ReadFrameLimit(enginewire.V1FrameBytes); err != nil {
				cancel()
				b.Fatal(err)
			}
			if _, err := reader.ReadFrameLimit(enginewire.V1FrameBytes); err != nil {
				cancel()
				b.Fatal(err)
			}
		}
		if err := stdin.Close(); err != nil {
			cancel()
			b.Fatal(err)
		}
		if _, err := reader.ReadFrameLimit(enginewire.V1FrameBytes); !errors.Is(err, io.EOF) {
			cancel()
			b.Fatalf("stdout close error = %v, want EOF", err)
		}
		if err := command.Wait(); err != nil {
			cancel()
			b.Fatal(err)
		}
		cancel()
		if stderr.Len() != 0 {
			b.Fatalf("stderr = %q, want empty", stderr.Bytes())
		}
	}
}
