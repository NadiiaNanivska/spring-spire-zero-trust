//go:build linux

package cache

import (
	"os"
	"syscall"
)

// statExtra returns the device number and the inode change time of a file.
//
// ctime is what makes the cache resistant to tampering. mtime can be set to an
// arbitrary value from user space via utimensat, so an attacker who overwrites a
// jar in place and restores its original mtime would otherwise get a stale cache
// hit for the clean hash. ctime cannot be set: any change to the inode, including
// a utimensat that rewinds mtime, moves ctime to the current time.
func statExtra(fi os.FileInfo) (dev uint64, ctimeNs int64, ok bool) {
	st, isStat := fi.Sys().(*syscall.Stat_t)
	if !isStat {
		return 0, 0, false
	}
	return uint64(st.Dev), st.Ctim.Nano(), true
}
