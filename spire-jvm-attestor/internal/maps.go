package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// mapsEntry represents a single jar file entry found in /proc/<PID>/maps.
// The inode is captured at parse time and used later for TOCTOU cross-check.
type mapsEntry struct {
	// path is the absolute path of the jar inside the container filesystem
	// (e.g. /app/payments-service.jar), as recorded by the kernel in maps.
	path string

	// inode is the kernel inode number for the mapped file at the time maps
	// was read. Used to detect bait-and-switch: if the on-disk inode differs
	// from this value, the file was replaced after the JVM loaded it.
	inode uint64
}

// parseJarPathsFromMaps reads /proc/<PID>/maps and returns all unique .jar
// file entries with their kernel inodes.
//
// Maps format (one entry per line):
//
//	<addr>-<addr> <perms> <offset> <dev> <inode> <pathname>
//	7f3a12000000-7f3a13000000 r--p 00000000 fd:01 1234567 /app/service.jar
//
// Only lines with a pathname ending in ".jar" are collected.
// Duplicate paths (same jar mapped multiple times at different addresses)
// are deduplicated — we only need one entry per jar.
func parseJarPathsFromMaps(procRoot string) ([]mapsEntry, error) {
	mapsPath := filepath.Join(procRoot, "maps")
	data, err := os.ReadFile(mapsPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", mapsPath, err)
	}

	var results []mapsEntry
	seen := make(map[string]bool)

	for _, line := range strings.Split(string(data), "\n") {
		// Split on whitespace; we need at least 6 fields including pathname
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		pathname := fields[5]
		if !strings.HasSuffix(pathname, ".jar") {
			continue
		}
		if seen[pathname] {
			continue // already collected this jar
		}
		seen[pathname] = true

		inode, err := strconv.ParseUint(fields[4], 10, 64)
		if err != nil {
			// Inode parse failure is non-fatal: we skip this entry.
			// verifyJarHash will fail later if no valid entries are found.
			continue
		}

		results = append(results, mapsEntry{path: pathname, inode: inode})
	}

	return results, nil
}

// extractJarsFromCmdline is a fallback for Spring Boot fat-jar deployments
// where the jar is not memory-mapped (so it won't appear in maps).
//
// Spring Boot launches as:
//
//	java -jar /app/application.jar
//
// We scan cmdline for "-jar" and return the following argument as a synthetic
// mapsEntry with inode=0 (TOCTOU check will use on-disk stat instead).
func extractJarsFromCmdline(procRoot string) ([]mapsEntry, error) {
	cmdlineRaw, err := os.ReadFile(filepath.Join(procRoot, "cmdline"))
	if err != nil {
		return nil, fmt.Errorf("cannot read cmdline for jar fallback: %w", err)
	}

	args := strings.Split(string(cmdlineRaw), "\x00")
	for i, arg := range args {
		if arg == "-jar" && i+1 < len(args) {
			jarPath := args[i+1]
			if strings.HasSuffix(jarPath, ".jar") {
				// inode=0 signals to verifyJarHash that it should use on-disk stat
				// for both the inode value and the TOCTOU check.
				return []mapsEntry{{path: jarPath, inode: 0}}, nil
			}
		}
	}

	return nil, nil
}
