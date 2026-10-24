// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Evernight275/cubicstart/internal/config"
	"github.com/Evernight275/cubicstart/internal/core"
)

type assetMirror struct {
	base string
	next http.RoundTripper
}

func (m assetMirror) RoundTrip(request *http.Request) (*http.Response, error) {
	if request.URL.Host == "resources.download.minecraft.net" {
		copy := request.Clone(request.Context())
		base, _ := url.Parse(m.base)
		base.Path = "/asset" + request.URL.Path
		copy.URL = base
		return m.next.RoundTrip(copy)
	}
	return m.next.RoundTrip(request)
}

func fixedJava(context.Context, string) (core.JavaRuntime, error) {
	return core.JavaRuntime{Path: "java", Major: 21, Arch: "amd64", OSVersion: "10.0"}, nil
}

func digest(data []byte) string { hash := sha1.Sum(data); return hex.EncodeToString(hash[:]) }

type installFixture struct {
	options   Options
	paths     config.Paths
	requests  *atomic.Int32
	broken    *atomic.Bool
	assetPath string
}

func newInstallFixture(t *testing.T) installFixture {
	t.Helper()
	var versionJSON, manifestJSON []byte
	asset := []byte("sample resource")
	assetHash := digest(asset)
	indexJSON := []byte(`{"objects":{"sound.ogg":{"hash":"` + assetHash + `","size":15}}}`)
	var payloadRequests atomic.Int32
	var broken atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/manifest":
			w.Write(manifestJSON)
		case r.URL.Path == "/version":
			w.Write(versionJSON)
		case r.URL.Path == "/index":
			w.Write(indexJSON)
		case r.URL.Path == "/client":
			payloadRequests.Add(1)
			io.WriteString(w, "client")
		case r.URL.Path == "/library":
			payloadRequests.Add(1)
			io.WriteString(w, "library")
		case r.URL.Path == "/logging":
			payloadRequests.Add(1)
			io.WriteString(w, "logging")
		case strings.HasPrefix(r.URL.Path, "/asset/"):
			payloadRequests.Add(1)
			if broken.Load() {
				io.WriteString(w, "damaged resource")
			} else {
				w.Write(asset)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	artifact := func(route, content string) map[string]any {
		return map[string]any{"url": server.URL + route, "sha1": digest([]byte(content)), "size": len(content)}
	}
	index := artifact("/index", string(indexJSON))
	index["id"] = "test-assets"
	library := artifact("/library", "library")
	library["path"] = "org/example/library.jar"
	logging := artifact("/logging", "logging")
	logging["id"] = "client.xml"
	var err error
	versionJSON, err = json.Marshal(map[string]any{
		"id": "1.21-test", "type": "release", "mainClass": "example.Main",
		"javaVersion": map[string]int{"majorVersion": 21}, "downloads": map[string]any{"client": artifact("/client", "client")},
		"assetIndex": index,
		"libraries":  []any{map[string]any{"name": "example:library:1", "downloads": map[string]any{"artifact": library}}},
		"logging":    map[string]any{"client": map[string]any{"argument": "-Dlog4j.configurationFile=${path}", "file": logging}},
		"arguments":  map[string]any{"jvm": []string{"-cp", "${classpath}", "-Djava.library.path=${natives_directory}"}, "game": []string{"--username", "${auth_player_name}", "--gameDir", "${game_directory}", "--accessToken", "${auth_access_token}"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	manifestJSON, err = json.Marshal(map[string]any{"latest": map[string]string{"release": "1.21-test"}, "versions": []any{map[string]any{"id": "1.21-test", "type": "release", "url": server.URL + "/version", "sha1": digest(versionJSON), "releaseTime": "2024-08-08T00:00:00Z"}}})
	if err != nil {
		t.Fatal(err)
	}
	client := server.Client()
	client.Transport = assetMirror{server.URL, client.Transport}
	options := Options{Root: filepath.Join(t.TempDir(), "中文 data with spaces"), ManifestURL: server.URL + "/manifest", HTTPClient: client, Workers: 3}
	paths, err := DataPaths(options.Root)
	if err != nil {
		t.Fatal(err)
	}
	return installFixture{options, paths, &payloadRequests, &broken, filepath.Join(paths.Assets, "objects", assetHash[:2], assetHash)}
}

func TestInstallCacheRepairAndPrepare(t *testing.T) {
	fixture := newInstallFixture(t)
	ctx := context.Background()
	versions, err := Versions(ctx, fixture.options, "release")
	if err != nil || len(versions) != 1 {
		t.Fatalf("versions: %v, %v", versions, err)
	}
	installed, err := Install(ctx, fixture.options, "latest", nil)
	if err != nil {
		t.Fatal(err)
	}
	version := installed.Version
	if version.ID != "1.21-test" || fixture.requests.Load() != 4 {
		t.Fatalf("installation: %s, %d requests", version.ID, fixture.requests.Load())
	}
	if _, err := Install(ctx, fixture.options, version.ID, nil); err != nil {
		t.Fatal(err)
	}
	if fixture.requests.Load() != 4 {
		t.Fatal("reinstall downloaded verified files again")
	}
	if err := os.WriteFile(fixture.assetPath, []byte("damaged resource"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Install(ctx, fixture.options, version.ID, nil); err != nil {
		t.Fatal(err)
	}
	if fixture.requests.Load() != 5 {
		t.Fatal("corrupt resource was not selectively repaired")
	}
	plan, err := prepareLaunch(ctx, fixture.paths, LaunchOptions{Version: version.ID, Offline: true, Username: "TestPlayer", MemoryMB: 2048}, fixedJava, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	if plan.Directory != filepath.Join(fixture.paths.Instances, version.ID) || !strings.Contains(strings.Join(plan.Args, "\n"), "TestPlayer") {
		t.Fatalf("launch plan: %+v", plan)
	}
	if strings.Contains(strings.Join(plan.Args, "\n"), "${") {
		t.Fatal("unresolved launch placeholder")
	}
}

func TestFailedInstallCannotBeLaunched(t *testing.T) {
	fixture := newInstallFixture(t)
	fixture.broken.Store(true)
	if _, err := Install(context.Background(), fixture.options, "1.21-test", nil); err == nil {
		t.Fatal("corrupt resource accepted")
	}
	marker := filepath.Join(fixture.paths.Versions, "1.21-test", "1.21-test.json")
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("failed installation was marked complete")
	}
	if _, err := PrepareLaunch(context.Background(), fixture.options.Root, LaunchOptions{Version: "1.21-test", Offline: true}); err == nil {
		t.Fatal("incomplete version was launchable")
	}
}

func TestInstallRejectsUnsafeAndUnknownVersions(t *testing.T) {
	fixture := newInstallFixture(t)
	for _, id := range []string{"../escape", "unknown"} {
		if _, err := Install(context.Background(), fixture.options, id, nil); err == nil {
			t.Errorf("accepted version %s", id)
		}
	}
}

func TestLaunchPreparationUsesRefreshedSession(t *testing.T) {
	fixture := newInstallFixture(t)
	ctx := context.Background()
	if _, err := Install(ctx, fixture.options, "1.21-test", nil); err != nil {
		t.Fatal(err)
	}
	refreshes := 0
	refresh := func(context.Context) (core.Session, error) {
		refreshes++
		return core.Session{Player: core.Player{ID: "0123456789abcdef0123456789abcdef", Name: "OnlinePlayer"}, AccessToken: "session-secret", UserType: "msa"}, nil
	}
	plan, err := prepareLaunch(ctx, fixture.paths, LaunchOptions{Version: "1.21-test", MemoryMB: 2048}, fixedJava, refresh)
	if err != nil {
		t.Fatal(err)
	}
	defer plan.Close()
	if refreshes != 1 || !strings.Contains(strings.Join(plan.Args, "\n"), "OnlinePlayer") || !strings.Contains(strings.Join(plan.Args, "\n"), "session-secret") {
		t.Fatal("launch did not use the refreshed player session")
	}
	if strings.Contains(strings.Join(plan.RedactedArgs(), "\n"), "session-secret") {
		t.Fatal("refreshed token leaked into launch preview")
	}
	refreshes = 0
	offline, err := prepareLaunch(ctx, fixture.paths, LaunchOptions{Version: "1.21-test", Offline: true, Username: "LocalPlayer", MemoryMB: 2048}, fixedJava, refresh)
	if err != nil {
		t.Fatal(err)
	}
	defer offline.Close()
	if refreshes != 0 {
		t.Fatal("explicit offline launch requested an online session")
	}
}
