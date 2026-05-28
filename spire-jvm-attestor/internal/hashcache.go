package internal

import (
	"crypto/sha256"
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
	mu    sync.RWMutex
	store map[cacheKey]string
}

func NewHashCache() *HashCache {
	return &HashCache{
		store: make(map[cacheKey]string),
	}
}

func (c *HashCache) GetOrCompute(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", path, err)
	}

	sys, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", fmt.Errorf("cannot retrieve syscall.Stat_t for %s", path)
	}

	key := cacheKey{inode: sys.Ino, mtime: sys.Mtim.Sec}

	c.mu.RLock()
	if cached, ok := c.store[key]; ok {
		c.mu.RUnlock()
		return cached, nil
	}
	c.mu.RUnlock()

	hash, err := sha256File(path)
	if err != nil {
		return "", fmt.Errorf("sha256 %s: %w", path, err)
	}

	c.mu.Lock()
	c.store[key] = hash
	c.mu.Unlock()

	return hash, nil
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
