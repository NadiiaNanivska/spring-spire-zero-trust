package hashsource

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// manifestSchema чітко повторює структуру, яку генерує скрипт generate-manifest.sh.
// Версія парситься як int, тому помилка unmarshal number into string зникає.
type manifestSchema struct {
	Version int               `json:"version"`
	Jars    map[string]string `json:"jars"`
}

type LocalManifestSource struct {
	manifestPath string
	mu           sync.RWMutex
	cache        map[string]string
	isLoaded     bool
}

func NewLocalManifestSource(manifestPath string) *LocalManifestSource {
	return &LocalManifestSource{
		manifestPath: manifestPath,
		cache:        make(map[string]string),
	}
}

func (l *LocalManifestSource) GetExpectedHash(ctx context.Context, jarPath string) (string, error) {
	l.mu.RLock()
	if l.isLoaded {
		hash, exists := l.cache[jarPath]
		l.mu.RUnlock()
		if !exists {
			return "", fmt.Errorf("jar %s not found in local manifest", jarPath)
		}
		return hash, nil
	}
	l.mu.RUnlock()

	l.mu.Lock()
	defer l.mu.Unlock()
	
	// Double-checked locking
	if l.isLoaded {
		hash, exists := l.cache[jarPath]
		if !exists {
			return "", fmt.Errorf("jar %s not found in local manifest", jarPath)
		}
		return hash, nil
	}

	data, err := os.ReadFile(l.manifestPath)
	if err != nil {
		return "", fmt.Errorf("failed to read manifest file: %w", err)
	}

	var schema manifestSchema
	if err := json.Unmarshal(data, &schema); err != nil {
		return "", fmt.Errorf("failed to parse manifest json: %w", err)
	}

	// Зберігаємо безпосередньо внутрішню мапу хешів
	l.cache = schema.Jars
	l.isLoaded = true

	hash, exists := l.cache[jarPath]
	if !exists {
		return "", fmt.Errorf("jar %s not found in local manifest", jarPath)
	}
	return hash, nil
}