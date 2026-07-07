// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package vdm

import (
	"encoding/json"
	"testing"

	"rsc.io/diff"
)

func TestJSON(t *testing.T) {
	tests := []struct {
		Name string
		In   any
		Want string
	}{
		{
			Name: "parsed-bool-true",
			In:   ParsedBool{Value: true, Pos: Pos{File: "deps.vdm", Line: 3}, Comment: "Testing a bool."},
			Want: `true`,
		},
		{
			Name: "parsed-bool-false",
			In:   ParsedBool{Value: false, Pos: Pos{File: "deps.vdm", Line: 3}, Comment: "Testing a bool."},
			Want: `false`,
		},
		{
			Name: "parsed-bool-true-omitzero",
			In: struct {
				Bool ParsedBool `json:"bool,omitzero"`
			}{
				Bool: ParsedBool{Value: true},
			},
			Want: `{"bool":true}`,
		},
		{
			Name: "parsed-bool-false-omitzero",
			In: struct {
				Bool ParsedBool `json:"bool,omitzero"`
			}{
				Bool: ParsedBool{Value: false},
			},
			Want: `{}`,
		},
		{
			Name: "parsed-string-full",
			In:   ParsedString{Value: "foo", Pos: Pos{File: "deps.vdm", Line: 3}, Comment: "Testing a bool."},
			Want: `"foo"`,
		},
		{
			Name: "parsed-string-empty",
			In:   ParsedString{Value: "", Pos: Pos{File: "deps.vdm", Line: 3}, Comment: "Testing a bool."},
			Want: `""`,
		},
		{
			Name: "parsed-string-full-omitzero",
			In: struct {
				String ParsedString `json:"string,omitzero"`
			}{
				String: ParsedString{Value: "foo"},
			},
			Want: `{"string":"foo"}`,
		},
		{
			Name: "parsed-string-empty-omitzero",
			In: struct {
				String ParsedString `json:"string,omitzero"`
			}{
				String: ParsedString{Value: ""},
			},
			Want: `{}`,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got, err := json.Marshal(test.In)
			if err != nil {
				t.Fatal(err)
			}

			gots := string(got)
			if gots != test.Want {
				t.Fatalf("bad output:\n%s", diff.Format(test.Want+"\n", gots+"\n"))
			}
		})
	}
}
