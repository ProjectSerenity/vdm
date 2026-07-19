// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package vendor

import (
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/ProjectSerenity/vdm/internal/digest"
	"github.com/ProjectSerenity/vdm/internal/vdmtest"
	"github.com/ProjectSerenity/vdm/internal/ves"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func s(str string) ves.ParsedString { return ves.ParsedString{Value: str} }

func digestDirectory(t testing.TB, fsys fs.FS, dir string, ignore ...string) string {
	got, err := digest.Directory(fsys, dir)
	if err != nil {
		t.Helper()
		t.Fatal(err)
	}

	return got
}

func TestVendor(t *testing.T) {
	tests := []struct {
		Name  string
		FS    fs.FS
		Want  []Action
		Error string
	}{
		{
			Name:  "invalid-bad-deps",
			FS:    vdmtest.TxtarFS(t, "testdata/archives/empty.txtar"),
			Error: `open deps.vdm: file does not exist`,
		},
		{
			Name: "valid-no-deps",
			FS:   vdmtest.TxtarFS(t, "testdata/archives/minimal.txtar"),
			Want: []Action{
				RemoveAll("vendor"),
			},
		},
		{
			Name:  "invalid-missing-package-deps",
			FS:    vdmtest.TxtarFS(t, "testdata/archives/missing-pkg-deps.txtar"),
			Error: "missing dependencies:\nGo package example.com/foo depends on example.com/bar, which is not specified.\n",
		},
		{
			Name: "invalid-bad-vendor-info",
			FS: vdmtest.TestFS(t,
				vdmtest.WithReadFile(map[string][]byte{
					"deps.vdm": []byte(strings.Join([]string{
						`go-modules:`,
						`	module "example.com/foo" v1.2.3`,
						`		packages:`,
						`			package "example.com/foo"`,
					}, "\n")),
				}),
				vdmtest.WithStatErrors("vendor", "bad stat"),
			),
			Error: `failed to stat "vendor": bad stat`,
		},
		{
			Name:  "invalid-bad-vendor-dir",
			FS:    vdmtest.TxtarFS(t, "testdata/archives/vendor-file.txtar"),
			Error: `failed to vendor dependencies: "vendor" exists and is not a directory`,
		},
		{
			Name: "invalid-bad-vendor-entries",
			FS: vdmtest.TestFS(t,
				vdmtest.WithReadFile(map[string][]byte{
					"deps.vdm": []byte(strings.Join([]string{
						`go-modules:`,
						`	module "example.com/foo" v1.2.3`,
						`		packages:`,
						`			package "example.com/foo"`,
					}, "\n")),
				}),
				vdmtest.WithStat(map[string]fs.FileInfo{
					"vendor": vdmtest.TestFileInfo("vendor", 0, 0o755, time.Time{}, true, nil),
				}),
				vdmtest.WithReadDirErrors("vendor", "bad stat"),
			),
			Error: `failed to read files in "vendor": bad stat`,
		},
		{
			Name:  "invalid-bad-package-path",
			FS:    vdmtest.TxtarFS(t, "testdata/archives/empty-package-path.txtar"),
			Error: `Go module example.com/foo has package 0 with no import path`,
		},
		{
			Name: "valid-complex",
			FS:   vdmtest.TxtarFS(t, "testdata/archives/complex.txtar"),
			Want: []Action{
				// Added individually by scanning vendor/.
				RemoveAll("vendor/b.txt"),
				RemoveAll("vendor/c.txt"),
				// Added later by processing Go packages.
				RemoveAll("vendor/a"),
				// Download modules and generate package BUILD files.
				DownloadGoModule{
					Module: &ves.GoModule{
						Name: "example.com/foo",
						Version: ves.ParsedString{
							Value: "v1.2.3",
							Pos:   ves.Pos{File: "deps.vdm", Line: 2},
						},
						Packages: []*ves.GoPackage{
							{
								Name: ves.ParsedString{
									Value: "example.com/foo",
									Pos:   ves.Pos{File: "deps.vdm", Line: 4},
								},
							},
						},
					},
					Manifest: &ves.GoModuleManifest{
						Name: "example.com/foo",
						Version: ves.ParsedString{
							Value: "v1.2.3",
							Pos:   ves.Pos{File: "deps.vdm", Line: 2},
						},
						Packages: s("sha256:OWsM+RnT2U96NqEa73dOI5FvpDkJrP1sJYjlqKHUG6A="),
					},
					ModuleProxy: "https://proxy.golang.org",
					Dir:         "vendor",
					Path:        "vendor/example.com/foo",
				},
				&GenerateGoPackageBUILD{
					Package: &ves.GoPackage{
						Name: ves.ParsedString{
							Value: "example.com/foo",
							Pos:   ves.Pos{File: "deps.vdm", Line: 4},
						},
					},
					Dir:  "vendor/example.com/foo",
					Path: "vendor/example.com/foo/BUILD.bazel",
				},
				// Build cache manifest.
				BuildCacheManifest{
					Manifests: &ves.Manifests{
						GoModules: []*ves.GoModuleManifest{
							{
								Name: "example.com/foo",
								Version: ves.ParsedString{
									Value: "v1.2.3",
									Pos:   ves.Pos{File: "deps.vdm", Line: 2},
								},
								Packages: s("sha256:OWsM+RnT2U96NqEa73dOI5FvpDkJrP1sJYjlqKHUG6A="),
							},
						},
					},
					Path: "vendor/manifests.vdm",
				},
			},
		},
		{
			Name: "valid-explicit-build",
			FS:   vdmtest.TxtarFS(t, "testdata/archives/explicit-build.txtar"),
			Want: []Action{
				// Added individually by scanning vendor/.
				RemoveAll("vendor/b.txt"),
				RemoveAll("vendor/c.txt"),
				// Added later by processing Go packages.
				RemoveAll("vendor/a"),
				// Download modules and generate package BUILD files.
				DownloadGoModule{
					Module: &ves.GoModule{
						Name: "example.com/foo",
						Version: ves.ParsedString{
							Value: "v1.2.3",
							Pos:   ves.Pos{File: "deps.vdm", Line: 2},
						},
						Packages: []*ves.GoPackage{
							{
								Name: ves.ParsedString{
									Value: "example.com/foo",
									Pos:   ves.Pos{File: "deps.vdm", Line: 4},
								},
								BuildFile: ves.ParsedString{
									Value: "bazel/example.com-foo.BUILD",
									Pos:   ves.Pos{File: "deps.vdm", Line: 5},
								},
							},
						},
					},
					Manifest: &ves.GoModuleManifest{
						Name: "example.com/foo",
						Version: ves.ParsedString{
							Value: "v1.2.3",
							Pos:   ves.Pos{File: "deps.vdm", Line: 2},
						},
						Packages: s("sha256:OWsM+RnT2U96NqEa73dOI5FvpDkJrP1sJYjlqKHUG6A="),
					},
					ModuleProxy: "https://proxy.golang.org",
					Dir:         "vendor",
					Path:        "vendor/example.com/foo",
				},
				CopyBUILD{
					Source: "bazel/example.com-foo.BUILD",
					Path:   "vendor/example.com/foo/BUILD.bazel",
				},
				// Build cache manifest.
				BuildCacheManifest{
					Manifests: &ves.Manifests{
						GoModules: []*ves.GoModuleManifest{
							{
								Name: "example.com/foo",
								Version: ves.ParsedString{
									Value: "v1.2.3",
									Pos:   ves.Pos{File: "deps.vdm", Line: 2},
								},
								Packages: s("sha256:OWsM+RnT2U96NqEa73dOI5FvpDkJrP1sJYjlqKHUG6A="),
							},
						},
					},
					Path: "vendor/manifests.vdm",
				},
			},
		},
	}

	options := []cmp.Option{
		// Ignore the module cache's, as its base path
		// will depend on environment variables.
		cmpopts.IgnoreFields(DownloadGoModule{}, "Cache"),
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got, err := Vendor(test.FS)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("Vendor(): unexpected lack of error")
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("Vendor(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("Vendor(): got unexpected error: %v", err)
			}

			if diff := cmp.Diff(test.Want, got, options...); diff != "" {
				t.Errorf("Vendor(): actions mismatch (-want, +got)\n%s", diff)
			}
		})
	}
}

func TestVendorGo(t *testing.T) {
	tests := []struct {
		Name    string
		FS      fs.FS
		Actions []Action
		Deps    *ves.Deps
		Want    []Action
		Error   string
	}{
		{
			Name: "invalid-no-module-name",
			Deps: &ves.Deps{
				GoModules: []*ves.GoModule{
					{
						Name: "",
					},
				},
			},
			Error: `Go module 0 has no name`,
		},
		{
			Name: "invalid-no-module-version",
			Deps: &ves.Deps{
				GoModules: []*ves.GoModule{
					{
						Name:    "example.com/foo",
						Version: ves.ParsedString{Value: ""},
					},
				},
			},
			Error: `Go module example.com/foo has no version`,
		},
		{
			Name: "invalid-no-packages",
			Deps: &ves.Deps{
				GoModules: []*ves.GoModule{
					{
						Name:    "example.com/foo",
						Version: s("v1.2.3"),
					},
				},
			},
			Error: `Go module example.com/foo has no packages`,
		},
		{
			Name: "invalid-no-package-path",
			Deps: &ves.Deps{
				GoModules: []*ves.GoModule{
					{
						Name:    "example.com/foo",
						Version: s("v1.2.3"),
						Packages: []*ves.GoPackage{
							{
								Name: ves.ParsedString{Value: ""},
							},
						},
					},
				},
			},
			Error: `Go module example.com/foo has package 0 with no import path`,
		},
		{
			Name: "invalid-bad-fs",
			FS:   vdmtest.TestFS(t, vdmtest.WithErrors("vendor", "bad FS")),
			Deps: &ves.Deps{
				GoModules: []*ves.GoModule{
					{
						Name:    "example.com/foo",
						Version: s("v1.2.3"),
						Packages: []*ves.GoPackage{
							{
								Name: s("example.com/foo"),
							},
						},
					},
				},
			},
			Error: `failed to walk vendor: bad FS`,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got, _, err := vendorGo(test.FS, test.Actions, test.Deps)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("vendorGo(): unexpected lack of error")
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("vendorGo(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("vendorGo(): got unexpected error: %v", err)
			}

			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Errorf("vendorGo(): actions mismatch (-want, +got)\n%s", diff)
			}
		})
	}
}
