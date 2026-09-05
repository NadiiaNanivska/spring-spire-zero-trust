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

// ErrNotJVM is returned by Check when the process holds no JAR files — i.e. it
// is not a JVM workload. Callers should treat this as "not applicable" rather
// than an integrity failure.
var ErrNotJVM = errors.New("no jar files found in maps, fd table or cmdline")

// JarHashChecker computes the SHA-256 of every JAR a JVM process has mapped or
// open and emits both per-jar and set-wide selectors. It does NOT compare against
// a reference value: the expected hash is enforced by the SPIRE registration
// entry, which the deployment step pins from the artifact registry. Keeping the
// comparison out of the hot path means the attestor never blocks on an external
// API call.
//
// Jars are read through /proc handles (map_files or fd) whenever the kernel
// offers one, so the bytes hashed belong to the inode the JVM actually holds
// rather than to whatever the pathname resolves to at attestation time.
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

// discoverJars collects every jar the process holds and reports which sources
// produced them.
//
// The two kernel-attested sources are UNIONED, not tried in turn. Taking the
// first non-empty source would let a process hide its own descriptor table:
// mapping a single approved jar into its address space (one FileChannel.map call
// from inside the JVM) makes maps non-empty, so an extra jar held open via fd
// would never be scanned. Since the aggregate set digest is what actually pins a
// workload, that would reinstate the very extra-code hole jar_set_sha256 exists
// to close — the process would publish exactly the approved selector set while
// running attacker code.
//
// cmdline stays a true fallback: it is consulted only when the kernel offers
// nothing, and it is reported as its own source so the caller can degrade the
// selectors.
//
// Results are sorted by path so the aggregate set digest depends only on WHICH
// jars are present, not on the order the kernel happened to report them in
// (address order in maps, descriptor-allocation order in the fd table). The
// deployment step has to reproduce that digest offline, so it must be canonical.
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
		// summariseSources must see the RAW mapped/opened lists, not the
		// post-merge entries: mergeByPath dedups by path and lets the fd entry
		// win when a jar is both mapped and held open (see its comment), which
		// overwrites that entry's Source to SourceFD. Summarising from the
		// merged list would then read as "fd only" for a jar the kernel also
		// reported in maps, hiding the maps+fd signal this selector exists to
		// surface.
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

// mergeByPath unions the kernel sources, keeping one entry per jar path.
//
// A jar that is both mapped and held open appears in both lists. The fd entry
// wins because its handle is always readable, whereas map_files needs
// CONFIG_CHECKPOINT_RESTORE and falls back to pathname resolution when absent.
// Deduplicating on the path (rather than the inode) also keeps the set digest
// well formed: it is built from one "<path>:<hash>" line per entry.
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

// hashEntry opens one jar, derives its cache identity from the open descriptor
// and returns its SHA-256. Opening once and calling fstat on that descriptor —
// rather than stat'ing a path and then opening it — removes the window in which
// the file could be swapped between the two calls.
func (c *JarHashChecker) hashEntry(ctx *AttestationContext, entry procfs.MapsEntry) (hashResult, error) {
	result := hashResult{inodeMatches: true}

	readPath := entry.KernelPath
	if readPath != "" {
		// map_files is only present with CONFIG_CHECKPOINT_RESTORE; degrade to the
		// namespace path and record the weaker guarantee instead of failing.
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

	// Inode 0 means the source recorded no kernel inode (cmdline fallback), so
	// there is nothing to compare against. A genuine mismatch means the file
	// behind the path changed after the JVM took it — typical for an OverlayFS
	// copy-up, but also what a swap would look like.
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
