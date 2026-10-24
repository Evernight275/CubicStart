//go:build linux || darwin

// SPDX-License-Identifier: GPL-3.0-only

package metadata

import "golang.org/x/sys/unix"

func platformVersion() string {
	var info unix.Utsname
	if unix.Uname(&info) != nil {
		return ""
	}
	var release []byte
	for _, value := range info.Release {
		if value == 0 {
			break
		}
		release = append(release, byte(value))
	}
	return string(release)
}
