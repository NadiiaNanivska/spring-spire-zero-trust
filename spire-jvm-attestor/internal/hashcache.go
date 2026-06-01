package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sync"
	"syscall"
)

type cacheKey struct {
	inode uint64
	mtime int64
}

type HashCache struct {
	mu          sync.RWMutex
	store       map[cacheKey]string
	stringStore map[string]string
}

func NewHashCache() *HashCache {
	return &HashCache{
		store:       make(map[cacheKey]string),
		stringStore: make(map[string]string),
	}
}

func (hc *HashCache) Get(inode uint64, mtime int64) (string, error) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	key := cacheKey{inode: inode, mtime: mtime}
	hash, exists := hc.store[key]
	if !exists {
		return "", fmt.Errorf("cache miss for inode %d", inode)
	}
	return hash, nil
}

func (hc *HashCache) Set(inode uint64, mtime int64, hash string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	key := cacheKey{inode: inode, mtime: mtime}
	hc.store[key] = hash
}

func (hc *HashCache) GetSpringBoot(key string) (string, bool) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	hash, exists := hc.stringStore[key]
	return hash, exists
}

func (hc *HashCache) SetSpringBoot(key string, hash string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	hc.stringStore[key] = hash
}

func (hc *HashCache) GetOrCompute(inode uint64, mtime int64, computeFn func() (string, error)) (string, error) {
	if hash, err := hc.Get(inode, mtime); err == nil {
		return hash, nil
	}

	hash, err := computeFn()
	if err != nil {
		return "", err
	}

	hc.Set(inode, mtime, hash)
	return hash, nil
}

func (hc *HashCache) GetOrComputeByPath(filePath string) (string, error) {
	diskInfo, err := os.Stat(filePath)
	if err != nil {
		return "", err
	}

	diskStat, ok := diskInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return "", fmt.Errorf("cannot retrieve syscall.Stat_t for %s", filePath)
	}

	return hc.GetOrCompute(diskStat.Ino, diskInfo.ModTime().Unix(), func() (string, error) {
		file, err := os.Open(filePath)
		if err != nil {
			return "", err
		}
		defer file.Close()

		hasher := sha256.New()
		if _, err := io.Copy(hasher, file); err != nil {
			return "", err
		}
		return hex.EncodeToString(hasher.Sum(nil)), nil
	})
}