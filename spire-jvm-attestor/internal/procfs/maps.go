package procfs

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/yourorg/spire-jvm-attestor/internal/cache"
)

const (
	SourceMaps = "maps"
	SourceFD = "fd"
	SourceCmdline = "cmdline"
	SourceMapsAndFD = "maps+fd"
)

// KernelPath binds reads to the JVM-held inode; Inode checks for backing-file changes.
type MapsEntry struct {
	Path       string
	Inode      uint64
	Source     string
	KernelPath string
}

func ParseJarPathsFromMaps(procRoot string) ([]MapsEntry, error) {
	mapsPath := filepath.Join(procRoot, "maps")
	data, err := os.ReadFile(mapsPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", mapsPath, err)
	}

	var results []MapsEntry
	seen := make(map[string]bool)

	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		pathname := stripDeleted(fields[5])
		if !strings.HasSuffix(pathname, ".jar") || seen[pathname] {
			continue
		}
		seen[pathname] = true

		inode, err := strconv.ParseUint(fields[4], 10, 64)
		if err != nil {
			continue
		}

		results = append(results, MapsEntry{
			Path:       pathname,
			Inode:      inode,
			Source:     SourceMaps,
			KernelPath: filepath.Join(procRoot, "map_files", fields[0]),
		})
	}

	return results, nil
}

// Spring Boot fat-jars use pread(), so discover them through fd rather than maps.
func ExtractJarsFromFDs(procRoot string) ([]MapsEntry, error) {
	fdDir := filepath.Join(procRoot, "fd")

	dirents, err := os.ReadDir(fdDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read %s: %w", fdDir, err)
	}

	type fileKey struct{ dev, ino uint64 }

	var results []MapsEntry
	seen := make(map[fileKey]bool)

	for _, dirent := range dirents {
		fdPath := filepath.Join(fdDir, dirent.Name())

		target, err := os.Readlink(fdPath)
		if err != nil {
			continue
		}
		target = stripDeleted(target)
		if !filepath.IsAbs(target) || !strings.HasSuffix(target, ".jar") {
			continue
		}

		fi, err := os.Stat(fdPath)
		if err != nil || !fi.Mode().IsRegular() {
			continue
		}

		dev, inode, err := cache.GetDevInode(fi)
		if err != nil || seen[fileKey{dev, inode}] {
			continue
		}
		seen[fileKey{dev, inode}] = true

		results = append(results, MapsEntry{
			Path:       target,
			Inode:      inode,
			Source:     SourceFD,
			KernelPath: fdPath,
		})
	}

	return results, nil
}

// Cmdline discovery is unverified; callers must degrade the selectors.
func ExtractJarsFromCmdline(procRoot string) ([]MapsEntry, error) {
	cmdlineRaw, err := os.ReadFile(filepath.Join(procRoot, "cmdline"))
	if err != nil {
		return nil, fmt.Errorf("cannot read cmdline for jar fallback: %w", err)
	}

	args := strings.Split(string(cmdlineRaw), "\x00")
	for i, arg := range args {
		if arg == "-jar" && i+1 < len(args) {
			jarPath := args[i+1]
			if !strings.HasSuffix(jarPath, ".jar") {
				continue
			}

			resolved, err := resolveJarPath(procRoot, jarPath)
			if err != nil {
				return nil, err
			}

			return []MapsEntry{{
				Path:   resolved,
				Inode:  0,
				Source: SourceCmdline,
			}}, nil
		}
	}

	return nil, nil
}

func resolveJarPath(procRoot, jarPath string) (string, error) {
	if filepath.IsAbs(jarPath) {
		return filepath.Clean(jarPath), nil
	}

	cwd, err := os.Readlink(filepath.Join(procRoot, "cwd"))
	if err != nil {
		return "", fmt.Errorf("cannot read cwd for relative jar path %q: %w", jarPath, err)
	}

	return filepath.Clean(filepath.Join(stripDeleted(cwd), jarPath)), nil
}

func stripDeleted(path string) string {
	if idx := strings.Index(path, " (deleted)"); idx >= 0 {
		return path[:idx]
	}
	return path
}
