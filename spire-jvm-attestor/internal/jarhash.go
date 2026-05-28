package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type JarHashChecker struct {
	manifestPath string
}

func NewJarHashChecker(manifestPath string) *JarHashChecker {
	return &JarHashChecker{manifestPath: manifestPath}
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

	manifest, err := ctx.ManifestCache.Load(c.manifestPath)
	if err != nil {
		return nil, fmt.Errorf("manifest load error: %w", err)
	}

	for _, entry := range jarEntries {
		nsPath := filepath.Join(ctx.ProcRoot, "root", entry.path)

		if entry.inode != 0 {
			diskInfo, err := os.Stat(nsPath)
			if err != nil {
				return nil, fmt.Errorf("cannot stat jar at %s: %w", nsPath, err)
			}

			diskStat, ok := diskInfo.Sys().(*syscall.Stat_t)
			if !ok {
				return nil, fmt.Errorf("cannot retrieve syscall.Stat_t for %s", nsPath)
			}

			if diskStat.Ino != entry.inode {
				return []string{"jvm:inode_consistent=false"}, nil
			}
		}

		hash, err := ctx.HashCache.GetOrCompute(nsPath)
		if err != nil {
			return nil, fmt.Errorf("hash computation failed for %s: %w", nsPath, err)
		}

		expected, ok := manifest[entry.path]
		if !ok {
			return nil, fmt.Errorf("jar %s is not listed in the CI manifest — refusing SVID", entry.path)
		}

		if hash != expected {
			return nil, fmt.Errorf(
				"jar hash mismatch for %s: computed=%s expected=%s",
				entry.path, hash, expected,
			)
		}

		return []string{
			"jvm:jar_sha256:" + hash,
			"jvm:maps_verified=true",
			"jvm:inode_consistent=true",
		}, nil
	}

	return nil, fmt.Errorf("no jar files could be verified against the manifest")
}