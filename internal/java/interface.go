// SPDX-License-Identifier: GPL-3.0-only

// Package java 负责 Java 可执行文件的查找、版本检查和运行环境选择。
// 本包的所有公开声明统一放在 interface.go 中。
package java

import (
	"context"

	"github.com/Evernight275/cubicstart/internal/core"
)

// Check inspects a Java executable, using PATH when path is empty.
func Check(ctx context.Context, path string) (core.JavaRuntime, error) { return check(ctx, path) }
