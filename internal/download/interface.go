// Package download 负责文件下载、重试、完整性校验和下载缓存。
// 下载进度通过调用方提供的回调或接口传递。
// 本包的所有公开声明统一放在 interface.go 中。
package download
