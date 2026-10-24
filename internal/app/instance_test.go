// SPDX-License-Identifier: GPL-3.0-only

package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Evernight275/cubicstart/internal/config"
	"github.com/Evernight275/cubicstart/internal/core"
	"github.com/Evernight275/cubicstart/internal/installation"
	"github.com/Evernight275/cubicstart/internal/launch"
)

func prepareTestInstance(t *testing.T, fixture installFixture, options LaunchOptions) launch.Plan {
	t.Helper()
	options.Offline, options.Username, options.MemoryMB = true, "TestPlayer", 2048
	plan, err := prepareLaunch(context.Background(), fixture.paths, options, fixedJava, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = plan.Close() })
	return plan
}

func TestInstancesShareInstallationAndKeepDataWhenChangingVersion(t *testing.T) {
	fixture := newInstallFixture(t)
	ctx := context.Background()
	installed, err := Install(ctx, fixture.options, "1.21-test", nil)
	if err != nil {
		t.Fatal(err)
	}
	survival := prepareTestInstance(t, fixture, LaunchOptions{Version: installed.Version.ID, Instance: "survival"})
	creative := prepareTestInstance(t, fixture, LaunchOptions{Version: installed.Version.ID, Instance: "creative"})
	if survival.Directory == creative.Directory {
		t.Fatal("independent instances share a game directory")
	}
	clientJar := filepath.Join(installed.Directory, installed.Version.ID+".jar")
	for _, plan := range []launch.Plan{survival, creative} {
		if !strings.Contains(strings.Join(plan.Args, "\n"), clientJar) {
			t.Fatal("instances do not share the installed client")
		}
	}
	world := filepath.Join(survival.Directory, "saves", "world", "level.dat")
	if err := os.MkdirAll(filepath.Dir(world), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(world, []byte("existing world"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(creative.Directory, "saves")); !os.IsNotExist(err) {
		t.Fatal("world leaked into the other instance")
	}

	// A second locally installed version reuses the same dependencies and assets.
	other := installed
	other.Version.ID = "1.21-other"
	other.Directory = filepath.Join(fixture.paths.Versions, other.Version.ID)
	raw, err := os.ReadFile(filepath.Join(installed.Directory, installed.Version.ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var description map[string]any
	if err := json.Unmarshal(raw, &description); err != nil {
		t.Fatal(err)
	}
	description["id"] = other.Version.ID
	raw, err = json.Marshal(description)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(other.Directory, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(other.Directory, other.Version.ID+".jar"), []byte("client"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := installation.Publish(other, raw); err != nil {
		t.Fatal(err)
	}
	changed := prepareTestInstance(t, fixture, LaunchOptions{Version: other.Version.ID, Instance: "survival"})
	if changed.Directory != survival.Directory {
		t.Fatal("changing the selected version moved the instance")
	}
	if data, err := os.ReadFile(world); err != nil || string(data) != "existing world" {
		t.Fatalf("world was changed: %q, %v", data, err)
	}
	saved, err := config.ReadInstance(fixture.paths, "survival")
	if err != nil || saved.VersionID != other.Version.ID {
		t.Fatalf("new selection was not saved: %+v, %v", saved, err)
	}
	resumed := prepareTestInstance(t, fixture, LaunchOptions{Instance: "survival"})
	if resumed.Directory != survival.Directory || !strings.Contains(strings.Join(resumed.Args, "\n"), filepath.Join(other.Directory, other.Version.ID+".jar")) {
		t.Fatal("resuming an instance did not use its saved version")
	}
	creativeSelection, err := config.ReadInstance(fixture.paths, "creative")
	if err != nil || creativeSelection.VersionID != installed.Version.ID {
		t.Fatal("changing one instance affected another instance")
	}

	// A preview can select another version without changing the saved binding.
	prepareTestInstance(t, fixture, LaunchOptions{Version: installed.Version.ID, Instance: "survival", Preview: true})
	saved, err = config.ReadInstance(fixture.paths, "survival")
	if err != nil || saved.VersionID != other.Version.ID {
		t.Fatal("preview changed the saved version")
	}
	_, err = prepareLaunch(ctx, fixture.paths, LaunchOptions{Version: installed.Version.ID, Instance: "survival", Offline: true, Username: "TestPlayer", MemoryMB: 1}, fixedJava, nil)
	if err == nil {
		t.Fatal("invalid launch settings accepted")
	}
	saved, err = config.ReadInstance(fixture.paths, "survival")
	if err != nil || saved.VersionID != other.Version.ID {
		t.Fatal("failed preparation changed the saved version")
	}
}

func TestLegacyInstanceAndPreview(t *testing.T) {
	fixture := newInstallFixture(t)
	installed, err := Install(context.Background(), fixture.options, "1.21-test", nil)
	if err != nil {
		t.Fatal(err)
	}
	legacyDir := filepath.Join(fixture.paths.Instances, installed.Version.ID)
	if err := os.MkdirAll(legacyDir, 0755); err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(legacyDir, "options.txt")
	if err := os.WriteFile(settings, []byte("existing settings"), 0600); err != nil {
		t.Fatal(err)
	}
	plan := prepareTestInstance(t, fixture, LaunchOptions{Version: installed.Version.ID})
	if plan.Directory != legacyDir {
		t.Fatal("legacy command changed the game directory")
	}
	if data, err := os.ReadFile(settings); err != nil || string(data) != "existing settings" {
		t.Fatal("legacy settings were overwritten")
	}
	prepareTestInstance(t, fixture, LaunchOptions{Version: installed.Version.ID, Instance: "preview", Preview: true})
	if _, err := config.ReadInstance(fixture.paths, "preview"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("preview saved a new instance: %v", err)
	}
	for _, options := range []LaunchOptions{{Instance: "missing"}, {Version: installed.Version.ID, Instance: "../escape"}} {
		if _, err := prepareLaunch(context.Background(), fixture.paths, options, fixedJava, nil); err == nil {
			t.Fatalf("invalid instance selection accepted: %+v", options)
		}
	}
}

func TestInstallationRecordDoesNotGuaranteeIntegrity(t *testing.T) {
	for _, damage := range []string{"client", "library", "asset", "index", "logging"} {
		t.Run(damage, func(t *testing.T) {
			fixture := newInstallFixture(t)
			ctx := context.Background()
			installed, err := Install(ctx, fixture.options, "1.21-test", nil)
			if err != nil {
				t.Fatal(err)
			}
			var path string
			switch damage {
			case "client":
				path = filepath.Join(installed.Directory, installed.Version.ID+".jar")
			case "library":
				path = filepath.Join(fixture.paths.Libraries, "org", "example", "library.jar")
			case "asset":
				path = fixture.assetPath
			case "index":
				path = filepath.Join(fixture.paths.Assets, "indexes", installed.Version.AssetIndex.ID+".json")
			case "logging":
				path = filepath.Join(fixture.paths.Assets, "log_configs", "client.xml")
			}
			if damage == "client" {
				// Same length: a size check alone must not accept this corruption.
				err = os.WriteFile(path, []byte("broken"), 0600)
			} else {
				err = os.Remove(path)
			}
			if err != nil {
				t.Fatal(err)
			}
			record, err := installation.Read(fixture.paths, installed.Version.ID, installed.Platform)
			if err != nil {
				t.Fatalf("record was lost when a payload was damaged: %v", err)
			}
			if err := installation.Verify(ctx, fixture.paths, record); err == nil {
				t.Fatal("record was mistaken for verified files")
			}
			refreshes := 0
			refresh := func(context.Context) (core.Session, error) {
				refreshes++
				return core.Session{}, nil
			}
			if _, err := prepareLaunch(ctx, fixture.paths, LaunchOptions{Version: installed.Version.ID, MemoryMB: 2048}, fixedJava, refresh); err == nil || !strings.Contains(err.Error(), "incomplete") {
				t.Fatalf("damaged installation was not rejected: %v", err)
			}
			if refreshes != 0 {
				t.Fatal("refreshed credentials before rejecting damaged local files")
			}
			if _, err := Install(ctx, fixture.options, installed.Version.ID, nil); err != nil {
				t.Fatal(err)
			}
			prepareTestInstance(t, fixture, LaunchOptions{Version: installed.Version.ID})
		})
	}
}
