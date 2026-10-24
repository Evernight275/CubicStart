// SPDX-License-Identifier: GPL-3.0-only

package installation

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Evernight275/cubicstart/internal/config"
	"github.com/Evernight275/cubicstart/internal/core"
)

func TestVerificationUsesTargetNativeLibraries(t *testing.T) {
	paths, err := config.Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	artifact := func(path, data string) core.Artifact {
		hash := sha1.Sum([]byte(data))
		return core.Artifact{Path: path, URL: "https://unused.invalid/" + path, SHA1: hex.EncodeToString(hash[:]), Size: int64(len(data))}
	}
	write := func(path, data string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	version := core.GameVersion{ID: "test", Downloads: map[string]core.Artifact{"client": artifact("client", "client")}}
	version.AssetIndex = artifact("index", `{"objects":{}}`)
	version.AssetIndex.ID = "test-assets"
	library := core.Library{Natives: map[string]string{"windows": "natives-windows", "linux": "natives-linux"}}
	library.Downloads.Classifiers = map[string]core.Artifact{
		"natives-windows": artifact("native/windows.jar", "windows"),
		"natives-linux":   artifact("native/linux.jar", "linux"),
	}
	version.Libraries = []core.Library{library}
	installed := core.Installation{Version: version, Directory: filepath.Join(paths.Versions, version.ID), Platform: core.Platform{OS: "windows", Arch: "amd64"}}
	write(filepath.Join(installed.Directory, "test.jar"), "client")
	write(filepath.Join(paths.Assets, "indexes", "test-assets.json"), `{"objects":{}}`)
	write(filepath.Join(paths.Libraries, "native", "windows.jar"), "windows")
	if err := Verify(context.Background(), paths, installed); err != nil {
		t.Fatal(err)
	}
	installed.Platform.OS = "linux"
	if err := Verify(context.Background(), paths, installed); err == nil {
		t.Fatal("files for another target platform were accepted")
	}
	write(filepath.Join(paths.Libraries, "native", "linux.jar"), "linux")
	if err := Verify(context.Background(), paths, installed); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Verify(ctx, paths, installed); !errors.Is(err, context.Canceled) {
		t.Fatalf("verification lost cancellation: %v", err)
	}
}
