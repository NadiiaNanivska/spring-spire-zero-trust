//go:build !linux

package cache

import "os"

// statExtra has no portable equivalent outside Linux. Callers degrade to path,
// size and mtime, which is enough to build and unit-test on developer machines;
// the plugin itself only runs on Linux.
func statExtra(os.FileInfo) (dev uint64, ctimeNs int64, ok bool) {
	return 0, 0, false
}
