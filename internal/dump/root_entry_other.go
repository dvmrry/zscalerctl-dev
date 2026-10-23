//go:build !unix

package dump

func rootEntryOpenFlags(bool) int {
	return 0
}
