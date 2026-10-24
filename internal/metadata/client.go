// SPDX-License-Identifier: GPL-3.0-only

package metadata

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Evernight275/cubicstart/internal/config"
	"github.com/Evernight275/cubicstart/internal/core"
)

type client struct {
	http        *http.Client
	manifestURL string
}

func newClient(httpClient *http.Client, manifestURL string) *client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	if manifestURL == "" {
		manifestURL = DefaultManifestURL
	}
	return &client{httpClient, manifestURL}
}

func (c *client) fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "CubicStart/0.1.0")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch metadata: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("metadata server returned HTTP %d", resp.StatusCode)
	}
	const limit = 16 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if len(data) > limit {
		return nil, fmt.Errorf("metadata exceeds 16 MiB")
	}
	return data, nil
}

func (c *client) Manifest(ctx context.Context) (core.VersionManifest, error) {
	data, err := c.fetch(ctx, c.manifestURL)
	if err != nil {
		return core.VersionManifest{}, err
	}
	var result core.VersionManifest
	if err := json.Unmarshal(data, &result); err != nil {
		return result, fmt.Errorf("decode manifest: %w", err)
	}
	if len(result.Versions) == 0 {
		return result, fmt.Errorf("version manifest is empty")
	}
	return result, nil
}

func (c *client) Version(ctx context.Context, summary core.VersionSummary) (core.GameVersion, []byte, error) {
	data, err := c.fetch(ctx, summary.URL)
	if err != nil {
		return core.GameVersion{}, nil, err
	}
	hash := sha1.Sum(data)
	if summary.SHA1 == "" || hex.EncodeToString(hash[:]) != summary.SHA1 {
		return core.GameVersion{}, nil, fmt.Errorf("version metadata SHA-1 mismatch")
	}
	version, err := parseVersion(data)
	if err != nil {
		return version, nil, err
	}
	if version.ID != summary.ID {
		return version, nil, fmt.Errorf("requested version %q, received %q", summary.ID, version.ID)
	}
	return version, data, nil
}

func parseVersion(data []byte) (core.GameVersion, error) {
	var version core.GameVersion
	if err := json.Unmarshal(data, &version); err != nil {
		return version, fmt.Errorf("decode version: %w", err)
	}
	if err := config.ValidID(version.ID); err != nil {
		return version, err
	}
	if version.MainClass == "" || version.Downloads["client"].URL == "" || version.AssetIndex.ID == "" || version.AssetIndex.URL == "" {
		return version, fmt.Errorf("version %s has incomplete client metadata", version.ID)
	}
	if len(version.Arguments.JVM) == 0 || len(version.Arguments.Game) == 0 {
		return version, fmt.Errorf("version %s uses legacy launch arguments; v0.1 supports modern vanilla versions", version.ID)
	}
	if version.JavaVersion.Major == 0 {
		return version, fmt.Errorf("version %s does not specify its Java requirement", version.ID)
	}
	return version, nil
}
