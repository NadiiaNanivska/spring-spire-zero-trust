package checkers

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func containsSelector(selectors []string, target string) bool {
	for _, s := range selectors {
		if s == target {
			return true
		}
	}
	return false
}

func computeRawSHA256(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func getInode(fi os.FileInfo) uint64 {
	stat, _ := fi.Sys().(*syscall.Stat_t)
	return stat.Ino
}

func createFakeMapsFile(t *testing.T, procRoot string, ino1 uint64, path1 string, ino2 uint64, path2 string) {
	t.Helper()
	mapsPath := filepath.Join(procRoot, "maps")
	require.NoError(t, os.MkdirAll(procRoot, 0755))

	var content string
	if path1 != "" {
		content += fmt.Sprintf("00400000-00452000 r-xp 00000000 08:02 %d %s\n", ino1, path1)
	}
	if path2 != "" {
		content += fmt.Sprintf("00600000-00620000 r-xp 00000000 08:02 %d %s\n", ino2, path2)
	}

	require.NoError(t, os.WriteFile(mapsPath, []byte(content), 0644))
}
