// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Evernight275/cubicstart/internal/config"
	"github.com/Evernight275/cubicstart/internal/core"
	"github.com/Evernight275/cubicstart/internal/launch"
	"github.com/Evernight275/cubicstart/internal/metadata"
)

func prepareLaunch(ctx context.Context, paths config.Paths, options LaunchOptions,
	checkJava func(context.Context, string) (core.JavaRuntime, error),
	refreshSession func(context.Context) (core.Session, error),
) (launch.Plan, error) {
	if err := config.ValidID(options.Version); err != nil {
		return launch.Plan{}, err
	}
	path := filepath.Join(paths.Versions, options.Version, options.Version+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return launch.Plan{}, fmt.Errorf("version %s is not installed; run cubicstart install %s: %w", options.Version, options.Version, err)
	}
	version, err := metadata.ParseVersion(raw)
	if err != nil {
		return launch.Plan{}, err
	}
	if version.ID != options.Version {
		return launch.Plan{}, fmt.Errorf("installed metadata has an unexpected version ID")
	}
	runtime, err := checkJava(ctx, options.JavaPath)
	if err != nil {
		return launch.Plan{}, err
	}
	if runtime.Major < version.JavaVersion.Major {
		return launch.Plan{}, fmt.Errorf("Minecraft %s requires Java %d, selected Java is %d; pass --java", version.ID, version.JavaVersion.Major, runtime.Major)
	}
	if runtime.Arch == "x86" || runtime.Arch == "i386" {
		return launch.Plan{}, fmt.Errorf("v0.1 requires a 64-bit Java runtime")
	}
	platform := metadata.CurrentPlatform()
	platform.Arch, platform.Version = runtime.Arch, runtime.OSVersion
	var session core.Session
	if options.Offline {
		session, err = launch.OfflineSession(options.Username)
	} else {
		session, err = refreshSession(ctx)
	}
	if err != nil {
		return launch.Plan{}, err
	}
	return launch.Prepare(ctx, launch.Options{Version: version, Paths: paths, Java: runtime, Platform: platform, Session: session, MemoryMB: options.MemoryMB})
}
