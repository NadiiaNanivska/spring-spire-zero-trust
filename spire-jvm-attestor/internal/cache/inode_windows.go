//go:build windows
// +build windows

package cache

import (
	"os"
)

func GetInode(fileInfo os.FileInfo) (uint64, error) {
	// On Windows, use a hash of the full file path as an inode substitute
	// since Windows doesn't have traditional inodes
	path := fileInfo.Name()
	return hashPath(path), nil
}

func hashPath(path string) uint64 {
	h := uint64(5381)
	for _, c := range path {
		h = ((h << 5) + h) + uint64(c)
	}
	return h
}
