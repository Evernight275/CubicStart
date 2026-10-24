// SPDX-License-Identifier: GPL-3.0-only

package installation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/Evernight275/cubicstart/internal/config"
	"github.com/Evernight275/cubicstart/internal/core"
	"github.com/Evernight275/cubicstart/internal/download"
	"github.com/Evernight275/cubicstart/internal/metadata"
)

var assetHashPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

func requiredFiles(ctx context.Context, paths config.Paths, installed core.Installation) ([]download.File, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	version := installed.Version
	if err := config.ValidID(version.ID); err != nil {
		return nil, err
	}
	if filepath.Clean(installed.Directory) != filepath.Clean(filepath.Join(paths.Versions, version.ID)) {
		return nil, fmt.Errorf("installation directory does not match its version")
	}
	if err := config.ValidID(version.AssetIndex.ID); err != nil {
		return nil, err
	}
	if installed.Platform.OS == "" || installed.Platform.Arch == "" {
		return nil, fmt.Errorf("installation target OS and architecture are required")
	}
	indexFile := download.File{URL: version.AssetIndex.URL, SHA1: version.AssetIndex.SHA1, Size: version.AssetIndex.Size,
		Path: filepath.Join(paths.Assets, "indexes", version.AssetIndex.ID+".json")}
	if err := download.Verify(ctx, indexFile); err != nil {
		return nil, fmt.Errorf("verify asset index: %w", err)
	}
	data, err := os.ReadFile(indexFile.Path)
	if err != nil {
		return nil, err
	}
	var index core.AssetIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("decode asset index: %w", err)
	}
	if index.Objects == nil {
		return nil, fmt.Errorf("asset index has no objects")
	}
	if index.Virtual || index.MapToResources {
		return nil, fmt.Errorf("legacy asset layouts are not supported by v0.1")
	}
	client := version.Downloads["client"]
	result := []download.File{indexFile, {URL: client.URL, Path: filepath.Join(installed.Directory, version.ID+".jar"), SHA1: client.SHA1, Size: client.Size}}
	for _, library := range version.Libraries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		artifacts, err := metadata.LibraryFiles(library, installed.Platform)
		if err != nil {
			return nil, err
		}
		for _, artifact := range artifacts {
			path, err := config.Join(paths.Libraries, artifact.Path)
			if err != nil {
				return nil, err
			}
			result = append(result, download.File{URL: artifact.URL, Path: path, SHA1: artifact.SHA1, Size: artifact.Size})
		}
	}
	for _, object := range index.Objects {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !assetHashPattern.MatchString(object.Hash) {
			return nil, fmt.Errorf("invalid asset hash")
		}
		relative := object.Hash[:2] + "/" + object.Hash
		result = append(result, download.File{URL: "https://resources.download.minecraft.net/" + relative,
			Path: filepath.Join(paths.Assets, "objects", filepath.FromSlash(relative)), SHA1: object.Hash, Size: object.Size})
	}
	if log, ok := version.Logging["client"]; ok {
		path, err := config.Join(filepath.Join(paths.Assets, "log_configs"), log.File.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, download.File{URL: log.File.URL, Path: path, SHA1: log.File.SHA1, Size: log.File.Size})
	}
	return result, nil
}

func verify(ctx context.Context, paths config.Paths, installed core.Installation) error {
	files, err := requiredFiles(ctx, paths, installed)
	if err != nil {
		return fmt.Errorf("installation %s is incomplete; run cubicstart install %s: %w", installed.Version.ID, installed.Version.ID, err)
	}
	for _, file := range files {
		if err := download.Verify(ctx, file); err != nil {
			return fmt.Errorf("installation %s is incomplete; run cubicstart install %s: %w", installed.Version.ID, installed.Version.ID, err)
		}
	}
	return nil
}
