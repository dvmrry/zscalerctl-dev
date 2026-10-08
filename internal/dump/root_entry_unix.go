//go:build unix

package dump

import "golang.org/x/sys/unix"

func rootEntryOpenFlags(directory bool) int {
	flags := unix.O_NONBLOCK | unix.O_NOFOLLOW
	if directory {
		flags |= unix.O_DIRECTORY
	}
	return flags
}
