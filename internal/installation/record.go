// SPDX-License-Identifier: GPL-3.0-only

package installation

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Evernight275/cubicstart/internal/config"
	"github.com/Evernight275/cubicstart/internal/core"
	"github.com/Evernight275/cubicstart/internal/metadata"
)

func read(paths config.Paths, id string, platform core.Platform) (core.Installation, error) {
	if err := config.ValidID(id); err != nil {
		return core.Installation{}, err
	}
	directory := filepath.Join(paths.Versions, id)
	raw, err := os.ReadFile(filepath.Join(directory, id+".json"))
	if err != nil {
		return core.Installation{}, fmt.Errorf("version %s is not installed; run cubicstart install %s: %w", id, id, err)
	}
	version, err := metadata.ParseVersion(raw)
	if err != nil {
		return core.Installation{}, err
	}
	if version.ID != id {
		return core.Installation{}, fmt.Errorf("installed metadata has an unexpected version ID")
	}
	return core.Installation{Version: version, Directory: directory, Platform: platform}, nil
}

func publish(installed core.Installation, raw []byte) error {
	version, err := metadata.ParseVersion(raw)
	if err != nil {
		return err
	}
	if version.ID != installed.Version.ID {
		return fmt.Errorf("installation metadata has an unexpected version ID")
	}
	if err := os.MkdirAll(installed.Directory, 0755); err != nil {
		return err
	}
	file, err := os.CreateTemp(installed.Directory, ".metadata-*.part")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if _, err := file.Write(raw); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), filepath.Join(installed.Directory, version.ID+".json"))
}
