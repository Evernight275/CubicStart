// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestHelpAndUsageErrors(t *testing.T) {
	for _, args := range [][]string{nil, {"--help"}, {"launch", "--help"}, {"version"}} {
		var output bytes.Buffer
		if err := Run(context.Background(), args, Streams{Out: &output}); err != nil || output.Len() == 0 {
			t.Fatalf("%v: output %q, error %v", args, output.String(), err)
		}
	}
	for _, args := range [][]string{{"unknown"}, {"install"}, {"install", "1.21.1", "--workers", "0"}, {"launch"}, {"launch", "1.21.1", "extra", "--instance", "survival"}, {"launch", "1.21.1", "--memory", "10"}, {"versions", "--type", "bad"}, {"versions", "--limit", "-1"}, {"launch", "1.21.1", "--unknown"}} {
		if err := Run(context.Background(), args, Streams{}); ExitCode(err) != 2 {
			t.Errorf("%v: want usage error, got %v", args, err)
		}
	}
}

func TestNamedInstanceWithoutVersion(t *testing.T) {
	err := Run(context.Background(), []string{"launch", "--instance", "survival", "--offline", "--dir", t.TempDir()}, Streams{})
	if err == nil || ExitCode(err) != 1 || !strings.Contains(err.Error(), "provide a version") {
		t.Fatalf("named instance was not parsed without a version: %v", err)
	}
}

func TestFlagsAfterPositionals(t *testing.T) {
	err := Run(context.Background(), []string{"launch", "not-installed", "--offline", "--dir", t.TempDir(), "--memory=1024"}, Streams{})
	if err == nil || ExitCode(err) != 1 || !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("flags after version were not parsed: %v", err)
	}
	if ExitCode(context.Canceled) != 130 {
		t.Fatal("unexpected cancellation exit code")
	}
}
