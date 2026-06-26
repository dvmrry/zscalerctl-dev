//go:build windows

package fileperm

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	accessAllowedObjectACEType         = 0x5
	accessAllowedCallbackACEType       = 0x9
	accessAllowedCallbackObjectACEType = 0xb
)

// Flags for GetFinalPathNameByHandleW. Both are 0 in the Win32 headers
// (FILE_NAME_NORMALIZED, VOLUME_NAME_DOS) but x/sys/windows does not export
// named constants for them, so we spell the value out with this name.
const fileNameNormalizedDOS = 0

// volumeNameGUID (VOLUME_NAME_GUID) asks GetFinalPathNameByHandleW for the
// volume-GUID form (`\\?\Volume{GUID}\...`). A local fixed volume always has a
// GUID even when it has no drive letter (letterless / folder-mounted volumes),
// so this is the fallback when the DOS-name form fails.
const volumeNameGUID = 0x1

// windowsCoveredAccessMask is the set of Windows access bits that constitute
// meaningful access to a config/secret file.  An ACE whose Mask has none of
// these bits set cannot read or execute the file and is skipped during DACL
// validation.
//
// Bits included:
//   - GENERIC_READ / GENERIC_WRITE / GENERIC_ALL / GENERIC_EXECUTE — generic
//     access wildcard bits; an ACE carrying any of these grants meaningful access.
//   - FILE_READ_DATA / FILE_WRITE_DATA / FILE_APPEND_DATA / FILE_EXECUTE —
//     concrete file-level data access bits (e.g. `icacls :R` grants FILE_READ_DATA).
//   - READ_CONTROL / WRITE_DAC / WRITE_OWNER / DELETE — security/ownership control.
//
// Bits intentionally omitted:
//   - FILE_GENERIC_READ and FILE_GENERIC_WRITE are composite constants that
//     expand to include FILE_READ_ATTRIBUTES, FILE_READ_EA, and SYNCHRONIZE
//     (metadata-only bits that disclose no file data). Including them would
//     cause the mask to match metadata-only ACEs (e.g. an ACE granting only
//     FILE_READ_ATTRIBUTES) and produce false rejects.  The non-composite
//     variants (GENERIC_READ, FILE_READ_DATA, etc.) already cover every real
//     data-read/write case without pulling in the metadata bits.
//   - FILE_GENERIC_EXECUTE is similarly a composite that includes
//     FILE_READ_ATTRIBUTES; GENERIC_EXECUTE and FILE_EXECUTE cover execute
//     access without the metadata-bit contamination.
const windowsCoveredAccessMask = windows.ACCESS_MASK(
	windows.GENERIC_READ |
		windows.GENERIC_WRITE |
		windows.GENERIC_ALL |
		windows.GENERIC_EXECUTE |
		windows.FILE_READ_DATA |
		windows.FILE_WRITE_DATA |
		windows.FILE_APPEND_DATA |
		windows.FILE_EXECUTE |
		windows.READ_CONTROL |
		windows.DELETE |
		windows.WRITE_DAC |
		windows.WRITE_OWNER,
)

func validate(path string) error {
	// Open a handle so the volume check and the security-descriptor read see
	// the same object (TOCTOU-consistent with the handle-based entry point).
	//
	// This intentionally follows reparse points / junctions and validates the
	// TARGET's volume + DACL: os.Open resolves symlinks/junctions, and the
	// volume gate then runs against whatever the handle actually backs, so a
	// junction pointing at a UNC/removable target is caught there. Windows has
	// no O_NOFOLLOW equivalent for this path, so we validate the resolved
	// target rather than refusing to follow.
	file, err := os.Open(path) // #nosec G304 -- caller-supplied config/secret path; the security descriptor is validated on the opened handle before any reads.
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	return validateOpenFile(file)
}

func validateOpenFile(file *os.File) error {
	path := file.Name()
	handle := windows.Handle(file.Fd())

	// Run the volume restriction BEFORE the DACL check so a network/removable/
	// non-NTFS/UNC file produces the clear remediation message instead of a
	// cryptic SID error (DAV-38: NFS/SMB/FAT files surface as opaque SIDs).
	if err := validateLocalFixedNTFS(handle, path); err != nil {
		return err
	}

	sd, err := windows.GetSecurityInfo(handle, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return fmt.Errorf("read file security descriptor: %w", err)
	}
	return validateSecurityDescriptor(path, sd)
}

func validateSecurityDescriptor(path string, sd *windows.SECURITY_DESCRIPTOR) error {
	if sd == nil {
		return fmt.Errorf("%w: missing security descriptor", ErrInsecurePermissions)
	}
	owner, _, err := sd.Owner()
	if err != nil {
		return fmt.Errorf("read security descriptor owner: %w", err)
	}
	if owner == nil {
		return fmt.Errorf("%w: missing owner", ErrInsecurePermissions)
	}
	if sidIsBlocked(owner) {
		return rejectSID(path, owner)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		return fmt.Errorf("%w: missing DACL", ErrInsecurePermissions)
	}
	if dacl == nil {
		return fmt.Errorf("%w: empty DACL", ErrInsecurePermissions)
	}

	allowed, err := windowsAllowedSIDs(owner)
	if err != nil {
		return err
	}

	for i := uint16(0); i < dacl.AceCount; i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, uint32(i), &ace); err != nil {
			return fmt.Errorf("read DACL ACE: %w", err)
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			if isUnsupportedAllowACEType(ace.Header.AceType) && ace.Mask&windowsCoveredAccessMask != 0 {
				return fmt.Errorf("%w: %s has an unsupported Windows allow ACE type %d; reset its permissions:  icacls %q /inheritance:r /grant:r \"%%USERNAME%%:F\"",
					ErrInsecurePermissions, path, ace.Header.AceType, path)
			}
			continue
		}
		if ace.Mask&windowsCoveredAccessMask == 0 {
			continue
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if sidIsBlocked(sid) {
			return rejectSID(path, sid)
		}
		if sidInSet(sid, allowed) {
			continue
		}
		return rejectSID(path, sid)
	}
	return nil
}

func writeOwnerOnly(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) // #nosec G304 -- caller-supplied config path; created O_EXCL and locked down via icacls + re-validated below.
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return fmt.Errorf("write file: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("close file: %w", err)
	}
	// Newly created files inherit the parent directory's DACL, which on a shared
	// or fold-redirected profile can grant broad principals. Break inheritance
	// and grant ONLY the current user so the file passes validateOpenFile.
	if err := lockDownToCurrentUser(path); err != nil {
		_ = os.Remove(path)
		return err
	}
	// Self-verify against the same read-side validator the loader uses; never
	// leave a config whose DACL the loader would later reject.
	verify, err := openOwnerOnly(path)
	if err != nil {
		_ = os.Remove(path)
		return fmt.Errorf("verify owner-only permissions: %w", err)
	}
	_ = verify.Close()
	return nil
}

// lockDownToCurrentUser breaks DACL inheritance on path and grants full control
// to only the current user, via the absolute icacls path with no shell. This is
// the verified A9 recipe: `icacls <path> /inheritance:r /grant:r <user>:F`.
func lockDownToCurrentUser(path string) error {
	user, err := currentUserName()
	if err != nil {
		return err
	}
	icacls, args, err := icaclsLockDownCommand(path, user, windows.GetSystemDirectory)
	if err != nil {
		return err
	}
	cmd := exec.Command(icacls, args...) // #nosec G204 -- absolute system binary from GetSystemDirectory, fixed flags; path is caller-supplied and user is derived from the process token.
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("icacls lock down %s: %w: %s", path, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func icaclsLockDownCommand(path, user string, systemDirectory func() (string, error)) (string, []string, error) {
	systemDir, err := systemDirectory()
	if err != nil {
		return "", nil, fmt.Errorf("%w: cannot determine Windows system directory: %w", ErrInsecurePermissions, err)
	}
	systemDir = strings.TrimSpace(systemDir)
	if systemDir == "" {
		return "", nil, fmt.Errorf("%w: cannot determine Windows system directory", ErrInsecurePermissions)
	}
	if !filepath.IsAbs(systemDir) {
		return "", nil, fmt.Errorf("%w: Windows system directory %q is not absolute", ErrInsecurePermissions, systemDir)
	}
	icacls := filepath.Join(systemDir, "icacls.exe")
	args := []string{path, "/inheritance:r", "/grant:r", user + ":F"}
	return icacls, args, nil
}

// currentUserName returns the account name (DOMAIN\user) for the process token
// user, the form icacls expects for a /grant principal. It falls back to the
// USERNAME environment variable only when the token lookup fails.
func currentUserName() (string, error) {
	token, err := windows.OpenCurrentProcessToken()
	if err == nil {
		defer token.Close()
		if user, uerr := token.GetTokenUser(); uerr == nil {
			if account, domain, _, aerr := user.User.Sid.LookupAccount(""); aerr == nil {
				if domain != "" {
					return domain + `\` + account, nil
				}
				return account, nil
			}
		}
	}
	if name := strings.TrimSpace(os.Getenv("USERNAME")); name != "" {
		return name, nil
	}
	return "", fmt.Errorf("%w: cannot determine current Windows user for owner-only write", ErrInsecurePermissions)
}

// rejectSID builds a value-free rejection error that names both the friendly
// principal and its raw SID, and appends the verified icacls remediation
// (DAV-38 A9). It carries only the path, the principal, and a fixed command —
// never a secret value.
func rejectSID(path string, sid *windows.SID) error {
	return fmt.Errorf("%w: %s is accessible by %s (%s); make it owner-only:  icacls %q /inheritance:r /grant:r \"%%USERNAME%%:F\"",
		ErrInsecurePermissions, path, friendlyName(sid), sid.String(), path)
}

// friendlyName resolves a SID to a human-readable "DOMAIN\account" principal
// via LookupAccountSid. On ANY error (unmapped SID, RPC failure, etc.) or an
// empty result it falls back to the raw SID string. It never panics.
func friendlyName(sid *windows.SID) string {
	if sid == nil {
		return "<nil SID>"
	}
	account, domain, _, err := sid.LookupAccount("")
	if err != nil || account == "" {
		return sid.String()
	}
	if domain == "" {
		return account
	}
	return domain + "\\" + account
}

// validateLocalFixedNTFS rejects config/secret files that do not live on a
// local, fixed, NTFS volume. DAV-38 confirmed the NTFS-DACL read cannot see
// SMB share permissions (false-ACCEPT risk) and mis-maps remote/NFS/removable
// identities (false-REJECT). We restrict to DRIVE_FIXED + NTFS + non-UNC so
// the user gets one clear message instead of an opaque SID error.
//
// Everything is handle-based (TOCTOU-safe): the file is already open.
func validateLocalFixedNTFS(handle windows.Handle, path string) error {
	// 1) Filesystem must be NTFS or ReFS. ReFS (Win11 Dev Drive, data volumes)
	// uses the same NTFS-style ACL/SID model this code validates, so rejecting
	// it would false-reject legitimate local files.
	fsName, err := fileSystemName(handle)
	if err != nil {
		return fmt.Errorf("read file system name: %w", err)
	}
	if !strings.EqualFold(fsName, "NTFS") && !strings.EqualFold(fsName, "ReFS") {
		return rejectVolume(path, fsName)
	}

	// 2) Recover the canonical path; reject UNC outright. A path we cannot
	// resolve is unverifiable as a local fixed volume, so reject it with the
	// same actionable message rather than a raw errno.
	//
	// Known fail-closed edge: a volume not registered with Mount Manager (some
	// third-party encrypted / virtual local volumes) can fail GetFinalPathNameByHandle
	// in BOTH the DOS and GUID forms; such a file is rejected, never falsely
	// accepted. Affected users keep config/secret files on %LOCALAPPDATA% (or
	// another Mount-Manager-backed local volume), or pass --config.
	finalPath, err := finalPathName(handle)
	if err != nil {
		return rejectVolume(path, fsName)
	}
	if isUNCPath(finalPath) {
		return rejectVolume(path, fsName)
	}

	// 3) The volume must be a fixed local drive.
	root := volumeRootFromFinalPath(finalPath)
	if root == "" {
		return rejectVolume(path, fsName)
	}
	rootPtr, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return fmt.Errorf("convert volume root: %w", err)
	}
	if windows.GetDriveType(rootPtr) != windows.DRIVE_FIXED {
		return rejectVolume(path, fsName)
	}
	return nil
}

func rejectVolume(path, fsName string) error {
	return fmt.Errorf("%w: %s must be on a local fixed NTFS or ReFS volume (move it to %%LOCALAPPDATA%%\\zscalerctl or pass --config with a local path); network, removable, and UNC paths can't be securely validated on Windows [filesystem seen: %s]",
		ErrInsecurePermissions, path, fsNameOrUnknown(fsName))
}

func fsNameOrUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}

// fileSystemName returns the file-system name (e.g. "NTFS") for the volume that
// backs the open handle, via GetVolumeInformationByHandleW.
func fileSystemName(handle windows.Handle) (string, error) {
	fsNameBuf := make([]uint16, windows.MAX_PATH+1)
	err := windows.GetVolumeInformationByHandle(
		handle,
		nil, // volume name buffer (unused)
		0,
		nil, // serial number (unused)
		nil, // max component length (unused)
		nil, // file system flags (unused)
		&fsNameBuf[0],
		uint32(len(fsNameBuf)),
	)
	if err != nil {
		return "", err
	}
	return windows.UTF16ToString(fsNameBuf), nil
}

// finalPathName returns the canonical path for the open handle, via
// GetFinalPathNameByHandleW. It tries the normalized DOS form first
// (`\\?\C:\...` for a local file, `\\?\UNC\server\share\...` for UNC). A
// letterless / folder-mounted volume has no DOS path, so on error it retries
// with the volume-GUID form (`\\?\Volume{GUID}\...`), which always exists for a
// local volume. If both fail it returns the original DOS error.
func finalPathName(handle windows.Handle) (string, error) {
	p, dosErr := finalPathNameFlags(handle, fileNameNormalizedDOS)
	if dosErr == nil {
		return p, nil
	}
	if p, err := finalPathNameFlags(handle, volumeNameGUID); err == nil {
		return p, nil
	}
	return "", dosErr
}

// finalPathNameFlags runs the two-call GetFinalPathNameByHandleW sizing dance
// for the given VOLUME_NAME_* flags.
func finalPathNameFlags(handle windows.Handle, flags uint32) (string, error) {
	// First call sizes the buffer (returns required length excluding the NUL).
	n, err := windows.GetFinalPathNameByHandle(handle, nil, 0, flags)
	if err != nil {
		return "", err
	}
	buf := make([]uint16, n+1)
	n, err = windows.GetFinalPathNameByHandle(handle, &buf[0], uint32(len(buf)), flags)
	if err != nil {
		return "", err
	}
	if int(n) >= len(buf) {
		// Path grew between calls; treat as unverifiable rather than truncating.
		return "", fmt.Errorf("final path length %d exceeds buffer %d", n, len(buf))
	}
	return windows.UTF16ToString(buf[:n]), nil
}

func openOwnerOnly(path string) (*os.File, error) {
	file, err := os.Open(path) // #nosec G304 -- caller-supplied config/secret path; security descriptor is validated on the opened handle before reads.
	if err != nil {
		return nil, err
	}
	if err := validateOpenFile(file); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

func windowsAllowedSIDs(owner *windows.SID) ([]*windows.SID, error) {
	system, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return nil, fmt.Errorf("create SYSTEM SID: %w", err)
	}
	admins, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return nil, fmt.Errorf("create Administrators SID: %w", err)
	}
	current, err := currentUserSID()
	if err != nil {
		return nil, err
	}
	return []*windows.SID{owner, current, system, admins}, nil
}

func currentUserSID() (*windows.SID, error) {
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return nil, fmt.Errorf("open current process token: %w", err)
	}
	defer token.Close()
	user, err := token.GetTokenUser()
	if err != nil {
		return nil, fmt.Errorf("read current user SID: %w", err)
	}
	sid, err := user.User.Sid.Copy()
	if err != nil {
		return nil, fmt.Errorf("copy current user SID: %w", err)
	}
	return sid, nil
}

func sidIsBlocked(sid *windows.SID) bool {
	types := []windows.WELL_KNOWN_SID_TYPE{
		windows.WinWorldSid,
		windows.WinBuiltinUsersSid,
		windows.WinAuthenticatedUserSid,
		windows.WinAccountDomainUsersSid,
		windows.WinInteractiveSid,
	}
	for _, typ := range types {
		if sid.IsWellKnown(typ) {
			return true
		}
	}
	return false
}

func isUnsupportedAllowACEType(aceType byte) bool {
	return aceType == accessAllowedObjectACEType ||
		aceType == accessAllowedCallbackACEType ||
		aceType == accessAllowedCallbackObjectACEType
}

func sidInSet(sid *windows.SID, set []*windows.SID) bool {
	for _, candidate := range set {
		if sid.Equals(candidate) {
			return true
		}
	}
	return false
}
