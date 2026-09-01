package procfs

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// MapsEntry is a jar entry from /proc/<PID>/maps.
// Inode is captured at parse time; a mismatch with on-disk inode means the file was replaced after JVM load.
type MapsEntry struct {
	Path  string
	Inode uint64
}

// ParseJarPathsFromMaps reads /proc/<PID>/maps and returns unique .jar entries with their kernel inodes.
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

		pathname := fields[5]
		if !strings.HasSuffix(pathname, ".jar") || seen[pathname] {
			continue
		}
		seen[pathname] = true

		inode, err := strconv.ParseUint(fields[4], 10, 64)
		if err != nil {
			continue
		}

		results = append(results, MapsEntry{Path: pathname, Inode: inode})
	}

	return results, nil
}

// ExtractJarsFromCmdline is a fallback for Spring Boot fat-jars that are not memory-mapped.
// Returns a synthetic MapsEntry with Inode=0 when "-jar <path>" is found in cmdline.
// Relative jar paths are resolved against /proc/<PID>/cwd (e.g. "app.jar" + cwd "/app" → "/app/app.jar").
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

			return []MapsEntry{{Path: resolved, Inode: 0}}, nil
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

	if idx := strings.Index(cwd, " (deleted)"); idx >= 0 {
		cwd = cwd[:idx]
	}

	return filepath.Clean(filepath.Join(cwd, jarPath)), nil
}
