// SPDX-License-Identifier: GPL-3.0-only

// Package core 定义启动器跨包共享的核心概念。
// 本包只依赖标准库，不包含网络、凭据存储或进程操作。
// 所有公开声明统一放在 interface.go 中。
package core

import "time"

// Player identifies a Minecraft Java player.
type Player struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Session is the player identity and authorization passed to the game.
// Offline sessions use UserType legacy and do not authenticate to online servers.
type Session struct {
	Player                                Player
	AccessToken, XUID, ClientID, UserType string
	ExpiresAt                             time.Time
}

type JavaRuntime struct {
	Path, Version, Arch, OSVersion string
	Major                          int
}

type VersionManifest struct {
	Latest struct {
		Release  string `json:"release"`
		Snapshot string `json:"snapshot"`
	} `json:"latest"`
	Versions []VersionSummary `json:"versions"`
}

type VersionSummary struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	URL         string    `json:"url"`
	SHA1        string    `json:"sha1"`
	ReleaseTime time.Time `json:"releaseTime"`
}

// Artifact describes a downloadable game file, including its integrity metadata.
type Artifact struct {
	ID   string `json:"id"`
	Path string `json:"path"`
	URL  string `json:"url"`
	SHA1 string `json:"sha1"`
	Size int64  `json:"size"`
}

type GameVersion struct {
	ID         string              `json:"id"`
	Type       string              `json:"type"`
	MainClass  string              `json:"mainClass"`
	Assets     string              `json:"assets"`
	Downloads  map[string]Artifact `json:"downloads"`
	AssetIndex Artifact            `json:"assetIndex"`
	Libraries  []Library           `json:"libraries"`
	Arguments  struct {
		JVM  []Argument `json:"jvm"`
		Game []Argument `json:"game"`
	} `json:"arguments"`
	JavaVersion struct {
		Major int `json:"majorVersion"`
	} `json:"javaVersion"`
	Logging map[string]LogConfig `json:"logging"`
}

type LogConfig struct {
	Argument string   `json:"argument"`
	File     Artifact `json:"file"`
}

type Library struct {
	Name      string `json:"name"`
	Downloads struct {
		Artifact    *Artifact           `json:"artifact"`
		Classifiers map[string]Artifact `json:"classifiers"`
	} `json:"downloads"`
	Natives map[string]string `json:"natives"`
	Extract struct {
		Exclude []string `json:"exclude"`
	} `json:"extract"`
	Rules []Rule `json:"rules"`
}

type Rule struct {
	Action   string          `json:"action"`
	OS       *OSRule         `json:"os"`
	Features map[string]bool `json:"features"`
}

type OSRule struct {
	Name    string `json:"name"`
	Arch    string `json:"arch"`
	Version string `json:"version"`
}

type Platform struct {
	OS, Arch, Version string
	Features          map[string]bool
}

// Argument accepts both plain strings and rule-guarded argument objects.
type Argument struct {
	Values []string
	Rules  []Rule
}

func (a *Argument) UnmarshalJSON(data []byte) error { return decodeArgument(data, a) }

type AssetIndex struct {
	Objects        map[string]AssetObject `json:"objects"`
	Virtual        bool                   `json:"virtual"`
	MapToResources bool                   `json:"map_to_resources"`
}

type AssetObject struct {
	Hash string `json:"hash"`
	Size int64  `json:"size"`
}
