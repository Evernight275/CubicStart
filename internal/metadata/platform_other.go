//go:build !windows && !linux && !darwin

// SPDX-License-Identifier: GPL-3.0-only

package metadata

func platformVersion() string { return "" }
