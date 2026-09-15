//go:build linux

package cache

import (
	"os"
	"syscall"
)

func statExtra(fi os.FileInfo) (dev uint64, ctimeNs int64, ok bool) {
	st, isStat := fi.Sys().(*syscall.Stat_t)
	if !isStat {
		return 0, 0, false
	}
	return uint64(st.Dev), st.Ctim.Nano(), true
}
