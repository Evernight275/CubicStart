// SPDX-License-Identifier: GPL-3.0-only

package core

import (
	"encoding/json"
	"fmt"
	"strings"
)

func decodeArgument(data []byte, arg *Argument) error {
	if strings.TrimSpace(string(data)) == "null" {
		return fmt.Errorf("argument cannot be null")
	}
	var text string
	if json.Unmarshal(data, &text) == nil {
		*arg = Argument{Values: []string{text}}
		return nil
	}
	var object struct {
		Rules []Rule          `json:"rules"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}
	var values []string
	if json.Unmarshal(object.Value, &text) == nil {
		values = []string{text}
	} else if err := json.Unmarshal(object.Value, &values); err != nil {
		return fmt.Errorf("invalid argument value: %w", err)
	}
	if len(values) == 0 {
		return fmt.Errorf("argument has no value")
	}
	*arg = Argument{Values: values, Rules: object.Rules}
	return nil
}
