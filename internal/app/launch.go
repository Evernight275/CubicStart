// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Evernight275/cubicstart/internal/config"
	"github.com/Evernight275/cubicstart/internal/core"
	"github.com/Evernight275/cubicstart/internal/installation"
	"github.com/Evernight275/cubicstart/internal/launch"
	"github.com/Evernight275/cubicstart/internal/metadata"
)

func resolveInstance(paths config.Paths, options LaunchOptions) (core.Instance, error) {
	id := options.Instance
	if id == "" {
		id = options.Version
	}
	if err := config.ValidID(id); err != nil {
		return core.Instance{}, fmt.Errorf("invalid instance ID: %w", err)
	}
	instance, err := config.ReadInstance(paths, id)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return core.Instance{}, err
		}
		if options.Version == "" {
			return core.Instance{}, fmt.Errorf("instance %q does not exist; provide a version to create it", id)
		}
		instance = core.Instance{ID: id, Directory: filepath.Join(paths.Instances, id)}
	}
	if options.Version != "" {
		if err := config.ValidID(options.Version); err != nil {
			return core.Instance{}, err
		}
		instance.VersionID = options.Version
	}
	return instance, nil
}

func prepareLaunch(ctx context.Context, paths config.Paths, options LaunchOptions,
	checkJava func(context.Context, string) (core.JavaRuntime, error),
	refreshSession func(context.Context) (core.Session, error),
) (launch.Plan, error) {
	instance, err := resolveInstance(paths, options)
	if err != nil {
		return launch.Plan{}, err
	}
	installed, err := installation.Read(paths, instance.VersionID, metadata.CurrentPlatform())
	if err != nil {
		return launch.Plan{}, err
	}
	version := installed.Version
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
	installed.Platform.Arch, installed.Platform.Version = runtime.Arch, runtime.OSVersion
	if err := installation.Verify(ctx, paths, installed); err != nil {
		return launch.Plan{}, err
	}
	var session core.Session
	if options.Offline {
		session, err = launch.OfflineSession(options.Username)
	} else {
		session, err = refreshSession(ctx)
	}
	if err != nil {
		return launch.Plan{}, err
	}
	plan, err := launch.Prepare(ctx, launch.Options{Installation: installed, Instance: instance, Paths: paths, Java: runtime, Session: session, MemoryMB: options.MemoryMB})
	if err != nil {
		return launch.Plan{}, err
	}
	if !options.Preview {
		if err := config.SaveInstance(paths, instance); err != nil {
			_ = plan.Close()
			return launch.Plan{}, fmt.Errorf("save instance selection: %w", err)
		}
	}
	return plan, nil
}
