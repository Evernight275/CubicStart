// SPDX-License-Identifier: GPL-3.0-only

// Package config 负责普通配置的读写，以及数据目录和实例路径的管理。
// 敏感令牌由 auth 包管理，不写入普通配置。
// 本包的所有公开声明统一放在 interface.go 中。
package config

import "github.com/Evernight275/cubicstart/internal/core"

// Paths describes shared installation caches and independent game instances.
type Paths struct {
	Root, Versions, Libraries, Assets, Instances string
}

// Resolve resolves an explicit root, or the user's default CubicStart directory.
func Resolve(root string) (Paths, error) { return resolve(root) }

// ValidID rejects identifiers that are unsafe as version or instance directory names.
func ValidID(id string) error { return validID(id) }

// Join resolves an untrusted relative path beneath a data directory.
func Join(root, relative string) (string, error) { return join(root, relative) }

// ReadInstance reads a saved version selection and resolves its game directory.
// A missing record returns an error wrapping os.ErrNotExist.
func ReadInstance(paths Paths, id string) (core.Instance, error) { return readInstance(paths, id) }

// SaveInstance atomically saves a version selection without moving game data.
func SaveInstance(paths Paths, instance core.Instance) error { return saveInstance(paths, instance) }
