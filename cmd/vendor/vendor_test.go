// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package vendor

import (
	"io/fs"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"rsc.io/diff"

	"github.com/ProjectSerenity/vdm/internal/vendeps"
)

func TestVendor(t *testing.T) {
	tests := []struct {
		Name string
		Fsys fs.FS
		Want []vendeps.Action
	}{
		{
			Name: "No deps",
			Fsys: fstest.MapFS{
				"deps.bzl": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{},
				},
			},
			Want: []vendeps.Action{
				vendeps.RemoveAll("vendor"),
			},
		},
		{
			Name: "Starting from scratch",
			Fsys: fstest.MapFS{
				"deps.bzl": &fstest.MapFile{
					Mode: 0666,
					Data: []byte(`
						go = [
							module(
								name = "rsc.io/quote",
								version = "v1.2.3",
								packages = [
									package(
										name = "rsc.io/quote",
									),
								],
							),
						]
					`),
				},
			},
			Want: []vendeps.Action{
				vendeps.DownloadGoModule{
					Module: &vendeps.GoModule{
						Name:    "rsc.io/quote",
						Version: "v1.2.3",
						Packages: []*vendeps.GoPackage{
							{Name: "rsc.io/quote"},
						},
					},
					Path: "vendor/rsc.io/quote",
				},
				GenerateGoPackageBUILD{
					Package: &vendeps.GoPackage{
						Name: "rsc.io/quote",
					},
					Path: "vendor/rsc.io/quote/BUILD.bazel",
				},
				BuildCacheManifest{
					Deps: &vendeps.Deps{
						Go: []*vendeps.GoModule{
							{
								Name:    "rsc.io/quote",
								Version: "v1.2.3",
								Packages: []*vendeps.GoPackage{
									{Name: "rsc.io/quote"},
								},
							},
						},
					},
					Path: "vendor/manifest.bzl",
				},
			},
		},
		{
			Name: "Clearing detritus",
			Fsys: fstest.MapFS{
				"deps.bzl": &fstest.MapFile{
					Mode: 0666,
					Data: []byte(`
						go = [
							module(
								name = "rsc.io/quote",
								version = "v1.2.3",
								packages = [
									package(
										name = "rsc.io/quote",
									),
								],
							),
						]
					`),
				},
				"vendor/manifest.bzl": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{},
				},
				"vendor/parent/child": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{},
				},
				"vendor/random": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{},
				},
			},
			Want: []vendeps.Action{
				vendeps.RemoveAll("vendor/random"),
				vendeps.RemoveAll("vendor/parent"),
				vendeps.DownloadGoModule{
					Module: &vendeps.GoModule{
						Name:    "rsc.io/quote",
						Version: "v1.2.3",
						Packages: []*vendeps.GoPackage{
							{Name: "rsc.io/quote"},
						},
					},
					Path: "vendor/rsc.io/quote",
				},
				GenerateGoPackageBUILD{
					Package: &vendeps.GoPackage{
						Name: "rsc.io/quote",
					},
					Path: "vendor/rsc.io/quote/BUILD.bazel",
				},
				BuildCacheManifest{
					Deps: &vendeps.Deps{
						Go: []*vendeps.GoModule{
							{
								Name:    "rsc.io/quote",
								Version: "v1.2.3",
								Packages: []*vendeps.GoPackage{
									{Name: "rsc.io/quote"},
								},
							},
						},
					},
					Path: "vendor/manifest.bzl",
				},
			},
		},
		{
			Name: "Clearing old crates/modules",
			Fsys: fstest.MapFS{
				"deps.bzl": &fstest.MapFile{
					Mode: 0666,
					Data: []byte(`
						go = [
							module(
								name = "rsc.io/diff",
								version = "v1.2.3",
								packages = [
									package(
										name = "rsc.io/diff",
									),
								],
							),
							module(
								name = "rsc.io/quote",
								version = "v1.2.3",
								packages = [
									package(
										name = "rsc.io/quote",
									),
								],
							),
						]
					`),
				},
				"vendor/golang.org/x/crypto/crypto.go": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{1, 2, 3},
				},
				"vendor/rsc.io/2fa/main.go": &fstest.MapFile{
					Mode: 0666,
					Data: []byte{1, 2, 3},
				},
			},
			Want: []vendeps.Action{
				vendeps.RemoveAll("vendor/golang.org"), // Root dir of an old module.
				vendeps.RemoveAll("vendor/rsc.io/2fa"), // Don't remove all of rsc.io.
				vendeps.DownloadGoModule{
					Module: &vendeps.GoModule{
						Name:    "rsc.io/diff",
						Version: "v1.2.3",
						Packages: []*vendeps.GoPackage{
							{Name: "rsc.io/diff"},
						},
					},
					Path: "vendor/rsc.io/diff",
				},
				GenerateGoPackageBUILD{
					Package: &vendeps.GoPackage{
						Name: "rsc.io/diff",
					},
					Path: "vendor/rsc.io/diff/BUILD.bazel",
				},
				vendeps.DownloadGoModule{
					Module: &vendeps.GoModule{
						Name:    "rsc.io/quote",
						Version: "v1.2.3",
						Packages: []*vendeps.GoPackage{
							{Name: "rsc.io/quote"},
						},
					},
					Path: "vendor/rsc.io/quote",
				},
				GenerateGoPackageBUILD{
					Package: &vendeps.GoPackage{
						Name: "rsc.io/quote",
					},
					Path: "vendor/rsc.io/quote/BUILD.bazel",
				},
				BuildCacheManifest{
					Deps: &vendeps.Deps{
						Go: []*vendeps.GoModule{
							{
								Name:    "rsc.io/diff",
								Version: "v1.2.3",
								Packages: []*vendeps.GoPackage{
									{Name: "rsc.io/diff"},
								},
							},
							{
								Name:    "rsc.io/quote",
								Version: "v1.2.3",
								Packages: []*vendeps.GoPackage{
									{Name: "rsc.io/quote"},
								},
							},
						},
					},
					Path: "vendor/manifest.bzl",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got, err := Vendor(test.Fsys, false)
			if err != nil {
				t.Fatalf("Vendor(): %v", err)
			}

			if !reflect.DeepEqual(got, test.Want) {
				var buf strings.Builder
				for _, action := range got {
					buf.WriteString(action.String())
					buf.WriteByte('\n')
				}

				g := buf.String()

				buf.Reset()
				for _, action := range test.Want {
					buf.WriteString(action.String())
					buf.WriteByte('\n')
				}

				w := buf.String()
				t.Fatalf("Vendor(): got mismatch:\n%s", diff.Format(g, w))
			}
		})
	}
}
