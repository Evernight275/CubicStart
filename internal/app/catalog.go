// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"context"
	"fmt"
	"sort"

	"github.com/Evernight275/cubicstart/internal/core"
	"github.com/Evernight275/cubicstart/internal/metadata"
)

func versions(ctx context.Context, catalog metadata.Catalog, kind string) ([]core.VersionSummary, error) {
	if kind != "all" && kind != "release" && kind != "snapshot" {
		return nil, fmt.Errorf("version type must be release, snapshot or all")
	}
	manifest, err := catalog.Manifest(ctx)
	if err != nil {
		return nil, err
	}
	var versions []core.VersionSummary
	for _, version := range manifest.Versions {
		if kind == "all" || version.Type == kind {
			versions = append(versions, version)
		}
	}
	sort.SliceStable(versions, func(i, j int) bool { return versions[i].ReleaseTime.After(versions[j].ReleaseTime) })
	return versions, nil
}
