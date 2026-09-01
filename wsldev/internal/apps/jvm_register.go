package apps

import (
	"fmt"
	"path/filepath"

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
//	k8s:ns:<namespace>          k8s:sa:<serviceAccount>
//	jvm:debug_clean=true        jvm:agent_flags_clean=true
//	jvm:maps_verified=true      jvm:jar_sha256=<expected>
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
func jvmEntrySelectors(svc jvmService, hash string) []string {
	return []string{
		fmt.Sprintf("k8s:ns:%s", spireNamespace),
		fmt.Sprintf("k8s:sa:%s", svc.serviceAccount),
		"jvm:debug_clean=true",
		"jvm:agent_flags_clean=true",
		"jvm:maps_verified=true",
		fmt.Sprintf("jvm:jar_sha256=%s", hash),
	}
}
