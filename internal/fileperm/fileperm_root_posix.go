//go:build !windows

package fileperm

import (
	"fmt"
	"os"
)

func writeOwnerOnlyRoot(root *os.Root, name string, data []byte) error {
	file, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	remove := true
	defer func() {
		_ = file.Close()
		if remove {
			_ = root.Remove(name)
		}
	}()
	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	if err := file.Chmod(0o600); err != nil {
		return fmt.Errorf("set owner-only permissions: %w", err)
	}
	if err := validateOpenFile(file); err != nil {
		return fmt.Errorf("verify owner-only permissions: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close file: %w", err)
	}
	remove = false
	return nil
}
