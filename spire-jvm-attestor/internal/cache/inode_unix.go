//go:build !windows
// +build !windows

package cache

import (
	"fmt"
	"os"
	"syscall"
)

func GetInode(fileInfo os.FileInfo) (uint64, error) {
	diskStat, ok := fileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("cannot retrieve syscall.Stat_t")
	}
	return diskStat.Ino, nil
}
