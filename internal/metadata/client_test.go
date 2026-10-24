// SPDX-License-Identifier: GPL-3.0-only

package metadata

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Evernight275/cubicstart/internal/core"
)

func TestVersionMetadataIntegrity(t *testing.T) {
	body := `{"id":"1.21-test","mainClass":"example.Main","downloads":{"client":{"url":"https://example.invalid/client"}},"assetIndex":{"id":"test","url":"https://example.invalid/index"},"javaVersion":{"majorVersion":21},"arguments":{"jvm":["-cp","${classpath}"],"game":["--username","${auth_player_name}"]}}`
	hash := sha1.Sum([]byte(body))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, body) }))
	defer server.Close()
	catalog := NewCatalog(server.Client(), server.URL)
	summary := core.VersionSummary{ID: "1.21-test", URL: server.URL, SHA1: hex.EncodeToString(hash[:])}
	if _, _, err := catalog.Version(context.Background(), summary); err != nil {
		t.Fatal(err)
	}
	summary.SHA1 = "0000000000000000000000000000000000000000"
	if _, _, err := catalog.Version(context.Background(), summary); err == nil {
		t.Fatal("tampered version metadata accepted")
	}
	summary.SHA1 = hex.EncodeToString(hash[:])
	summary.ID = "another-version"
	if _, _, err := catalog.Version(context.Background(), summary); err == nil {
		t.Fatal("unexpected version ID accepted")
	}
}
