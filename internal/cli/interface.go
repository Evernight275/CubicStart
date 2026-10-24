// SPDX-License-Identifier: GPL-3.0-only

// Package cli 负责命令解析、参数校验，以及终端中的进度和结果展示。
// 业务操作通过 app 包执行。
// 本包的所有公开声明统一放在 interface.go 中。
package cli

import (
	"context"
	"io"
)

type Streams struct {
	In       io.Reader
	Out, Err io.Writer
}

// Run executes one CLI command. args excludes the executable name.
func Run(ctx context.Context, args []string, streams Streams) error { return run(ctx, args, streams) }
func ExitCode(err error) int                                        { return exitCode(err) }
