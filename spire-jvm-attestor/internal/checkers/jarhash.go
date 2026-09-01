package checkers

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/yourorg/spire-jvm-attestor/internal/cache"
	"github.com/yourorg/spire-jvm-attestor/internal/procfs"
)

// ErrNotJVM is returned by Check when the process has no JAR files — i.e. it
// is not a JVM workload. Callers should treat this as "not applicable" rather
// than an integrity failure.
var ErrNotJVM = errors.New("no jar files found in maps or cmdline")

// JarHashChecker computes the SHA-256 of every JAR mapped into a JVM process and
// emits it as a jvm:jar_sha256=<hash> selector. It does NOT compare against a
// reference value: the expected hash is enforced by the SPIRE registration entry
// (populated by CI/CD from jvm-hashes-configmap.yaml). Keeping the comparison out
// of the hot path removes the per-attestation manifest/Artifactory lookup, so the
// attestor never blocks on an external API call.
type JarHashChecker struct {
	bufPool sync.Pool
}

func NewJarHashChecker() *JarHashChecker {
	return &JarHashChecker{
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
	discoverStart := time.Now()
	jarEntries, err := procfs.ParseJarPathsFromMaps(ctx.ProcRoot)
	if err != nil {
		return nil, fmt.Errorf("maps parse error: %w", err)
	}

	if len(jarEntries) == 0 {
		jarEntries, err = procfs.ExtractJarsFromCmdline(ctx.ProcRoot)
		if err != nil {
			return nil, fmt.Errorf("jar cmdline fallback error: %w", err)
		}
		if len(jarEntries) == 0 {
			return nil, ErrNotJVM
		}
	}
	discoverDuration := time.Since(discoverStart)

	var allSelectors []string
	globalInodeConsistent := true
	var hashComputeDuration time.Duration

	for _, entry := range jarEntries {
		nsPath := filepath.Join(ctx.ProcRoot, "root", entry.Path)

		diskInfo, err := os.Stat(nsPath)
		if err != nil {
			return nil, fmt.Errorf("cannot stat jar at %s: %w", nsPath, err)
		}

		diskInode, err := cache.GetInode(diskInfo)
		if err != nil {
			return nil, fmt.Errorf("cannot retrieve inode for %s: %w", nsPath, err)
		}

		var actualHash string

		if entry.Inode == 0 {
			sbKey := fmt.Sprintf("%s:%d:%d", entry.Path, diskInfo.ModTime().UnixNano(), diskInfo.Size())

			if cachedHash, found := ctx.HashCache.GetSpringBoot(sbKey); found {
				actualHash = cachedHash
			} else {
				hashStart := time.Now()
				computedHash, err := c.calculateFileSHA256(nsPath)
				hashComputeDuration += time.Since(hashStart)
				if err != nil {
					return nil, fmt.Errorf("failed calculating spring boot jar hash for %s: %w", nsPath, err)
				}
				actualHash = computedHash
				ctx.HashCache.SetSpringBoot(sbKey, actualHash)
			}

		} else if diskInode != entry.Inode {
			globalInodeConsistent = false

			if cachedHash, err := ctx.HashCache.Get(diskInode, diskInfo.ModTime().UnixNano()); err == nil && cachedHash != "" {
				actualHash = cachedHash
			} else {
				hashStart := time.Now()
				computedHash, err := c.calculateFileSHA256(nsPath)
				hashComputeDuration += time.Since(hashStart)
				if err != nil {
					return nil, fmt.Errorf("forced hash computation failed for overlayfs path %s: %w", nsPath, err)
				}
				actualHash = computedHash
				ctx.HashCache.Set(diskInode, diskInfo.ModTime().UnixNano(), actualHash)
			}

		} else {
			hash, err := ctx.HashCache.GetOrCompute(entry.Inode, diskInfo.ModTime().UnixNano(), func() (string, error) {
				hashStart := time.Now()
				defer func() { hashComputeDuration += time.Since(hashStart) }()
				return c.calculateFileSHA256(nsPath)
			})
			if err != nil {
				return nil, fmt.Errorf("hash cache fetch failed for %s: %w", nsPath, err)
			}
			actualHash = hash
		}

		// No reference comparison here: the computed hash is published as a
		// selector and the SPIRE registration entry decides whether it is
		// authorized. A mismatch simply means the workload won't match any
		// entry that requires the expected jvm:jar_sha256 value.
		allSelectors = append(allSelectors, SelectorJarSha256Prefix+actualHash)
	}

	allSelectors = append(allSelectors, SelectorMapsVerified)
	if globalInodeConsistent {
		allSelectors = append(allSelectors, SelectorInodeConsistentTrue)
	} else {
		allSelectors = append(allSelectors, SelectorInodeConsistentFalse)
	}

	if ctx.Logger != nil {
		ctx.Logger.Debug("jar-hash stage timing",
			"pid", ctx.PID,
			"jar_discovery_us", discoverDuration.Microseconds(),
			"jar_hash_compute_us", hashComputeDuration.Microseconds(),
			"jar_count", len(jarEntries),
		)
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
