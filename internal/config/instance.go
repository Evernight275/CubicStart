// SPDX-License-Identifier: GPL-3.0-only

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Evernight275/cubicstart/internal/core"
)

const instanceFile = ".cubicstart-instance.json"

func readInstance(paths Paths, id string) (core.Instance, error) {
	if err := validID(id); err != nil {
		return core.Instance{}, fmt.Errorf("invalid instance ID: %w", err)
	}
	directory := filepath.Join(paths.Instances, id)
	data, err := os.ReadFile(filepath.Join(directory, instanceFile))
	if err != nil {
		return core.Instance{}, fmt.Errorf("read instance %q: %w", id, err)
	}
	var instance core.Instance
	if err := json.Unmarshal(data, &instance); err != nil {
		return instance, fmt.Errorf("decode instance %q: %w", id, err)
	}
	if instance.ID != id {
		return instance, fmt.Errorf("instance record has an unexpected ID")
	}
	if err := validID(instance.VersionID); err != nil {
		return instance, err
	}
	instance.Directory = directory
	return instance, nil
}

func saveInstance(paths Paths, instance core.Instance) error {
	if err := validID(instance.ID); err != nil {
		return err
	}
	if err := validID(instance.VersionID); err != nil {
		return err
	}
	directory := filepath.Join(paths.Instances, instance.ID)
	if filepath.Clean(instance.Directory) != filepath.Clean(directory) {
		return fmt.Errorf("instance directory does not match its ID")
	}
	data, err := json.MarshalIndent(instance, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(directory, 0755); err != nil {
		return err
	}
	file, err := os.CreateTemp(directory, ".instance-*.part")
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
	return os.Rename(file.Name(), filepath.Join(directory, instanceFile))
}
