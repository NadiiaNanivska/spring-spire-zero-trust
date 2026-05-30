package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"github.com/yourorg/spire-jvm-attestor/internal/hashsource"
)

// Створюємо глобальний пул буферів для оптимізації GC при обчисленні SHA-256 (Ішка №7)
var bufferPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 64*1024) // 64KB буфер для io.CopyBuffer
		return &b
	},
}

type JarHashChecker struct {
	hashSource hashsource.HashSource
}

func NewJarHashChecker(hashSource hashsource.HashSource) *JarHashChecker {
	return &JarHashChecker{hashSource: hashSource}
}

func (c *JarHashChecker) Name() string {
	return "jar-hash"
}

func (c *JarHashChecker) Check(ctx *AttestationContext) ([]string, error) {
	jarEntries, err := parseJarPathsFromMaps(ctx.ProcRoot)
	if err != nil {
		return nil, fmt.Errorf("maps parse error: %w", err)
	}

	if len(jarEntries) == 0 {
		jarEntries, err = extractJarsFromCmdline(ctx.ProcRoot)
		if err != nil {
			return nil, fmt.Errorf("jar cmdline fallback error: %w", err)
		}
		if len(jarEntries) == 0 {
			return nil, fmt.Errorf("no jar files found in maps or cmdline")
		}
	}

	var allSelectors []string

	// Флаг для відслідковування загального стану inode по всім jar-файлам додатка
	globalInodeConsistent := true

	// Виправляємо Баг №8: ітеруємося по ВСІХ jar-файлах, прибираємо ранній return
	for _, entry := range jarEntries {
		nsPath := filepath.Join(ctx.ProcRoot, "root", entry.path)

		diskInfo, err := os.Stat(nsPath)
		if err != nil {
			return nil, fmt.Errorf("cannot stat jar at %s: %w", nsPath, err)
		}

		diskStat, ok := diskInfo.Sys().(*syscall.Stat_t)
		if !ok {
			return nil, fmt.Errorf("cannot retrieve syscall.Stat_t for %s", nsPath)
		}

		var actualHash string
		
		// Виправляємо Баг №9 + Оптимізуємо OverlayFS (Ішка №4):
		// Якщо entry.inode == 0 (Spring Boot), або diskStat.Ino != entry.inode (OverlayFS Copy-Up / bait-and-switch)
		// Ми НЕ перериваємо роботу hard-fail помилкою. Ми інвалідуємо швидкий шлях кешу,
		// вираховуємо хеш напряму з диску і даємо криптографічну гарантію.
		if entry.inode == 0 || diskStat.Ino != entry.inode {
			if entry.inode != 0 {
				// Якщо inode в maps був, але на диску змінився — фіксуємо асиметрію метаданих
				globalInodeConsistent = false
			}

			// Примусовий перерахунок SHA-256 (минаючи GetOrCompute, якщо inode нестабільний)
			computedHash, err := c.calculateFileSHA256(nsPath)
			if err != nil {
				return nil, fmt.Errorf("forced hash computation failed for %s: %w", nsPath, err)
			}
			actualHash = computedHash
		} else {
			// Якщо inode збігається — використовуємо твій стандартний швидкий кеш
			hash, err := ctx.HashCache.GetOrCompute(nsPath)
			if err != nil {
				return nil, fmt.Errorf("hash computation failed for %s: %w", nsPath, err)
			}
			actualHash = hash
		}

		// Запитуємо очікуваний хеш через новий інтерфейс джерела (Локальний чи Artifactory)
		expected, err := c.hashSource.GetExpectedHash(ctx.Context, entry.path)
		if err != nil {
			return nil, fmt.Errorf("integrity configuration error for %s: %w", entry.path, err)
		}

		// Криптографічний hard-fail (Рівень 3). Тут компромісів немає.
		if actualHash != expected {
			return nil, fmt.Errorf(
				"jar crypto integrity mismatch for %s: computed=%s expected=%s",
				entry.path, actualHash, expected,
			)
		}

		// Додаємо селектор хешу конкретного перевіреного JAR файлу додатка
		allSelectors = append(allSelectors, SelectorJarSha256Prefix+actualHash)
	}

	// Коли ВСІ файли успішно пройшли перевірку, формуємо фінальні селектори стану
	allSelectors = append(allSelectors, SelectorMapsVerified)
	
	if globalInodeConsistent {
		allSelectors = append(allSelectors, SelectorInodeConsistentTrue)
	} else {
		allSelectors = append(allSelectors, SelectorInodeConsistentFalse)
	}

	return allSelectors, nil
}

// Оптимізована функція підрахунку хешу з використанням sync.Pool буферів
func (c *JarHashChecker) calculateFileSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	hasher := sha256.New()
	bufPtr := bufferPool.Get().(*[]byte)
	defer bufferPool.Put(bufPtr) // Обов'язкове повернення в пул

	if _, err := io.CopyBuffer(hasher, f, *bufPtr); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}