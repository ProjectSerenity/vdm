// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package vendor

import (
	"bytes"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ProjectSerenity/vdm/internal/gomodzip"
	"github.com/ProjectSerenity/vdm/internal/simplehttp"
	"github.com/ProjectSerenity/vdm/internal/vdmtest"
	"github.com/ProjectSerenity/vdm/internal/ves"

	"github.com/google/go-cmp/cmp"
)

func TestRemoveAll(t *testing.T) {
	tests := []struct {
		Name  string
		Init  fs.FS
		Want  fs.FS
		Make  func(dir string) (Action, string)
		Error string
	}{
		{
			Name: "simple",
			Init: vdmtest.TxtarFS(t, "testdata/actions/simple.txtar"),
			Want: vdmtest.TxtarFS(t, "testdata/actions/removed.txtar"),
			Make: func(dir string) (Action, string) {
				str := filepath.Join(dir, "foo/second.txt")
				return RemoveAll(str), fmt.Sprintf("delete %s", str)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			dir := t.ArtifactDir()
			dirFS := vdmtest.DirFS(t)
			err := vdmtest.ExtractFS(test.Init, dir)
			if err != nil {
				t.Fatal(err)
			}

			action, str := test.Make(dir)
			if got := action.String(); got != str {
				t.Errorf("RemoveAll(): string mismatch:\nGot:  %q\nWant: %q", got, str)
			}

			err = action.Do(context.Background(), dirFS, io.Discard)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("RemoveAll(): unexpected lack of error")
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("RemoveAll(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("RemoveAll(): got unexpected error: %v", err)
			}

			err = vdmtest.DiffTextFilesystems(dirFS, test.Want, ".")
			if err != nil {
				t.Errorf("RemoveAll(): %v", err)
			}
		})
	}
}

//go:embed testdata/zips/rsc.io/diff/@v/v0.0.0-20190621135850-fe3479844c3c.zip
var savedPackageZIP []byte

func TestDownloadGoModule(t *testing.T) {
	tests := []struct {
		Name         string
		Init         func(t *testing.T, action *DownloadGoModule) // Optional setup code.
		Action       *DownloadGoModule
		WantManifest *ves.GoModuleManifest
		WantFS       fs.FS
		String       string
		Error        string
		ErrorCmp     func(string, string) bool
	}{
		{
			Name: "invalid-bad-module-name",
			Action: &DownloadGoModule{
				Module: &ves.GoModule{
					Name: "-!",
				},
				Manifest: &ves.GoModuleManifest{
					Name: "-!",
				},
				Path: "vendor/rsc.io/diff",
			},
			String: "download module -! to vendor/rsc.io/diff",
			Error:  `malformed module path "-!": leading dash`,
		},
		{
			Name: "invalid-bad-download-path",
			Init: func(t *testing.T, action *DownloadGoModule) {
				path, err := action.Cache.Path(action.Manifest)
				if err != nil {
					t.Fatalf("failed to determine download path: %v", err)
				}

				err = os.MkdirAll(filepath.Dir(filepath.Dir(path)), 0o755)
				if err != nil {
					t.Fatalf("failed to create download path: %v", err)
				}

				// Make the download path's directory a file,
				// so os.MkdirAll fails.
				err = os.WriteFile(filepath.Dir(path), []byte("foo"), 0o644)
				if err != nil {
					t.Fatalf("failed to create download path: %v", err)
				}
			},
			Action: &DownloadGoModule{
				Module: &ves.GoModule{
					Name:    "rsc.io/diff",
					Version: ves.S("v0.0.0-20190621135850-fe3479844c3c"),
					Packages: []*ves.GoPackage{
						{
							Name: ves.S("rsc.io/diff"),
						},
					},
				},
				Manifest: &ves.GoModuleManifest{
					Name:     "rsc.io/diff",
					Version:  ves.S("v0.0.0-20190621135850-fe3479844c3c"),
					Download: ves.S("sha256:/WCDjRGIVDjKlhtSc1PEApp2fR58gfSVK62dr/yQNyQ="),
				},
				Path: "vendor/rsc.io/diff",
			},
			String:   "download module rsc.io/diff to vendor/rsc.io/diff",
			Error:    `not a directory`,
			ErrorCmp: strings.HasSuffix,
		},
		{
			Name: "invalid-bad-module-cache",
			Init: func(t *testing.T, action *DownloadGoModule) {
				action.ModuleProxy = "https://0.0.0.0:0"
			},
			Action: &DownloadGoModule{
				Module: &ves.GoModule{
					Name:    "rsc.io/diff",
					Version: ves.S("v0.0.0-20190621135850-fe3479844c3c"),
					Packages: []*ves.GoPackage{
						{
							Name: ves.S("rsc.io/diff"),
						},
					},
				},
				Manifest: &ves.GoModuleManifest{
					Name:     "rsc.io/diff",
					Version:  ves.S("v0.0.0-20190621135850-fe3479844c3c"),
					Download: ves.S("sha256:/WCDjRGIVDjKlhtSc1PEApp2fR58gfSVK62dr/yQNyQ="),
				},
				Path: "vendor/rsc.io/diff",
			},
			String: "download module rsc.io/diff to vendor/rsc.io/diff",
			Error:  `failed to fetch Go module rsc.io/diff: Get "https://0.0.0.0:0/rsc.io/diff/@v/v0.0.0-20190621135850-fe3479844c3c.zip": dial tcp 0.0.0.0:0: connect: connection refused`,
		},
		{
			Name: "invalid-bad-download-checksum",
			Init: func(t *testing.T, action *DownloadGoModule) {
				path, err := action.Cache.Path(action.Manifest)
				if err != nil {
					t.Fatalf("failed to determine download path: %v", err)
				}

				err = os.MkdirAll(filepath.Dir(path), 0o755)
				if err != nil {
					t.Fatalf("failed to create download path: %v", err)
				}

				err = os.WriteFile(path, savedPackageZIP, 0o644)
				if err != nil {
					t.Fatalf("failed to write ZIP: %v", err)
				}
			},
			Action: &DownloadGoModule{
				Module: &ves.GoModule{
					Name:    "rsc.io/diff",
					Version: ves.S("v0.0.0-20190621135850-fe3479844c3c"),
					Patches: []ves.ParsedString{
						ves.S("patches/foo.patch"),
						ves.S("patches/bar.patch"),
					},
					Packages: []*ves.GoPackage{
						{
							Name: ves.S("rsc.io/diff"),
						},
					},
				},
				Manifest: &ves.GoModuleManifest{
					Name:     "rsc.io/diff",
					Version:  ves.S("v0.0.0-20190621135850-fe3479844c3c"),
					Download: ves.S("sha256:AB6TWADCiFzYx4nzfwjeNQBxOA+FM7yLQGFe0PKx38k="), // This is wrong.
				},
				Path: "vendor/rsc.io/diff",
			},
			String: "download module rsc.io/diff to vendor/rsc.io/diff with 2 patches",
			Error:  `failed to verify Go module rsc.io/diff: got checksum sha256:/WCDjRGIVDjKlhtSc1PEApp2fR58gfSVK62dr/yQNyQ=, want sha256:AB6TWADCiFzYx4nzfwjeNQBxOA+FM7yLQGFe0PKx38k=`,
		},
		{
			Name: "valid-extract-only",
			Init: func(t *testing.T, action *DownloadGoModule) {
				path, err := action.Cache.Path(action.Manifest)
				if err != nil {
					t.Fatalf("failed to determine download path: %v", err)
				}

				err = os.MkdirAll(filepath.Dir(path), 0o755)
				if err != nil {
					t.Fatalf("failed to create download path: %v", err)
				}

				err = os.WriteFile(path, savedPackageZIP, 0o644)
				if err != nil {
					t.Fatalf("failed to write ZIP: %v", err)
				}
			},
			Action: &DownloadGoModule{
				Module: &ves.GoModule{
					Name:    "rsc.io/diff",
					Version: ves.S("v0.0.0-20190621135850-fe3479844c3c"),
					Packages: []*ves.GoPackage{
						{
							Name: ves.S("rsc.io/diff"),
						},
					},
				},
				Manifest: &ves.GoModuleManifest{
					Name:     "rsc.io/diff",
					Version:  ves.S("v0.0.0-20190621135850-fe3479844c3c"),
					Download: ves.S("sha256:/WCDjRGIVDjKlhtSc1PEApp2fR58gfSVK62dr/yQNyQ="),
				},
				Path: "vendor/rsc.io/diff",
			},
			WantManifest: &ves.GoModuleManifest{
				Name:     "rsc.io/diff",
				Version:  ves.S("v0.0.0-20190621135850-fe3479844c3c"),
				Download: ves.S("sha256:/WCDjRGIVDjKlhtSc1PEApp2fR58gfSVK62dr/yQNyQ="),
				Vendored: ves.S("sha256:LQjoHErpqhowZ2HdaZmwfORwBXrj3spMkOvCByUIEn4="),
			},
			WantFS: vdmtest.TxtarFS(t, "testdata/extracted/rsc.io-diff.txtar"),
			String: "download module rsc.io/diff to vendor/rsc.io/diff",
		},
		{
			Name: "valid-download-and-extract",
			Action: &DownloadGoModule{
				Module: &ves.GoModule{
					Name:    "rsc.io/diff",
					Version: ves.S("v0.0.0-20190621135850-fe3479844c3c"),
					Packages: []*ves.GoPackage{
						{
							Name: ves.S("rsc.io/diff"),
						},
					},
				},
				Manifest: &ves.GoModuleManifest{
					Name:     "rsc.io/diff",
					Version:  ves.S("v0.0.0-20190621135850-fe3479844c3c"),
					Download: ves.S("sha256:/WCDjRGIVDjKlhtSc1PEApp2fR58gfSVK62dr/yQNyQ="),
				},
				Path: "vendor/rsc.io/diff",
			},
			WantManifest: &ves.GoModuleManifest{
				Name:     "rsc.io/diff",
				Version:  ves.S("v0.0.0-20190621135850-fe3479844c3c"),
				Download: ves.S("sha256:/WCDjRGIVDjKlhtSc1PEApp2fR58gfSVK62dr/yQNyQ="),
				Vendored: ves.S("sha256:LQjoHErpqhowZ2HdaZmwfORwBXrj3spMkOvCByUIEn4="),
			},
			WantFS: vdmtest.TxtarFS(t, "testdata/extracted/rsc.io-diff.txtar"),
			String: "download module rsc.io/diff to vendor/rsc.io/diff",
		},
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.NotFoundHandler()) // Default handler.
	mux.HandleFunc("GET /rsc.io/diff/@v/v0.0.0-20190621135850-fe3479844c3c.zip", func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "v0.0.0-20190621135850-fe3479844c3c.zip", time.Time{}, bytes.NewReader(savedPackageZIP))
	})

	srv := httptest.NewTLSServer(mux)
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	defer srv.Close()

	simplehttp.Client = srv.Client()
	simplehttp.UserAgent = "tests"
	simplehttp.SetInterval(time.Millisecond)

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			// Test the String method.
			if got := test.Action.String(); got != test.String {
				t.Errorf("DownloadGoModule.String():\nGot:  %q\nWant: %q", got, test.String)
			}

			// Test the action.
			target := t.ArtifactDir()
			dirFS := vdmtest.DirFS(t)
			test.Action.Cache = gomodzip.ModuleCacheFromBase(target)
			test.Action.ModuleProxy = "https://example.com" // Use our test server instead of the real module proxy.
			test.Action.Dir = filepath.Join(target, ves.Vendor)
			err := os.MkdirAll(filepath.Join(target, filepath.FromSlash(test.Action.Path)), 0o777)
			if err != nil {
				t.Fatalf("failed to create vendor directory: %v", err)
			}

			if test.Init != nil {
				test.Init(t, test.Action)
			}

			err = test.Action.Do(context.Background(), dirFS, io.Discard)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("DownloadGoModule(): unexpected lack of error")
				}

				e := err.Error()
				if test.ErrorCmp != nil && !test.ErrorCmp(e, test.Error) {
					t.Fatalf("DownloadGoModule(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				} else if test.ErrorCmp == nil && e != test.Error {
					t.Fatalf("DownloadGoModule(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("DownloadGoModule(): got unexpected error: %v", err)
			}

			if diff := cmp.Diff(test.WantManifest, test.Action.Manifest); diff != "" {
				t.Fatalf("DownloadGoModule(): manifest mismatch (-want, +got)\n%s", diff)
			}

			// Check the contents.
			err = vdmtest.DiffTextFilesystems(dirFS, test.WantFS, ves.Vendor)
			if err != nil {
				t.Fatalf("DownloadGoModule(): contents mismatch: %v", err)
			}
		})
	}
}

func TestCopyBUILD(t *testing.T) {
	tests := []struct {
		Name     string
		Init     func(t *testing.T, action *CopyBUILD) // Optional setup code.
		FS       fs.FS
		Action   *CopyBUILD
		Want     fs.FS
		String   string
		Error    string
		ErrorCmp func(string, string) bool
	}{
		{
			Name: "invalid-missing-source",
			FS:   vdmtest.TxtarFS(t, "testdata/actions/copyBUILD.txtar"),
			Action: &CopyBUILD{
				Source: "nonexistant",
				Path:   "vendor/rsc.io/diff/BUILD.bazel",
			},
			String: "copy BUILD file nonexistant to vendor/rsc.io/diff/BUILD.bazel",
			Error:  `failed to open BUILD file nonexistant: open nonexistant: file does not exist`,
		},
		{
			Name: "invalid-bad-destination-dir",
			Init: func(t *testing.T, action *CopyBUILD) {
				// Make the parent directory for the destination
				// unreadable so the stat fails.
				err := os.Chmod(filepath.Dir(action.Path), 0o222)
				if err != nil {
					t.Fatalf("failed to make parent directory unreadable: %v", err)
				}
			},
			FS: vdmtest.TxtarFS(t, "testdata/actions/copyBUILD.txtar"),
			Action: &CopyBUILD{
				Source: "bazel/rsc.io/diff/BUILD.bazel",
				Path:   "vendor/rsc.io/diff/BUILD.bazel",
			},
			String:   "copy BUILD file bazel/rsc.io/diff/BUILD.bazel to vendor/rsc.io/diff/BUILD.bazel",
			Error:    `vendor/rsc.io/diff/BUILD.bazel: permission denied`,
			ErrorCmp: strings.HasSuffix,
		},
		{
			Name: "invalid-bad-source-file-read",
			FS: vdmtest.TestFS(t,
				vdmtest.WithFiles(map[string]fs.File{
					"bazel/rsc.io/diff/BUILD.bazel": vdmtest.TestFile(nil, nil, vdmtest.ErrReader("bad source file"), nil),
				}),
			),
			Action: &CopyBUILD{
				Source: "bazel/rsc.io/diff/BUILD.bazel",
				Path:   "vendor/rsc.io/diff/BUILD.bazel",
			},
			String:   "copy BUILD file bazel/rsc.io/diff/BUILD.bazel to vendor/rsc.io/diff/BUILD.bazel",
			Error:    `bad source file`,
			ErrorCmp: strings.HasSuffix,
		},
		{
			Name: "invalid-bad-source-file-close",
			FS: vdmtest.TestFS(t,
				vdmtest.WithFiles(map[string]fs.File{
					"bazel/rsc.io/diff/BUILD.bazel": vdmtest.TestFile(nil, nil, strings.NewReader("foo"), errors.New("bad close")),
				}),
			),
			Action: &CopyBUILD{
				Source: "bazel/rsc.io/diff/BUILD.bazel",
				Path:   "vendor/rsc.io/diff/BUILD.bazel",
			},
			String:   "copy BUILD file bazel/rsc.io/diff/BUILD.bazel to vendor/rsc.io/diff/BUILD.bazel",
			Error:    `bad close`,
			ErrorCmp: strings.HasSuffix,
		},
		{
			Name: "valid-simple",
			FS:   vdmtest.TxtarFS(t, "testdata/actions/copyBUILD.txtar"),
			Action: &CopyBUILD{
				Source: "bazel/rsc.io/diff/BUILD.bazel",
				Path:   "vendor/rsc.io/diff/BUILD.bazel",
			},
			String: "copy BUILD file bazel/rsc.io/diff/BUILD.bazel to vendor/rsc.io/diff/BUILD.bazel",
			Want:   vdmtest.TxtarFS(t, "testdata/extracted/copyBUILD.txtar"),
		},
		{
			Name: "valid-overwrite",
			Init: func(t *testing.T, action *CopyBUILD) {
				// Make the destination file as read-only, so
				// we have to change it back to writeable.
				err := os.WriteFile(action.Path, []byte("foo"), 0o444)
				if err != nil {
					t.Fatalf("failed to write destination file: %v", err)
				}
			},
			FS: vdmtest.TxtarFS(t, "testdata/actions/copyBUILD.txtar"),
			Action: &CopyBUILD{
				Source: "bazel/rsc.io/diff/BUILD.bazel",
				Path:   "vendor/rsc.io/diff/BUILD.bazel",
			},
			String: "copy BUILD file bazel/rsc.io/diff/BUILD.bazel to vendor/rsc.io/diff/BUILD.bazel",
			Want:   vdmtest.TxtarFS(t, "testdata/extracted/copyBUILD.txtar"),
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			// Test the String method.
			if got := test.Action.String(); got != test.String {
				t.Errorf("CopyBUILD.String():\nGot:  %q\nWant: %q", got, test.String)
			}

			// Test the action.
			target := t.ArtifactDir()
			dirFS := vdmtest.DirFS(t)
			test.Action.Path = filepath.Join(target, filepath.FromSlash(test.Action.Path))
			err := os.MkdirAll(filepath.Dir(test.Action.Path), 0o777)
			if err != nil {
				t.Fatalf("failed to create target directory: %v", err)
			}

			if test.Init != nil {
				test.Init(t, test.Action)
			}

			err = test.Action.Do(context.Background(), test.FS, io.Discard)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("CopyBUILD(): unexpected lack of error")
				}

				e := err.Error()
				if test.ErrorCmp != nil && !test.ErrorCmp(e, test.Error) {
					t.Fatalf("CopyBUILD(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				} else if test.ErrorCmp == nil && e != test.Error {
					t.Fatalf("CopyBUILD(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("CopyBUILD(): got unexpected error: %v", err)
			}

			// Check the contents.
			err = vdmtest.DiffTextFilesystems(dirFS, test.Want, ves.Vendor)
			if err != nil {
				t.Fatalf("CopyBUILD(): contents mismatch: %v", err)
			}
		})
	}
}

func TestGenerateGoPackageBUILD(t *testing.T) {
	tests := []struct {
		Name     string
		Init     func(t *testing.T, action *GenerateGoPackageBUILD) // Optional setup code.
		FS       fs.FS
		Action   *GenerateGoPackageBUILD
		Want     fs.FS
		String   string
		Error    string
		ErrorCmp func(string, string) bool
	}{
		{
			Name: "invalid-bad-fs",
			FS:   vdmtest.TestFS(t, vdmtest.WithErrors("vendor/rsc.io/diff", "bad dir")),
			Action: &GenerateGoPackageBUILD{
				Package: &ves.GoPackage{
					Name: ves.S("rsc.io/diff"),
				},
				Dir:  "vendor/rsc.io/diff",
				Path: "vendor/rsc.io/diff/BUILD.bazel",
			},
			String: "generate BUILD file for Go package rsc.io/diff to vendor/rsc.io/diff/BUILD.bazel",
			Error:  `failed to read directory containing rsc.io/diff: bad dir`,
		},
		{
			Name: "invalid-bad-destination-dir",
			Init: func(t *testing.T, action *GenerateGoPackageBUILD) {
				// Make the parent directory for the destination
				// unreadable so the stat fails.
				err := os.Chmod(filepath.Dir(action.Path), 0o222)
				if err != nil {
					t.Fatalf("failed to make parent directory unreadable: %v", err)
				}
			},
			FS: vdmtest.TxtarFS(t, "testdata/actions/no-tests-package.txtar"),
			Action: &GenerateGoPackageBUILD{
				Package: &ves.GoPackage{
					Name: ves.S("example.com/foo"),
				},
				Dir:  "vendor/example.com/foo",
				Path: "vendor/example.com/foo/BUILD.bazel",
			},
			String:   "generate BUILD file for Go package example.com/foo to vendor/example.com/foo/BUILD.bazel",
			Error:    `vendor/example.com/foo/BUILD.bazel: permission denied`,
			ErrorCmp: strings.HasSuffix,
		},
		{
			Name: "valid-no-tests-package",
			FS:   vdmtest.TxtarFS(t, "testdata/actions/no-tests-package.txtar"),
			Action: &GenerateGoPackageBUILD{
				Package: &ves.GoPackage{
					Name: ves.S("example.com/foo"),
				},
				Dir:  "vendor/example.com/foo",
				Path: "vendor/example.com/foo/BUILD.bazel",
			},
			String: "generate BUILD file for Go package example.com/foo to vendor/example.com/foo/BUILD.bazel",
			Want:   vdmtest.TxtarFS(t, "testdata/extracted/no-tests-package.txtar"),
		},
		{
			Name: "valid-complex-package",
			Init: func(t *testing.T, action *GenerateGoPackageBUILD) {
				// Make the destination file as read-only, so
				// we have to change it back to writeable.
				err := os.WriteFile(action.Path, []byte("foo"), 0o444)
				if err != nil {
					t.Fatalf("failed to write destination file: %v", err)
				}
			},
			FS: vdmtest.TxtarFS(t, "testdata/actions/complex-package.txtar"),
			Action: &GenerateGoPackageBUILD{
				Package: &ves.GoPackage{
					Name: ves.S("example.com/foo"),
				},
				Dir:  "vendor/example.com/foo",
				Path: "vendor/example.com/foo/BUILD.bazel",
			},
			String: "generate BUILD file for Go package example.com/foo to vendor/example.com/foo/BUILD.bazel",
			Want:   vdmtest.TxtarFS(t, "testdata/extracted/complex-package.txtar"),
		},
		{
			Name: "valid-complex-binary",
			FS:   vdmtest.TxtarFS(t, "testdata/actions/complex-binary.txtar"),
			Action: &GenerateGoPackageBUILD{
				Package: &ves.GoPackage{
					Name: ves.S("example.com/foo"),
					Deps: []ves.ParsedString{
						ves.S("rsc.io/diff"),
						ves.S("rsc.io/quote"),
					},
					Embed: []ves.ParsedString{
						ves.S("README.md"),
						ves.S("a.go"),
					},
					EmbedGlobs: []ves.ParsedString{
						ves.S("templates/**/*.tmpl"),
					},

					Binary: ves.B(true),
					BinaryDeps: []ves.ParsedString{
						ves.S("github.com/spf13/cobra"),
					},

					TestSize: ves.S("medium"),
					TestData: []ves.ParsedString{
						ves.S("README.md"),
					},
					TestDataGlobs: []ves.ParsedString{
						ves.S("*_test.go"),
						ves.S("templates/**/*.tmpl"),
					},
					TestDeps: []ves.ParsedString{
						ves.S("github.com/google/go-cmp/cmp"),
						ves.S("github.com/google/go-cmp/cmp/cmpopts"),
					},
					TestEnv: map[string]ves.ParsedString{
						"GOPATH": ves.S("/home/test/go"),
						"HOME":   ves.S("/home/test"),
					},
				},
				Dir:  "vendor/example.com/foo",
				Path: "vendor/example.com/foo/BUILD.bazel",
			},
			String: "generate BUILD file for Go package example.com/foo to vendor/example.com/foo/BUILD.bazel",
			Want:   vdmtest.TxtarFS(t, "testdata/extracted/complex-binary.txtar"),
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			// Test the String method.
			if got := test.Action.String(); got != test.String {
				t.Errorf("GenerateGoPackageBUILD.String():\nGot:  %q\nWant: %q", got, test.String)
			}

			// Test the action.
			target := t.ArtifactDir()
			dirFS := vdmtest.DirFS(t)
			test.Action.Path = filepath.Join(target, filepath.FromSlash(test.Action.Path))
			err := os.MkdirAll(filepath.Dir(test.Action.Path), 0o777)
			if err != nil {
				t.Fatalf("failed to create target directory: %v", err)
			}

			if test.Init != nil {
				test.Init(t, test.Action)
			}

			err = test.Action.Do(context.Background(), test.FS, io.Discard)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("GenerateGoPackageBUILD(): unexpected lack of error")
				}

				e := err.Error()
				if test.ErrorCmp != nil && !test.ErrorCmp(e, test.Error) {
					t.Fatalf("GenerateGoPackageBUILD(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				} else if test.ErrorCmp == nil && e != test.Error {
					t.Fatalf("GenerateGoPackageBUILD(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("GenerateGoPackageBUILD(): got unexpected error: %v", err)
			}

			// Check the contents.
			err = vdmtest.DiffTextFilesystems(dirFS, test.Want, ves.Vendor)
			if err != nil {
				t.Fatalf("GenerateGoPackageBUILD(): contents mismatch: %v", err)
			}
		})
	}
}
