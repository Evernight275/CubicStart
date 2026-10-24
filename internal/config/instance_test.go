// SPDX-License-Identifier: GPL-3.0-only

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Evernight275/cubicstart/internal/core"
)

func TestInstanceRecordsCannotRedirectGameDirectory(t *testing.T) {
	paths, err := Resolve(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(paths.Instances, "survival")
	if err := os.MkdirAll(directory, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, instanceFile)
	// An absolute directory in edited metadata must never redirect an instance.
	if err := os.WriteFile(path, []byte(`{"id":"survival","version":"1.21.1","Directory":"C:/outside"}`), 0600); err != nil {
		t.Fatal(err)
	}
	instance, err := ReadInstance(paths, "survival")
	if err != nil || instance.Directory != directory {
		t.Fatalf("instance path was redirected: %+v, %v", instance, err)
	}
	instance.Directory = t.TempDir()
	if err := SaveInstance(paths, instance); err == nil {
		t.Fatal("saved an instance with an unrelated directory")
	}
	for _, data := range []string{`{"id":"other","version":"1.21.1"}`, `{"id":"survival","version":"../outside"}`, `{broken`} {
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadInstance(paths, "survival"); err == nil {
			t.Fatalf("accepted invalid instance record: %s", data)
		}
	}
	if err := SaveInstance(paths, core.Instance{ID: "../outside", VersionID: "1.21.1", Directory: directory}); err == nil {
		t.Fatal("saved an unsafe instance ID")
	}
}
