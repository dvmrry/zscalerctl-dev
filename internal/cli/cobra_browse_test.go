package cli

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/tui/launcher"
)

func TestBrowseWithoutTUIReturnsUsageError(t *testing.T) {
	a := New(io.Discard, io.Discard, nil)
	err := a.Run(context.Background(), []string{"browse"})
	if err == nil {
		t.Fatal("browse without --tui: error = nil, want usage error")
	}
	if !errors.Is(err, ErrUsage) {
		t.Errorf("error = %v, want ErrUsage", err)
	}
	if !strings.Contains(err.Error(), "browse currently requires --tui") {
		t.Errorf("error = %q, want 'browse currently requires --tui'", err.Error())
	}
}

func TestBrowseTUIRequiresInteractiveTTY(t *testing.T) {
	a := New(io.Discard, io.Discard, nil)
	err := a.Run(context.Background(), []string{"browse", "--tui"})
	if err == nil {
		t.Fatal("browse --tui in non-TTY env: error = nil, want usage error")
	}
	if !errors.Is(err, ErrUsage) {
		t.Errorf("error = %v, want ErrUsage", err)
	}
	if !strings.Contains(err.Error(), "tui disabled") {
		t.Errorf("error = %q, want 'tui disabled'", err.Error())
	}
	var launchErr launcher.LaunchError
	if errors.As(err, &launchErr) {
		t.Errorf("error should be wrapped as UsageError, not bare LaunchError")
	}
}

func TestBrowseCommandIsKnownCommand(t *testing.T) {
	a := New(io.Discard, io.Discard, nil)
	if !isKnownCommand("browse", a.resourceCatalog()) {
		t.Error("browse is not recognized as a known command")
	}
	if !isRunnableCommand("browse", a.resourceCatalog()) {
		t.Error("browse is not recognized as a runnable command")
	}
}

func TestBrowseCommandIsHidden(t *testing.T) {
	a := New(io.Discard, io.Discard, nil)
	root := a.buildCommandTree(globalOptions{})
	for _, cmd := range root.Commands() {
		if cmd.Name() == "browse" {
			if !cmd.Hidden {
				t.Error("browse command should be hidden")
			}
			return
		}
	}
	t.Fatal("browse command not found in command tree")
}
