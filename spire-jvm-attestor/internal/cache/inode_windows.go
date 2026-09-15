//go:build windows
// +build windows

package cache

import (
	"os"
)

func GetInode(fileInfo os.FileInfo) (uint64, error) {
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
