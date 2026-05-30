package internal

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

// setupCleanProcFS розгортає базову валідну структуру процесу в procFS для інтеграційних тестів конвеєра.
func setupCleanProcFS(t *testing.T, procRoot string, inode uint64, jarPath string) {
	require.NoError(t, os.MkdirAll(procRoot, 0755))

	// Рівень 1: Чистий status (без дебагера)
	require.NoError(t, os.WriteFile(
		filepath.Join(procRoot, "status"),
		[]byte("Name: java\nState: S (sleeping)\nTgid: 4321\nPid: 4321\nTracerPid: 0\n"),
		0644,
	))

	// Рівень 2: Чистий cmdline та environ (без небезпечних прапорців)
	require.NoError(t, os.WriteFile(filepath.Join(procRoot, "cmdline"), []byte("java\x00-jar\x00/app/payments-service.jar\x00"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(procRoot, "environ"), []byte("PATH=/usr/bin\x00HOME=/root\x00"), 0644))

	// Рівень 3: Карта пам'яті maps
	createFakeMapsFile(t, procRoot, inode, jarPath, 0, "")
}

// computeRawSHA256 рахує SHA-256 від байтового масиву
func computeRawSHA256(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// getInode дістає системний inode файлу
func getInode(fi os.FileInfo) uint64 {
	stat, _ := fi.Sys().(*syscall.Stat_t)
	return stat.Ino
}

// createFakeMapsFile генерує реальний файл proc maps на фейковому диску
func createFakeMapsFile(t *testing.T, procRoot string, ino1 uint64, path1 string, ino2 uint64, path2 string) {
	mapsPath := filepath.Join(procRoot, "maps")
	require.NoError(t, os.MkdirAll(procRoot, 0755))

	var content string
	if path1 != "" {
		// Формат рядка proc maps: address perms offset dev inode path
		content += fmt.Sprintf("00400000-00452000 r-xp 00000000 08:02 %d %s\n", ino1, path1)
	}
	if path2 != "" {
		content += fmt.Sprintf("00600000-00620000 r-xp 00000000 08:02 %d %s\n", ino2, path2)
	}

	require.NoError(t, os.WriteFile(mapsPath, []byte(content), 0644))
}