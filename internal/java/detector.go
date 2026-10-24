// SPDX-License-Identifier: GPL-3.0-only

package java

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Evernight275/cubicstart/internal/core"
)

func check(ctx context.Context, path string) (core.JavaRuntime, error) {
	if path == "" {
		path = "java"
	}
	resolved, err := exec.LookPath(path)
	if err != nil {
		return core.JavaRuntime{}, fmt.Errorf("Java not found; install Java or pass --java: %w", err)
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return core.JavaRuntime{}, err
	}
	output, err := exec.CommandContext(ctx, resolved, "-XshowSettings:properties", "-version").CombinedOutput()
	if err != nil {
		return core.JavaRuntime{}, fmt.Errorf("inspect Java: %w", err)
	}
	info, err := parseProperties(string(output))
	info.Path = resolved
	return info, err
}

func parseProperties(text string) (core.JavaRuntime, error) {
	properties := make(map[string]string)
	for _, line := range strings.Split(text, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if ok {
			properties[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	version := properties["java.version"]
	if version == "" {
		match := regexp.MustCompile(`version "([^"]+)"`).FindStringSubmatch(text)
		if len(match) > 1 {
			version = match[1]
		}
	}
	majorText := version
	if strings.HasPrefix(majorText, "1.") {
		majorText = strings.TrimPrefix(majorText, "1.")
	}
	parts := strings.FieldsFunc(majorText, func(r rune) bool { return r < '0' || r > '9' })
	if len(parts) == 0 {
		return core.JavaRuntime{}, fmt.Errorf("cannot determine Java version from output")
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil || major < 1 {
		return core.JavaRuntime{}, fmt.Errorf("cannot determine Java version from output")
	}
	return core.JavaRuntime{Version: version, Major: major, Arch: properties["os.arch"], OSVersion: properties["os.version"]}, nil
}
