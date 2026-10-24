// SPDX-License-Identifier: GPL-3.0-only

package download

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type downloader struct {
	client  *http.Client
	workers int
}

func newDownloader(client *http.Client, workers int) *downloader {
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	if workers < 1 {
		workers = 8
	}
	if workers > 32 {
		workers = 32
	}
	return &downloader{client, workers}
}

func validate(file File) error {
	hash, err := hex.DecodeString(file.SHA1)
	if err != nil || len(hash) != sha1.Size {
		return fmt.Errorf("invalid SHA-1 for %s", file.Path)
	}
	if file.Size < 0 || file.Path == "" || file.URL == "" {
		return fmt.Errorf("incomplete download description")
	}
	return nil
}

func validCached(file File) bool {
	f, err := os.Open(file.Path)
	if err != nil {
		return false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != file.Size {
		return false
	}
	hash := sha1.New()
	if _, err := io.Copy(hash, f); err != nil {
		return false
	}
	return strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), file.SHA1)
}

func (d *downloader) Fetch(ctx context.Context, file File) error {
	if err := validate(file); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if validCached(file) {
		return nil
	}
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			timer := time.NewTimer(time.Duration(attempt) * 300 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
		if last = d.fetchOnce(ctx, file); last == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return fmt.Errorf("download %s: %w", filepath.Base(file.Path), last)
}

func (d *downloader) fetchOnce(ctx context.Context, file File) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, file.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "CubicStart/0.1.0")
	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(file.Path), 0755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(file.Path), ".cubicstart-*.part")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	hash := sha1.New()
	written, err := io.Copy(io.MultiWriter(temp, hash), io.LimitReader(resp.Body, file.Size+1))
	if err != nil {
		return err
	}
	if written != file.Size {
		return fmt.Errorf("size mismatch: got %d, expected %d", written, file.Size)
	}
	if !strings.EqualFold(hex.EncodeToString(hash.Sum(nil)), file.SHA1) {
		return fmt.Errorf("SHA-1 mismatch")
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(temp.Name(), file.Path)
}

func (d *downloader) FetchAll(ctx context.Context, files []File, report func(Progress)) error {
	unique := make(map[string]File)
	var queue []File
	for _, file := range files {
		if err := validate(file); err != nil {
			return err
		}
		key := filepath.Clean(file.Path)
		if old, ok := unique[key]; ok {
			if old.SHA1 != file.SHA1 || old.Size != file.Size {
				return fmt.Errorf("conflicting downloads for %s", key)
			}
			continue
		}
		unique[key] = file
		queue = append(queue, file)
	}
	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan File)
	var wg sync.WaitGroup
	var lock sync.Mutex
	var firstErr error
	completed := 0
	for i := 0; i < d.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range jobs {
				err := d.Fetch(workCtx, file)
				lock.Lock()
				if err != nil {
					if firstErr == nil {
						firstErr = err
						cancel()
					}
				} else {
					completed++
					if report != nil {
						report(Progress{completed, len(queue), file.Path})
					}
				}
				lock.Unlock()
				if err != nil {
					return
				}
			}
		}()
	}
send:
	for _, file := range queue {
		select {
		case <-workCtx.Done():
			break send
		case jobs <- file:
		}
	}
	close(jobs)
	wg.Wait()
	if firstErr != nil {
		return firstErr
	}
	return ctx.Err()
}
