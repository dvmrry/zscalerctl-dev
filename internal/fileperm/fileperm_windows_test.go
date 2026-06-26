//go:build windows

package fileperm

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

const testPath = `C:\Users\test\config.yaml`

func TestWindowsDACLAcceptsOwnerAdminSystemOnly(t *testing.T) {
	t.Parallel()

	for _, sddl := range []string{
		"O:BAD:P(A;;GRGW;;;BA)(A;;GRGW;;;SY)",
		// Stock Windows file: owner = Administrators (BA), with Administrators
		// and SYSTEM carrying FA (Full Access = GENERIC_ALL). BA and SY are in
		// the allowed-SID set. This case proves the mask change (removing the
		// FILE_GENERIC_READ / FILE_GENERIC_WRITE composites and adding the
		// execute bits) does not false-reject the execute/full bits that real
		// Windows files carry for these trusted principals.
		// (No OWNER RIGHTS / S-1-3-4 ACE: that is a distinct well-known SID, not
		// the owner, and is correctly rejected as a non-allowed principal.)
		"O:BAG:BAD:P(A;;FA;;;BA)(A;;FA;;;SY)",
	} {
		sddl := sddl
		t.Run(sddl, func(t *testing.T) {
			t.Parallel()
			sd := securityDescriptorFromString(t, sddl)
			if err := validateSecurityDescriptor(testPath, sd); err != nil {
				t.Fatalf("validateSecurityDescriptor(%q) error = %v, want nil (stock BA/SY/OW ACEs must be accepted)", sddl, err)
			}
		})
	}
}

func TestWindowsDACLRejectsBroadPrincipal(t *testing.T) {
	t.Parallel()

	for _, sddl := range []string{
		"O:BUD:P(A;;GR;;;BA)",
		"O:BAD:P(A;;GR;;;WD)",
		"O:BAD:P(A;;GR;;;BU)",
		"O:BAD:P(A;;GR;;;AU)",
		"O:BAD:P(A;ID;GR;;;BU)",
		"O:BAD:P(A;ID;GRGW;;;S-1-5-21-1-2-3-500)",
	} {
		sddl := sddl
		t.Run(sddl, func(t *testing.T) {
			t.Parallel()
			sd := securityDescriptorFromString(t, sddl)
			err := validateSecurityDescriptor(testPath, sd)
			if !errors.Is(err, ErrInsecurePermissions) {
				t.Fatalf("validateSecurityDescriptor(%q) error = %v, want ErrInsecurePermissions", sddl, err)
			}
			// Every reject must carry the actionable icacls remediation and the
			// file path, and must not leak any value (only path + command).
			assertRemediation(t, sddl, err)
		})
	}
}

// TestWindowsDACLRejectsExecuteOnlyForeignSID verifies that an ACE granting
// GENERIC_EXECUTE or FILE_EXECUTE only (no read/write bits) to a broad
// principal is still rejected.  Prior to Fix 2 the mask did not include the
// execute bits, so such an ACE was silently skipped, leaving a gap in the
// owner-only invariant.  SDDL "GX" encodes GENERIC_EXECUTE (0x20000000);
// "WD" is the Everyone (S-1-1-0) well-known SID shorthand.
func TestWindowsDACLRejectsExecuteOnlyForeignSID(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		sddl string
	}{
		// GENERIC_EXECUTE granted to Everyone — the primary gap case.
		{name: "Everyone:GX", sddl: "O:BAD:P(A;;GX;;;WD)"},
		// FILE_EXECUTE (0x0020) granted to Everyone, expressed as a hex rights
		// mask in SDDL.  SDDL accepts 0x-prefixed hex for rights fields.
		{name: "Everyone:FILE_EXECUTE", sddl: "O:BAD:P(A;;0x20;;;WD)"},
		// GENERIC_EXECUTE granted to a non-admin domain account SID — ensures
		// the check is not limited to well-known blocked SIDs.
		{name: "DomainUser:GX", sddl: "O:BAD:P(A;;GX;;;S-1-5-21-1-2-3-500)"},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sd := securityDescriptorFromString(t, tc.sddl)
			err := validateSecurityDescriptor(testPath, sd)
			if !errors.Is(err, ErrInsecurePermissions) {
				t.Fatalf("validateSecurityDescriptor(%q) error = %v, want ErrInsecurePermissions (execute-only foreign SID must be rejected)", tc.sddl, err)
			}
			assertRemediation(t, tc.name, err)
		})
	}
}

func TestWindowsDACLRejectsUnsupportedAllowACE(t *testing.T) {
	t.Parallel()

	sd := securityDescriptorFromString(t, "O:BAD:P(A;;GR;;;BA)")
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatalf("DACL() error = %v, want nil", err)
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if err := windows.GetAce(dacl, 0, &ace); err != nil {
		t.Fatalf("GetAce() error = %v, want nil", err)
	}
	ace.Header.AceType = accessAllowedObjectACEType

	verr := validateSecurityDescriptor(testPath, sd)
	if !errors.Is(verr, ErrInsecurePermissions) {
		t.Fatalf("validateSecurityDescriptor(object ACE) error = %v, want ErrInsecurePermissions", verr)
	}
	assertRemediation(t, "unsupported ACE", verr)
}

// TestOpenOwnerOnlyAcceptsLocalOwnerOnlyFile is the single guard that the
// ACCEPT path isn't broken. The other tests call validateSecurityDescriptor
// directly and bypass validateLocalFixedNTFS/validateOpenFile; this exercises
// the full stack — volume gate (local fixed NTFS/ReFS on the CI/host runner)
// PLUS the DACL check — against a normal local owner-only file, and asserts it
// is accepted. It MUST run on the DAV-38 Windows host.
func TestOpenOwnerOnlyAcceptsLocalOwnerOnlyFile(t *testing.T) {
	dir := t.TempDir() // local fixed NTFS on the host / CI runner
	file := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(file, []byte("token: x\n"), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	// Make it owner-only via the verified A9 remediation (icacls): reset
	// inheritance and grant the current user full control only.
	user := os.Getenv("USERNAME")
	if user == "" {
		t.Skip("USERNAME not set; cannot run icacls remediation")
	}
	out, err := exec.Command("icacls", file, "/inheritance:r", "/grant:r", user+":F").CombinedOutput()
	if err != nil {
		t.Fatalf("icacls remediation failed: %v\n%s", err, out)
	}

	f, err := OpenOwnerOnly(file)
	if err != nil {
		t.Fatalf("OpenOwnerOnly(local owner-only file) error = %v, want nil (accept path)", err)
	}
	if f == nil {
		t.Fatalf("OpenOwnerOnly(local owner-only file) returned nil *os.File, want non-nil")
	}
	_ = f.Close()
}

func TestICACLSLockDownCommandUsesSystemDirectory(t *testing.T) {
	t.Setenv("SystemRoot", `C:\hostile`)

	gotName, gotArgs, err := icaclsLockDownCommand(testPath, `DOMAIN\alice`, func() (string, error) {
		return `C:\Windows\System32`, nil
	})
	if err != nil {
		t.Fatalf("icaclsLockDownCommand(%q) error = %v, want nil", testPath, err)
	}
	if wantName := `C:\Windows\System32\icacls.exe`; gotName != wantName {
		t.Fatalf("icaclsLockDownCommand(%q) name = %q, want %q", testPath, gotName, wantName)
	}
	if gotName == `C:\hostile\System32\icacls.exe` || strings.Contains(gotName, `C:\hostile`) {
		t.Fatalf("icaclsLockDownCommand(%q) name = %q, want independent of SystemRoot", testPath, gotName)
	}
	wantArgs := []string{testPath, "/inheritance:r", "/grant:r", `DOMAIN\alice:F`}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("icaclsLockDownCommand(%q) args = %q, want %q", testPath, gotArgs, wantArgs)
	}
	if gotName == "cmd" || gotName == "cmd.exe" || gotName == "powershell" || gotName == "powershell.exe" {
		t.Fatalf("icaclsLockDownCommand(%q) name = %q, want direct helper execution", testPath, gotName)
	}
	for _, arg := range gotArgs {
		if arg == "/c" || arg == "-Command" {
			t.Fatalf("icaclsLockDownCommand(%q) args = %q, want no shell control arguments", testPath, gotArgs)
		}
	}
}

func TestICACLSLockDownCommandRejectsInvalidSystemDirectory(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name            string
		systemDirectory func() (string, error)
	}{
		{
			name:            "empty",
			systemDirectory: func() (string, error) { return "", nil },
		},
		{
			name:            "relative",
			systemDirectory: func() (string, error) { return `Windows\System32`, nil },
		},
		{
			name:            "api-error",
			systemDirectory: func() (string, error) { return "", windows.ERROR_PATH_NOT_FOUND },
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, _, err := icaclsLockDownCommand(testPath, `DOMAIN\alice`, tc.systemDirectory)
			if !errors.Is(err, ErrInsecurePermissions) {
				t.Fatalf("icaclsLockDownCommand(%q, %s) error = %v, want ErrInsecurePermissions", testPath, tc.name, err)
			}
		})
	}
}

// TestFriendlyNameFallsBackToSIDString verifies friendlyName never panics and
// degrades to the raw SID string for an unmapped (synthetic) SID, returns the
// fixed sentinel for nil, and resolves a friendly form for a well-known SID.
func TestFriendlyNameFallsBackToSIDString(t *testing.T) {
	t.Parallel()

	// (a) Unmapped synthetic SID: must fall back to the raw SID string exactly.
	sid, err := windows.StringToSid("S-1-5-21-1-2-3-1234")
	if err != nil {
		t.Fatalf("StringToSid() error = %v, want nil", err)
	}
	if got := friendlyName(sid); got != sid.String() {
		t.Fatalf("friendlyName(unmapped) = %q, want raw SID string %q", got, sid.String())
	}

	// (b) nil SID: fixed sentinel.
	if got := friendlyName(nil); got != "<nil SID>" {
		t.Fatalf("friendlyName(nil) = %q, want %q", got, "<nil SID>")
	}

	// (c) Well-known SID (LocalSystem): a friendly form must resolve, i.e. the
	// result must NOT equal the raw SID string.
	system, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		t.Fatalf("CreateWellKnownSid() error = %v, want nil", err)
	}
	if got := friendlyName(system); got == system.String() {
		t.Fatalf("friendlyName(well-known) = raw SID %q, want a resolved friendly name", got)
	}
}

// assertRemediation checks that a reject message names the path, includes the
// icacls remediation, and embeds the %USERNAME%:F grant.
func assertRemediation(t *testing.T, label string, err error) {
	t.Helper()
	msg := err.Error()
	for _, want := range []string{testPath, "icacls", "/inheritance:r", "%USERNAME%:F"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("reject %q message %q missing %q", label, msg, want)
		}
	}
}

func securityDescriptorFromString(t *testing.T, sddl string) *windows.SECURITY_DESCRIPTOR {
	t.Helper()
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		t.Fatalf("SecurityDescriptorFromString(%q) error = %v, want nil", sddl, err)
	}
	return sd
}
