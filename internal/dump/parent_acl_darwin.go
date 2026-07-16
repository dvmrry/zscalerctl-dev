//go:build darwin

package dump

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
)

const (
	darwinFileSecurityMagic = 0x012cc16d
	darwinFileSecuritySize  = 44
	darwinACESize           = 24
	darwinACLMaxEntries     = 128
	darwinACEKindMask       = 0x0f
	darwinACEPermit         = 1
	darwinACEDeny           = 2
)

// validateStableNamespaceACLFile allows absent and deny-only ACLs. Any permit
// ACE is rejected: even an inheritance-only grant can widen access on staging
// children, while a direct grant can let another principal rewrite the parent
// namespace despite owner-only mode bits.
func validateStableNamespaceACLFile(file *os.File) error {
	security, err := readDarwinExtendedSecurity(file)
	if err != nil {
		return err
	}
	if len(security) == 0 {
		return nil
	}
	if len(security) < darwinFileSecuritySize {
		return errors.New("malformed extended ACL")
	}
	if magic := binary.LittleEndian.Uint32(security[:4]); magic != darwinFileSecurityMagic {
		return fmt.Errorf("unexpected extended ACL magic %#x", magic)
	}
	count := binary.LittleEndian.Uint32(security[36:40])
	if count == math.MaxUint32 {
		return nil
	}
	if count > darwinACLMaxEntries {
		return fmt.Errorf("extended ACL entry count %d exceeds limit", count)
	}
	want := darwinFileSecuritySize + int(count)*darwinACESize
	if len(security) < want {
		return errors.New("truncated extended ACL")
	}
	for index := uint32(0); index < count; index++ {
		offset := darwinFileSecuritySize + int(index)*darwinACESize
		flags := binary.LittleEndian.Uint32(security[offset+16 : offset+20])
		switch flags & darwinACEKindMask {
		case darwinACEDeny:
			continue
		case darwinACEPermit:
			return errors.New("extended ACL contains a permit entry")
		default:
			return fmt.Errorf("extended ACL entry %d has unsupported kind %d", index, flags&darwinACEKindMask)
		}
	}
	return nil
}
