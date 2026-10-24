// SPDX-License-Identifier: GPL-3.0-only

package download

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func fixtureFile(url, path, body string) File {
	hash := sha1.Sum([]byte(body))
	return File{URL: url, Path: path, SHA1: hex.EncodeToString(hash[:]), Size: int64(len(body))}
}

func TestCacheRetryAndRepair(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			http.Error(w, "temporary failure", 503)
			return
		}
		io.WriteString(w, "correct bytes")
	}))
	defer server.Close()
	file := fixtureFile(server.URL, filepath.Join(t.TempDir(), "space dir", "client.jar"), "correct bytes")
	downloader := NewDownloader(server.Client(), 2)
	if err := downloader.Fetch(context.Background(), file); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("requests = %d, want retry", calls.Load())
	}
	if err := downloader.Fetch(context.Background(), file); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatal("verified cache caused another request")
	}
	if err := os.WriteFile(file.Path, []byte("corrupt bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := downloader.Fetch(context.Background(), file); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(file.Path)
	if string(data) != "correct bytes" {
		t.Fatal("corrupt file was not repaired")
	}
	parts, _ := filepath.Glob(filepath.Join(filepath.Dir(file.Path), "*.part"))
	if len(parts) != 0 {
		t.Fatalf("temporary files remain: %v", parts)
	}
}

func TestCorruptDownloadNeverPublished(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "bad") }))
	defer server.Close()
	file := fixtureFile(server.URL, filepath.Join(t.TempDir(), "client.jar"), "good")
	if err := NewDownloader(server.Client(), 1).Fetch(context.Background(), file); err == nil {
		t.Fatal("corrupt download accepted")
	}
	if _, err := os.Stat(file.Path); !os.IsNotExist(err) {
		t.Fatal("corrupt file was published")
	}
	entries, _ := os.ReadDir(filepath.Dir(file.Path))
	if len(entries) != 0 {
		t.Fatal("partial download was not cleaned up")
	}
}

func TestFetchAllDeduplicatesAndHonorsCancellation(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1); io.WriteString(w, "data") }))
	defer server.Close()
	file := fixtureFile(server.URL, filepath.Join(t.TempDir(), "file"), "data")
	downloader := NewDownloader(server.Client(), 4)
	var progress []Progress
	if err := downloader.FetchAll(context.Background(), []File{file, file}, func(p Progress) { progress = append(progress, p) }); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 || len(progress) != 1 || progress[0].Total != 1 {
		t.Fatalf("deduplication failed: calls %d, progress %v", calls.Load(), progress)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := downloader.Fetch(ctx, file); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled fetch: %v", err)
	}
	conflict := file
	conflict.Size++
	if err := downloader.FetchAll(context.Background(), []File{file, conflict}, nil); err == nil {
		t.Fatal("conflicting destinations accepted")
	}
}

func TestDownloadCancellationInFlight(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		result <- NewDownloader(server.Client(), 1).Fetch(ctx, fixtureFile(server.URL, filepath.Join(t.TempDir(), "file"), "data"))
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("request did not start")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("in-flight cancellation: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancellation did not stop the request")
	}
}

func TestDownloadWorkerLimit(t *testing.T) {
	var running, maximum atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current := running.Add(1)
		defer running.Add(-1)
		for old := maximum.Load(); current > old; old = maximum.Load() {
			if maximum.CompareAndSwap(old, current) {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
		io.WriteString(w, "data")
	}))
	defer server.Close()
	var files []File
	root := t.TempDir()
	for _, name := range []string{"a", "b", "c", "d", "e", "f"} {
		files = append(files, fixtureFile(server.URL, filepath.Join(root, name), "data"))
	}
	if err := NewDownloader(server.Client(), 2).FetchAll(context.Background(), files, nil); err != nil {
		t.Fatal(err)
	}
	if maximum.Load() > 2 || maximum.Load() < 1 {
		t.Fatalf("concurrent downloads = %d", maximum.Load())
	}
}
