package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"sync"

	"golang.org/x/sync/singleflight"
)

type cacheKey struct {
	inode uint64
	mtime int64
}

type HashCache struct {
	mu          sync.RWMutex
	store       map[cacheKey]string
	stringStore map[string]string
	sfGroup     singleflight.Group
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

	sfKey := fmt.Sprintf("%d:%d", inode, mtime)
	v, err, _ := hc.sfGroup.Do(sfKey, func() (interface{}, error) {
		if hash, err := hc.Get(inode, mtime); err == nil {
			return hash, nil
		}
		hash, err := computeFn()
		if err != nil {
			return "", err
		}
		hc.Set(inode, mtime, hash)
		return hash, nil
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

func (hc *HashCache) GetOrComputeByPath(filePath string) (string, error) {
	diskInfo, err := os.Stat(filePath)
	if err != nil {
		return "", err
	}

	inode, err := GetInode(diskInfo)
	if err != nil {
		return "", fmt.Errorf("cannot retrieve inode for %s: %w", filePath, err)
	}

	return hc.GetOrCompute(inode, diskInfo.ModTime().UnixNano(), func() (string, error) {
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
