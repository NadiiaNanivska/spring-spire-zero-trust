package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/yourorg/spire-jvm-attestor/internal/hashsource"
)

type JarHashChecker struct {
	hashSource hashsource.HashSource
	bufPool    sync.Pool
}

func NewJarHashChecker(hashSource hashsource.HashSource) *JarHashChecker {
	return &JarHashChecker{
		hashSource: hashSource,
		bufPool: sync.Pool{
			New: func() interface{} {
				return make([]byte, 64*1024)
			},
		},
	}
}

func (c *JarHashChecker) Name() string {
	return "jar-hash"
}

func (c *JarHashChecker) Check(ctx *AttestationContext) ([]string, error) {
	jarEntries, err := parseJarPathsFromMaps(ctx.ProcRoot)
	if err != nil {
		return nil, fmt.Errorf("maps parse error: %w", err)
	}

	if len(jarEntries) == 0 {
		jarEntries, err = extractJarsFromCmdline(ctx.ProcRoot)
		if err != nil {
			return nil, fmt.Errorf("jar cmdline fallback error: %w", err)
		}
		if len(jarEntries) == 0 {
			return nil, fmt.Errorf("no jar files found in maps or cmdline")
		}
	}

	var allSelectors []string
	globalInodeConsistent := true

	for _, entry := range jarEntries {
		nsPath := filepath.Join(ctx.ProcRoot, "root", entry.path)

		diskInfo, err := os.Stat(nsPath)
		if err != nil {
			return nil, fmt.Errorf("cannot stat jar at %s: %w", nsPath, err)
		}

		diskStat, ok := diskInfo.Sys().(*syscall.Stat_t)
		if !ok {
			return nil, fmt.Errorf("cannot retrieve syscall.Stat_t for %s", nsPath)
		}

		var actualHash string

		if entry.inode == 0 {
			sbKey := fmt.Sprintf("%s:%d:%d", entry.path, diskInfo.ModTime().Unix(), diskInfo.Size())

			if cachedHash, found := ctx.HashCache.GetSpringBoot(sbKey); found {
				actualHash = cachedHash
			} else {
				computedHash, err := c.calculateFileSHA256(nsPath)
				if err != nil {
					return nil, fmt.Errorf("failed calculating spring boot jar hash for %s: %w", nsPath, err)
				}
				actualHash = computedHash
				ctx.HashCache.SetSpringBoot(sbKey, actualHash)
			}

		} else if diskStat.Ino != entry.inode {
			globalInodeConsistent = false

			if cachedHash, err := ctx.HashCache.Get(diskStat.Ino, diskInfo.ModTime().Unix()); err == nil && cachedHash != "" {
				actualHash = cachedHash
			} else {
				computedHash, err := c.calculateFileSHA256(nsPath)
				if err != nil {
					return nil, fmt.Errorf("forced hash computation failed for overlayfs path %s: %w", nsPath, err)
				}
				actualHash = computedHash
				ctx.HashCache.Set(diskStat.Ino, diskInfo.ModTime().Unix(), actualHash)
			}

		} else {
			hash, err := ctx.HashCache.GetOrCompute(entry.inode, diskInfo.ModTime().Unix(), func() (string, error) {
				return c.calculateFileSHA256(nsPath)
			})
			if err != nil {
				return nil, fmt.Errorf("hash cache fetch failed for %s: %w", nsPath, err)
			}
			actualHash = hash
		}

		expected, err := c.hashSource.GetExpectedHash(ctx.Context, entry.path)
		if err != nil {
			return nil, fmt.Errorf("integrity configuration error for %s: %w", entry.path, err)
		}

		if actualHash != expected {
			return nil, fmt.Errorf("jar crypto integrity mismatch for %s: computed=%s expected=%s", entry.path, actualHash, expected)
		}

		allSelectors = append(allSelectors, SelectorJarSha256Prefix+actualHash)
	}

	// Додаємо інфраструктурні селектори верифікації
	allSelectors = append(allSelectors, SelectorMapsVerified)
	if globalInodeConsistent {
		allSelectors = append(allSelectors, SelectorInodeConsistentTrue)
	} else {
		allSelectors = append(allSelectors, SelectorInodeConsistentFalse)
	}

	return allSelectors, nil
}

func (c *JarHashChecker) calculateFileSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	buf := c.bufPool.Get().([]byte)
	defer c.bufPool.Put(buf)

	hasher := sha256.New()
	if _, err := io.CopyBuffer(hasher, file, buf); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}