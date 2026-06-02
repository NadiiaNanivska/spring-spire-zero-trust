package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func setupCleanProcFS(t *testing.T, procRoot string, inode uint64, jarPath string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(procRoot, 0755))

	require.NoError(t, os.WriteFile(
		filepath.Join(procRoot, "status"),
		[]byte("Name: java\nState: S (sleeping)\nTgid: 4321\nPid: 4321\nTracerPid: 0\n"),
		0644,
	))

	require.NoError(t, os.WriteFile(filepath.Join(procRoot, "cmdline"), []byte("java\x00-jar\x00/app/payments-service.jar\x00"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(procRoot, "environ"), []byte("PATH=/usr/bin\x00HOME=/root\x00"), 0644))

	createFakeMapsFile(t, procRoot, inode, jarPath, 0, "")
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
