// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package vdm

import (
	"path/filepath"
	"testing"

	"golang.org/x/tools/txtar"
	"rsc.io/diff"
)

func TestPos_String(t *testing.T) {
	tests := []struct {
		Name string
		Pos  Pos
		Want string
	}{
		{
			Name: "zero-value",
			Pos:  Pos{},
			Want: "???:?",
		},
		{
			Name: "no-file",
			Pos:  Pos{Line: 3},
			Want: "???:3",
		},
		{
			Name: "no-line",
			Pos:  Pos{File: "foo.txt"},
			Want: "foo.txt:?",
		},
		{
			Name: "normal",
			Pos:  Pos{File: "foo.txt", Line: 3},
			Want: "foo.txt:3",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got := test.Pos.String()
			if got != test.Want {
				t.Fatalf("%#v.String():\nGot:  %q\nWant: %q", test.Pos, got, test.Want)
			}
		})
	}
}

func TestVectorsDeps_Sort(t *testing.T) {
	tests := []string{
		"nil",
		"complex",
	}

	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			ar, err := txtar.ParseFile(filepath.Join("testdata", "sorting", test+".txtar"))
			if err != nil {
				t.Fatal(err)
			}

			if len(ar.Files) < 0 || 2 < len(ar.Files) {
				t.Fatalf("got %d files, want 0-2", len(ar.Files))
			}

			if len(ar.Files) >= 1 && ar.Files[0].Name != "input.vdm" ||
				len(ar.Files) == 2 && ar.Files[1].Name != "sorted.vdm" {
				names := make([]string, len(ar.Files))
				for i, f := range ar.Files {
					names[i] = f.Name
				}

				t.Fatalf("got wrong filenames:\nGot:  %q\nWant: %q", names, []string{"input.vdm", "sorted.vdm"})
			}

			var inputDeps *Deps
			if len(ar.Files) >= 1 {
				inputDeps, err = ParseDeps(ar.Files[0].Name, string(ar.Files[0].Data))
				if err != nil {
					t.Fatalf("failed to parse %s: %v", ar.Files[0].Name, err)
				}
			}

			var sorted string
			if len(ar.Files) >= 2 {
				sorted = string(ar.Files[1].Data)
			}

			inputDeps.Sort()
			input := string(inputDeps.Encode())
			if input != sorted {
				t.Errorf("Deps.Sort(): order mismatch (-want, +got)\n%s", diff.Format(sorted, input))
			}
		})
	}
}
