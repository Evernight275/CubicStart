// SPDX-License-Identifier: GPL-3.0-only

// Package download 负责文件下载、重试、完整性校验和下载缓存。
// 下载进度通过调用方提供的回调或接口传递。
// 本包的所有公开声明统一放在 interface.go 中。
package download

import (
	"context"
	"net/http"
)

type File struct {
	URL, Path, SHA1 string
	Size            int64
}
type Progress struct {
	Completed, Total int
	Path             string
}

// Downloader retrieves and verifies individual files or batches of files.
type Downloader interface {
	Fetch(context.Context, File) error
	FetchAll(context.Context, []File, func(Progress)) error
}

func NewDownloader(client *http.Client, workers int) Downloader {
	return newDownloader(client, workers)
}
