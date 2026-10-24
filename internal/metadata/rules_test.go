// SPDX-License-Identifier: GPL-3.0-only

package metadata

import (
	"encoding/json"
	"testing"

	"github.com/Evernight275/cubicstart/internal/core"
)

func TestRuleOrderAndFeatures(t *testing.T) {
	platform := core.Platform{OS: "windows", Arch: "amd64", Version: "10.0"}
	cases := []struct {
		name    string
		rules   []core.Rule
		want    bool
		wantErr bool
	}{
		{"unconditional", nil, true, false},
		{"other OS", []core.Rule{{Action: "allow", OS: &core.OSRule{Name: "linux"}}}, false, false},
		{"last matching rule", []core.Rule{{Action: "allow"}, {Action: "disallow", OS: &core.OSRule{Name: "windows"}}}, false, false},
		{"missing feature is false", []core.Rule{{Action: "allow", Features: map[string]bool{"is_demo_user": false}}}, true, false},
		{"disabled demo", []core.Rule{{Action: "allow", Features: map[string]bool{"is_demo_user": true}}}, false, false},
		{"version regex", []core.Rule{{Action: "allow", OS: &core.OSRule{Version: `^10\.`}}}, true, false},
		{"invalid regex", []core.Rule{{Action: "allow", OS: &core.OSRule{Arch: "["}}}, false, true},
		{"unknown action", []core.Rule{{Action: "unknown"}}, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Allowed(tc.rules, platform)
			if got != tc.want || (err != nil) != tc.wantErr {
				t.Fatalf("Allowed = %v, %v", got, err)
			}
		})
	}
}

func TestNativeClassifier(t *testing.T) {
	var lib core.Library
	if err := json.Unmarshal([]byte(`{"name":"native","natives":{"windows":"natives-windows-${arch}"},"downloads":{"classifiers":{"natives-windows-64":{"path":"native.jar"}}}}`), &lib); err != nil {
		t.Fatal(err)
	}
	files, err := LibraryFiles(lib, core.Platform{OS: "windows", Arch: "amd64"})
	if err != nil || len(files) != 1 || files[0].Path != "native.jar" {
		t.Fatalf("native selection: %v, %v", files, err)
	}
	if _, err := LibraryFiles(lib, core.Platform{OS: "windows", Arch: "x86"}); err == nil {
		t.Fatal("missing classifier accepted")
	}
}
