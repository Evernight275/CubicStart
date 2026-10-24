// SPDX-License-Identifier: GPL-3.0-only

package config

import (
	"path/filepath"
	"testing"
)

func TestUnsafePaths(t *testing.T) {
	for _, id := range []string{"", "..", "../escape", `..\escape`, "CON", "nul.txt", "COM1", "version.", "version ", "C:escape"} {
		if err := ValidID(id); err == nil {
			t.Errorf("accepted unsafe ID %q", id)
		}
	}
	for _, id := range []string{"1.21.1", "26.1", "24w14a", "1.20.5-pre1"} {
		if err := ValidID(id); err != nil {
			t.Errorf("rejected %q: %v", id, err)
		}
	}
	root := t.TempDir()
	for _, path := range []string{"../outside", "/absolute", `C:\outside`, `x\..\outside`, "a/../../outside", "a/../b", "a//b"} {
		if _, err := Join(root, path); err == nil {
			t.Errorf("accepted unsafe path %q", path)
		}
	}
	path, err := Join(root, "org/example/library.jar")
	if err != nil || path != filepath.Join(root, "org", "example", "library.jar") {
		t.Fatalf("safe path = %q, %v", path, err)
	}
}
