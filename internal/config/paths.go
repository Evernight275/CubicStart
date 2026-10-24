// SPDX-License-Identifier: GPL-3.0-only

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+\-]*$`)

func validID(id string) error {
	base := strings.ToUpper(strings.SplitN(id, ".", 2)[0])
	reserved := base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" ||
		regexp.MustCompile(`^(COM|LPT)[0-9]$`).MatchString(base)
	if !idPattern.MatchString(id) || strings.Contains(id, "..") || strings.HasSuffix(id, ".") || reserved {
		return fmt.Errorf("invalid directory identifier %q", id)
	}
	return nil
}

func resolve(root string) (Paths, error) {
	if root == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return Paths{}, fmt.Errorf("find user data directory: %w", err)
		}
		root = filepath.Join(base, "CubicStart")
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return Paths{}, err
	}
	return Paths{Root: root, Versions: filepath.Join(root, "versions"), Libraries: filepath.Join(root, "libraries"), Assets: filepath.Join(root, "assets"), Instances: filepath.Join(root, "instances")}, nil
}

func join(root, relative string) (string, error) {
	// Reject Windows separators and volumes even when validating on Unix.
	if relative == "" || strings.ContainsAny(relative, `\:`) || strings.HasPrefix(relative, "/") {
		return "", fmt.Errorf("unsafe relative path %q", relative)
	}
	for _, component := range strings.Split(relative, "/") {
		if component == ".." || component == "." || component == "" || strings.HasSuffix(component, ".") || strings.HasSuffix(component, " ") {
			return "", fmt.Errorf("unsafe relative path %q", relative)
		}
	}
	path := filepath.Join(root, filepath.FromSlash(relative))
	rel, err := filepath.Rel(root, path)
	if err != nil || !filepath.IsLocal(rel) {
		return "", fmt.Errorf("path escapes data directory: %q", relative)
	}
	return path, nil
}
