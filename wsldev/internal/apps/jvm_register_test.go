package apps

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestJarSetDigest_MatchesPluginWireFormat(t *testing.T) {
	const wire = "/app/a.jar:aaa\n/app/b.jar:bbb\n"
	sum := sha256.Sum256([]byte(wire))
	want := hex.EncodeToString(sum[:])

	got := jarSetDigest(map[string]string{
		"/app/b.jar": "bbb",
		"/app/a.jar": "aaa",
	})

	if got != want {
		t.Errorf("digest %s does not match the pinned wire format %q (want %s)", got, wire, want)
	}
}

func TestJvmEntrySelectors_PinsSetDigest(t *testing.T) {
	svc := jvmService{
		serviceAccount: "payments-sa",
		manifestKey:    "/app/payments-service.jar",
	}
	const hash = "d1e2f3"

	sum := sha256.Sum256([]byte("/app/payments-service.jar:" + hash + "\n"))
	wantSet := "jvm:jar_set_sha256=" + hex.EncodeToString(sum[:])

	selectors := jvmEntrySelectors(svc, hash)

	for _, want := range []string{
		"jvm:debug_clean=true",
		"jvm:agent_flags_clean=true",
		"jvm:maps_verified=true",
		"jvm:hash_via_kernel_handle=true",
		"jvm:jar_sha256=" + hash,
		wantSet,
	} {
		if !contains(selectors, want) {
			t.Errorf("missing selector %s in %v", want, selectors)
		}
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
