// SPDX-License-Identifier: GPL-3.0-only

package launch

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Evernight275/cubicstart/internal/core"
)

func TestArgumentBoundariesAndRedaction(t *testing.T) {
	args, err := expandArguments([]core.Argument{{Values: []string{"-cp", "${classpath}", "--accessToken", "${auth_access_token}"}}}, core.Platform{}, map[string]string{"classpath": `C:\my game\client.jar`, "auth_access_token": "secret-token"})
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 4 || args[1] != `C:\my game\client.jar` {
		t.Fatalf("argument boundaries changed: %v", args)
	}
	plan := Plan{Args: args}
	if strings.Contains(strings.Join(plan.RedactedArgs(), " "), "secret-token") {
		t.Fatal("token leaked into dry-run output")
	}
	if plan.Args[3] != "secret-token" {
		t.Fatal("redaction modified executable arguments")
	}
	if _, err := expand("${unknown}", nil); err == nil {
		t.Fatal("unsupported placeholder accepted")
	}
}

func TestOfflineUUID(t *testing.T) {
	session, err := OfflineSession("Notch")
	if err != nil || session.Player.ID != "b50ad385829d3141a2167e7d7539ba7f" || session.UserType != "legacy" {
		t.Fatalf("offline session: %+v, %v", session, err)
	}
	if _, err := OfflineSession("bad name"); err == nil {
		t.Fatal("invalid offline username accepted")
	}
}

func makeArchive(t *testing.T, name string) string {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	file, err := writer.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(file, "native bytes"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "native.jar")
	if err := os.WriteFile(path, buffer.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNativeExtractionRejectsTraversal(t *testing.T) {
	for _, name := range []string{"../escape.dll", "/absolute.dll", `C:\escape.dll`, `dir\..\escape.dll`} {
		if err := extractArchive(context.Background(), makeArchive(t, name), t.TempDir(), nil); err == nil {
			t.Errorf("accepted native path %s", name)
		}
	}
	dest := t.TempDir()
	if err := extractArchive(context.Background(), makeArchive(t, "native.dll"), dest, nil); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(filepath.Join(dest, "native.dll")); err != nil || string(data) != "native bytes" {
		t.Fatalf("extracted native: %q, %v", data, err)
	}
}

func TestProcessExitAndCleanup(t *testing.T) {
	if os.Getenv("CUBICSTART_TEST_CHILD") == "1" {
		os.Exit(7)
	}
	t.Setenv("CUBICSTART_TEST_CHILD", "1")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	nativeDir := filepath.Join(t.TempDir(), "natives")
	if err := os.Mkdir(nativeDir, 0755); err != nil {
		t.Fatal(err)
	}
	plan := Plan{Executable: executable, Args: []string{"-test.run=^TestProcessExitAndCleanup$"}, Directory: t.TempDir(), nativeDir: nativeDir}
	err = Run(context.Background(), plan, nil, io.Discard, io.Discard)
	var exited *exec.ExitError
	if !errors.As(err, &exited) || exited.ExitCode() != 7 {
		t.Fatalf("exit status was lost: %v", err)
	}
	if _, err := os.Stat(nativeDir); !os.IsNotExist(err) {
		t.Fatal("temporary natives directory was not cleaned up")
	}
}
