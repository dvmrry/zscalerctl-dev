//go:build darwin || linux

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigInitForceUsesOSPathResolutionAfterSymlinkDotDot(t *testing.T) {
	t.Parallel()

	destination, lexicalSibling, resolvedDestination := symlinkDotDotDestination(t)
	const sentinel = "lexical sibling must remain unchanged"
	if err := os.WriteFile(lexicalSibling, []byte(sentinel), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", lexicalSibling, err)
	}
	if err := os.WriteFile(resolvedDestination, []byte("old config"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", resolvedDestination, err)
	}

	var out, errOut bytes.Buffer
	app := &App{}
	err := app.runConfigInitWithForce(globalOptions{configPath: destination}, true, &out, &errOut)
	if err != nil {
		t.Fatalf("config init --force with symlink/.. destination error = %v, want nil", err)
	}
	if got := out.String(); got != destination+"\n" {
		t.Fatalf("config init stdout = %q, want raw destination %q", got, destination+"\n")
	}
	if got, err := os.ReadFile(lexicalSibling); err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want sentinel unchanged", lexicalSibling, err)
	} else if string(got) != sentinel {
		t.Errorf("lexical sibling body = %q, want %q", got, sentinel)
	}
	if got, err := os.ReadFile(resolvedDestination); err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want config template", resolvedDestination, err)
	} else if string(got) != configInitTemplate {
		t.Errorf("resolved config body = %q, want starter template", got)
	}

	reportedInfo, err := os.Stat(strings.TrimSpace(out.String()))
	if err != nil {
		t.Fatalf("os.Stat(reported config path) error = %v, want resolved destination", err)
	}
	resolvedInfo, err := os.Stat(resolvedDestination)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v, want nil", resolvedDestination, err)
	}
	if !os.SameFile(reportedInfo, resolvedInfo) {
		t.Errorf("reported config path does not identify the resolved destination")
	}
}

func TestWriteOutputFileUsesOSPathResolutionAfterSymlinkDotDot(t *testing.T) {
	t.Parallel()

	destination, lexicalSibling, resolvedDestination := symlinkDotDotDestination(t)
	const sentinel = "lexical sibling must remain unchanged"
	if err := os.WriteFile(lexicalSibling, []byte(sentinel), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", lexicalSibling, err)
	}
	if err := os.WriteFile(resolvedDestination, []byte("old output"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", resolvedDestination, err)
	}

	if err := writeOutputFile(destination, []byte("new output")); err != nil {
		t.Fatalf("writeOutputFile with symlink/.. destination error = %v, want nil", err)
	}
	if got, err := os.ReadFile(lexicalSibling); err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want sentinel unchanged", lexicalSibling, err)
	} else if string(got) != sentinel {
		t.Errorf("lexical sibling body = %q, want %q", got, sentinel)
	}
	if got, err := os.ReadFile(resolvedDestination); err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want new output", resolvedDestination, err)
	} else if string(got) != "new output" {
		t.Errorf("resolved output body = %q, want %q", got, "new output")
	}
}

func symlinkDotDotDestination(t *testing.T) (string, string, string) {
	t.Helper()

	root := t.TempDir()
	lexicalParent := filepath.Join(root, "a")
	targetParent := filepath.Join(root, "b")
	targetChild := filepath.Join(targetParent, "child")
	for _, path := range []string{lexicalParent, targetChild} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatalf("os.MkdirAll(%q) error = %v, want nil", path, err)
		}
	}
	if err := os.Symlink(targetChild, filepath.Join(lexicalParent, "link")); err != nil {
		t.Fatalf("os.Symlink(%q) error = %v, want nil", filepath.Join(lexicalParent, "link"), err)
	}

	// Keep this raw spelling; filepath.Join would clean away the symlink/.. case.
	destination := lexicalParent + string(filepath.Separator) + "link" +
		string(filepath.Separator) + ".." + string(filepath.Separator) + "result.yaml"
	return destination, filepath.Join(lexicalParent, "result.yaml"), filepath.Join(targetParent, "result.yaml")
}

func TestConfigInitForcePinsParentDuringAncestorSubstitution(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ancestor := filepath.Join(root, "ancestor")
	parent := filepath.Join(ancestor, "parent")
	movedAncestor := filepath.Join(root, "original-ancestor")
	redirected := filepath.Join(root, "redirected")
	redirectedParent := filepath.Join(redirected, "parent")
	for _, path := range []string{parent, redirectedParent} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatalf("os.MkdirAll(%q) error = %v, want nil", path, err)
		}
	}

	configPath := filepath.Join(parent, "config.yaml")
	redirectedConfig := filepath.Join(redirectedParent, "config.yaml")
	const sentinel = "redirected config must remain unchanged"
	if err := os.WriteFile(configPath, []byte("old config"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", configPath, err)
	}
	if err := os.WriteFile(redirectedConfig, []byte(sentinel), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", redirectedConfig, err)
	}

	var out, errOut bytes.Buffer
	app := &App{}
	err := app.runConfigInitWithForceHook(
		globalOptions{configPath: configPath},
		true,
		&out,
		&errOut,
		func() error {
			if err := os.Rename(ancestor, movedAncestor); err != nil {
				return err
			}
			return os.Symlink(redirected, ancestor)
		},
	)
	if err != nil {
		t.Fatalf("config init --force during ancestor substitution error = %v, want nil", err)
	}

	if got, err := os.ReadFile(redirectedConfig); err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want sentinel unchanged", redirectedConfig, err)
	} else if string(got) != sentinel {
		t.Errorf("redirected config = %q, want %q", got, sentinel)
	}
	if got, err := os.ReadFile(filepath.Join(movedAncestor, "parent", "config.yaml")); err != nil {
		t.Fatalf("os.ReadFile(moved config) error = %v, want config template", err)
	} else if string(got) != configInitTemplate {
		t.Errorf("moved config = %q, want starter template", got)
	}
}

func TestWriteOutputFilePinsParentDuringAncestorSubstitution(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	ancestor := filepath.Join(root, "ancestor")
	parent := filepath.Join(ancestor, "parent")
	movedAncestor := filepath.Join(root, "original-ancestor")
	redirected := filepath.Join(root, "redirected")
	redirectedParent := filepath.Join(redirected, "parent")
	for _, path := range []string{parent, redirectedParent} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatalf("os.MkdirAll(%q) error = %v, want nil", path, err)
		}
	}

	destination := filepath.Join(parent, "output.json")
	redirectedDestination := filepath.Join(redirectedParent, "output.json")
	const sentinel = "redirected output must remain unchanged"
	if err := os.WriteFile(destination, []byte("old output"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", destination, err)
	}
	if err := os.WriteFile(redirectedDestination, []byte(sentinel), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", redirectedDestination, err)
	}

	err := writeOutputFileWithHook(destination, []byte("new output"), func() error {
		if err := os.Rename(ancestor, movedAncestor); err != nil {
			return err
		}
		return os.Symlink(redirected, ancestor)
	})
	if err != nil {
		t.Fatalf("writeOutputFile during ancestor substitution error = %v, want nil", err)
	}

	if got, err := os.ReadFile(redirectedDestination); err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want sentinel unchanged", redirectedDestination, err)
	} else if string(got) != sentinel {
		t.Errorf("redirected output = %q, want %q", got, sentinel)
	}
	if got, err := os.ReadFile(filepath.Join(movedAncestor, "parent", "output.json")); err != nil {
		t.Fatalf("os.ReadFile(moved output) error = %v, want new output", err)
	} else if string(got) != "new output" {
		t.Errorf("moved output = %q, want %q", got, "new output")
	}
}
