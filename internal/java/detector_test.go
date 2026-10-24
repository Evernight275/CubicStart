// SPDX-License-Identifier: GPL-3.0-only

package java

import "testing"

func TestJavaVersionParsing(t *testing.T) {
	for _, tc := range []struct {
		text  string
		major int
	}{
		{"java.version = 1.8.0_412\nos.arch = amd64\nos.version = 10.0", 8},
		{"java.version = 21.0.11\r\n    os.arch = amd64", 21},
		{`openjdk version "17.0.12" 2024-07-16`, 17},
		{"java.version = 25-ea", 25},
		{"not java", 0},
	} {
		got, err := parseProperties(tc.text)
		if tc.major == 0 {
			if err == nil {
				t.Error("malformed Java output accepted")
			}
			continue
		}
		if err != nil || got.Major != tc.major {
			t.Errorf("parse %q = %+v, %v", tc.text, got, err)
		}
	}
}
