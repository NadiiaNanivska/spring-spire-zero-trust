package hashsource

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/sync/singleflight"
)

type ArtifactorySource struct {
	baseURL string
	apiKey  string
	client  *http.Client
	sfGroup singleflight.Group
}

type ArtifactoryResponse struct {
	Checksums struct {
		Sha256 string `json:"sha256"`
	} `json:"checksums"`
}

func NewArtifactorySource(baseURL, apiKey string) *ArtifactorySource {
	return &ArtifactorySource{
		baseURL: baseURL,
		apiKey:  apiKey,
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

func (a *ArtifactorySource) GetExpectedHash(ctx context.Context, jarPath string) (string, error) {
	v, err, _ := a.sfGroup.Do(jarPath, func() (interface{}, error) {
		return a.fetchFromArtifactory(ctx, jarPath)
	})

	if err != nil {
		return "", err
	}
	return v.(string), nil
}

func (a *ArtifactorySource) fetchFromArtifactory(ctx context.Context, jarPath string) (string, error) {
	url := fmt.Sprintf("%s/api/storage/%s", a.baseURL, jarPath)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("X-JFrog-Art-Api", a.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("artifactory request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", fmt.Errorf("artifact not found in artifactory: %s", jarPath)
	} else if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("artifactory returned unexpected status: %d", resp.StatusCode)
	}

	var artResp ArtifactoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&artResp); err != nil {
		return "", fmt.Errorf("failed to decode artifactory response: %w", err)
	}

	if artResp.Checksums.Sha256 == "" {
		return "", fmt.Errorf("artifactory response does not contain sha256 checksum")
	}

	return artResp.Checksums.Sha256, nil
}

func (a *ArtifactorySource) Close() error {
	a.client.CloseIdleConnections()
	return nil
}
