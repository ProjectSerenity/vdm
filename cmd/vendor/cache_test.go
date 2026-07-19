// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package vendor

import (
	"io/fs"
	"slices"
	"testing"

	"github.com/ProjectSerenity/vdm/internal/vdm"
	"github.com/ProjectSerenity/vdm/internal/vdmtest"

	"github.com/google/go-cmp/cmp"
)

func TestStripCachedActions(t *testing.T) {
	tests := []struct {
		Name    string
		FS      fs.FS
		Actions []Action
		Want    []Action
	}{
		{
			Name: "no-cache",
			FS:   vdmtest.TxtarFS(t, "testdata/caching/no-cache.txtar"),
			Actions: []Action{
				DownloadGoModule{
					Module: &vdm.GoModule{
						Name:    "rsc.io/quote",
						Version: s("v1.2.3"),
						Packages: []*vdm.GoPackage{
							{Name: s("rsc.io/quote")},
						},
					},
					Manifest: &vdm.GoModuleManifest{
						Name:    "rsc.io/quote",
						Version: s("v1.2.3"),
					},
					Path: "vendor/rsc.io/quote",
				},
			},
			Want: []Action{
				DownloadGoModule{
					Module: &vdm.GoModule{
						Name:    "rsc.io/quote",
						Version: s("v1.2.3"),
						Packages: []*vdm.GoPackage{
							{Name: s("rsc.io/quote")},
						},
					},
					Manifest: &vdm.GoModuleManifest{
						Name:    "rsc.io/quote",
						Version: s("v1.2.3"),
					},
					Path: "vendor/rsc.io/quote",
				},
			},
		},
		{
			Name: "empty-cache",
			FS:   vdmtest.TxtarFS(t, "testdata/caching/empty-cache.txtar"),
			Actions: []Action{
				DownloadGoModule{
					Module: &vdm.GoModule{
						Name:    "rsc.io/quote",
						Version: s("v1.2.3"),
						Packages: []*vdm.GoPackage{
							{Name: s("rsc.io/quote")},
						},
					},
					Manifest: &vdm.GoModuleManifest{
						Name:    "rsc.io/quote",
						Version: s("v1.2.3"),
					},
					Path: "vendor/rsc.io/quote",
				},
			},
			Want: []Action{
				DownloadGoModule{
					Module: &vdm.GoModule{
						Name:    "rsc.io/quote",
						Version: s("v1.2.3"),
						Packages: []*vdm.GoPackage{
							{Name: s("rsc.io/quote")},
						},
					},
					Manifest: &vdm.GoModuleManifest{
						Name:    "rsc.io/quote",
						Version: s("v1.2.3"),
					},
					Path: "vendor/rsc.io/quote",
				},
			},
		},
		{
			Name: "partial-cache",
			FS:   vdmtest.TxtarFS(t, "testdata/caching/partial-cache.txtar"),
			Actions: func() []Action {
				// This one is a little more complicated, as we want to
				// test a subtle behaviour. When we eliminate a module
				// download because the cache is already up to date,
				// we save the recorded digests into the download's
				// manifest. As this is normally a pointer to the same
				// manifest that's stored in the BuildCacheManifest
				// action, that action gets updated in the process,
				// even though we drop the DownloadGoModule action.
				//
				// In this test, we check that behaviour, making sure
				// that the digests are updated in the cache build.
				// Doing so requires us to manage pointers carefully,
				// so we build the objects up individually and then
				// assemble the actions afterwards.
				rma := RemoveAll("vendor/example.com/foo")
				dls := []DownloadGoModule{
					{
						Module: &vdm.GoModule{
							Name:    "example.com/bar",
							Version: s("v1.2.3"),
							Packages: []*vdm.GoPackage{
								{Name: s("example.com/bar")},
								{Name: s("example.com/bar/baz")},
							},
						},
						Manifest: &vdm.GoModuleManifest{
							Name:     "example.com/bar",
							Version:  s("v1.2.3"),
							Packages: s("sha256:9nIFKNUR+Ycz3wTrCUEbPfQjw0s3D7nRvw/7N4bMRWc="),
						},
						Path: "vendor/example.com/bar",
					},
					{
						Module: &vdm.GoModule{
							Name:    "golang.org/x/crypto",
							Version: s("v1.2.3"),
							Packages: []*vdm.GoPackage{
								{Name: s("golang.org/x/crypto")},
							},
						},
						Manifest: &vdm.GoModuleManifest{
							Name:    "golang.org/x/crypto",
							Version: s("v1.2.3"),
						},
						Path: "vendor/golang.org/x/crypto",
					},
					{
						Module: &vdm.GoModule{
							Name:    "golang.org/x/mod",
							Version: s("v1.2.3"),
							Packages: []*vdm.GoPackage{
								{Name: s("golang.org/x/mod/module")},
								{Name: s("golang.org/x/mod/zip")},
							},
						},
						Manifest: &vdm.GoModuleManifest{
							Name:    "golang.org/x/mod",
							Version: s("v1.2.3"),
						},
						Path: "vendor/golang.org/x/mod",
					},
					{
						Module: &vdm.GoModule{
							Name:    "rsc.io/diff",
							Version: s("v1.2.3"),
							Packages: []*vdm.GoPackage{
								{Name: s("rsc.io/diff")},
							},
						},
						Manifest: &vdm.GoModuleManifest{
							Name:    "rsc.io/diff",
							Version: s("v1.2.3"),
						},
						Path: "vendor/rsc.io/diff",
					},
					{
						Module: &vdm.GoModule{
							Name:    "rsc.io/quote",
							Version: s("v1.5.2"),
							Patches: []vdm.ParsedString{
								{Value: "patches/quote.patch"},
							},
							Packages: []*vdm.GoPackage{
								{Name: s("rsc.io/quote")},
							},
						},
						Manifest: &vdm.GoModuleManifest{
							Name:    "rsc.io/quote",
							Version: s("v1.5.2"),
						},
						Path: "vendor/rsc.io/quote",
					},
					{
						Module: &vdm.GoModule{
							Name:    "rsc.io/tmp",
							Version: s("v0.0.0-20260706223531-5a501281bc9f"),
							Patches: []vdm.ParsedString{
								{Value: "patches/foo.patch"},
							},
							Packages: []*vdm.GoPackage{
								{Name: s("rsc.io/tmp/patch")},
							},
						},
						Manifest: &vdm.GoModuleManifest{
							Name:    "rsc.io/tmp",
							Version: s("v0.0.0-20260706223531-5a501281bc9f"),
						},
						Path: "vendor/rsc.io/tmp",
					},
				}
				bcm := BuildCacheManifest{
					Manifests: &vdm.Manifests{
						GoModules: make([]*vdm.GoModuleManifest, len(dls)),
					},
					Path: "vendor/manifests.vdm",
				}

				for i, dl := range dls {
					bcm.Manifests.GoModules[i] = dl.Manifest
				}

				actions := make([]Action, 0, 1+len(dls)+1)
				actions = append(actions, rma)
				for _, dl := range dls {
					actions = append(actions, dl)
				}
				actions = append(actions, bcm)

				return actions
			}(),
			Want: []Action{
				RemoveAll("vendor/example.com/foo"),
				DownloadGoModule{
					Module: &vdm.GoModule{
						Name:    "example.com/bar",
						Version: s("v1.2.3"),
						Packages: []*vdm.GoPackage{
							{Name: s("example.com/bar")},
							{Name: s("example.com/bar/baz")},
						},
					},
					Manifest: &vdm.GoModuleManifest{
						Name:     "example.com/bar",
						Version:  s("v1.2.3"),
						Packages: s("sha256:9nIFKNUR+Ycz3wTrCUEbPfQjw0s3D7nRvw/7N4bMRWc="),
					},
					Path: "vendor/example.com/bar",
				},
				DownloadGoModule{
					Module: &vdm.GoModule{
						Name:    "golang.org/x/crypto",
						Version: s("v1.2.3"),
						Packages: []*vdm.GoPackage{
							{Name: s("golang.org/x/crypto")},
						},
					},
					Manifest: &vdm.GoModuleManifest{
						Name:    "golang.org/x/crypto",
						Version: s("v1.2.3"),
					},
					Path: "vendor/golang.org/x/crypto",
				},
				DownloadGoModule{
					Module: &vdm.GoModule{
						Name:    "golang.org/x/mod",
						Version: s("v1.2.3"),
						Packages: []*vdm.GoPackage{
							{Name: s("golang.org/x/mod/module")},
							{Name: s("golang.org/x/mod/zip")},
						},
					},
					Manifest: &vdm.GoModuleManifest{
						Name:    "golang.org/x/mod",
						Version: s("v1.2.3"),
					},
					Path: "vendor/golang.org/x/mod",
				},
				DownloadGoModule{
					Module: &vdm.GoModule{
						Name:    "rsc.io/diff",
						Version: s("v1.2.3"),
						Packages: []*vdm.GoPackage{
							{Name: s("rsc.io/diff")},
						},
					},
					Manifest: &vdm.GoModuleManifest{
						Name:    "rsc.io/diff",
						Version: s("v1.2.3"),
					},
					Path: "vendor/rsc.io/diff",
				},
				// Strip the download for rsc.io/quote, as it's cached.
				DownloadGoModule{
					Module: &vdm.GoModule{
						Name:    "rsc.io/tmp",
						Version: s("v0.0.0-20260706223531-5a501281bc9f"),
						Patches: []vdm.ParsedString{
							{Value: "patches/foo.patch"},
						},
						Packages: []*vdm.GoPackage{
							{Name: s("rsc.io/tmp/patch")},
						},
					},
					Manifest: &vdm.GoModuleManifest{
						Name:    "rsc.io/tmp",
						Version: s("v0.0.0-20260706223531-5a501281bc9f"),
					},
					Path: "vendor/rsc.io/tmp",
				},
				BuildCacheManifest{
					Manifests: &vdm.Manifests{
						GoModules: []*vdm.GoModuleManifest{
							{
								Name:     "example.com/bar",
								Version:  s("v1.2.3"),
								Packages: s("sha256:9nIFKNUR+Ycz3wTrCUEbPfQjw0s3D7nRvw/7N4bMRWc="),
							},
							{
								Name:    "golang.org/x/crypto",
								Version: s("v1.2.3"),
							},
							{
								Name:    "golang.org/x/mod",
								Version: s("v1.2.3"),
							},
							{
								Name:    "rsc.io/diff",
								Version: s("v1.2.3"),
							},
							{
								Name:     "rsc.io/quote",
								Version:  s("v1.5.2"),
								Download: s("sha256:w5fcysjrx7yqtD/aO+QwRjYZOKnaM9Uh2b40tElTs3Y="), // <- This is important. See the comment above for context.
								Vendored: s("sha256:i5SdzwqJBHWVTijZj/xRa3bm8B332MO+Mb+pk+COe8g="), // <- This is important. See the comment above for context.
								Patches:  s("sha256:65IDd8bxwMukByl+92FI0wlqz0M1WSMr633KW1xhbBA="), // <- This is important. See the comment above for context.
							},
							{
								Name:    "rsc.io/tmp",
								Version: s("v0.0.0-20260706223531-5a501281bc9f"),
							},
						},
					},
					Path: "vendor/manifests.vdm",
				},
			},
		},
		{
			Name: "nested-modules",
			FS:   vdmtest.TxtarFS(t, "testdata/caching/nested-modules.txtar"),
			Actions: []Action{
				DownloadGoModule{
					Module: &vdm.GoModule{
						Name:    "example.com/foo",
						Version: s("v1.2.3"),
						Packages: []*vdm.GoPackage{
							{Name: s("example.com/foo")},
						},
					},
					Manifest: &vdm.GoModuleManifest{
						Name:     "example.com/foo",
						Version:  s("v1.2.3"),
						Packages: s("sha256:OWsM+RnT2U96NqEa73dOI5FvpDkJrP1sJYjlqKHUG6A="),
					},
					Path: "vendor/example.com/foo",
				},
				DownloadGoModule{
					Module: &vdm.GoModule{
						Name:    "example.com/foo/bar",
						Version: s("v1.2.3"),
						Packages: []*vdm.GoPackage{
							{Name: s("example.com/foo/bar")},
						},
					},
					Manifest: &vdm.GoModuleManifest{
						Name:     "example.com/foo/bar",
						Version:  s("v1.2.3"),
						Packages: s("sha256:UNub1wOV86JWl0yt1GHvkF14UY1YNOf6qz/Hx3iLFko="),
					},
					Path: "vendor/example.com/foo/bar",
				},
			},
			Want: []Action{
				DownloadGoModule{ // Keep because the packages digest doesn't match.
					Module: &vdm.GoModule{
						Name:    "example.com/foo",
						Version: s("v1.2.3"),
						Packages: []*vdm.GoPackage{
							{Name: s("example.com/foo")},
						},
					},
					Manifest: &vdm.GoModuleManifest{
						Name:     "example.com/foo",
						Version:  s("v1.2.3"),
						Packages: s("sha256:OWsM+RnT2U96NqEa73dOI5FvpDkJrP1sJYjlqKHUG6A="),
					},
					Path: "vendor/example.com/foo",
				},
				DownloadGoModule{ // Keep because the parent module will be deleted.
					Module: &vdm.GoModule{
						Name:    "example.com/foo/bar",
						Version: s("v1.2.3"),
						Packages: []*vdm.GoPackage{
							{Name: s("example.com/foo/bar")},
						},
					},
					Manifest: &vdm.GoModuleManifest{
						Name:     "example.com/foo/bar",
						Version:  s("v1.2.3"),
						Packages: s("sha256:UNub1wOV86JWl0yt1GHvkF14UY1YNOf6qz/Hx3iLFko="),
					},
					Path: "vendor/example.com/foo/bar",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got := StripCachedActions(test.FS, test.Actions)
			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Errorf("StripCachedActions(): stripped actions mismatch (-want, +got)\n%s", diff)
			}
		})
	}
}

func TestParentModules(t *testing.T) {
	tests := []struct {
		Module string
		Want   []string
	}{
		{"github.com/SlyMarbo/marbo", []string{"github.com/SlyMarbo", "github.com"}},
		{"github.com/SlyMarbo/marbo/foo", []string{"github.com/SlyMarbo/marbo", "github.com/SlyMarbo", "github.com"}},
	}

	for _, test := range tests {
		got := slices.Collect(parentModules(test.Module))
		if diff := cmp.Diff(test.Want, got); diff != "" {
			t.Errorf("parentModules(%q): modules mismatch (-want, +got)\n%s", test.Module, diff)
		}
	}
}

func TestCloneStrings(t *testing.T) {
	tests := []struct {
		Name string
		SS   []vdm.ParsedString
		Want []string
	}{
		{
			Name: "nil",
			SS:   nil,
			Want: nil,
		},
		{
			Name: "some",
			SS: []vdm.ParsedString{
				{Value: "foo", Pos: vdm.Pos{File: "foo.vdm", Line: 7}},
				{Value: "bar"},
			},
			Want: []string{
				"foo",
				"bar",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got := cloneStrings(test.SS)
			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Errorf("cloneStrings(): strings mismatch (-want, +got)\n%s", diff)
			}
		})
	}
}
