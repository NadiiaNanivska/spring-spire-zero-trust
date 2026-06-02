package checkers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourorg/spire-jvm-attestor/internal/cache"
)

// MockHashSource реалізує інтерфейс HashSource для тестування
type MockHashSource struct {
	expectedHashes map[string]string
}

func (m *MockHashSource) GetExpectedHash(ctx context.Context, jarPath string) (string, error) {
	hash, exists := m.expectedHashes[jarPath]
	if !exists {
		return "", fmt.Errorf("artifact not found in mock source: %s", jarPath)
	}
	return hash, nil
}

func TestJarHashChecker_Check(t *testing.T) {
	// Створюємо тимчасову директорію під фейковий /proc
	tmpDir, err := os.MkdirTemp("", "jarhash-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	procRoot := filepath.Join(tmpDir, "proc", "1234")
	nsRoot := filepath.Join(procRoot, "root")
	appDir := filepath.Join(nsRoot, "app")
	require.NoError(t, os.MkdirAll(appDir, 0755))

	// 1. Створюємо реальні файли JAR на фейковому диску для підрахунку хешів
	jar1Path := filepath.Join(appDir, "service.jar")
	jar2Path := filepath.Join(appDir, "lib-core.jar")
	jar1NsPath := "/app/service.jar"
	jar2NsPath := "/app/lib-core.jar"

	require.NoError(t, os.WriteFile(jar1Path, []byte("fake-jar-1-content"), 0644))
	require.NoError(t, os.WriteFile(jar2Path, []byte("fake-jar-2-content"), 0644))

	// Рахуємо їхні справжні SHA-256
	hash1 := computeRawSHA256([]byte("fake-jar-1-content"))
	hash2 := computeRawSHA256([]byte("fake-jar-2-content"))

	// Отримуємо реальні inode з диску
	stat1, err := os.Stat(jar1Path)
	require.NoError(t, err)
	ino1 := getInode(stat1)

	stat2, err := os.Stat(jar2Path)
	require.NoError(t, err)
	ino2 := getInode(stat2)

	// Налаштовуємо MockHashSource з еталонними хешами
	mockSource := &MockHashSource{
		expectedHashes: map[string]string{
			"/app/service.jar":  hash1,
			"/app/lib-core.jar": hash2,
		},
	}

	checker := NewJarHashChecker(mockSource)
	hashCache := cache.NewHashCache()

	t.Run("Success: Multi-JAR verification (Fix Bug 8)", func(t *testing.T) {
		ctx := &AttestationContext{
			Context:   context.Background(),
			PID:       1234,
			ProcRoot:  procRoot,
			HashCache: hashCache,
		}

		// Підсовуємо entries в контекст через кастомну логіку (або фейковий maps файл, залежно від твоєї реалізації parseJarPathsFromMaps)
		// Для чистоти тесту передамо jarEntries безпосередньо в логіку, якщо Check() адаптовано під ін'єкцію,
		// або створимо реальний файл maps у procRoot:
		createFakeMapsFile(t, procRoot, ino1, jar1NsPath, ino2, jar2NsPath)

		selectors, err := checker.Check(ctx)
		assert.NoError(t, err)

		// Перевіряємо, що повернулися хеші для ОБОХ файлів (Баг №8 успішно закрито)
		assert.Contains(t, selectors, SelectorJarSha256Prefix+hash1)
		assert.Contains(t, selectors, SelectorJarSha256Prefix+hash2)
		assert.Contains(t, selectors, SelectorMapsVerified)
		assert.Contains(t, selectors, SelectorInodeConsistentTrue)
	})

	t.Run("Success: Spring Boot Fat-JAR support (Fix Bug 9)", func(t *testing.T) {
		ctx := &AttestationContext{
			Context:   context.Background(),
			PID:       1234,
			ProcRoot:  procRoot,
			HashCache: cache.NewHashCache(), // чистий кеш
		}

		// Для Spring Boot inode в карті пам'яті буде 0
		// Очистимо maps і запишемо туди cmdline структуру або 0 inode
		createFakeMapsFile(t, procRoot, 0, jar1NsPath, 0, "")

		selectors, err := checker.Check(ctx)
		assert.NoError(t, err)
		assert.Contains(t, selectors, SelectorJarSha256Prefix+hash1)
		assert.Contains(t, selectors, SelectorInodeConsistentTrue) // Оскільки збігається (0 == 0 або ігнорується)
	})

	t.Run("Success: OverlayFS Copy-Up Adaptation (Fix Issue 4)", func(t *testing.T) {
		ctx := &AttestationContext{
			Context:   context.Background(),
			PID:       1234,
			ProcRoot:  procRoot,
			HashCache: hashCache,
		}

		// Симулюємо ситуацію Copy-Up: в maps записано старий inode (наприклад, 99999),
		// але на диску реальний файл має свій поточний ino1.
		// Плагін НЕ повинен кидати hard-fail. Він має вирахувати хеш і дати soft-selector.
		createFakeMapsFile(t, procRoot, 99999, jar1NsPath, 0, "")

		selectors, err := checker.Check(ctx)
		assert.NoError(t, err)
		assert.Contains(t, selectors, SelectorJarSha256Prefix+hash1)
		// Перевіряємо, що зафіксовано нестабільність inode, але SVID видано!
		assert.Contains(t, selectors, SelectorInodeConsistentFalse)
	})

	t.Run("Crypto Hard-Fail: Modified JAR file", func(t *testing.T) {
		ctx := &AttestationContext{
			Context:   context.Background(),
			PID:       1234,
			ProcRoot:  procRoot,
			HashCache: cache.NewHashCache(),
		}

		// Підміняємо файл шкідливим контентом
		require.NoError(t, os.WriteFile(jar1Path, []byte("MALICIOUS_BYTECODE_INJECTED"), 0644))
		statMod, err := os.Stat(jar1Path)
		require.NoError(t, err)

		createFakeMapsFile(t, procRoot, getInode(statMod), jar1NsPath, 0, "")

		// Має спрацювати криптографічний Hard-fail Рівня 3
		selectors, err := checker.Check(ctx)
		assert.Error(t, err)
		assert.Nil(t, selectors)
		assert.Contains(t, err.Error(), "jar crypto integrity mismatch")
	})
}
