//go:build windows

package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dvmrry/zscalerctl/internal/fileperm"
	"golang.org/x/sys/windows"
)

func TestWritersAcceptDestinationWithoutReadData(t *testing.T) {
	t.Parallel()

	var versionOut, versionErr bytes.Buffer
	app := New(&versionOut, &versionErr, nil)
	if err := app.Run(context.Background(), []string{"--format", "json", "version"}); err != nil {
		t.Fatalf("App.Run(version) error = %v, want nil", err)
	}

	const oldBody = "old body\n"
	for _, tc := range []struct {
		name      string
		fileName  string
		config    bool
		force     bool
		wantUsage bool
		want      string
	}{
		{name: "output-replace", fileName: "output.json", want: versionOut.String()},
		{name: "config-replace", fileName: "config.yaml", config: true, force: true, want: configInitTemplate},
		{name: "config-existing", fileName: "config.yaml", config: true, wantUsage: true, want: oldBody},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			destination := filepath.Join(dir, tc.fileName)
			if err := os.WriteFile(destination, []byte(oldBody), 0o600); err != nil {
				t.Fatalf("os.WriteFile(%q) error = %v, want nil", destination, err)
			}
			restore := denyWindowsDestinationReadData(t, destination)

			var out, errOut bytes.Buffer
			app := New(&out, &errOut, nil)
			args := []string{"--format", "json", "--output", destination, "version"}
			if tc.config {
				args = []string{"--config", destination, "config", "init"}
				if tc.force {
					args = append(args, "--force")
				}
			}
			err := app.Run(context.Background(), args)
			if tc.wantUsage {
				if !errors.Is(err, ErrUsage) {
					t.Fatalf("config init (existing, no --force) error = %v, want ErrUsage", err)
				}
				wantMessage := "config already exists at " + destination + "; pass --force to overwrite"
				if err.Error() != wantMessage {
					t.Errorf("config init error = %q, want %q", err, wantMessage)
				}
				if _, err := os.ReadFile(destination); !errors.Is(err, os.ErrPermission) {
					t.Fatalf("os.ReadFile(existing config) error = %v, want ErrPermission", err)
				}
				restore()
			} else if err != nil {
				t.Fatalf("App.Run with destination read denied error = %v, want nil", err)
			}

			if tc.config && !tc.wantUsage {
				if got := out.String(); got != destination+"\n" {
					t.Errorf("config init stdout = %q, want %q", got, destination+"\n")
				}
				file, err := fileperm.OpenOwnerOnly(destination)
				if err != nil {
					t.Fatalf("OpenOwnerOnly(created config) error = %v, want nil", err)
				}
				_ = file.Close()
			} else if out.Len() != 0 {
				t.Errorf("stdout = %q, want empty", out.String())
			}
			if got, err := os.ReadFile(destination); err != nil {
				t.Fatalf("os.ReadFile(%q) error = %v, want nil", destination, err)
			} else if string(got) != tc.want {
				t.Errorf("destination body = %q, want %q", got, tc.want)
			}
			assertNoOutputTempFiles(t, dir)
		})
	}
}

func denyWindowsDestinationReadData(t *testing.T, path string) func() {
	t.Helper()

	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		t.Skipf("cannot open current process token: %v", err)
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		t.Skipf("cannot read current user SID: %v", err)
	}
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil || sd == nil {
		t.Skipf("cannot read destination security descriptor: %v", err)
	}
	oldDACL, _, err := sd.DACL()
	if err != nil || oldDACL == nil {
		t.Skipf("cannot read destination DACL: %v", err)
	}

	// Preserve existing grants and deny only file-data reads for this user.
	entries := []windows.EXPLICIT_ACCESS{{
		AccessPermissions: windows.FILE_READ_DATA,
		AccessMode:        windows.DENY_ACCESS,
		Inheritance:       windows.NO_INHERITANCE,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_USER,
			TrusteeValue: windows.TrusteeValueFromSID(user.User.Sid),
		},
	}}
	dacl, err := windows.ACLFromEntries(entries, oldDACL)
	runtime.KeepAlive(user)
	if err != nil {
		t.Skipf("cannot create destination read-data deny ACE: %v", err)
	}
	if err := windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION,
		nil,
		nil,
		dacl,
		nil,
	); err != nil {
		t.Skipf("cannot set destination read-data deny ACE: %v", err)
	}
	restore := func() {
		t.Helper()
		if err := windows.SetNamedSecurityInfo(
			path,
			windows.SE_FILE_OBJECT,
			windows.DACL_SECURITY_INFORMATION,
			nil,
			nil,
			oldDACL,
			nil,
		); err != nil {
			t.Errorf("restore destination DACL: %v", err)
		}
	}
	t.Cleanup(restore)

	if _, err := os.ReadFile(path); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("os.ReadFile(%q) error = %v, want ErrPermission", path, err)
	}
	if _, err := os.Lstat(path); err != nil {
		t.Fatalf("os.Lstat(%q) error = %v, want attribute inspection to succeed", path, err)
	}
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		t.Fatalf("os.OpenRoot(parent) error = %v, want nil", err)
	}
	defer root.Close()
	if _, err := root.Lstat(filepath.Base(path)); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("root.Lstat(destination) error = %v, want ErrPermission", err)
	}
	return restore
}
