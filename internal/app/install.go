// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/Evernight275/cubicstart/internal/config"
	"github.com/Evernight275/cubicstart/internal/core"
	"github.com/Evernight275/cubicstart/internal/download"
	"github.com/Evernight275/cubicstart/internal/installation"
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

func install(ctx context.Context, options Options, id string, progress func(download.Progress)) (core.Installation, error) {
	installer, err := newInstaller(options)
	if err != nil {
		return core.Installation{}, err
	}
	return installer.install(ctx, id, progress)
}

func (a *installer) install(ctx context.Context, id string, progress func(download.Progress)) (core.Installation, error) {
	if err := config.ValidID(id); err != nil {
		return core.Installation{}, err
	}
	manifest, err := a.metadata.Manifest(ctx)
	if err != nil {
		return core.Installation{}, err
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
		return core.Installation{}, fmt.Errorf("unknown Minecraft version %q", id)
	}
	version, raw, err := a.metadata.Version(ctx, *selected)
	if err != nil {
		return core.Installation{}, err
	}
	if err := config.ValidID(version.AssetIndex.ID); err != nil {
		return core.Installation{}, err
	}
	installed := core.Installation{Version: version, Directory: filepath.Join(a.paths.Versions, id), Platform: metadata.CurrentPlatform()}
	indexPath := filepath.Join(a.paths.Assets, "indexes", version.AssetIndex.ID+".json")
	if err := a.downloads.Fetch(ctx, download.File{URL: version.AssetIndex.URL, Path: indexPath, SHA1: version.AssetIndex.SHA1, Size: version.AssetIndex.Size}); err != nil {
		return core.Installation{}, fmt.Errorf("install asset index: %w", err)
	}
	files, err := installation.Files(ctx, a.paths, installed)
	if err != nil {
		return core.Installation{}, err
	}
	if err := a.downloads.FetchAll(ctx, files, progress); err != nil {
		return core.Installation{}, err
	}
	// Publishing the version JSON last makes it the installation completion marker.
	if err := installation.Publish(installed, raw); err != nil {
		return core.Installation{}, err
	}
	return installed, nil
}
