package apps

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sort"

	"wsldev/internal/spire"
)

// RegisterJVMWorkloads pins expected hashes in SPIRE entries, which enforce integrity.
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

func JVMServiceNames() []string {
	names := make([]string, 0, len(jvmServices))
	for name := range jvmServices {
		names = append(names, name)
	}
	return names
}

// Pin the whole jar set: SPIRE subset matching would otherwise allow extra jars.
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

// jarSetDigest must match spire-jvm-attestor/internal/checkers/jarhash.go byte for byte.
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
