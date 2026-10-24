// SPDX-License-Identifier: GPL-3.0-only

package core_test

import (
	"encoding/json"
	"testing"

	"github.com/Evernight275/cubicstart/internal/core"
)

func TestArgumentDecoding(t *testing.T) {
	var args []core.Argument
	if err := json.Unmarshal([]byte(`["-cp",{"rules":[{"action":"allow"}],"value":["a b","c"]}]`), &args); err != nil {
		t.Fatal(err)
	}
	if len(args) != 2 || len(args[1].Values) != 2 || args[1].Values[0] != "a b" {
		t.Fatalf("arguments: %#v", args)
	}
	for _, bad := range []string{`null`, `123`, `{"value":42}`, `{"value":[]}`} {
		var arg core.Argument
		if json.Unmarshal([]byte(bad), &arg) == nil {
			t.Errorf("accepted argument %s", bad)
		}
	}
}
