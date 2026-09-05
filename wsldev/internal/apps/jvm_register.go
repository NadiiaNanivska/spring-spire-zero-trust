package apps

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sort"

	"wsldev/internal/spire"
)

// RegisterJVMWorkloads is the CI/CD step that turns the expected jar hashes in
// spiffe-spire/base/jvm-hashes-configmap.yaml into SPIRE registration entries.
//
// This is where the jar-integrity policy now lives. The jvm-attestor plugin only
// COMPUTES the running jar's SHA-256 and emits it as a jvm:jar_sha256=<hash>
// selector; it performs no reference comparison. By pinning the expected hash in
// the registration entry, the SPIRE server refuses to issue an SVID unless the
// workload's computed hash matches — no Artifactory/manifest lookup on the
// attestation hot path, and the trust anchor is the entry created with server
// credentials rather than a file readable on the node.
//
// Each JVM workload gets an entry requiring the full clean-attestation selector
// set:
//
//	k8s:ns:<namespace>               k8s:sa:<serviceAccount>
//	jvm:debug_clean=true             jvm:agent_flags_clean=true
//	jvm:maps_verified=true           jvm:hash_via_kernel_handle=true
//	jvm:jar_sha256=<expected>        jvm:jar_set_sha256=<digest of the whole set>
//
// Non-JVM args are ignored. Registration is idempotent: existing entries for the
// SPIFFE ID are deleted first, so re-deploying refreshes the pinned hash.
func RegisterJVMWorkloads(deployed []string) error {
	targets := make([]string, 0, len(deployed))
	for _, name := range deployed {
		if _, ok := jvmServices[name]; ok {
			targets = append(targets, name)
		}
	}
	if len(targets) == 0 {
		return nil
	}

	root, err := resolveRepoRoot()
	if err != nil {
		return err
	}
	manifestPath := filepath.Join(root, filepath.FromSlash(jvmHashesManifestRel))

	jars, err := readExistingHashes(manifestPath)
	if err != nil {
		return fmt.Errorf("read jvm hashes: %w", err)
	}

	parentID, err := spire.GetAgentParentID()
	if err != nil {
		return err
	}

	for _, name := range targets {
		svc := jvmServices[name]

		hash := jars[svc.manifestKey]
		if hash == "" {
			return fmt.Errorf("no hash for %s in %s; run the deploy so SyncJVMHashes populates it first",
				svc.manifestKey, jvmHashesManifestRel)
		}

		selectors := jvmEntrySelectors(svc, hash)

		if err := spire.EntryDeleteBySpiffeID(svc.spiffeID); err != nil {
			return fmt.Errorf("clear existing entry for %s: %w", svc.spiffeID, err)
		}
		if err := spire.EntryCreateWithSelectors(svc.spiffeID, parentID, selectors); err != nil {
			return fmt.Errorf("register %s: %w", svc.spiffeID, err)
		}

		fmt.Printf("registered %s (parent=%s) jar_sha256=%s...\n", svc.spiffeID, parentID, hash[:16])
	}

	return nil
}

// JVMServiceNames returns the deploy-arg names of all known JVM workloads, used
// as the default target set when re-running registration standalone.
func JVMServiceNames() []string {
	names := make([]string, 0, len(jvmServices))
	for name := range jvmServices {
		names = append(names, name)
	}
	return names
}

// jvmEntrySelectors builds the required selector set for a JVM workload entry.
// Values use the plugin's "key=value" form (e.g. jvm:jar_sha256=<hash>), which is
// exactly what the jvm-attestor emits, so the SPIRE server can match them.
//
// jar_set_sha256 is what actually pins the workload. SPIRE matches an entry when
// its selectors are a SUBSET of the workload's, so pinning only jar_sha256 leaves
// a hole: a process that additionally opens an attacker-supplied jar still carries
// the approved per-jar selector and would match. The set digest covers every jar
// the process holds, so any extra one breaks the match.
//
// hash_via_kernel_handle=true requires that the agent read those jars through a
// /proc handle bound to the inode rather than by resolving their pathname. That is
// the property that makes the symlink-redirection defense enforceable instead of
// merely observable.
func jvmEntrySelectors(svc jvmService, hash string) []string {
	return []string{
		fmt.Sprintf("k8s:ns:%s", spireNamespace),
		fmt.Sprintf("k8s:sa:%s", svc.serviceAccount),
		"jvm:debug_clean=true",
		"jvm:agent_flags_clean=true",
		"jvm:maps_verified=true",
		"jvm:hash_via_kernel_handle=true",
		fmt.Sprintf("jvm:jar_sha256=%s", hash),
		fmt.Sprintf("jvm:jar_set_sha256=%s", jarSetDigest(map[string]string{svc.manifestKey: hash})),
	}
}

// jarSetDigest reproduces, offline, the aggregate digest the jvm-attestor computes
// at attestation time: SHA-256 over "<path>:<sha256>\n" lines for every discovered
// jar, ordered by path.
//
// This MUST stay byte-for-byte identical to the plugin's implementation in
// spire-jvm-attestor/internal/checkers/jarhash.go — the two live in separate Go
// modules, so the compiler cannot catch a drift; TestJarSetDigest pins the format.
func jarSetDigest(jars map[string]string) string {
	paths := make([]string, 0, len(jars))
	for path := range jars {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	digest := sha256.New()
	for _, path := range paths {
		fmt.Fprintf(digest, "%s:%s\n", path, jars[path])
	}
	return hex.EncodeToString(digest.Sum(nil))
}
