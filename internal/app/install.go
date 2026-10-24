// SPDX-License-Identifier: GPL-3.0-only

package app

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

type installer struct {
	paths     config.Paths
	metadata  metadata.Catalog
	downloads download.Downloader
}

func newInstaller(options Options) (*installer, error) {
	paths, err := config.Resolve(options.Root)
	if err != nil {
		return nil, err
	}
	return &installer{paths, metadata.NewCatalog(options.HTTPClient, options.ManifestURL), download.NewDownloader(options.HTTPClient, options.Workers)}, nil
}

func install(ctx context.Context, options Options, id string, progress func(download.Progress)) (core.GameVersion, error) {
	installer, err := newInstaller(options)
	if err != nil {
		return core.GameVersion{}, err
	}
	return installer.install(ctx, id, progress)
}

func (a *installer) install(ctx context.Context, id string, progress func(download.Progress)) (core.GameVersion, error) {
	if err := config.ValidID(id); err != nil {
		return core.GameVersion{}, err
	}
	manifest, err := a.metadata.Manifest(ctx)
	if err != nil {
		return core.GameVersion{}, err
	}
	if id == "latest" {
		id = manifest.Latest.Release
	}
	var selected *core.VersionSummary
	for i := range manifest.Versions {
		if manifest.Versions[i].ID == id {
			selected = &manifest.Versions[i]
			break
		}
	}
	if selected == nil {
		return core.GameVersion{}, fmt.Errorf("unknown Minecraft version %q", id)
	}
	version, raw, err := a.metadata.Version(ctx, *selected)
	if err != nil {
		return version, err
	}
	if err := config.ValidID(version.AssetIndex.ID); err != nil {
		return version, err
	}
	versionDir := filepath.Join(a.paths.Versions, id)
	indexPath := filepath.Join(a.paths.Assets, "indexes", version.AssetIndex.ID+".json")
	if err := a.downloads.Fetch(ctx, download.File{URL: version.AssetIndex.URL, Path: indexPath, SHA1: version.AssetIndex.SHA1, Size: version.AssetIndex.Size}); err != nil {
		return version, fmt.Errorf("install asset index: %w", err)
	}
	indexData, err := os.ReadFile(indexPath)
	if err != nil {
		return version, err
	}
	var index core.AssetIndex
	if err := json.Unmarshal(indexData, &index); err != nil {
		return version, fmt.Errorf("decode asset index: %w", err)
	}
	if index.Objects == nil {
		return version, fmt.Errorf("asset index has no objects")
	}
	if index.Virtual || index.MapToResources {
		return version, fmt.Errorf("legacy asset layouts are not supported by v0.1")
	}
	client := version.Downloads["client"]
	files := []download.File{{URL: client.URL, Path: filepath.Join(versionDir, id+".jar"), SHA1: client.SHA1, Size: client.Size}}
	platform := metadata.CurrentPlatform()
	for _, library := range version.Libraries {
		artifacts, err := metadata.LibraryFiles(library, platform)
		if err != nil {
			return version, err
		}
		for _, artifact := range artifacts {
			path, err := config.Join(a.paths.Libraries, artifact.Path)
			if err != nil {
				return version, err
			}
			files = append(files, download.File{URL: artifact.URL, Path: path, SHA1: artifact.SHA1, Size: artifact.Size})
		}
	}
	for _, object := range index.Objects {
		if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(object.Hash) {
			return version, fmt.Errorf("invalid asset hash")
		}
		relative := object.Hash[:2] + "/" + object.Hash
		files = append(files, download.File{URL: "https://resources.download.minecraft.net/" + relative, Path: filepath.Join(a.paths.Assets, "objects", filepath.FromSlash(relative)), SHA1: object.Hash, Size: object.Size})
	}
	if log, ok := version.Logging["client"]; ok {
		path, err := config.Join(filepath.Join(a.paths.Assets, "log_configs"), log.File.ID)
		if err != nil {
			return version, err
		}
		files = append(files, download.File{URL: log.File.URL, Path: path, SHA1: log.File.SHA1, Size: log.File.Size})
	}
	if err := a.downloads.FetchAll(ctx, files, progress); err != nil {
		return version, err
	}
	// Publishing the version JSON last makes it the installation completion marker.
	if err := writeVersion(filepath.Join(versionDir, id+".json"), raw); err != nil {
		return version, err
	}
	return version, nil
}

func writeVersion(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".metadata-*.part")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
