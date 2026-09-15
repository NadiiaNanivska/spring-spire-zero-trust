package checkers

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/yourorg/spire-jvm-attestor/internal/cache"
	"github.com/yourorg/spire-jvm-attestor/internal/procfs"
)

// ErrNotJVM means attestation is not applicable, not an integrity failure.
var ErrNotJVM = errors.New("no jar files found in maps, fd table or cmdline")

// JarHashChecker emits hashes; SPIRE registration entries enforce the expected values.
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
	jarEntries, source, err := discoverJars(ctx.ProcRoot)
	if err != nil {
		return nil, err
	}
	if len(jarEntries) == 0 {
		return nil, ErrNotJVM
	}
	discoverDuration := time.Since(discoverStart)

	var (
		allSelectors    []string
		hashDuration    time.Duration
		inodeConsistent = true
		kernelHandle    = true
		setDigest       = sha256.New()
	)

	for _, entry := range jarEntries {
		result, err := c.hashEntry(ctx, entry)
		if err != nil {
			return nil, err
		}

		hashDuration += result.computeDuration
		inodeConsistent = inodeConsistent && result.inodeMatches
		kernelHandle = kernelHandle && result.viaKernelHandle

		allSelectors = append(allSelectors, SelectorJarSha256Prefix+result.hash)
		fmt.Fprintf(setDigest, "%s:%s\n", entry.Path, result.hash)
	}

	allSelectors = append(allSelectors,
		SelectorJarSetSha256Prefix+hex.EncodeToString(setDigest.Sum(nil)),
		SelectorJarSourcePrefix+source,
	)

	if source == procfs.SourceCmdline {
		allSelectors = append(allSelectors, SelectorMapsVerifiedFalse)
	} else {
		allSelectors = append(allSelectors, SelectorMapsVerified)
	}

	if kernelHandle {
		allSelectors = append(allSelectors, SelectorKernelHandleTrue)
	} else {
		allSelectors = append(allSelectors, SelectorKernelHandleFalse)
	}

	if inodeConsistent {
		allSelectors = append(allSelectors, SelectorInodeConsistentTrue)
	} else {
		allSelectors = append(allSelectors, SelectorInodeConsistentFalse)
	}

	if ctx.Logger != nil {
		ctx.Logger.Debug("jar-hash stage timing",
			"pid", ctx.PID,
			"jar_discovery_us", discoverDuration.Microseconds(),
			"jar_hash_compute_us", hashDuration.Microseconds(),
			"jar_count", len(jarEntries),
			"jar_source", source,
			"kernel_handle", kernelHandle,
		)
	}

	return allSelectors, nil
}

// Union maps and fd sources so a mapped jar cannot hide additional open jars.
func discoverJars(procRoot string) ([]procfs.MapsEntry, string, error) {
	mapped, err := procfs.ParseJarPathsFromMaps(procRoot)
	if err != nil {
		return nil, "", fmt.Errorf("maps parse error: %w", err)
	}

	opened, err := procfs.ExtractJarsFromFDs(procRoot)
	if err != nil {
		return nil, "", fmt.Errorf("fd table scan error: %w", err)
	}

	if entries := mergeByPath(mapped, opened); len(entries) > 0 {
		// Summarise before merging: deduplication replaces mapped entries with fd entries.
		return sortByPath(entries), summariseSources(mapped, opened), nil
	}

	entries, err := procfs.ExtractJarsFromCmdline(procRoot)
	if err != nil {
		return nil, "", fmt.Errorf("jar cmdline fallback error: %w", err)
	}
	if len(entries) > 0 {
		return sortByPath(entries), procfs.SourceCmdline, nil
	}

	return nil, "", nil
}

// mergeByPath prefers fd handles because map_files may be unavailable.
func mergeByPath(mapped, opened []procfs.MapsEntry) []procfs.MapsEntry {
	merged := make(map[string]procfs.MapsEntry, len(mapped)+len(opened))

	for _, entry := range mapped {
		merged[entry.Path] = entry
	}
	for _, entry := range opened {
		merged[entry.Path] = entry
	}

	entries := make([]procfs.MapsEntry, 0, len(merged))
	for _, entry := range merged {
		entries = append(entries, entry)
	}
	return entries
}

func summariseSources(mapped, opened []procfs.MapsEntry) string {
	viaMaps := len(mapped) > 0
	viaFD := len(opened) > 0

	switch {
	case viaMaps && viaFD:
		return procfs.SourceMapsAndFD
	case viaMaps:
		return procfs.SourceMaps
	default:
		return procfs.SourceFD
	}
}

func sortByPath(entries []procfs.MapsEntry) []procfs.MapsEntry {
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return entries
}

type hashResult struct {
	hash            string
	viaKernelHandle bool
	inodeMatches    bool
	computeDuration time.Duration
}

// hashEntry hashes and stats the same descriptor to avoid path-swap races.
func (c *JarHashChecker) hashEntry(ctx *AttestationContext, entry procfs.MapsEntry) (hashResult, error) {
	result := hashResult{inodeMatches: true}

	readPath := entry.KernelPath
	if readPath != "" {
		// Missing map_files: fall back to the namespace path and report the weaker guarantee.
		if _, err := os.Stat(readPath); err != nil {
			readPath = ""
		}
	}
	if readPath == "" {
		readPath = filepath.Join(ctx.ProcRoot, "root", entry.Path)
	} else {
		result.viaKernelHandle = true
	}

	file, err := os.Open(readPath)
	if err != nil {
		return result, fmt.Errorf("cannot open jar %s via %s: %w", entry.Path, readPath, err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return result, fmt.Errorf("cannot stat open jar %s: %w", entry.Path, err)
	}

	// Inode 0 is unverified; mismatches can indicate OverlayFS copy-up or a file swap.
	if entry.Inode != 0 {
		if diskInode, err := cache.GetInode(info); err == nil && diskInode != entry.Inode {
			result.inodeMatches = false
		}
	}

	fileID := cache.NewFileID(entry.Path, info)

	hashStart := time.Now()
	hash, err := ctx.HashCache.GetOrCompute(fileID, func() (string, error) {
		buf := c.bufPool.Get().([]byte)
		defer c.bufPool.Put(buf) //nolint:staticcheck // SA6002: pooling a fixed-size slice is intentional
		return cache.SHA256Reader(file, buf)
	})
	result.computeDuration = time.Since(hashStart)
	if err != nil {
		return result, fmt.Errorf("hash computation failed for %s: %w", entry.Path, err)
	}

	result.hash = hash
	return result, nil
}
