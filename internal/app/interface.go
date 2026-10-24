// SPDX-License-Identifier: GPL-3.0-only

// Package app 协调登录、安装和启动等应用用例。
// 它组织各职责包之间的调用，不依赖命令行交互。
// 本包的所有公开声明统一放在 interface.go 中。
package app

import (
	"context"
	"io"
	"net/http"

	"github.com/Evernight275/cubicstart/internal/auth"
	"github.com/Evernight275/cubicstart/internal/config"
	"github.com/Evernight275/cubicstart/internal/core"
	"github.com/Evernight275/cubicstart/internal/download"
	"github.com/Evernight275/cubicstart/internal/java"
	"github.com/Evernight275/cubicstart/internal/launch"
	"github.com/Evernight275/cubicstart/internal/metadata"
)

const Version = "0.1.0"
const DefaultClientID = auth.DefaultClientID

type Options struct {
	Root, ManifestURL string
	HTTPClient        *http.Client
	Workers           int
}
type LaunchOptions struct {
	Version, JavaPath, Username string
	Offline                     bool
	MemoryMB                    int
}

// DataPaths resolves the launcher's data directories.
func DataPaths(root string) (config.Paths, error) { return config.Resolve(root) }

// Versions lists and filters the official version catalog.
func Versions(ctx context.Context, options Options, kind string) ([]core.VersionSummary, error) {
	return versions(ctx, metadata.NewCatalog(options.HTTPClient, options.ManifestURL), kind)
}

// Install downloads and verifies one complete vanilla installation.
func Install(ctx context.Context, options Options, id string, report func(download.Progress)) (core.GameVersion, error) {
	return install(ctx, options, id, report)
}

func CheckJava(ctx context.Context, path string) (core.JavaRuntime, error) {
	return java.Check(ctx, path)
}

func Login(ctx context.Context, clientID string, notify func(auth.DeviceCode)) (core.Player, error) {
	return auth.Login(ctx, clientID, notify)
}

func Whoami(ctx context.Context) (core.Player, error) { return whoami(ctx) }
func Logout() error                                   { return auth.Logout() }

// PrepareLaunch resolves an installed version, Java runtime and player session.
func PrepareLaunch(ctx context.Context, root string, options LaunchOptions) (launch.Plan, error) {
	paths, err := config.Resolve(root)
	if err != nil {
		return launch.Plan{}, err
	}
	return prepareLaunch(ctx, paths, options, java.Check, auth.RefreshSession)
}

func RunGame(ctx context.Context, plan launch.Plan, stdin io.Reader, stdout, stderr io.Writer) error {
	return launch.Run(ctx, plan, stdin, stdout, stderr)
}
