// SPDX-License-Identifier: GPL-3.0-only

// Package launch 负责根据已解析的启动信息准备原生库、构建 JVM 和游戏参数，
// 并管理游戏进程、日志输出和退出状态。
// 本包的所有公开声明统一放在 interface.go 中。
package launch

import (
	"context"
	"io"

	"github.com/Evernight275/cubicstart/internal/config"
	"github.com/Evernight275/cubicstart/internal/core"
)

type Options struct {
	Installation core.Installation
	Instance     core.Instance
	Paths        config.Paths
	Java         core.JavaRuntime
	Session      core.Session
	MemoryMB     int
}

type Plan struct {
	Executable, Directory string
	Args                  []string
	nativeDir             string
}

// Close removes only the temporary native directory created for this plan.
func (p Plan) Close() error           { return closePlan(p) }
func (p Plan) RedactedArgs() []string { return redactedArgs(p.Args) }

// Prepare builds arguments for an instance using its selected installation,
// whose files the caller has verified, and prepares temporary native libraries.
func Prepare(ctx context.Context, options Options) (Plan, error) { return prepare(ctx, options) }

// Run starts a prepared game process and releases its temporary native directory.
func Run(ctx context.Context, plan Plan, stdin io.Reader, stdout, stderr io.Writer) error {
	return run(ctx, plan, stdin, stdout, stderr)
}

func OfflineSession(name string) (core.Session, error) { return offlineSession(name) }
