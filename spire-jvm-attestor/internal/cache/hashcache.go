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

// FileID identifies one content revision of one file.
//
// Dev+Ino pin the file itself, Size and MtimeNs catch ordinary rewrites, and
// CtimeNs closes the tampering case: rewriting a jar in place and restoring its
// mtime still moves ctime, so the cache no longer returns the pre-tamper hash.
type FileID struct {
	Path    string
	Dev     uint64
	Ino     uint64
	Size    int64
	MtimeNs int64
	CtimeNs int64
}

// NewFileID builds a cache identity from a stat result. path must be the logical
// jar path (e.g. /app/service.jar) rather than the /proc handle used to read it,
// so the same file attested through different PIDs shares a single entry.
func NewFileID(path string, fi os.FileInfo) FileID {
	ino, _ := GetInode(fi)
	dev, ctimeNs, _ := statExtra(fi)

	return FileID{
		Path:    path,
		Dev:     dev,
		Ino:     ino,
		Size:    fi.Size(),
		MtimeNs: fi.ModTime().UnixNano(),
		CtimeNs: ctimeNs,
	}
}

func (id FileID) key() string {
	return fmt.Sprintf("%s|%d:%d:%d:%d:%d",
		id.Path, id.Dev, id.Ino, id.Size, id.MtimeNs, id.CtimeNs)
}

// GetDevInode returns the (device, inode) pair identifying a file. Inode numbers
// are only unique within a filesystem, so callers deduplicating files across
// mounts must key on both — a bare inode collides between, say, an overlay upper
// dir and a tmpfs mount.
func GetDevInode(fi os.FileInfo) (dev, ino uint64, err error) {
	ino, err = GetInode(fi)
	if err != nil {
		return 0, 0, err
	}
	dev, _, _ = statExtra(fi)
	return dev, ino, nil
}

type HashCache struct {
	mu      sync.RWMutex
	store   map[string]string
	sfGroup singleflight.Group
}

func NewHashCache() *HashCache {
	return &HashCache{store: make(map[string]string)}
}

func (hc *HashCache) Get(id FileID) (string, bool) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	hash, exists := hc.store[id.key()]
	return hash, exists
}

func (hc *HashCache) Set(id FileID, hash string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	hc.store[id.key()] = hash
}

func (hc *HashCache) Len() int {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	return len(hc.store)
}

// GetOrCompute returns the cached hash for id, or computes and stores it.
// Concurrent attestations of the same file collapse into a single computation.
func (hc *HashCache) GetOrCompute(id FileID, computeFn func() (string, error)) (string, error) {
	if hash, ok := hc.Get(id); ok {
		return hash, nil
	}

	v, err, _ := hc.sfGroup.Do(id.key(), func() (interface{}, error) {
		if hash, ok := hc.Get(id); ok {
			return hash, nil
		}
		hash, err := computeFn()
		if err != nil {
			return "", err
		}
		hc.Set(id, hash)
		return hash, nil
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

// GetOrComputeByPath is a convenience wrapper that opens the file once and
// derives the cache identity from the open descriptor, so the bytes hashed and
// the metadata keyed on always belong to the same file.
func (hc *HashCache) GetOrComputeByPath(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("cannot stat open file %s: %w", filePath, err)
	}

	return hc.GetOrCompute(NewFileID(filePath, fi), func() (string, error) {
		return SHA256Reader(file, nil)
	})
}

// SHA256Reader hashes r, optionally reusing a caller-supplied buffer.
func SHA256Reader(r io.Reader, buf []byte) (string, error) {
	hasher := sha256.New()
	if _, err := io.CopyBuffer(hasher, r, buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
