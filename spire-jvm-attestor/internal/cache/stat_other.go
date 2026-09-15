//go:build !linux

package cache

import "os"

// Only Linux provides the metadata required for production attestation.
func statExtra(os.FileInfo) (dev uint64, ctimeNs int64, ok bool) {
	return 0, 0, false
}
