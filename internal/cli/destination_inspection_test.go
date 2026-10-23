package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWritersFallBackOnDestinationInspectionError(t *testing.T) {
	t.Parallel()

	const oldBody = "old body\n"
	inspectionErr := errors.New("injected destination inspection failure")
	for _, tc := range []struct {
		name      string
		config    bool
		existing  bool
		directory bool
		wantUsage bool
		want      string
	}{
		{name: "output-replace", existing: true, want: "new output"},
		{name: "output-directory", directory: true, wantUsage: true},
		{name: "config-existing", config: true, existing: true, wantUsage: true, want: oldBody},
		{name: "config-create", config: true, want: configInitTemplate},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			destination := filepath.Join(dir, "result")
			if tc.directory {
				if err := os.Mkdir(destination, 0o700); err != nil {
					t.Fatalf("os.Mkdir(%q) error = %v, want nil", destination, err)
				}
			} else if tc.existing {
				if err := os.WriteFile(destination, []byte(oldBody), 0o600); err != nil {
					t.Fatalf("os.WriteFile(%q) error = %v, want nil", destination, err)
				}
			}

			inspected := false
			inspect := func(name string) (os.FileInfo, error) {
				inspected = true
				if name != filepath.Base(destination) {
					t.Errorf("inspection path = %q, want %q", name, filepath.Base(destination))
				}
				return nil, &os.PathError{Op: "lstat", Path: name, Err: inspectionErr}
			}

			var out, errOut bytes.Buffer
			var err error
			if tc.config {
				app := &App{}
				err = app.runConfigInitWithForceLstat(
					globalOptions{configPath: destination}, false, &out, &errOut, nil, inspect,
				)
			} else {
				err = writeOutputFileWithLstat(destination, []byte("new output"), nil, inspect)
			}
			if !inspected {
				t.Fatal("destination inspection was not called")
			}
			if tc.wantUsage {
				if !errors.Is(err, ErrUsage) {
					t.Fatalf("writer error = %v, want ErrUsage", err)
				}
				wantMessage := "write --output: " + destination + " is not a regular file"
				if tc.config {
					wantMessage = "config already exists at " + destination + "; pass --force to overwrite"
				}
				if err.Error() != wantMessage {
					t.Errorf("writer error = %q, want %q", err, wantMessage)
				}
			} else if err != nil {
				t.Fatalf("writer error = %v, want nil", err)
			}

			if tc.directory {
				info, err := os.Lstat(destination)
				if err != nil {
					t.Fatalf("os.Lstat(%q) error = %v, want directory to remain", destination, err)
				}
				if !info.IsDir() {
					t.Errorf("os.Lstat(%q).IsDir() = false, want true", destination)
				}
			} else if got, err := os.ReadFile(destination); err != nil {
				t.Fatalf("os.ReadFile(%q) error = %v, want nil", destination, err)
			} else if string(got) != tc.want {
				t.Errorf("destination body = %q, want %q", got, tc.want)
			}

			if tc.config && !tc.wantUsage {
				if got := out.String(); got != destination+"\n" {
					t.Errorf("config init stdout = %q, want %q", got, destination+"\n")
				}
			} else if out.Len() != 0 {
				t.Errorf("stdout = %q, want empty", out.String())
			}
			assertNoOutputTempFiles(t, dir)
		})
	}
}

func TestWritersDoNotFallBackForMissingDestination(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name   string
		config bool
	}{
		{name: "output"},
		{name: "config", config: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			destination := filepath.Join(dir, "result")
			if err := os.Mkdir(destination, 0o700); err != nil {
				t.Fatalf("os.Mkdir(%q) error = %v, want nil", destination, err)
			}

			// Pathname inspection would reject this directory. A not-exist
			// result from the pinned inspection must reach the hook instead.
			inspect := func(name string) (os.FileInfo, error) {
				return nil, &os.PathError{Op: "lstat", Path: name, Err: os.ErrNotExist}
			}
			stopped := errors.New("stop after destination check")
			afterDestinationCheck := func() error { return stopped }

			var out, errOut bytes.Buffer
			var err error
			if tc.config {
				app := &App{}
				err = app.runConfigInitWithForceLstat(
					globalOptions{configPath: destination}, false, &out, &errOut, afterDestinationCheck, inspect,
				)
			} else {
				err = writeOutputFileWithLstat(destination, []byte("must not write"), afterDestinationCheck, inspect)
			}
			if !errors.Is(err, stopped) {
				t.Fatalf("writer error = %v, want hook error", err)
			}
			info, err := os.Lstat(destination)
			if err != nil {
				t.Fatalf("os.Lstat(%q) error = %v, want directory to remain", destination, err)
			}
			if !info.IsDir() {
				t.Errorf("os.Lstat(%q).IsDir() = false, want true", destination)
			}
			assertNoOutputTempFiles(t, dir)
		})
	}
}
