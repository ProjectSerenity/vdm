// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package ves

import (
	"crypto/sha256"
	"encoding/base64"
	"path/filepath"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
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

func TestGoModule_Directories(t *testing.T) {
	tests := []struct {
		Name   string
		Module *GoModule
		Want   string
	}{
		{
			Name:   "nil",
			Module: nil,
			Want: func() string {
				sum := sha256.Sum256([]byte(nil))
				return "sha256:" + base64.StdEncoding.EncodeToString(sum[:])
			}(),
		},
		{
			Name: "simple",
			Module: &GoModule{
				Name: "rsc.io/diff",
				Packages: []*GoPackage{
					{
						Name: S("rsc.io/diff"),
					},
				},
			},
			Want: "sha256:8VGfNFMK4xqwLc8Mkm7zdIwtlTkuBgEp+V1zG6j5C5c=",
		},
		{
			Name: "complex",
			Module: &GoModule{
				Name: "rsc.io/diff",
				Packages: []*GoPackage{
					{
						Name: S("rsc.io/diff"),
						Directories: []ParsedString{
							{Value: "foo"},
							{Value: "bar"},
						},
					},
					{
						Name: S("rsc.io/quote"),
					},
				},
			},
			Want: "sha256:cOClr3lDh2FMO7KPCLTA2S0pasdS6NS1OXocSRSyhno=",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got := test.Module.Directories()
			if got != test.Want {
				t.Errorf("GoModule.Directories(): digest mismatch:\nGot:  %s\nWant: %s", got, test.Want)
			}
		})
	}
}

func TestGoModule_directories(t *testing.T) {
	tests := []struct {
		Name   string
		Module *GoModule
		Want   []string
	}{
		{
			Name:   "nil",
			Module: nil,
			Want:   nil,
		},
		{
			Name: "simple",
			Module: &GoModule{
				Name: "rsc.io/diff",
				Packages: []*GoPackage{
					{
						Name: S("rsc.io/diff"),
					},
				},
			},
			Want: []string{
				`package "rsc.io/diff"`,
			},
		},
		{
			Name: "complex",
			Module: &GoModule{
				Name: "rsc.io/diff",
				Packages: []*GoPackage{
					{
						Name: S("rsc.io/diff"),
						Directories: []ParsedString{
							{Value: "foo"},
							{Value: "bar"},
						},
					},
					{
						Name: S("rsc.io/quote"),
					},
				},
			},
			Want: []string{
				`package "rsc.io/diff"`,
				`directory "rsc.io/diff/foo"`,
				`directory "rsc.io/diff/bar"`,
				`package "rsc.io/quote"`,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got := slices.Collect(test.Module.directories())
			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Errorf("GoModule.directories(): lines mismatch (-want, +got)\n%s", diff)
			}

			// Check we can use the iterator to get the
			// first entry only.
			if len(test.Want) > 0 {
				var first string
				for line := range test.Module.directories() {
					first = line
					break
				}

				if first != test.Want[0] {
					t.Errorf("GoModule.directories(): first line mismatch:\nGot:  %q\nWant: %q", first, test.Want[0])
				}
			}

			// And again for the second.
			if len(test.Want) > 1 {
				var first, second string
				for line := range test.Module.directories() {
					if first == "" {
						first = line
					} else if second == "" {
						second = line
						break
					}
				}

				if first != test.Want[0] {
					t.Errorf("GoModule.directories(): first line mismatch:\nGot:  %q\nWant: %q", first, test.Want[0])
				}

				if second != test.Want[1] {
					t.Errorf("GoModule.directories(): second line mismatch:\nGot:  %q\nWant: %q", second, test.Want[1])
				}
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
