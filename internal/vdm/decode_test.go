package vdm

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/tools/txtar"
)

func TestVectorsParseDeps(t *testing.T) {
	tests := []string{
		"empty",
		"simple",
		"commented",
		"complex",
	}

	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			ar, err := txtar.ParseFile(filepath.Join("testdata", "encoding", test+".txtar"))
			if err != nil {
				t.Fatal(err)
			}

			if len(ar.Files) != 2 {
				t.Fatalf("got %d files, want 2", len(ar.Files))
			}

			if ar.Files[0].Name != "deps.json" || ar.Files[1].Name != "deps.vdm" {
				t.Fatalf("got wrong filenames:\nGot:  %q, %q\nWant: %q, %q", ar.Files[0].Name, ar.Files[1].Name, "deps.json", "deps.vdm")
			}

			var want Deps
			err = json.Unmarshal(ar.Files[0].Data, &want)
			if err != nil {
				t.Fatalf("failed to parse JSON: %v", err)
			}

			got, err := ParseDeps("deps.vdm", string(ar.Files[1].Data))
			if err != nil {
				t.Fatalf("failed to parse VDM: %v", err)
			}

			if diff := cmp.Diff(&want, got); diff != "" {
				t.Errorf("ParseDeps(): deps mismatch (-want, +got)\n%s", diff)
			}
		})
	}
}

func TestParseDeps(t *testing.T) {
	tests := []struct {
		Name  string
		Data  []string
		Want  *Deps
		Error string
	}{
		{
			Name: "empty-nothing",
			Data: []string{""},
			Want: &Deps{},
		},
		{
			Name: "invalid-bad-keyword",
			Data: []string{
				`not-a-keyword`,
			},
			Error: `deps.vdm:1: expected a keyword, got invalid token "not-a-keyword"`,
		},

		// banned-go-packages
		{
			Name: "invalid-duplicate-banned-go-packages",
			Data: []string{
				`banned-go-packages:`,
				`	"example.com/foo"`,
				`banned-go-packages:`,
			},
			Error: `deps.vdm:3: duplicate banned Go package set, first found at deps.vdm:2`,
		},
		{
			Name: "invalid-missing-banned-go-packages",
			Data: []string{
				`banned-go-packages:`,
				``,
			},
			Error: "deps.vdm:2: expected a quoted string after banned-go-packages:, got EOF",
		},
		{
			Name: "invalid-bad-banned-go-packages",
			Data: []string{
				`banned-go-packages:`,
				`	"first"`,
				`		"second"`,
			},
			Error: "deps.vdm:3: expected a quoted string, got excessive indentation",
		},
		{
			Name: "valid-simple-banned-go-packages",
			Data: []string{
				`banned-go-packages:`,
				`	"first"`,
				`	"second" // Check comments work.`,
			},
			Want: &Deps{
				BannedGoPackages: []ParsedString{
					{
						Value: "first",
						Pos:   Pos{File: "deps.vdm", Line: 2},
					},
					{
						Value:   "second",
						Pos:     Pos{File: "deps.vdm", Line: 3},
						Comment: "Check comments work.",
					},
				},
			},
		},

		// go-modules
		{
			Name: "invalid-duplicate-go-modules",
			Data: []string{
				`go-modules:`,
				`	module "example.com/foo" v1.2.3`,
				`go-modules:`,
			},
			Error: `deps.vdm:3: duplicate Go module set, first found at deps.vdm:2`,
		},
		{
			Name: "invalid-missing-go-modules",
			Data: []string{
				`go-modules:`,
				``,
			},
			Error: `deps.vdm:2: no Go modules provided after "go-modules" keyword at deps.vdm:1`,
		},
		{
			Name: "invalid-bad-go-modules",
			Data: []string{
				`go-modules:`,
				`	"first"`,
				`		"second"`,
			},
			Error: `deps.vdm:2: expected keyword "module", got invalid token "\t\"first\""`,
		},
		{
			Name: "valid-simple-go-modules",
			Data: []string{
				`go-modules:`,
				`	module "first" v1.2.3`,
			},
			Want: &Deps{
				GoModules: []*GoModule{
					{
						Name: "first",
						Version: ParsedString{
							Value: "v1.2.3",
							Pos:   Pos{File: "deps.vdm", Line: 2},
						},
					},
				},
			},
		},

		{
			Name: "invalid-unrecognised-keyword",
			Data: []string{
				``,
				`something-bizarre:`,
			},
			Error: `deps.vdm:2: unrecognised keyword "something-bizarre"`,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got, err := ParseDeps("deps.vdm", strings.Join(test.Data, "\n"))
			if test.Error != "" {
				if err == nil {
					t.Fatalf("ParseDeps(): unexpected lack of error")
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("ParseDeps(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("ParseDeps(): got unexpected error: %v", err)
			}

			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Errorf("ParseDeps(): module mismatch (-want, +got)\n%s", diff)
			}
		})
	}
}

func TestParser_ParseGoModule(t *testing.T) {
	tests := []struct {
		Name  string
		Data  []string
		Want  *GoModule
		Error string
		Next  string
	}{
		{
			Name:  "empty-nothing",
			Data:  []string{""},
			Error: "EOF",
			Next:  "",
		},
		{
			Name: "empty-newlines",
			Data: []string{
				``,
				``,
				``,
			},
			Error: "EOF",
			Next:  "",
		},
		{
			Name: "valid-missing",
			Data: []string{
				``,
				``,
				`go:`,
			},
			Next: "go:",
		},
		{
			Name: "invalid-no-name",
			Data: []string{
				`	module `,
			},
			Error: `deps.vdm:1: expected module name, got EOF`,
		},
		{
			Name: "invalid-bad-name",
			Data: []string{
				`	module "\x"`,
			},
			Error: `deps.vdm:1: expected a quoted string, got invalid syntax`,
		},
		{
			Name: "invalid-no-space-after-name",
			Data: []string{
				`	module "foo"`,
			},
			Error: `deps.vdm:1: expected space after module name, got ""`,
		},
		{
			Name: "invalid-string-after-name",
			Data: []string{
				`	module "foo"bar`,
			},
			Error: `deps.vdm:1: expected space after module name, got "bar"`,
		},
		{
			Name: "invalid-no-version",
			Data: []string{
				`	module "foo" `,
			},
			Error: `deps.vdm:1: expected module version, got EOF`,
		},
		{
			Name: "invalid-bad-version",
			Data: []string{
				`	module "foo" BAD!`,
			},
			Error: `deps.vdm:1: expected a string, got '!'`,
		},
		{
			Name: "invalid-bad-version-comment",
			Data: []string{
				`	module "foo" v1.2.3 foo`,
			},
			Error: `deps.vdm:1: expected a newline or comment, got "foo"`,
		},
		{
			Name: "valid-simple",
			Data: []string{
				`	module "foo" v1.2.3`,
			},
			Want: &GoModule{
				Name: "foo",
				Version: ParsedString{
					Value: "v1.2.3",
					Pos:   Pos{File: "deps.vdm", Line: 1},
				},
			},
			Next: "",
		},
		{
			Name: "valid-simple-newline",
			Data: []string{
				`	module "foo" v1.2.3`,
				``,
			},
			Want: &GoModule{
				Name: "foo",
				Version: ParsedString{
					Value: "v1.2.3",
					Pos:   Pos{File: "deps.vdm", Line: 1},
				},
			},
			Next: "",
		},
		{
			Name: "valid-simple-comment",
			Data: []string{
				`	module "foo" v1.2.3 // Simple.`,
				``,
			},
			Want: &GoModule{
				Name: "foo",
				Version: ParsedString{
					Value:   "v1.2.3",
					Pos:     Pos{File: "deps.vdm", Line: 1},
					Comment: "Simple.",
				},
			},
			Next: "",
		},
		{
			Name: "invalid-duplicate-bad-keyword",
			Data: []string{
				`	module "foo" v1.2.3`,
				`		patch-args`,
			},
			Error: `deps.vdm:2: expected a keyword, got invalid token "\t\tpatch-args"`,
		},
		{
			Name: "invalid-duplicate-unrecognised-keyword",
			Data: []string{
				`	module "foo" v1.2.3`,
				`		wibble:`,
			},
			Error: `deps.vdm:2: unrecognised keyword "wibble"`,
		},
		{
			Name: "invalid-duplicate-patch-args",
			Data: []string{
				`	module "foo" v1.2.3`,
				`		patch-args:`,
				`			"-first"`,
				`		patch-args:`,
				`			"-second"`,
			},
			Error: "deps.vdm:4: duplicate patch args, first found at deps.vdm:3",
		},
		{
			Name: "invalid-bad-patch-args",
			Data: []string{
				`	module "foo" v1.2.3`,
				`		patch-args:`,
				`			"-first"`,
				`				"-second"`,
			},
			Error: "deps.vdm:4: expected a quoted string, got excessive indentation",
		},
		{
			Name: "invalid-duplicate-patches",
			Data: []string{
				`	module "foo" v1.2.3`,
				`		patches:`,
				`			"first"`,
				`		patches:`,
				`			"second"`,
			},
			Error: "deps.vdm:4: duplicate patches, first found at deps.vdm:3",
		},
		{
			Name: "invalid-bad-patches",
			Data: []string{
				`	module "foo" v1.2.3`,
				`		patches:`,
				`			"first"`,
				`				"second"`,
			},
			Error: "deps.vdm:4: expected a quoted string, got excessive indentation",
		},
		{
			Name: "invalid-duplicate-packages",
			Data: []string{
				`	module "foo" v1.2.3`,
				`		packages:`,
				`			package "first"`,
				`		packages:`,
				`			package "second"`,
			},
			Error: "deps.vdm:4: duplicate Go package set, first found at deps.vdm:3",
		},
		{
			Name: "invalid-missing-packages",
			Data: []string{
				`	module "foo" v1.2.3`,
				`		packages:`,
			},
			Error: `deps.vdm:2: no Go packages provided after "packages" keyword at deps.vdm:2`,
		},
		{
			Name: "valid-simple-packages-eof",
			Data: []string{
				`	module "foo" v1.2.3`,
				`		packages:`,
				`			package "first"`,
				`			package "second"`,
			},
			Want: &GoModule{
				Name: "foo",
				Version: ParsedString{
					Value: "v1.2.3",
					Pos:   Pos{File: "deps.vdm", Line: 1},
				},
				Packages: []*GoPackage{
					{
						Name: ParsedString{
							Value: "first",
							Pos:   Pos{File: "deps.vdm", Line: 3},
						},
					},
					{
						Name: ParsedString{
							Value: "second",
							Pos:   Pos{File: "deps.vdm", Line: 4},
						},
					},
				},
			},
		},
		{
			Name: "valid-simple-packages-suffixed",
			Data: []string{
				`	module "foo" v1.2.3`,
				`		packages:`,
				`			package "first"`,
				`			package "second"`,
				`	other`,
			},
			Want: &GoModule{
				Name: "foo",
				Version: ParsedString{
					Value: "v1.2.3",
					Pos:   Pos{File: "deps.vdm", Line: 1},
				},
				Packages: []*GoPackage{
					{
						Name: ParsedString{
							Value: "first",
							Pos:   Pos{File: "deps.vdm", Line: 3},
						},
					},
					{
						Name: ParsedString{
							Value: "second",
							Pos:   Pos{File: "deps.vdm", Line: 4},
						},
					},
				},
			},
			Next: "\tother",
		},
		{
			Name: "invalid-bad-packages",
			Data: []string{
				`	module "foo" v1.2.3`,
				`		packages:`,
				`			package "first"`,
				`				"second"`,
			},
			Error: `deps.vdm:4: expected a keyword, got invalid token "\t\t\t\t\"second\""`,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			p := &parser{
				Data: strings.Join(test.Data, "\n"),
				File: "deps.vdm",
				Line: 1,
			}

			got, err := p.ParseGoModule()
			if test.Error != "" {
				if err == nil {
					t.Fatalf("parser.ParseGoModule(): unexpected lack of error")
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("parser.ParseGoModule(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("parser.ParseGoModule(): got unexpected error: %v", err)
			}

			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Errorf("parser.ParseGoModule(): module mismatch (-want, +got)\n%s", diff)
			}

			if p.Data != test.Next {
				t.Errorf("parser.ParseGoModule(): got wrong next data:\nGot:  %q\nWant: %q", p.Data, test.Next)
			}
		})
	}
}

func TestParser_ParseGoPackage(t *testing.T) {
	tests := []struct {
		Name  string
		Data  []string
		Want  *GoPackage
		Error string
		Next  string
	}{
		{
			Name:  "empty-nothing",
			Data:  []string{""},
			Error: "EOF",
			Next:  "",
		},
		{
			Name: "valid-no-package",
			Data: []string{"foo"},
			Next: "foo",
		},
		{
			Name:  "invalid-no-package-name",
			Data:  []string{`			package `},
			Error: `deps.vdm:1: expected package name, got EOF`,
		},
		{
			Name:  "invalid-bad-package-name",
			Data:  []string{`			package foo`},
			Error: `deps.vdm:1: expected a quoted string, got 'f'`,
		},
		{
			Name:  "invalid-bad-package-name-comment",
			Data:  []string{`			package "foo" /`},
			Error: `deps.vdm:1: expected a newline or comment, got "/"`,
		},
		{
			Name: "valid-simple-eof",
			Data: []string{
				`			package "foo"`,
			},
			Want: &GoPackage{
				Name: ParsedString{
					Value: "foo",
					Pos:   Pos{File: "deps.vdm", Line: 1},
				},
			},
			Next: "",
		},
		{
			Name: "invalid-bad-keyword",
			Data: []string{
				`			package "foo"`,
				`					bar`,
			},
			Error: `deps.vdm:2: expected a keyword, got invalid token "\t\t\t\t\tbar"`,
		},
		{
			Name: "valid-simple-suffix",
			Data: []string{
				`			package "foo"`,
				`	bar`,
			},
			Want: &GoPackage{
				Name: ParsedString{
					Value: "foo",
					Pos:   Pos{File: "deps.vdm", Line: 1},
				},
			},
			Next: "\tbar",
		},

		// build-file
		{
			Name: "invalid-abutting-build-file",
			Data: []string{
				`			package "foo"`,
				`				build-file:"first"`,
			},
			Error: `deps.vdm:2: expected a space after build-file:, got "\"first\""`,
		},
		{
			Name: "invalid-duplicate-build-file",
			Data: []string{
				`			package "foo"`,
				`				build-file: "first"`,
				`				build-file: "second"`,
			},
			Error: `deps.vdm:3: duplicate build file, first found at deps.vdm:2`,
		},
		{
			Name: "invalid-missing-build-file",
			Data: []string{
				`			package "foo"`,
				`				build-file: `,
			},
			Error: `deps.vdm:2: expected a quoted string after build-file:, got EOF`,
		},
		{
			Name: "invalid-bad-build-file",
			Data: []string{
				`			package "foo"`,
				`				build-file: first`,
			},
			Error: `deps.vdm:2: expected a quoted string, got 'f'`,
		},
		{
			Name: "valid-simple-build-file",
			Data: []string{
				`			package "foo"`,
				`				build-file: "BUILD.bazel" // Check comments work.`,
			},
			Want: &GoPackage{
				Name: ParsedString{
					Value: "foo",
					Pos:   Pos{File: "deps.vdm", Line: 1},
				},
				BuildFile: ParsedString{
					Value:   "BUILD.bazel",
					Pos:     Pos{File: "deps.vdm", Line: 2},
					Comment: "Check comments work.",
				},
			},
			Next: "",
		},

		// deps
		{
			Name: "invalid-duplicate-deps",
			Data: []string{
				`			package "foo"`,
				`				deps:`,
				`					"first"`,
				`				deps:`,
			},
			Error: "deps.vdm:4: duplicate deps, first found at deps.vdm:3",
		},
		{
			Name: "invalid-missing-deps",
			Data: []string{
				`			package "foo"`,
				`				deps:`,
				``,
			},
			Error: "deps.vdm:3: expected a quoted string after deps:, got EOF",
		},
		{
			Name: "invalid-bad-deps",
			Data: []string{
				`			package "foo"`,
				`				deps:`,
				`					"first"`,
				`						"-second"`,
			},
			Error: "deps.vdm:4: expected a quoted string, got excessive indentation",
		},
		{
			Name: "valid-simple-deps",
			Data: []string{
				`			package "foo"`,
				`				deps:`,
				`					"first"`,
				`					"second" // Check comments work.`,
			},
			Want: &GoPackage{
				Name: ParsedString{
					Value: "foo",
					Pos:   Pos{File: "deps.vdm", Line: 1},
				},
				Deps: []ParsedString{
					{
						Value: "first",
						Pos:   Pos{File: "deps.vdm", Line: 3},
					},
					{
						Value:   "second",
						Pos:     Pos{File: "deps.vdm", Line: 4},
						Comment: "Check comments work.",
					},
				},
			},
			Next: "",
		},

		// embed
		{
			Name: "invalid-duplicate-embed",
			Data: []string{
				`			package "foo"`,
				`				embed:`,
				`					"first"`,
				`				embed:`,
			},
			Error: "deps.vdm:4: duplicate embed, first found at deps.vdm:3",
		},
		{
			Name: "invalid-missing-embed",
			Data: []string{
				`			package "foo"`,
				`				embed:`,
				``,
			},
			Error: "deps.vdm:3: expected a quoted string after embed:, got EOF",
		},
		{
			Name: "invalid-bad-embed",
			Data: []string{
				`			package "foo"`,
				`				embed:`,
				`					"first"`,
				`						"-second"`,
			},
			Error: "deps.vdm:4: expected a quoted string, got excessive indentation",
		},
		{
			Name: "valid-simple-embed",
			Data: []string{
				`			package "foo"`,
				`				embed:`,
				`					"first"`,
				`					"second" // Check comments work.`,
			},
			Want: &GoPackage{
				Name: ParsedString{
					Value: "foo",
					Pos:   Pos{File: "deps.vdm", Line: 1},
				},
				Embed: []ParsedString{
					{
						Value: "first",
						Pos:   Pos{File: "deps.vdm", Line: 3},
					},
					{
						Value:   "second",
						Pos:     Pos{File: "deps.vdm", Line: 4},
						Comment: "Check comments work.",
					},
				},
			},
			Next: "",
		},

		// embed-globs
		{
			Name: "invalid-duplicate-embed-globs",
			Data: []string{
				`			package "foo"`,
				`				embed-globs:`,
				`					"first"`,
				`				embed-globs:`,
			},
			Error: "deps.vdm:4: duplicate embed globs, first found at deps.vdm:3",
		},
		{
			Name: "invalid-missing-embed-globs",
			Data: []string{
				`			package "foo"`,
				`				embed-globs:`,
				``,
			},
			Error: "deps.vdm:3: expected a quoted string after embed-globs:, got EOF",
		},
		{
			Name: "invalid-bad-embed-globs",
			Data: []string{
				`			package "foo"`,
				`				embed-globs:`,
				`					"first"`,
				`						"-second"`,
			},
			Error: "deps.vdm:4: expected a quoted string, got excessive indentation",
		},
		{
			Name: "valid-simple-embed-globs",
			Data: []string{
				`			package "foo"`,
				`				embed-globs:`,
				`					"first"`,
				`					"second" // Check comments work.`,
			},
			Want: &GoPackage{
				Name: ParsedString{
					Value: "foo",
					Pos:   Pos{File: "deps.vdm", Line: 1},
				},
				EmbedGlobs: []ParsedString{
					{
						Value: "first",
						Pos:   Pos{File: "deps.vdm", Line: 3},
					},
					{
						Value:   "second",
						Pos:     Pos{File: "deps.vdm", Line: 4},
						Comment: "Check comments work.",
					},
				},
			},
			Next: "",
		},

		// binary
		{
			Name: "invalid-abutting-binary",
			Data: []string{
				`			package "foo"`,
				`				binary:true`,
			},
			Error: `deps.vdm:2: expected a space after binary:, got "true"`,
		},
		{
			Name: "invalid-duplicate-binary",
			Data: []string{
				`			package "foo"`,
				`				binary: true`,
				`				binary: false`,
			},
			Error: `deps.vdm:3: duplicate binary, first found at deps.vdm:2`,
		},
		{
			Name: "invalid-missing-binary",
			Data: []string{
				`			package "foo"`,
				`				binary: `,
			},
			Error: `deps.vdm:2: expected a boolean after binary:, got EOF`,
		},
		{
			Name: "invalid-bad-binary",
			Data: []string{
				`			package "foo"`,
				`				binary: first`,
			},
			Error: `deps.vdm:2: expected a boolean, got "first"`,
		},
		{
			Name: "valid-simple-binary",
			Data: []string{
				`			package "foo"`,
				`				binary: true // Check comments work.`,
			},
			Want: &GoPackage{
				Name: ParsedString{
					Value: "foo",
					Pos:   Pos{File: "deps.vdm", Line: 1},
				},
				Binary: ParsedBool{
					Value:   true,
					Pos:     Pos{File: "deps.vdm", Line: 2},
					Comment: "Check comments work.",
				},
			},
			Next: "",
		},

		// binary-deps
		{
			Name: "invalid-duplicate-binary-deps",
			Data: []string{
				`			package "foo"`,
				`				binary-deps:`,
				`					"first"`,
				`				binary-deps:`,
			},
			Error: "deps.vdm:4: duplicate binary deps, first found at deps.vdm:3",
		},
		{
			Name: "invalid-missing-binary-deps",
			Data: []string{
				`			package "foo"`,
				`				binary-deps:`,
				``,
			},
			Error: "deps.vdm:3: expected a quoted string after binary-deps:, got EOF",
		},
		{
			Name: "invalid-bad-binary-deps",
			Data: []string{
				`			package "foo"`,
				`				binary-deps:`,
				`					"first"`,
				`						"-second"`,
			},
			Error: "deps.vdm:4: expected a quoted string, got excessive indentation",
		},
		{
			Name: "valid-simple-binary-deps",
			Data: []string{
				`			package "foo"`,
				`				binary-deps:`,
				`					"first"`,
				`					"second" // Check comments work.`,
			},
			Want: &GoPackage{
				Name: ParsedString{
					Value: "foo",
					Pos:   Pos{File: "deps.vdm", Line: 1},
				},
				BinaryDeps: []ParsedString{
					{
						Value: "first",
						Pos:   Pos{File: "deps.vdm", Line: 3},
					},
					{
						Value:   "second",
						Pos:     Pos{File: "deps.vdm", Line: 4},
						Comment: "Check comments work.",
					},
				},
			},
			Next: "",
		},

		// no-tests
		{
			Name: "invalid-abutting-no-tests",
			Data: []string{
				`			package "foo"`,
				`				no-tests:true`,
			},
			Error: `deps.vdm:2: expected a space after no-tests:, got "true"`,
		},
		{
			Name: "invalid-duplicate-no-tests",
			Data: []string{
				`			package "foo"`,
				`				no-tests: true`,
				`				no-tests: false`,
			},
			Error: `deps.vdm:3: duplicate no tests, first found at deps.vdm:2`,
		},
		{
			Name: "invalid-missing-no-tests",
			Data: []string{
				`			package "foo"`,
				`				no-tests: `,
			},
			Error: `deps.vdm:2: expected a boolean after no-tests:, got EOF`,
		},
		{
			Name: "invalid-bad-no-tests",
			Data: []string{
				`			package "foo"`,
				`				no-tests: first`,
			},
			Error: `deps.vdm:2: expected a boolean, got "first"`,
		},
		{
			Name: "valid-simple-no-tests",
			Data: []string{
				`			package "foo"`,
				`				no-tests: true // Check comments work.`,
			},
			Want: &GoPackage{
				Name: ParsedString{
					Value: "foo",
					Pos:   Pos{File: "deps.vdm", Line: 1},
				},
				NoTests: ParsedBool{
					Value:   true,
					Pos:     Pos{File: "deps.vdm", Line: 2},
					Comment: "Check comments work.",
				},
			},
			Next: "",
		},

		// test-size
		{
			Name: "invalid-abutting-test-size",
			Data: []string{
				`			package "foo"`,
				`				test-size:first`,
			},
			Error: `deps.vdm:2: expected a space after test-size:, got "first"`,
		},
		{
			Name: "invalid-duplicate-test-size",
			Data: []string{
				`			package "foo"`,
				`				test-size: first`,
				`				test-size: second`,
			},
			Error: `deps.vdm:3: duplicate test size, first found at deps.vdm:2`,
		},
		{
			Name: "invalid-missing-test-size",
			Data: []string{
				`			package "foo"`,
				`				test-size: `,
			},
			Error: `deps.vdm:2: expected a string after test-size:, got EOF`,
		},
		{
			Name: "invalid-bad-test-size",
			Data: []string{
				`			package "foo"`,
				`				test-size: !`,
			},
			Error: `deps.vdm:2: expected a string, got '!'`,
		},
		{
			Name: "valid-simple-test-size",
			Data: []string{
				`			package "foo"`,
				`				test-size: small // Check comments work.`,
			},
			Want: &GoPackage{
				Name: ParsedString{
					Value: "foo",
					Pos:   Pos{File: "deps.vdm", Line: 1},
				},
				TestSize: ParsedString{
					Value:   "small",
					Pos:     Pos{File: "deps.vdm", Line: 2},
					Comment: "Check comments work.",
				},
			},
			Next: "",
		},

		// test-data
		{
			Name: "invalid-duplicate-test-data",
			Data: []string{
				`			package "foo"`,
				`				test-data:`,
				`					"first"`,
				`				test-data:`,
			},
			Error: "deps.vdm:4: duplicate test data, first found at deps.vdm:3",
		},
		{
			Name: "invalid-missing-test-data",
			Data: []string{
				`			package "foo"`,
				`				test-data:`,
				``,
			},
			Error: "deps.vdm:3: expected a quoted string after test-data:, got EOF",
		},
		{
			Name: "invalid-bad-test-data",
			Data: []string{
				`			package "foo"`,
				`				test-data:`,
				`					"first"`,
				`						"-second"`,
			},
			Error: "deps.vdm:4: expected a quoted string, got excessive indentation",
		},
		{
			Name: "valid-simple-test-data",
			Data: []string{
				`			package "foo"`,
				`				test-data:`,
				`					"first"`,
				`					"second" // Check comments work.`,
			},
			Want: &GoPackage{
				Name: ParsedString{
					Value: "foo",
					Pos:   Pos{File: "deps.vdm", Line: 1},
				},
				TestData: []ParsedString{
					{
						Value: "first",
						Pos:   Pos{File: "deps.vdm", Line: 3},
					},
					{
						Value:   "second",
						Pos:     Pos{File: "deps.vdm", Line: 4},
						Comment: "Check comments work.",
					},
				},
			},
			Next: "",
		},

		// test-data-globs
		{
			Name: "invalid-duplicate-test-data-globs",
			Data: []string{
				`			package "foo"`,
				`				test-data-globs:`,
				`					"first"`,
				`				test-data-globs:`,
			},
			Error: "deps.vdm:4: duplicate test data globs, first found at deps.vdm:3",
		},
		{
			Name: "invalid-missing-test-data-globs",
			Data: []string{
				`			package "foo"`,
				`				test-data-globs:`,
				``,
			},
			Error: "deps.vdm:3: expected a quoted string after test-data-globs:, got EOF",
		},
		{
			Name: "invalid-bad-test-data-globs",
			Data: []string{
				`			package "foo"`,
				`				test-data-globs:`,
				`					"first"`,
				`						"-second"`,
			},
			Error: "deps.vdm:4: expected a quoted string, got excessive indentation",
		},
		{
			Name: "valid-simple-test-data-globs",
			Data: []string{
				`			package "foo"`,
				`				test-data-globs:`,
				`					"first"`,
				`					"second" // Check comments work.`,
			},
			Want: &GoPackage{
				Name: ParsedString{
					Value: "foo",
					Pos:   Pos{File: "deps.vdm", Line: 1},
				},
				TestDataGlobs: []ParsedString{
					{
						Value: "first",
						Pos:   Pos{File: "deps.vdm", Line: 3},
					},
					{
						Value:   "second",
						Pos:     Pos{File: "deps.vdm", Line: 4},
						Comment: "Check comments work.",
					},
				},
			},
			Next: "",
		},

		// test-deps
		{
			Name: "invalid-duplicate-test-deps",
			Data: []string{
				`			package "foo"`,
				`				test-deps:`,
				`					"first"`,
				`				test-deps:`,
			},
			Error: "deps.vdm:4: duplicate test deps, first found at deps.vdm:3",
		},
		{
			Name: "invalid-missing-test-deps",
			Data: []string{
				`			package "foo"`,
				`				test-deps:`,
				``,
			},
			Error: "deps.vdm:3: expected a quoted string after test-deps:, got EOF",
		},
		{
			Name: "invalid-bad-test-deps",
			Data: []string{
				`			package "foo"`,
				`				test-deps:`,
				`					"first"`,
				`						"-second"`,
			},
			Error: "deps.vdm:4: expected a quoted string, got excessive indentation",
		},
		{
			Name: "valid-simple-test-deps",
			Data: []string{
				`			package "foo"`,
				`				test-deps:`,
				`					"first"`,
				`					"second" // Check comments work.`,
			},
			Want: &GoPackage{
				Name: ParsedString{
					Value: "foo",
					Pos:   Pos{File: "deps.vdm", Line: 1},
				},
				TestDeps: []ParsedString{
					{
						Value: "first",
						Pos:   Pos{File: "deps.vdm", Line: 3},
					},
					{
						Value:   "second",
						Pos:     Pos{File: "deps.vdm", Line: 4},
						Comment: "Check comments work.",
					},
				},
			},
			Next: "",
		},

		// test-env
		{
			Name: "invalid-duplicate-test-env",
			Data: []string{
				`			package "foo"`,
				`				test-env:`,
				`					"first": "left"`, // Test that we find the right first value.
				`					"second" :"right"`,
				`				test-env:`,
			},
			Error: "deps.vdm:5: duplicate test env, first found at deps.vdm:3",
		},
		{
			Name: "invalid-missing-test-env",
			Data: []string{
				`			package "foo"`,
				`				test-env:`,
				``,
			},
			Error: "deps.vdm:3: expected a quoted string after test-env:, got EOF",
		},
		{
			Name: "invalid-bad-test-env",
			Data: []string{
				`			package "foo"`,
				`				test-env:`,
				`					"first": "foo"`,
				`						"second"`,
			},
			Error: "deps.vdm:4: expected a quoted string, got excessive indentation",
		},
		{
			Name: "valid-simple-test-env",
			Data: []string{
				`			package "foo"`,
				`				test-env:`,
				`					"first": "one"`,
				`					"second": "two" // Check comments work.`,
			},
			Want: &GoPackage{
				Name: ParsedString{
					Value: "foo",
					Pos:   Pos{File: "deps.vdm", Line: 1},
				},
				TestEnv: map[string]ParsedString{
					"first": {
						Value: "one",
						Pos:   Pos{File: "deps.vdm", Line: 3},
					},
					"second": {
						Value:   "two",
						Pos:     Pos{File: "deps.vdm", Line: 4},
						Comment: "Check comments work.",
					},
				},
			},
			Next: "",
		},

		{
			Name: "invalid-unrecognised-keyword",
			Data: []string{
				`			package "foo"`,
				`				something-bizarre:`,
			},
			Error: `deps.vdm:2: unrecognised keyword "something-bizarre"`,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			p := &parser{
				Data: strings.Join(test.Data, "\n"),
				File: "deps.vdm",
				Line: 1,
			}

			got, err := p.ParseGoPackage()
			if test.Error != "" {
				if err == nil {
					t.Fatalf("parser.ParseGoPackage(): unexpected lack of error")
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("parser.ParseGoPackage(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("parser.ParseGoPackage(): got unexpected error: %v", err)
			}

			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Errorf("parser.ParseGoPackage(): module mismatch (-want, +got)\n%s", diff)
			}

			if p.Data != test.Next {
				t.Errorf("parser.ParseGoPackage(): got wrong next data:\nGot:  %q\nWant: %q", p.Data, test.Next)
			}
		})
	}
}

func TestParser_ParseComment(t *testing.T) {
	tests := []struct {
		Name  string
		Data  string
		Want  string
		Error string
		Next  string
		Line  int
	}{
		{
			Name: "empty",
			Data: "",
			Want: "",
			Line: 1,
		},
		{
			Name:  "invalid-no-comment-not-whitespace",
			Data:  `foo`,
			Error: `deps.vdm:1: expected a newline or comment, got "foo"`,
		},
		{
			Name: "valid-no-comment-newline",
			Data: "   \n",
			Next: "",
			Line: 2,
		},
		{
			Name: "valid-no-comment-EOF",
			Data: "   ",
			Next: "",
			Line: 2,
		},
		{
			Name:  "invalid-non-spaces-before-comment",
			Data:  "foo // bar\n",
			Error: `deps.vdm:1: expected a newline or comment, got "foo"`,
		},
		{
			Name: "valid-simple",
			Data: " // Foo bar.\n",
			Want: `Foo bar.`,
			Next: "",
			Line: 2,
		},
		{
			Name: "valid-spaced",
			Data: "           // Foo bar.\n",
			Want: `Foo bar.`,
			Next: "",
			Line: 2,
		},
		{
			Name: "valid-suffixed",
			Data: " // Foo bar.\nbar",
			Want: `Foo bar.`,
			Next: "bar",
			Line: 2,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			p := &parser{
				Data: test.Data,
				File: "deps.vdm",
				Line: 1,
			}

			got, err := p.ParseComment()
			if test.Error != "" {
				if err == nil {
					t.Fatalf("parser.ParseComment(): unexpected lack of error, got comment %q", got)
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("parser.ParseComment(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("parser.ParseComment(): got unexpected error: %v", err)
			}

			if got != test.Want {
				t.Errorf("parser.ParseComment(): got wrong comment:\nGot:  %q\nWant: %q", got, test.Want)
			}

			if p.Data != test.Next {
				t.Errorf("parser.ParseComment(): got wrong next data:\nGot:  %q\nWant: %q", p.Data, test.Next)
			}

			if p.Line != test.Line {
				t.Errorf("parser.ParseComment(): got wrong final line:\nGot:  %d\nWant: %d", p.Line, test.Line)
			}
		})
	}
}

func TestParser_ParseBool(t *testing.T) {
	tests := []struct {
		Name  string
		Data  string
		Want  bool
		Error string
		Next  string
	}{
		{
			Name:  "empty",
			Data:  "",
			Error: `EOF`,
		},
		{
			Name:  "invalid-bad-string",
			Data:  `"`,
			Error: `deps.vdm:1: expected a boolean, got "\""`,
		},
		{
			Name: "valid-simple",
			Data: `true `,
			Want: true,
			Next: " ",
		},
		{
			Name: "valid-suffixed",
			Data: `false suffix`,
			Want: false,
			Next: " suffix",
		},
		{
			Name: "valid-EOF",
			Data: `true`,
			Want: true,
			Next: "",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			p := &parser{
				Data: test.Data,
				File: "deps.vdm",
				Line: 1,
			}

			got, err := p.ParseBool()
			if test.Error != "" {
				if err == nil {
					t.Fatalf("parser.ParseBool(): unexpected lack of error, got bool %v", got)
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("parser.ParseBool(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("parser.ParseBool(): got unexpected error: %v", err)
			}

			if got != test.Want {
				t.Errorf("parser.ParseBool(): got wrong bool:\nGot:  %v\nWant: %v", got, test.Want)
			}

			if p.Data != test.Next {
				t.Errorf("parser.ParseBool(): got wrong next data:\nGot:  %q\nWant: %q", p.Data, test.Next)
			}
		})
	}
}

func TestParser_ParseQuotedString(t *testing.T) {
	tests := []struct {
		Name  string
		Data  string
		Want  string
		Error string
		Next  string
	}{
		{
			Name:  "empty",
			Data:  "",
			Error: `EOF`,
		},
		{
			Name:  "invalid-no-opening-quotes",
			Data:  ":",
			Error: `deps.vdm:1: expected a quoted string, got ':'`,
		},
		{
			Name:  "invalid-bad-string",
			Data:  `"\x"`,
			Error: `deps.vdm:1: expected a quoted string, got invalid syntax`,
		},
		{
			Name: "valid-simple",
			Data: `"foo"`,
			Want: "foo",
			Next: "",
		},
		{
			Name: "valid-escaped-suffixed",
			Data: `"foo \"\n bar" suffix`,
			Want: "foo \"\n bar",
			Next: " suffix",
		},
		{
			Name:  "invalid-unterminated-newline",
			Data:  "\"foo\n",
			Error: `deps.vdm:1: unterminated string`,
		},
		{
			Name:  "invalid-unterminated-EOF",
			Data:  `"foo`,
			Error: `deps.vdm:1: unterminated string`,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			p := &parser{
				Data: test.Data,
				File: "deps.vdm",
				Line: 1,
			}

			got, err := p.ParseQuotedString()
			if test.Error != "" {
				if err == nil {
					t.Fatalf("parser.ParseQuotedString(): unexpected lack of error, got string %q", got)
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("parser.ParseQuotedString(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("parser.ParseQuotedString(): got unexpected error: %v", err)
			}

			if got != test.Want {
				t.Errorf("parser.ParseQuotedString(): got wrong string:\nGot:  %q\nWant: %q", got, test.Want)
			}

			if p.Data != test.Next {
				t.Errorf("parser.ParseQuotedString(): got wrong next data:\nGot:  %q\nWant: %q", p.Data, test.Next)
			}
		})
	}
}

func TestParser_ParseString(t *testing.T) {
	tests := []struct {
		Name  string
		Data  string
		Want  string
		Error string
		Next  string
	}{
		{
			Name:  "empty",
			Data:  "",
			Error: `EOF`,
		},
		{
			Name:  "invalid-bad-string",
			Data:  `"`,
			Error: `deps.vdm:1: expected a string, got '"'`,
		},
		{
			Name:  "invalid-empty-string-space",
			Data:  ` `,
			Error: `deps.vdm:1: expected a string, got space`,
		},
		{
			Name:  "invalid-empty-string-newline",
			Data:  "\n",
			Error: `deps.vdm:1: expected a string, got nothing`,
		},
		{
			Name: "valid-simple",
			Data: `foo `,
			Want: "foo",
			Next: " ",
		},
		{
			Name: "valid-suffixed",
			Data: `foo-bar suffix`,
			Want: "foo-bar",
			Next: " suffix",
		},
		{
			Name: "valid-EOF",
			Data: `foobar`,
			Want: "foobar",
			Next: "",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			p := &parser{
				Data: test.Data,
				File: "deps.vdm",
				Line: 1,
			}

			got, err := p.ParseString()
			if test.Error != "" {
				if err == nil {
					t.Fatalf("parser.ParseString(): unexpected lack of error, got string %q", got)
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("parser.ParseString(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("parser.ParseString(): got unexpected error: %v", err)
			}

			if got != test.Want {
				t.Errorf("parser.ParseString(): got wrong string:\nGot:  %q\nWant: %q", got, test.Want)
			}

			if p.Data != test.Next {
				t.Errorf("parser.ParseString(): got wrong next data:\nGot:  %q\nWant: %q", p.Data, test.Next)
			}
		})
	}
}

func TestParser_ParseParsedBool(t *testing.T) {
	tests := []struct {
		Name  string
		Data  string
		Want  ParsedBool
		Error string
		Next  string
	}{
		{
			Name:  "empty",
			Data:  "",
			Error: `EOF`,
		},
		{
			Name:  "invalid-bad-bool",
			Data:  `"`,
			Error: `deps.vdm:1: expected a boolean, got "\""`,
		},
		{
			Name: "valid-simple",
			Data: `true `,
			Want: ParsedBool{
				Value: true,
				Pos:   Pos{File: "deps.vdm", Line: 1},
			},
			Next: "",
		},
		{
			Name: "valid-EOF",
			Data: `false`,
			Want: ParsedBool{
				Value: false,
				Pos:   Pos{File: "deps.vdm", Line: 1},
			},
			Next: "",
		},
		{
			Name: "valid-commented",
			Data: `false // Comment.`,
			Want: ParsedBool{
				Value:   false,
				Pos:     Pos{File: "deps.vdm", Line: 1},
				Comment: "Comment.",
			},
			Next: "",
		},
		{
			Name:  "invalid-suffixed",
			Data:  `false suffix`,
			Error: `deps.vdm:1: expected a newline or comment, got "suffix"`,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			p := &parser{
				Data: test.Data,
				File: "deps.vdm",
				Line: 1,
			}

			got, err := p.ParseParsedBool()
			if test.Error != "" {
				if err == nil {
					t.Fatalf("parser.ParseParsedBool(): unexpected lack of error, got bool %v", got)
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("parser.ParseParsedBool(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("parser.ParseParsedBool(): got unexpected error: %v", err)
			}

			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Errorf("parser.ParseParsedBool(): parsed bool mismatch (-want, +got)\n%s", diff)
			}

			if p.Data != test.Next {
				t.Errorf("parser.ParseParsedBool(): got wrong next data:\nGot:  %q\nWant: %q", p.Data, test.Next)
			}
		})
	}
}

func TestParser_ParseQuotedParsedString(t *testing.T) {
	tests := []struct {
		Name  string
		Data  string
		Want  ParsedString
		Error string
		Next  string
	}{
		{
			Name:  "empty",
			Data:  "",
			Error: `EOF`,
		},
		{
			Name:  "invalid-no-opening-quotes",
			Data:  ":",
			Error: `deps.vdm:1: expected a quoted string, got ':'`,
		},
		{
			Name:  "invalid-bad-string",
			Data:  `"\x"`,
			Error: `deps.vdm:1: expected a quoted string, got invalid syntax`,
		},
		{
			Name: "valid-simple",
			Data: `"foo"`,
			Want: ParsedString{
				Value: "foo",
				Pos:   Pos{File: "deps.vdm", Line: 1},
			},
			Next: "",
		},
		{
			Name: "valid-escaped-suffixed",
			Data: "\"foo \\\"\\n bar\"\nsuffix",
			Want: ParsedString{
				Value: "foo \"\n bar",
				Pos:   Pos{File: "deps.vdm", Line: 1},
			},
			Next: "suffix",
		},
		{
			Name: "valid-commented",
			Data: `"foo"    // bar`,
			Want: ParsedString{
				Value:   "foo",
				Pos:     Pos{File: "deps.vdm", Line: 1},
				Comment: "bar",
			},
			Next: "",
		},
		{
			Name:  "invalid-unterminated-newline",
			Data:  "\"foo\n",
			Error: `deps.vdm:1: unterminated string`,
		},
		{
			Name:  "invalid-unterminated-EOF",
			Data:  `"foo`,
			Error: `deps.vdm:1: unterminated string`,
		},
		{
			Name:  "invalid-bad-comment",
			Data:  `"foo" bar`,
			Error: `deps.vdm:1: expected a newline or comment, got "bar"`,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			p := &parser{
				Data: test.Data,
				File: "deps.vdm",
				Line: 1,
			}

			got, err := p.ParseQuotedParsedString()
			if test.Error != "" {
				if err == nil {
					t.Fatalf("parser.ParseQuotedParsedString(): unexpected lack of error, got string %q", got)
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("parser.ParseQuotedParsedString(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("parser.ParseQuotedParsedString(): got unexpected error: %v", err)
			}

			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Errorf("parser.ParseQuotedParsedString(): parsed string mismatch (-want, +got)\n%s", diff)
			}

			if p.Data != test.Next {
				t.Errorf("parser.ParseQuotedParsedString(): got wrong next data:\nGot:  %q\nWant: %q", p.Data, test.Next)
			}
		})
	}
}

func TestParser_ParseParsedString(t *testing.T) {
	tests := []struct {
		Name  string
		Data  string
		Want  ParsedString
		Error string
		Next  string
	}{
		{
			Name:  "empty",
			Data:  "",
			Error: `EOF`,
		},
		{
			Name:  "invalid-bad-string",
			Data:  `"`,
			Error: `deps.vdm:1: expected a string, got '"'`,
		},
		{
			Name:  "invalid-empty-string-space",
			Data:  ` `,
			Error: `deps.vdm:1: expected a string, got space`,
		},
		{
			Name:  "invalid-empty-string-newline",
			Data:  "\n",
			Error: `deps.vdm:1: expected a string, got nothing`,
		},
		{
			Name: "valid-simple",
			Data: `foo `,
			Want: ParsedString{
				Value: "foo",
				Pos:   Pos{File: "deps.vdm", Line: 1},
			},
			Next: "",
		},
		{
			Name: "valid-EOF",
			Data: `foobar`,
			Want: ParsedString{
				Value: "foobar",
				Pos:   Pos{File: "deps.vdm", Line: 1},
			},
			Next: "",
		},
		{
			Name: "valid-commented",
			Data: `foo    // bar`,
			Want: ParsedString{
				Value:   "foo",
				Pos:     Pos{File: "deps.vdm", Line: 1},
				Comment: "bar",
			},
			Next: "",
		},
		{
			Name:  "invalid-suffixed",
			Data:  `foo-bar suffix`,
			Error: `deps.vdm:1: expected a newline or comment, got "suffix"`,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			p := &parser{
				Data: test.Data,
				File: "deps.vdm",
				Line: 1,
			}

			got, err := p.ParseParsedString()
			if test.Error != "" {
				if err == nil {
					t.Fatalf("parser.ParseParsedString(): unexpected lack of error, got string %q", got)
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("parser.ParseParsedString(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("parser.ParseParsedString(): got unexpected error: %v", err)
			}

			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Errorf("parser.ParseParsedString(): parsed string mismatch (-want, +got)\n%s", diff)
			}

			if p.Data != test.Next {
				t.Errorf("parser.ParseParsedString(): got wrong next data:\nGot:  %q\nWant: %q", p.Data, test.Next)
			}
		})
	}
}

func TestParser_FindColon(t *testing.T) {
	tests := []struct {
		Name    string
		Data    string
		Indents int
		Keyword string
		Error   string
		Next    string
	}{
		{
			Name:  "empty-nothing",
			Data:  "",
			Error: "EOF",
		},
		{
			Name:  "empty-newlines",
			Data:  "\n\n\n",
			Error: "EOF",
		},
		{
			Name:    "valid-exact",
			Data:    "go:",
			Indents: 0,
			Keyword: "go",
			Next:    "",
		},
		{
			Name:    "valid-prefixed",
			Data:    "\n\ngo:",
			Indents: 0,
			Keyword: "go",
			Next:    "",
		},
		{
			Name:    "valid-suffixed",
			Data:    "\n\ngo:\n\n",
			Indents: 0,
			Keyword: "go",
			Next:    "\n\n",
		},
		{
			Name:    "valid-indented",
			Data:    "\n\n\tgo:",
			Indents: 1,
			Keyword: "go",
			Next:    "",
		},
		{
			Name:    "valid-unindented",
			Data:    "go",
			Indents: 1,
			Keyword: "",
			Next:    "go",
		},
		{
			Name:  "invalid-no-colon-single-token",
			Data:  "go",
			Error: `deps.vdm:1: expected a keyword, got invalid token "go"`,
		},
		{
			Name:  "invalid-no-colon-prefixed-single-token",
			Data:  "\ngo",
			Error: `deps.vdm:2: expected a keyword, got invalid token "go"`,
		},
		{
			Name:  "invalid-no-colon-multi-token",
			Data:  " go for the moon ",
			Error: `deps.vdm:1: expected a keyword, got invalid token " go"`,
		},
		{
			Name:    "valid-no-keyword",
			Data:    "go:",
			Indents: 1,
			Keyword: "",
			Next:    "go:",
		},
		{
			Name:    "valid-no-keyword-prefixed",
			Data:    "\n\ngo:",
			Indents: 1,
			Keyword: "",
			Next:    "go:",
		},
		{
			Name:    "invalid-too-indented",
			Data:    "\tgo:",
			Indents: 0,
			Error:   `deps.vdm:1: expected a keyword, got excessive indentation`,
		},
		{
			Name:    "invalid-no-keyword",
			Data:    ":",
			Indents: 0,
			Error:   `deps.vdm:1: expected a keyword, got nothing`,
		},
		{
			Name:    "invalid-bad-keyword",
			Data:    "BAD:",
			Indents: 0,
			Error:   `deps.vdm:1: expected a keyword, got invalid rune 'B'`,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			p := &parser{
				Data: test.Data,
				File: "deps.vdm",
				Line: 1,
			}

			keyword, err := p.FindColon(test.Indents)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("parser.FindColon(%d): unexpected lack of error, got keyword %q", test.Indents, keyword)
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("parser.FindColon(%d): got wrong error:\nGot:  %s\nWant: %s", test.Indents, e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("parser.FindColon(%d): got unexpected error: %v", test.Indents, err)
			}

			if keyword != test.Keyword {
				t.Errorf("parser.FindColon(%d): got wrong keyword:\nGot:  %s\nWant: %s", test.Indents, keyword, test.Keyword)
			}

			if p.Data != test.Next {
				t.Errorf("parser.FindColon(%d): got wrong next data:\nGot:  %q\nWant: %q", test.Indents, p.Data, test.Next)
			}
		})
	}
}

func TestParser_FindKeyword(t *testing.T) {
	tests := []struct {
		Name    string
		Data    string
		Indents int
		Keyword string
		Ok      bool
		Error   string
		Next    string
	}{
		{
			Name:    "empty-nothing",
			Data:    "",
			Keyword: "go",
			Error:   "EOF",
		},
		{
			Name:    "empty-newlines",
			Data:    "\n\n\n",
			Keyword: "go",
			Error:   "EOF",
		},
		{
			Name:    "valid-exact",
			Data:    "go ",
			Indents: 0,
			Keyword: "go",
			Ok:      true,
			Next:    "",
		},
		{
			Name:    "valid-prefixed",
			Data:    "\n\ngo ",
			Indents: 0,
			Keyword: "go",
			Ok:      true,
			Next:    "",
		},
		{
			Name:    "valid-suffixed",
			Data:    "\n\ngo \n\n",
			Indents: 0,
			Keyword: "go",
			Ok:      true,
			Next:    "\n\n",
		},
		{
			Name:    "valid-indented",
			Data:    "\n\n\tgo ",
			Indents: 1,
			Keyword: "go",
			Ok:      true,
			Next:    "",
		},
		{
			Name:    "valid-less-indented",
			Data:    "\ngo",
			Indents: 1,
			Keyword: "go",
			Next:    "go",
		},
		{
			Name:    "invalid-no-space-single-token",
			Data:    "go",
			Keyword: "go",
			Error:   `deps.vdm:1: expected keyword "go", got invalid token "go"`,
		},
		{
			Name:    "invalid-no-space-prefixed-single-token",
			Data:    "\ngo",
			Keyword: "go",
			Error:   `deps.vdm:2: expected keyword "go", got invalid token "go"`,
		},
		{
			Name:    "valid-no-keyword",
			Data:    "go ",
			Indents: 1,
			Keyword: "go",
			Ok:      false,
			Next:    "go ",
		},
		{
			Name:    "valid-no-keyword-prefixed",
			Data:    "\n\ngo ",
			Indents: 1,
			Keyword: "go",
			Ok:      false,
			Next:    "go ",
		},
		{
			Name:    "invalid-too-indented",
			Data:    "\tgo ",
			Indents: 0,
			Keyword: "go",
			Error:   `deps.vdm:1: expected keyword "go", got excessive indentation`,
		},
		{
			Name:    "invalid-wrong-keyword",
			Data:    "other ",
			Indents: 0,
			Keyword: "go",
			Error:   `deps.vdm:1: expected keyword "go", got "other"`,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			p := &parser{
				Data: test.Data,
				File: "deps.vdm",
				Line: 1,
			}

			ok, err := p.FindKeyword(test.Indents, test.Keyword)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("parser.FindKeyword(%d, %q): unexpected lack of error", test.Indents, test.Keyword)
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("parser.FindKeyword(%d, %q): got wrong error:\nGot:  %s\nWant: %s", test.Indents, test.Keyword, e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("parser.FindKeyword(%d, %q): got unexpected error: %v", test.Indents, test.Keyword, err)
			}

			if ok != test.Ok {
				t.Errorf("parser.FindKeyword(%d, %q): got wrong success:\nGot:  %v\nWant: %v", test.Indents, test.Keyword, ok, test.Ok)
			}

			if p.Data != test.Next {
				t.Errorf("parser.FindKeyword(%d, %q): got wrong next data:\nGot:  %q\nWant: %q", test.Indents, test.Keyword, p.Data, test.Next)
			}
		})
	}
}

func TestParser_FindQuotedString(t *testing.T) {
	tests := []struct {
		Name    string
		Data    string
		Indents int
		Want    ParsedString
		Error   string
		Next    string
	}{
		{
			Name:  "empty",
			Data:  "",
			Error: `EOF`,
		},
		{
			Name:  "invalid-no-opening-quotes",
			Data:  ":",
			Error: `deps.vdm:1: expected a quoted string, got ":"`,
		},
		{
			Name:    "valid-less-indented-string",
			Data:    "\n\"foo\"",
			Indents: 1,
			Next:    "\"foo\"",
		},
		{
			Name:    "valid-less-indented-other",
			Data:    "\nfoo",
			Indents: 1,
			Next:    "foo",
		},
		{
			Name:    "invalid-too-indented",
			Data:    "\t\"foo\"",
			Indents: 0,
			Error:   `deps.vdm:1: expected a quoted string, got excessive indentation`,
		},
		{
			Name:  "invalid-bad-string",
			Data:  `"\x"`,
			Error: `deps.vdm:1: expected a quoted string, got invalid syntax`,
		},
		{
			Name: "valid-simple",
			Data: `"foo"`,
			Want: ParsedString{
				Value: "foo",
				Pos:   Pos{File: "deps.vdm", Line: 1},
			},
			Next: "",
		},
		{
			Name:    "valid-indented",
			Data:    `	"foo"`,
			Indents: 1,
			Want: ParsedString{
				Value: "foo",
				Pos:   Pos{File: "deps.vdm", Line: 1},
			},
			Next: "",
		},
		{
			Name: "valid-escaped-suffixed",
			Data: "\"foo \\\"\\n bar\"\n suffix",
			Want: ParsedString{
				Value: "foo \"\n bar",
				Pos:   Pos{File: "deps.vdm", Line: 1},
			},
			Next: " suffix",
		},
		{
			Name:    "valid-commented",
			Data:    "\n\n\t\t\"foo bar\"  // Lots of space here.\n\n",
			Indents: 2,
			Want: ParsedString{
				Value:   "foo bar",
				Pos:     Pos{File: "deps.vdm", Line: 3},
				Comment: "Lots of space here.",
			},
			Next: "\n",
		},
		{
			Name:  "invalid-unterminated-newline",
			Data:  "\"foo\n",
			Error: `deps.vdm:1: unterminated string`,
		},
		{
			Name:  "invalid-bad-comment",
			Data:  `"foo" /`,
			Error: `deps.vdm:1: expected a newline or comment, got "/"`,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			p := &parser{
				Data: test.Data,
				File: "deps.vdm",
				Line: 1,
			}

			got, err := p.FindQuotedString(test.Indents)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("parser.FindQuotedString(%d): unexpected lack of error, got string %q", test.Indents, got)
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("parser.FindQuotedString(%d): got wrong error:\nGot:  %s\nWant: %s", test.Indents, e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("parser.FindQuotedString(%d): got unexpected error: %v", test.Indents, err)
			}

			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Errorf("parser.FindQuotedString(%d): parsed string mismatch (-want, +got)\n%s", test.Indents, diff)
			}

			if p.Data != test.Next {
				t.Errorf("parser.FindQuotedString(%d): got wrong next data:\nGot:  %q\nWant: %q", test.Indents, p.Data, test.Next)
			}
		})
	}
}

func TestParser_FindQuotedStringKey(t *testing.T) {
	tests := []struct {
		Name    string
		Data    string
		Indents int
		Want    string
		Error   string
		Next    string
	}{
		{
			Name:  "empty",
			Data:  "",
			Error: `EOF`,
		},
		{
			Name:  "invalid-no-opening-quotes",
			Data:  ":",
			Error: `deps.vdm:1: expected a quoted string, got ":"`,
		},
		{
			Name:    "valid-less-indented-string",
			Data:    "\n\"foo\"",
			Indents: 1,
			Next:    "\"foo\"",
		},
		{
			Name:    "valid-less-indented-other",
			Data:    "\nfoo",
			Indents: 1,
			Next:    "foo",
		},
		{
			Name:    "invalid-too-indented",
			Data:    "\t\"foo\"",
			Indents: 0,
			Error:   `deps.vdm:1: expected a quoted string, got excessive indentation`,
		},
		{
			Name:  "invalid-bad-string",
			Data:  `"\x"`,
			Error: `deps.vdm:1: expected a quoted string, got invalid syntax`,
		},
		{
			Name: "valid-simple",
			Data: `"foo"`,
			Want: "foo",
			Next: "",
		},
		{
			Name:    "valid-indented",
			Data:    `	"foo"`,
			Indents: 1,
			Want:    "foo",
			Next:    "",
		},
		{
			Name: "valid-escaped-suffixed",
			Data: "\"foo \\\"\\n bar\"\n suffix",
			Want: "foo \"\n bar",
			Next: "\n suffix",
		},
		{
			Name:    "valid-commented",
			Data:    "\n\n\t\t\"foo bar\"  // Lots of space here.\n\n",
			Indents: 2,
			Want:    "foo bar",
			Next:    "  // Lots of space here.\n\n",
		},
		{
			Name:  "invalid-unterminated-newline",
			Data:  "\"foo\n",
			Error: `deps.vdm:1: unterminated string`,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			p := &parser{
				Data: test.Data,
				File: "deps.vdm",
				Line: 1,
			}

			got, err := p.FindQuotedStringKey(test.Indents)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("parser.FindQuotedStringKey(%d): unexpected lack of error, got string %q", test.Indents, got)
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("parser.FindQuotedStringKey(%d): got wrong error:\nGot:  %s\nWant: %s", test.Indents, e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("parser.FindQuotedStringKey(%d): got unexpected error: %v", test.Indents, err)
			}

			if got != test.Want {
				t.Errorf("parser.FindQuotedStringKey(%d): got wrong quoted string:\nGot:  %q\nWant: %q", test.Indents, got, test.Want)
			}

			if p.Data != test.Next {
				t.Errorf("parser.FindQuotedStringKey(%d): got wrong next data:\nGot:  %q\nWant: %q", test.Indents, p.Data, test.Next)
			}
		})
	}
}

func TestParser_FindQuotedStrings(t *testing.T) {
	tests := []struct {
		Name    string
		Data    []string
		Indents int
		Want    []ParsedString
		Error   string
		Next    string
	}{
		{
			Name:  "empty",
			Data:  []string{""},
			Error: `EOF`,
		},
		{
			Name:    "invalid-no-string",
			Data:    []string{"\nfoo"},
			Indents: 1,
			Error:   `deps.vdm:2: expected a quoted string, got "foo"`,
		},
		{
			Name:    "valid-one",
			Data:    []string{`	"foo"`},
			Indents: 1,
			Want: []ParsedString{
				{
					Value: "foo",
					Pos:   Pos{File: "deps.vdm", Line: 1},
				},
			},
			Next: "",
		},
		{
			Name: "invalid-bad-second-string",
			Data: []string{
				`	"foo"`,
				`	"bar`,
			},
			Indents: 1,
			Error:   `deps.vdm:2: unterminated string`,
		},
		{
			Name: "valid-no-second-string",
			Data: []string{
				`	"foo"`,
				`bar`,
			},
			Indents: 1,
			Want: []ParsedString{
				{
					Value: "foo",
					Pos:   Pos{File: "deps.vdm", Line: 1},
				},
			},
			Next: "bar",
		},
		{
			Name: "valid-second-string",
			Data: []string{
				`	"foo" // The first.`,
				`	"bar" // Second.`,
			},
			Indents: 1,
			Want: []ParsedString{
				{
					Value:   "foo",
					Pos:     Pos{File: "deps.vdm", Line: 1},
					Comment: "The first.",
				},
				{
					Value:   "bar",
					Pos:     Pos{File: "deps.vdm", Line: 2},
					Comment: "Second.",
				},
			},
			Next: "",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			p := &parser{
				Data: strings.Join(test.Data, "\n"),
				File: "deps.vdm",
				Line: 1,
			}

			got, err := p.FindQuotedStrings(test.Indents)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("parser.FindQuotedStrings(%d): unexpected lack of error, got string %q", test.Indents, got)
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("parser.FindQuotedStrings(%d): got wrong error:\nGot:  %s\nWant: %s", test.Indents, e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("parser.FindQuotedStrings(%d): got unexpected error: %v", test.Indents, err)
			}

			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Errorf("parser.FindQuotedStrings(%d): parsed string mismatch (-want, +got)\n%s", test.Indents, diff)
			}

			if p.Data != test.Next {
				t.Errorf("parser.FindQuotedStrings(%d): got wrong next data:\nGot:  %q\nWant: %q", test.Indents, p.Data, test.Next)
			}
		})
	}
}

func TestParser_FindQuotedStringsMap(t *testing.T) {
	tests := []struct {
		Name    string
		Data    []string
		Indents int
		Want    map[string]ParsedString
		Error   string
		Next    string
	}{
		{
			Name:  "empty",
			Data:  []string{""},
			Error: `EOF`,
		},
		{
			Name:    "invalid-no-key",
			Data:    []string{"\nfoo"},
			Indents: 1,
			Error:   `deps.vdm:2: expected a quoted string, got "foo"`,
		},
		{
			Name: "invalid-no-colon",
			Data: []string{
				``,
				`	"foo"`,
			},
			Indents: 1,
			Error:   `deps.vdm:2: expected a colon after map key, got ""`,
		},
		{
			Name: "invalid-no-value",
			Data: []string{
				``,
				`	"foo":`,
			},
			Indents: 1,
			Error:   `deps.vdm:2: expected a quoted string after map key, got EOF`,
		},
		{
			Name: "invalid-bad-value",
			Data: []string{
				``,
				`	"foo": bar`,
			},
			Indents: 1,
			Error:   `deps.vdm:2: expected a quoted string, got 'b'`,
		},
		{
			Name: "valid-one-pair",
			Data: []string{
				`	"foo": "bar"`,
			},
			Indents: 1,
			Want: map[string]ParsedString{
				"foo": {
					Value: "bar",
					Pos:   Pos{File: "deps.vdm", Line: 1},
				},
			},
			Next: "",
		},
		{
			Name: "invalid-bad-second-pair",
			Data: []string{
				`	"foo": "bar"`,
				`	"baz`,
			},
			Indents: 1,
			Error:   `deps.vdm:2: unterminated string`,
		},
		{
			Name: "valid-no-second-pair",
			Data: []string{
				`	"foo": "bar"`,
				`baz`,
			},
			Indents: 1,
			Want: map[string]ParsedString{
				"foo": {
					Value: "bar",
					Pos:   Pos{File: "deps.vdm", Line: 1},
				},
			},
			Next: "baz",
		},
		{
			Name: "invalid-no-second-colon",
			Data: []string{
				`	"foo": "bar"`,
				`	"baz"`,
			},
			Indents: 1,
			Error:   `deps.vdm:2: expected a colon after map key, got ""`,
		},
		{
			Name: "invalid-no-second-value",
			Data: []string{
				`	"foo": "bar"`,
				`	"baz":`,
			},
			Indents: 1,
			Error:   `deps.vdm:2: expected a quoted string after map key, got EOF`,
		},
		{
			Name: "invalid-bad-second-value",
			Data: []string{
				`	"foo": "bar"`,
				`	"baz": bamf`,
			},
			Indents: 1,
			Error:   `deps.vdm:2: expected a quoted string, got 'b'`,
		},
		{
			Name: "invalid-duplicate-second-value",
			Data: []string{
				`	"foo": "bar"`,
				`	"foo": "bamf"`,
			},
			Indents: 1,
			Error:   `deps.vdm:2: duplicate map key "foo", first found at deps.vdm:1`,
		},
		{
			Name: "valid-second-string",
			Data: []string{
				`	"foo": "bar" // The first.`,
				`	"baz": "bamf" // Second.`,
			},
			Indents: 1,
			Want: map[string]ParsedString{
				"foo": {
					Value:   "bar",
					Pos:     Pos{File: "deps.vdm", Line: 1},
					Comment: "The first.",
				},
				"baz": {
					Value:   "bamf",
					Pos:     Pos{File: "deps.vdm", Line: 2},
					Comment: "Second.",
				},
			},
			Next: "",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			p := &parser{
				Data: strings.Join(test.Data, "\n"),
				File: "deps.vdm",
				Line: 1,
			}

			got, err := p.FindQuotedStringsMap(test.Indents)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("parser.FindQuotedStringsMap(%d): unexpected lack of error", test.Indents)
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("parser.FindQuotedStringsMap(%d): got wrong error:\nGot:  %s\nWant: %s", test.Indents, e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("parser.FindQuotedStringsMap(%d): got unexpected error: %v", test.Indents, err)
			}

			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Errorf("parser.FindQuotedStringsMap(%d): parsed string mismatch (-want, +got)\n%s", test.Indents, diff)
			}

			if p.Data != test.Next {
				t.Errorf("parser.FindQuotedStringsMap(%d): got wrong next data:\nGot:  %q\nWant: %q", test.Indents, p.Data, test.Next)
			}
		})
	}
}
