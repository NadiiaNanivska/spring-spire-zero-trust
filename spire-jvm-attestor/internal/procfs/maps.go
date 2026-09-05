package procfs

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/yourorg/spire-jvm-attestor/internal/cache"
)

// Discovery sources, in descending order of trustworthiness.
const (
	// SourceMaps: the jar is file-backed in the process address space. The path
	// and inode are recorded by the kernel in the VMA.
	SourceMaps = "maps"
	// SourceFD: the jar is held open by the process. The link target is recorded
	// by the kernel at open() time and is not writable from user space.
	SourceFD = "fd"
	// SourceCmdline: last resort. /proc/<PID>/cmdline is a window into the
	// process' own argv memory, which the process can rewrite (MITRE T1036.011),
	// so a jar discovered this way carries no kernel guarantee at all.
	SourceCmdline = "cmdline"
	// SourceMapsAndFD: the same process contributed jars through both kernel
	// sources. Reported instead of either one alone so the selector cannot be
	// read as "only the address space was consulted".
	SourceMapsAndFD = "maps+fd"
)

// MapsEntry is one jar discovered inside a JVM process.
//
// KernelPath, when non-empty, is a /proc handle resolving to the exact inode the
// kernel associates with the mapping or descriptor. Reading through it bypasses
// pathname resolution, so a symlink swap or a path-level TOCTOU on the jar cannot
// redirect the hash to a different file. Inode is the value the kernel recorded
// at map/open time; comparing it against the fstat of the handle detects that the
// file backing the path changed after the JVM took it.
type MapsEntry struct {
	Path       string
	Inode      uint64
	Source     string
	KernelPath string
}

// ParseJarPathsFromMaps reads /proc/<PID>/maps and returns unique .jar entries
// with their kernel inodes and their map_files handles.
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

		// map_files entries are named after the address range exactly as maps
		// prints it. The directory requires CONFIG_CHECKPOINT_RESTORE, so the
		// caller must tolerate the handle not existing.
		results = append(results, MapsEntry{
			Path:       pathname,
			Inode:      inode,
			Source:     SourceMaps,
			KernelPath: filepath.Join(procRoot, "map_files", fields[0]),
		})
	}

	return results, nil
}

// ExtractJarsFromFDs discovers jars the process currently holds open by walking
// /proc/<PID>/fd.
//
// This is the discovery path for Spring Boot fat-jars: the JDK zip implementation
// reads the archive with pread() rather than mapping it, so the jar never appears
// in maps, but the JVM keeps its descriptor open for the process lifetime. Unlike
// cmdline, the link target here is recorded by the kernel and cannot be rewritten
// by the process, and the descriptor itself can be reopened to reach the very
// inode the JVM holds.
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
		// An unlinked jar still yields a usable descriptor; keep it and drop the
		// kernel's " (deleted)" suffix so the logical path stays comparable.
		target = stripDeleted(target)
		if !filepath.IsAbs(target) || !strings.HasSuffix(target, ".jar") {
			continue
		}

		// Stat the descriptor rather than the target path: this is the inode the
		// process actually holds, even if the name now points elsewhere.
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

	// Returned in descriptor-allocation order; the caller canonicalises the order.
	return results, nil
}

// ExtractJarsFromCmdline is the last-resort fallback when neither maps nor the
// descriptor table yields a jar. Inode is 0 and KernelPath is empty because
// nothing here is kernel-attested: the caller must degrade its selectors
// accordingly rather than treat the result as verified.
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

// stripDeleted removes the " (deleted)" marker the kernel appends to maps
// pathnames and fd link targets whose file has been unlinked.
func stripDeleted(path string) string {
	if idx := strings.Index(path, " (deleted)"); idx >= 0 {
		return path[:idx]
	}
	return path
}
