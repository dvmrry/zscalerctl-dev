package cli

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteOutputFileRejectsDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	destination := filepath.Join(dir, "destination")
	if err := os.Mkdir(destination, 0o700); err != nil {
		t.Fatalf("os.Mkdir(%q) error = %v, want nil", destination, err)
	}

	err := writeOutputFile(destination, []byte("must not replace directory"))
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("writeOutputFile(directory) error = %v, want ErrUsage", err)
	}
	wantMessage := "write --output: " + destination + " is not a regular file"
	if err.Error() != wantMessage {
		t.Errorf("writeOutputFile(directory) error = %q, want %q", err, wantMessage)
	}
	if info, statErr := os.Stat(destination); statErr != nil {
		t.Fatalf("os.Stat(%q) error = %v, want directory to remain", destination, statErr)
	} else if !info.IsDir() {
		t.Errorf("os.Stat(%q).IsDir() = false, want true", destination)
	}
	assertNoOutputTempFiles(t, dir)
}

func TestWriteOutputFileRejectsTrailingSeparator(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	parent := filepath.Join(dir, "export")
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatalf("os.Mkdir(%q) error = %v, want nil", parent, err)
	}

	nested := filepath.Join(parent, "export")
	const sentinel = "nested file must remain unchanged"
	if err := os.WriteFile(nested, []byte(sentinel), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", nested, err)
	}

	destination := parent + string(filepath.Separator)
	if err := writeOutputFile(destination, []byte("must be rejected")); !errors.Is(err, ErrUsage) {
		t.Fatalf("writeOutputFile(trailing separator) error = %v, want ErrUsage", err)
	}
	if got, err := os.ReadFile(nested); err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want sentinel unchanged", nested, err)
	} else if string(got) != sentinel {
		t.Errorf("nested file body = %q, want %q", got, sentinel)
	}

	missingParent := filepath.Join(dir, "missing")
	if err := os.Mkdir(missingParent, 0o700); err != nil {
		t.Fatalf("os.Mkdir(%q) error = %v, want nil", missingParent, err)
	}
	destination = missingParent + string(filepath.Separator)
	if err := writeOutputFile(destination, []byte("must be rejected")); !errors.Is(err, ErrUsage) {
		t.Fatalf("writeOutputFile(missing trailing separator) error = %v, want ErrUsage", err)
	}
	missingNested := filepath.Join(missingParent, filepath.Base(missingParent))
	if _, err := os.Lstat(missingNested); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Lstat(%q) error = %v, want ErrNotExist", missingNested, err)
	}

	assertNoOutputTempFiles(t, parent)
	assertNoOutputTempFiles(t, missingParent)
}

func TestWriteOutputFileRejectsSymlink(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires platform-specific privileges on Windows")
	}

	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	destination := filepath.Join(dir, "destination")
	const original = "target must remain unchanged"
	if err := os.WriteFile(target, []byte(original), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", target, err)
	}
	if err := os.Symlink(target, destination); err != nil {
		t.Fatalf("os.Symlink(%q, %q) error = %v, want nil", target, destination, err)
	}

	err := writeOutputFile(destination, []byte("must not follow symlink"))
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("writeOutputFile(symlink) error = %v, want ErrUsage", err)
	}
	wantMessage := "write --output: " + destination + " is a symlink"
	if err.Error() != wantMessage {
		t.Errorf("writeOutputFile(symlink) error = %q, want %q", err, wantMessage)
	}
	if got, readErr := os.ReadFile(target); readErr != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want target unchanged", target, readErr)
	} else if string(got) != original {
		t.Errorf("target body = %q, want %q", got, original)
	}
	assertNoOutputTempFiles(t, dir)
}

func TestWriteOutputFileRejectsDestinationInspectionErrorBeforeTemp(t *testing.T) {
	t.Parallel()

	// NUL is rejected by the OS during Lstat, exercising the non-not-exist
	// inspection path without relying on process permissions or a live tenant.
	const destination = "destination\x00invalid"
	err := writeOutputFile(destination, []byte("must not create a temp file"))
	if !errors.Is(err, ErrUsage) {
		t.Fatalf("writeOutputFile(invalid destination) error = %v, want ErrUsage", err)
	}
	if !strings.Contains(err.Error(), "cannot inspect "+destination) {
		t.Errorf("writeOutputFile(invalid destination) error = %q, want inspection guidance", err)
	}
	if strings.Contains(err.Error(), ".tmp-") {
		t.Errorf("writeOutputFile(invalid destination) error = %q, want no temp-file name", err)
	}
}

func TestWriteOutputFileOverwritesRegularFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	destination := filepath.Join(dir, "destination")
	if err := os.WriteFile(destination, []byte("old body"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v, want nil", destination, err)
	}
	const want = "new body"
	if err := writeOutputFile(destination, []byte(want)); err != nil {
		t.Fatalf("writeOutputFile(existing regular file) error = %v, want nil", err)
	}
	if got, err := os.ReadFile(destination); err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", destination, err)
	} else if string(got) != want {
		t.Errorf("destination body = %q, want %q", got, want)
	}
	assertNoOutputTempFiles(t, dir)
}

func TestWriteOutputFileCreatesMissingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	destination := filepath.Join(dir, "destination")
	const want = "new body"
	if err := writeOutputFile(destination, []byte(want)); err != nil {
		t.Fatalf("writeOutputFile(missing file) error = %v, want nil", err)
	}
	if got, err := os.ReadFile(destination); err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v, want nil", destination, err)
	} else if string(got) != want {
		t.Errorf("destination body = %q, want %q", got, want)
	}
	assertNoOutputTempFiles(t, dir)
}

func TestWriteOutputFileAcceptsLongFileName(t *testing.T) {
	t.Parallel()

	// The temp name adds ".tmp-", "-" and at most ten digits, as os.CreateTemp
	// does, so a 230-byte name still fits a 255-byte NAME_MAX.
	for _, existing := range []bool{false, true} {
		dir := t.TempDir()
		destination := filepath.Join(dir, strings.Repeat("a", 230))
		if existing {
			if err := os.WriteFile(destination, []byte("old body"), 0o600); err != nil {
				t.Fatalf("os.WriteFile(long name) error = %v, want nil", err)
			}
		}
		const want = "new body"
		if err := writeOutputFile(destination, []byte(want)); err != nil {
			t.Fatalf("writeOutputFile(230-byte name, existing=%v) error = %v, want nil", existing, err)
		}
		if got, err := os.ReadFile(destination); err != nil || string(got) != want {
			t.Errorf("destination body = %q, %v, want %q", got, err, want)
		}
		assertNoOutputTempFiles(t, dir)
	}
}

func assertNoOutputTempFiles(t *testing.T, dir string) {
	t.Helper()

	leftovers, err := filepath.Glob(filepath.Join(dir, ".tmp-*"))
	if err != nil {
		t.Fatalf("filepath.Glob(%q) error = %v, want nil", dir, err)
	}
	if len(leftovers) != 0 {
		t.Errorf("output temp files leftover = %v, want none", leftovers)
	}
}
