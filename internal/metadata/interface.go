// SPDX-License-Identifier: GPL-3.0-only

// Package metadata 负责 Minecraft 版本列表、版本描述和资源索引的获取与解析，
// 并根据操作系统、架构及特性评估元数据中的条件规则。
// 本包的所有公开声明统一放在 interface.go 中。
package metadata

import (
	"context"
	"net/http"

	"github.com/Evernight275/cubicstart/internal/core"
)

const DefaultManifestURL = "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json"

// Catalog reads the official version list and individual version metadata.
type Catalog interface {
	Manifest(context.Context) (core.VersionManifest, error)
	Version(context.Context, core.VersionSummary) (core.GameVersion, []byte, error)
}

func NewCatalog(client *http.Client, manifestURL string) Catalog {
	return newClient(client, manifestURL)
}
func ParseVersion(data []byte) (core.GameVersion, error) { return parseVersion(data) }
func CurrentPlatform() core.Platform                     { return currentPlatform() }
func Allowed(rules []core.Rule, platform core.Platform) (bool, error) {
	return allowed(rules, platform)
}
func LibraryFiles(lib core.Library, platform core.Platform) ([]core.Artifact, error) {
	return libraryFiles(lib, platform)
}
