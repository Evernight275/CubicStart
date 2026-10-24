// SPDX-License-Identifier: GPL-3.0-only

// Package config 负责普通配置的读写，以及数据目录和实例路径的管理。
// 敏感令牌由 auth 包管理，不写入普通配置。
// 本包的所有公开声明统一放在 interface.go 中。
package config
