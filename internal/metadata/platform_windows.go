// SPDX-License-Identifier: GPL-3.0-only

package metadata

import (
	"fmt"

	"golang.org/x/sys/windows"
)

func platformVersion() string {
	version := windows.RtlGetVersion()
	return fmt.Sprintf("%d.%d", version.MajorVersion, version.MinorVersion)
}
