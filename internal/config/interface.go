// SPDX-License-Identifier: GPL-3.0-only

// Package config 负责普通配置的读写，以及数据目录和实例路径的管理。
// 敏感令牌由 auth 包管理，不写入普通配置。
// 本包的所有公开声明统一放在 interface.go 中。
package config

// Paths describes the launcher's data layout. Game instances are isolated by version.
type Paths struct {
	Root, Versions, Libraries, Assets, Instances string
}

// Resolve resolves an explicit root, or the user's default CubicStart directory.
func Resolve(root string) (Paths, error) { return resolve(root) }

// ValidID rejects version identifiers that are unsafe as directory names.
func ValidID(id string) error { return validID(id) }

// Join resolves an untrusted relative path beneath a data directory.
func Join(root, relative string) (string, error) { return join(root, relative) }
