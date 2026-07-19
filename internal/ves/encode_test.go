// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package ves

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"
	"rsc.io/diff"
)

func TestVectorsDeps_Encode(t *testing.T) {
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

			var deps Deps
			err = json.Unmarshal(ar.Files[0].Data, &deps)
			if err != nil {
				t.Fatalf("failed to parse JSON: %v", err)
			}

			got := deps.Encode()
			if !bytes.Equal(got, ar.Files[1].Data) {
				t.Fatalf("bad output:\n%s", diff.Format(string(ar.Files[1].Data), string(got)))
			}
		})
	}
}

func TestDeps_Encode(t *testing.T) {
	tests := []struct {
		Name string
		Deps *Deps
		Want []string
	}{
		{
			Name: "nil",
			Deps: (*Deps)(nil),
			Want: []string{""},
		},
		{
			Name: "simple",
			Deps: &Deps{
				GoModules: []*GoModule{
					{
						Name: "rsc.io/diff",
						Version: ParsedString{
							Value: "v0.0.0-20190621135850-fe3479844c3c",
							Pos: Pos{
								File: "deps.vdm",
								Line: 2,
							},
						},
						Packages: []*GoPackage{
							{
								Name: ParsedString{
									Value: "rsc.io/diff",
									Pos: Pos{
										File: "deps.vdm",
										Line: 4,
									},
								},
							},
						},
					},
				},
			},
			Want: []string{
				`go-modules:`,
				`	module "rsc.io/diff" v0.0.0-20190621135850-fe3479844c3c`,
				`		packages:`,
				`			package "rsc.io/diff"`,
				``,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got := string(test.Deps.Encode())
			want := strings.Join(test.Want, "\n")
			if got != want {
				t.Fatalf("bad output:\n%s", diff.Format(want, got))
			}
		})
	}
}

func TestManifests_Encode(t *testing.T) {
	tests := []struct {
		Name      string
		Manifests *Manifests
		Want      []string
	}{
		{
			Name:      "nil",
			Manifests: (*Manifests)(nil),
			Want:      []string{""},
		},
		{
			Name: "simple-no-patches",
			Manifests: &Manifests{
				GoModules: []*GoModuleManifest{
					{
						Name: "rsc.io/diff",
						Version: ParsedString{
							Value: "v0.0.0-20190621135850-fe3479844c3c",
							Pos: Pos{
								File: "deps.vdm",
								Line: 2,
							},
						},
						Download: ParsedString{
							Value: "sha256:/WCDjRGIVDjKlhtSc1PEApp2fR58gfSVK62dr/yQNyQ=",
							Pos: Pos{
								File: "manifests.vdm",
								Line: 3,
							},
						},
						Vendored: ParsedString{
							Value: "sha256:AB6TWADCiFzYx4nzfwjeNQBxOA+FM7yLQGFe0PKx38k=",
							Pos: Pos{
								File: "manifests.vdm",
								Line: 4,
							},
						},
					},
				},
			},
			Want: []string{
				`go-modules:`,
				`	module "rsc.io/diff" v0.0.0-20190621135850-fe3479844c3c`,
				`		download: sha256:/WCDjRGIVDjKlhtSc1PEApp2fR58gfSVK62dr/yQNyQ=`,
				`		vendored: sha256:AB6TWADCiFzYx4nzfwjeNQBxOA+FM7yLQGFe0PKx38k=`,
				``,
			},
		},
		{
			Name: "simple-patches",
			Manifests: &Manifests{
				GoModules: []*GoModuleManifest{
					{
						Name: "rsc.io/diff",
						Version: ParsedString{
							Value: "v0.0.0-20190621135850-fe3479844c3c",
							Pos: Pos{
								File: "deps.vdm",
								Line: 2,
							},
						},
						Download: ParsedString{
							Value: "sha256:/WCDjRGIVDjKlhtSc1PEApp2fR58gfSVK62dr/yQNyQ=",
							Pos: Pos{
								File: "manifests.vdm",
								Line: 3,
							},
						},
						Vendored: ParsedString{
							Value: "sha256:AB6TWADCiFzYx4nzfwjeNQBxOA+FM7yLQGFe0PKx38k=",
							Pos: Pos{
								File: "manifests.vdm",
								Line: 4,
							},
						},
						Patches: ParsedString{
							Value: "sha256:zeHhy1A/3rpHyenrSFE6TgQixKDzw9ZNM1dEHwAdN4I=",
							Pos: Pos{
								File: "manifests.vdm",
								Line: 5,
							},
						},
					},
				},
			},
			Want: []string{
				`go-modules:`,
				`	module "rsc.io/diff" v0.0.0-20190621135850-fe3479844c3c`,
				`		download: sha256:/WCDjRGIVDjKlhtSc1PEApp2fR58gfSVK62dr/yQNyQ=`,
				`		vendored: sha256:AB6TWADCiFzYx4nzfwjeNQBxOA+FM7yLQGFe0PKx38k=`,
				`		patches:  sha256:zeHhy1A/3rpHyenrSFE6TgQixKDzw9ZNM1dEHwAdN4I=`,
				``,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got := string(test.Manifests.Encode())
			want := strings.Join(test.Want, "\n")
			if got != want {
				t.Fatalf("bad output:\n%s", diff.Format(want, got))
			}
		})
	}
}
