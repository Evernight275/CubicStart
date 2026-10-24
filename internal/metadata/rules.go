// SPDX-License-Identifier: GPL-3.0-only

package metadata

import (
	"fmt"
	"regexp"
	"runtime"
	"strings"

	"github.com/Evernight275/cubicstart/internal/core"
)

func currentPlatform() core.Platform {
	osName := runtime.GOOS
	if osName == "darwin" {
		osName = "osx"
	}
	arch := runtime.GOARCH
	if arch == "386" {
		arch = "x86"
	}
	if arch == "arm64" {
		arch = "aarch64"
	}
	return core.Platform{OS: osName, Arch: arch, Version: platformVersion()}
}

func allowed(rules []core.Rule, platform core.Platform) (bool, error) {
	result := len(rules) == 0
	for _, rule := range rules {
		if rule.Action != "allow" && rule.Action != "disallow" {
			return false, fmt.Errorf("unknown rule action %q", rule.Action)
		}
		matches := true
		if rule.OS != nil {
			if rule.OS.Name != "" && rule.OS.Name != platform.OS {
				continue
			}
			for _, condition := range []struct{ pattern, value string }{{rule.OS.Arch, platform.Arch}, {rule.OS.Version, platform.Version}} {
				if condition.pattern == "" {
					continue
				}
				re, err := regexp.Compile(condition.pattern)
				if err != nil {
					return false, fmt.Errorf("invalid platform rule: %w", err)
				}
				matches = matches && re.MatchString(condition.value)
			}
		}
		for feature, expected := range rule.Features {
			matches = matches && platform.Features[feature] == expected
		}
		if matches {
			result = rule.Action == "allow"
		}
	}
	return result, nil
}

func libraryFiles(lib core.Library, platform core.Platform) ([]core.Artifact, error) {
	ok, err := allowed(lib.Rules, platform)
	if err != nil || !ok {
		return nil, err
	}
	var files []core.Artifact
	if lib.Downloads.Artifact != nil {
		files = append(files, *lib.Downloads.Artifact)
	}
	if classifier, exists := lib.Natives[platform.OS]; exists {
		bits := "64"
		if platform.Arch == "x86" || platform.Arch == "i386" {
			bits = "32"
		}
		classifier = strings.ReplaceAll(classifier, "${arch}", bits)
		artifact, found := lib.Downloads.Classifiers[classifier]
		if !found {
			return nil, fmt.Errorf("library %s lacks native classifier %s", lib.Name, classifier)
		}
		files = append(files, artifact)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("library %s has no supported downloads", lib.Name)
	}
	return files, nil
}
