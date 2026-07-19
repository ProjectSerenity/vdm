// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package gomodzip

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ProjectSerenity/vdm/internal/vdmtest"
	"github.com/ProjectSerenity/vdm/internal/ves"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/tools/txtar"
	"rsc.io/diff"
	"rsc.io/tmp/patch"
)

func TestExtract(t *testing.T) {
	tests := []struct {
		Name     string
		ZIP      string
		Module   *ves.GoModule
		Manifest *ves.GoModuleManifest
		Want     string
		Error    string
	}{
		{
			Name: "invalid-bad-checksum",
			ZIP:  filepath.FromSlash("testdata/zips/golang.org/x/arch/@v/v0.13.0.zip"),
			Module: &ves.GoModule{
				Name:    "golang.org/x/arch",
				Version: ves.S("v0.13.0"),
				Packages: []*ves.GoPackage{
					{Name: ves.S("golang.org/x/arch/x86/x86asm")},
				},
			},
			Manifest: &ves.GoModuleManifest{
				Name:     "golang.org/x/arch",
				Version:  ves.S("v0.13.0"),
				Download: ves.S("sha256:checksum"),
			},
			Error: `got checksum sha256:KCkqVVV1kGg0X87TFysjCJ8MxtZEIU4Ja/yXGeoECdA=, want sha256:checksum`,
		},
		{
			Name: "invalid-bad-target",
			ZIP:  filepath.FromSlash("testdata/zips/golang.org/x/arch/@v/v0.13.0.zip"),
			Module: &ves.GoModule{
				Name:    "golang.org/x/arch",
				Version: ves.S("v0.13.0"),
				Packages: []*ves.GoPackage{
					{Name: ves.S("golang.org/x/arch/x86/x86asm")},
				},
			},
			Manifest: &ves.GoModuleManifest{
				Name:     "golang.org/x/arch",
				Version:  ves.S("v0.14.0"),
				Download: ves.S("sha256:KCkqVVV1kGg0X87TFysjCJ8MxtZEIU4Ja/yXGeoECdA="),
			},
			Error: `path does not have prefix "golang.org/x/arch@v0.14.0/"`,
		},
		{
			Name: "invalid-bad-patches",
			ZIP:  filepath.FromSlash("testdata/zips/golang.org/x/arch/@v/v0.13.0.zip"),
			Module: &ves.GoModule{
				Name:    "golang.org/x/arch",
				Version: ves.S("v0.13.0"),
				Patches: []ves.ParsedString{
					ves.S("testdata/nonexistant.patch"),
				},
				Packages: []*ves.GoPackage{
					{Name: ves.S("golang.org/x/arch/x86/x86asm")},
				},
			},
			Manifest: &ves.GoModuleManifest{
				Name:     "golang.org/x/arch",
				Version:  ves.S("v0.13.0"),
				Download: ves.S("sha256:KCkqVVV1kGg0X87TFysjCJ8MxtZEIU4Ja/yXGeoECdA="),
			},
			Error: `failed to open patch path "testdata/nonexistant.patch"`,
		},
		{
			Name: "valid-complex-no-patches",
			ZIP:  filepath.FromSlash("testdata/zips/golang.org/x/arch/@v/v0.13.0.zip"),
			Module: &ves.GoModule{
				Name:    "golang.org/x/arch",
				Version: ves.S("v0.13.0"),
				Packages: []*ves.GoPackage{
					{Name: ves.S("golang.org/x/arch/x86/x86asm")},
				},
			},
			Manifest: &ves.GoModuleManifest{
				Name:     "golang.org/x/arch",
				Version:  ves.S("v0.13.0"),
				Download: ves.S("sha256:KCkqVVV1kGg0X87TFysjCJ8MxtZEIU4Ja/yXGeoECdA="),
			},
			Want: filepath.FromSlash("testdata/zips/golang.org/x/arch/@v/v0.13.0-pruned.zip"),
		},
		{
			Name: "valid-complex-with-patches",
			ZIP:  filepath.FromSlash("testdata/zips/golang.org/x/arch/@v/v0.13.0.zip"),
			Module: &ves.GoModule{
				Name:    "golang.org/x/arch",
				Version: ves.S("v0.13.0"),
				Patches: []ves.ParsedString{
					ves.S("testdata/patching/golang.org_x_arch.patch"),
				},
				Packages: []*ves.GoPackage{
					{Name: ves.S("golang.org/x/arch/x86/x86asm")},
				},
			},
			Manifest: &ves.GoModuleManifest{
				Name:     "golang.org/x/arch",
				Version:  ves.S("v0.13.0"),
				Download: ves.S("sha256:KCkqVVV1kGg0X87TFysjCJ8MxtZEIU4Ja/yXGeoECdA="),
			},
			Want: filepath.FromSlash("testdata/zips/golang.org/x/arch/@v/v0.13.0-patched.zip"),
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			x := new(extractor)
			gotTarget := filepath.Join(t.ArtifactDir(), "got")
			dst := filepath.Join(gotTarget, filepath.FromSlash(test.Manifest.Name))
			err := os.MkdirAll(dst, 0o777)
			if err != nil {
				t.Fatalf("Extract(): failed to create %s: %v", dst, err)
			}

			err = Extract(gotTarget, test.ZIP, test.Module, test.Manifest)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("Extract(): unexpected lack of error")
				}

				e := err.Error()
				if !strings.Contains(e, test.Error) {
					t.Fatalf("Extract(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("Extract(): got unexpected error: %v", err)
			}

			// Check we got the files we wanted.
			wantTarget := filepath.Join(t.ArtifactDir(), "want")
			dst = filepath.Join(wantTarget, filepath.FromSlash(test.Manifest.Name))
			err = os.MkdirAll(dst, 0o777)
			if err != nil {
				t.Fatalf("Extract(): failed to create %s: %v", dst, err)
			}

			err = x.ExtractAndPrune(wantTarget, test.Want, test.Module, test.Manifest)
			if err != nil {
				t.Fatalf("Extract(): failed to extract %s: %v", test.Manifest.Name, err)
			}

			err = vdmtest.DiffTextDirectories(vdmtest.DirFS(t), "got", "want")
			if err != nil {
				t.Fatalf("Extract(): %v", err)
			}
		})
	}
}

func TestExtractor_SaveError(t *testing.T) {
	tests := []struct {
		Name  string
		X     *extractor
		Err   error
		Error string
	}{
		{
			Name:  "keep-saved",
			X:     &extractor{SavedError: errors.New("first error")},
			Err:   errors.New("second error"),
			Error: "first error",
		},
		{
			Name:  "keep-new",
			X:     &extractor{},
			Err:   errors.New("second error"),
			Error: "second error",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			test.X.SaveError(test.Err)
			err := test.X.SavedError
			if test.Error != "" {
				if err == nil {
					t.Fatalf("SaveError(): unexpected lack of error")
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("SaveError(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("SaveError(): got unexpected error: %v", err)
			}
		})
	}
}

func TestExtractor_CompareChecksum(t *testing.T) {
	tests := []struct {
		Name     string
		Got      string
		Manifest *ves.GoModuleManifest
		Error    string
	}{
		{
			Name:     "invalid-bad-format",
			Got:      "checksum",
			Manifest: &ves.GoModuleManifest{Name: "rsc.io/diff"},
			Error:    `failed to verify Go module rsc.io/diff: invalid checksum "checksum": missing colon`,
		},
		{
			Name: "invalid-mismatch",
			Got:  "h1:asdf",
			Manifest: &ves.GoModuleManifest{
				Name:     "rsc.io/diff",
				Version:  ves.S("v0.0.0-20190621135850-fe3479844c3c"),
				Download: ves.S("sha256:/WCDjRGIVDjKlhtSc1PEApp2fR58gfSVK62dr/yQNyQ="),
			},
			Error: `failed to verify Go module rsc.io/diff: got checksum sha256:asdf, want sha256:/WCDjRGIVDjKlhtSc1PEApp2fR58gfSVK62dr/yQNyQ=`,
		},
		{
			Name: "valid-simple",
			Got:  "h1:/WCDjRGIVDjKlhtSc1PEApp2fR58gfSVK62dr/yQNyQ=",
			Manifest: &ves.GoModuleManifest{
				Name:     "rsc.io/diff",
				Version:  ves.S("v0.0.0-20190621135850-fe3479844c3c"),
				Download: ves.S("sha256:/WCDjRGIVDjKlhtSc1PEApp2fR58gfSVK62dr/yQNyQ="),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			x := new(extractor)
			err := x.CompareChecksum(test.Got, test.Manifest)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("VerifyChecksum(): unexpected lack of error")
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("VerifyChecksum(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("VerifyChecksum(): got unexpected error: %v", err)
			}
		})
	}
}

func TestExtractor_VerifyChecksum(t *testing.T) {
	tests := []struct {
		Name     string
		Path     string
		Manifest *ves.GoModuleManifest
		Error    string
	}{
		{
			Name: "invalid-bad-path",
			Path: filepath.FromSlash("testdata/nonexistant.zip"),
			Manifest: &ves.GoModuleManifest{
				Name:     "rsc.io/diff",
				Version:  ves.S("v0.0.0-20190621135850-fe3479844c3c"),
				Download: ves.S("sha256:/WCDjRGIVDjKlhtSc1PEApp2fR58gfSVK62dr/yQNyQ="),
			},
			Error: `failed to verify Go module rsc.io/diff: open testdata/nonexistant.zip: no such file or directory`,
		},
		{
			Name: "valid-simple",
			Path: filepath.FromSlash("testdata/zips/rsc.io/diff/@v/v0.0.0-20190621135850-fe3479844c3c.zip"),
			Manifest: &ves.GoModuleManifest{
				Name:     "rsc.io/diff",
				Version:  ves.S("v0.0.0-20190621135850-fe3479844c3c"),
				Download: ves.S("sha256:/WCDjRGIVDjKlhtSc1PEApp2fR58gfSVK62dr/yQNyQ="),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			x := new(extractor)
			err := x.VerifyChecksum(test.Path, test.Manifest)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("VerifyChecksum(): unexpected lack of error")
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("VerifyChecksum(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("VerifyChecksum(): got unexpected error: %v", err)
			}
		})
	}
}

func TestExtractor_Extract(t *testing.T) {
	tests := []struct {
		Name     string
		Dst      string
		Path     string
		Manifest *ves.GoModuleManifest
		Error    string
	}{
		{
			Name: "invalid-missing-zip",
			Dst:  t.ArtifactDir(),
			Path: filepath.FromSlash("testdata/nonexistant.zip"),
			Manifest: &ves.GoModuleManifest{
				Name:    "rsc.io/diff",
				Version: ves.S("v0.0.0-20190621135850-fe3479844c3c"),
			},
			Error: `failed to unzip Go module rsc.io/diff: unzip testdata/nonexistant.zip: open testdata/nonexistant.zip: no such file or directory`,
		},
		{
			Name: "valid-simple",
			Dst:  t.ArtifactDir(),
			Path: filepath.FromSlash("testdata/zips/rsc.io/diff/@v/v0.0.0-20190621135850-fe3479844c3c.zip"),
			Manifest: &ves.GoModuleManifest{
				Name:    "rsc.io/diff",
				Version: ves.S("v0.0.0-20190621135850-fe3479844c3c"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			x := new(extractor)
			err := x.Extract(test.Dst, test.Path, test.Manifest)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("Extract(): unexpected lack of error")
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("Extract(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("Extract(): got unexpected error: %v", err)
			}
		})
	}
}

func TestParentDirectories(t *testing.T) {
	tests := []struct {
		Dir  string
		Want []string
	}{
		{"github.com/SlyMarbo/marbo", []string{"github.com/SlyMarbo", "github.com"}},
		{"github.com/SlyMarbo/marbo/foo", []string{"github.com/SlyMarbo/marbo", "github.com/SlyMarbo", "github.com"}},
	}

	for _, test := range tests {
		got := slices.Collect(parentDirectories(test.Dir))
		if diff := cmp.Diff(test.Want, got); diff != "" {
			t.Errorf("parentDirectories(%q): modules mismatch (-want, +got)\n%s", test.Dir, diff)
		}

		// Increase test coverage.
		for range parentDirectories(test.Dir) {
			break
		}
	}
}

func TestExtractor_IdentifyDeletions(t *testing.T) {
	tests := []struct {
		Name   string
		FS     fs.FS
		Module *ves.GoModule
		Want   []string
		Error  string
	}{
		{
			Name: "invalid-bad-fs",
			FS:   vdmtest.TestFS(t, vdmtest.WithErrors("rsc.io/diff", "file does not exist")),
			Module: &ves.GoModule{
				Name:    "rsc.io/diff",
				Version: ves.S("v1.2.3"),
			},
			Error: `failed to identify unused Go packages to delete in rsc.io/diff: file does not exist`,
		},
		{
			Name: "valid-single-module",
			FS:   vdmtest.TxtarFS(t, "testdata/pruning/complex.txtar"),
			Module: &ves.GoModule{
				Name:    "rsc.io/diff",
				Version: ves.S("v1.2.3"),
				Packages: []*ves.GoPackage{
					{Name: ves.S("rsc.io/diff")},
					{Name: ves.S("rsc.io/diff/deeply/nested/pkg")},
				},
			},
			Want: []string{
				"rsc.io/diff/BUILD.bazel",
				"rsc.io/diff/deeply/nested/other",
				"rsc.io/diff/deeply/nested/pkg/child",
			},
		},
		{
			Name: "valid-multi-module",
			FS:   vdmtest.TxtarFS(t, "testdata/pruning/complex.txtar"),
			Module: &ves.GoModule{
				Name:    "rsc.io/diff",
				Version: ves.S("v1.2.3"),
				Packages: []*ves.GoPackage{
					{Name: ves.S("rsc.io/diff")},
					{
						Name: ves.S("rsc.io/diff/deeply/nested/pkg"),
						Directories: []ves.ParsedString{
							ves.S("nonexistant"),
						},
					},
				},
			},
			Want: []string{
				"rsc.io/diff/BUILD.bazel",
				"rsc.io/diff/deeply/nested/other",
				"rsc.io/diff/deeply/nested/pkg/child",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			x := new(extractor)
			got, err := x.IdentifyDeletions(test.FS, test.Module)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("IdentifyDeletions(): unexpected lack of error")
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("IdentifyDeletions(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("IdentifyDeletions(): got unexpected error: %v", err)
			}

			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Errorf("IdentifyDeletions(): deletions mismatch (-want, +got)\n%s", diff)
			}
		})
	}
}

func TestExtractor_ExtractAndPrune(t *testing.T) {
	tests := []struct {
		Name     string
		Path     string
		Module   *ves.GoModule
		Manifest *ves.GoModuleManifest
		Want     string
		Error    string
	}{
		{
			Name: "invalid-missing-zip",
			Path: filepath.FromSlash("testdata/nonexistant.zip"),
			Manifest: &ves.GoModuleManifest{
				Name:    "rsc.io/diff",
				Version: ves.S("v0.0.0-20190621135850-fe3479844c3c"),
			},
			Error: `failed to unzip Go module rsc.io/diff: unzip testdata/nonexistant.zip: open testdata/nonexistant.zip: no such file or directory`,
		},
		{
			Name: "valid-golang.org-x-arch",
			Path: filepath.FromSlash("testdata/zips/golang.org/x/arch/@v/v0.13.0.zip"),
			Module: &ves.GoModule{
				Name:    "golang.org/x/arch",
				Version: ves.S("v0.13.0"),
				Packages: []*ves.GoPackage{
					{Name: ves.S("golang.org/x/arch/x86/x86asm")},
				},
			},
			Manifest: &ves.GoModuleManifest{
				Name:    "golang.org/x/arch",
				Version: ves.S("v0.13.0"),
			},
			Want: filepath.FromSlash("testdata/zips/golang.org/x/arch/@v/v0.13.0-pruned.zip"),
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			x := new(extractor)
			gotTarget := filepath.Join(t.ArtifactDir(), "got")
			dst := filepath.Join(gotTarget, filepath.FromSlash(test.Manifest.Name))
			err := os.MkdirAll(dst, 0o777)
			if err != nil {
				t.Fatalf("failed to create %s: %v", dst, err)
			}

			err = x.ExtractAndPrune(gotTarget, test.Path, test.Module, test.Manifest)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("ExtractAndPrune(): unexpected lack of error")
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("ExtractAndPrune(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("ExtractAndPrune(): got unexpected error: %v", err)
			}

			wantTarget := filepath.Join(t.ArtifactDir(), "want")
			dst = filepath.Join(wantTarget, filepath.FromSlash(test.Manifest.Name))
			err = os.MkdirAll(dst, 0o777)
			if err != nil {
				t.Fatalf("failed to create %s: %v", dst, err)
			}

			err = x.Extract(dst, test.Want, test.Manifest)
			if err != nil {
				t.Fatalf("failed to extract want: %v", err)
			}

			err = vdmtest.DiffFilenames(vdmtest.DirFS(t), "got", "want")
			if err != nil {
				t.Fatalf("ExtractAndPrune(): %v", err)
			}
		})
	}
}

func TestExtractor_CloseClosers(t *testing.T) {
	tests := []struct {
		Name    string
		Closers []io.Closer
		Names   []string
		Context string
		Error   string
	}{
		{
			Name: "invalid-error",
			Closers: []io.Closer{
				readCloser{err: errors.New("close error")},
			},
			Names: []string{
				"foo",
			},
			Context: "test case",
			Error:   "failed to close test case foo: close error",
		},
		{
			Name: "valid-no-error",
			Closers: []io.Closer{
				readCloser{},
			},
			Names: []string{
				"foo",
			},
			Context: "test case",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			x := new(extractor)
			x.CloseClosers(test.Closers, test.Names, test.Context)
			err := x.SavedError
			if test.Error != "" {
				if err == nil {
					t.Fatalf("CloseClosers(): unexpected lack of error")
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("CloseClosers(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("CloseClosers(): got unexpected error: %v", err)
			}
		})
	}
}

func TestExtractor_PatchWithBinary(t *testing.T) {
	tests := []struct {
		Name     string
		Path     string
		ZIP      string
		Module   *ves.GoModule
		Manifest *ves.GoModuleManifest
		FS       fs.FS
		Patches  []string
		Want     string
		Error    string
	}{
		{
			Name:    "invalid-bad-filesystem",
			FS:      vdmtest.TestFS(t, vdmtest.WithErrors("open", "bad open")),
			Patches: []string{"open"},
			Error:   `failed to open patch path "open": bad open`,
		},
		{
			Name:    "invalid-bad-path",
			Path:    filepath.FromSlash("testdata/nonexistant"),
			FS:      os.DirFS("testdata"),
			Patches: []string{"patching/golang.org_x_arch.patch"},
			Error:   `failed to run patch: chdir testdata/nonexistant: no such file or directory`,
		},
		{
			Name: "valid-complex",
			ZIP:  filepath.FromSlash("testdata/zips/golang.org/x/arch/@v/v0.13.0.zip"),
			Module: &ves.GoModule{
				Name:    "golang.org/x/arch",
				Version: ves.S("v0.13.0"),
				Packages: []*ves.GoPackage{
					{Name: ves.S("golang.org/x/arch/x86/x86asm")},
				},
			},
			Manifest: &ves.GoModuleManifest{
				Name:    "golang.org/x/arch",
				Version: ves.S("v0.13.0"),
			},
			FS:      os.DirFS("testdata"),
			Patches: []string{"patching/golang.org_x_arch.patch"},
			Want:    filepath.FromSlash("testdata/zips/golang.org/x/arch/@v/v0.13.0-patched.zip"),
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			x := new(extractor)
			var gotTarget, path string
			if test.ZIP != "" {
				gotTarget = filepath.Join(t.ArtifactDir(), "got")
				dst := filepath.Join(gotTarget, filepath.FromSlash(test.Manifest.Name))
				err := os.MkdirAll(dst, 0o777)
				if err != nil {
					t.Fatalf("PatchWithBinary(): failed to create %s: %v", dst, err)
				}

				err = x.ExtractAndPrune(gotTarget, test.ZIP, test.Module, test.Manifest)
				if err != nil {
					t.Fatalf("PatchWithBinary(): failed to extract %s: %v", test.Manifest.Name, err)
				}

				path = dst
			}

			if test.Path != "" {
				path = test.Path
			}

			err := x.PatchWithBinary(io.Discard, test.FS, path, test.Patches)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("PatchWithBinary(): unexpected lack of error")
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("PatchWithBinary(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("PatchWithBinary(): got unexpected error: %v", err)
			}

			wantTarget := filepath.Join(t.ArtifactDir(), "want")
			dst := filepath.Join(wantTarget, filepath.FromSlash(test.Manifest.Name))
			err = os.MkdirAll(dst, 0o777)
			if err != nil {
				t.Fatalf("PatchWithBinary(): failed to create %s: %v", dst, err)
			}

			err = x.ExtractAndPrune(wantTarget, test.Want, test.Module, test.Manifest)
			if err != nil {
				t.Fatalf("PatchWithBinary(): failed to extract %s: %v", test.Manifest.Name, err)
			}

			err = vdmtest.DiffTextDirectories(vdmtest.DirFS(t), "got", "want")
			if err != nil {
				t.Fatalf("PatchWithBinary(): %v", err)
			}
		})
	}
}

func readPatch(t *testing.T, name string) *patch.File {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", filepath.FromSlash(name)))
	if err != nil {
		t.Fatalf("failed to read patch %s: %v", name, err)
	}

	set, err := patch.Parse([]byte(data))
	if err != nil {
		t.Fatalf("failed to parse patch: %v", err)
	}

	if len(set.File) != 1 {
		t.Fatalf("patch has %d files, want %d", len(set.File), 1)
	}

	return set.File[0]
}

func TestExtractor_SinglePatchWithPackage(t *testing.T) {
	tests := []struct {
		Name  string
		File  *patch.File
		Init  string
		Want  string
		Error string
	}{
		{
			Name:  "invalid-add-bad-destination",
			File:  readPatch(t, "patching/add1.patch"),
			Error: `nonexistant/added.txt: no such file or directory`,
		},
		{
			Name: "valid-add-simple",
			File: readPatch(t, "patching/add2.patch"),
			Want: "testdata/patching/add2-want.txtar",
		},
		{
			Name:  "invalid-remove-missing",
			File:  readPatch(t, "patching/remove1.patch"),
			Error: `redundant.txt: no such file or directory`,
		},
		{
			Name: "valid-remove-simple",
			File: readPatch(t, "patching/remove2.patch"),
			Init: "testdata/patching/remove2-init.txtar",
			Want: "testdata/patching/remove2-want.txtar",
		},
		{
			Name:  "invalid-edit-missing",
			File:  readPatch(t, "patching/edit1.patch"),
			Error: `nonexistant/text.txt: no such file or directory`,
		},
		{
			Name:  "invalid-edit-wrong-content",
			File:  readPatch(t, "patching/edit2.patch"),
			Init:  "testdata/patching/edit2-init.txtar",
			Error: `failed to edit "text.txt": patch did not apply cleanly`,
		},
		{
			Name: "invalid-edit-bad-destination",
			File: func() *patch.File {
				base := readPatch(t, "patching/edit3.patch")
				base.Dst = "nonexistant/text.txt" // The parser uses the same file for both, so we have to modify it.
				return base
			}(),
			Init:  "testdata/patching/edit3-init.txtar",
			Error: `nonexistant/text.txt: no such file or directory`,
		},
		{
			Name: "valid-edit-simple",
			File: readPatch(t, "patching/edit4.patch"),
			Init: "testdata/patching/edit4-init.txtar",
			Want: "testdata/patching/edit4-want.txtar",
		},
		{
			Name:  "invalid-rename-missing",
			File:  readPatch(t, "patching/rename1.patch"),
			Error: `no such file or directory`,
		},
		{
			Name: "valid-rename-simple",
			File: readPatch(t, "patching/rename2.patch"),
			Init: "testdata/patching/rename2-init.txtar",
			Want: "testdata/patching/rename2-want.txtar",
		},
		{
			Name:  "invalid-bad-verb",
			File:  &patch.File{Verb: "wibble"},
			Error: `unexpected patch operation "wibble"`,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			x := new(extractor)
			dir := t.ArtifactDir()
			if test.Init != "" {
				vdmtest.ExtractTxtar(t, dir, test.Init)
			}

			err := x.SinglePatchWithPackage(dir, test.File)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("SinglePatchWithPackage(): unexpected lack of error")
				}

				e := err.Error()
				if !strings.Contains(e, test.Error) {
					t.Fatalf("SinglePatchWithPackage(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("SinglePatchWithPackage(): got unexpected error: %v", err)
			}

			ar, err := txtar.ParseFile(filepath.FromSlash(test.Want))
			if err != nil {
				t.Fatalf("SinglePatchWithPackage(): failed to read want txtar: %v", err)
			}

			done := make(map[string]bool)
			for _, file := range ar.Files {
				done[file.Name] = true
				got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(file.Name)))
				if err != nil {
					t.Errorf("SinglePatchWithPackage(): failed to read wanted file %s: %v", file.Name, err)
					continue
				}

				if !bytes.Equal(file.Data, got) {
					delta := diff.Format(string(file.Data), string(got))
					t.Errorf("SinglePatchWithPackage(): wrong contents for %s:\n%s", file.Name, delta)
					continue
				}
			}

			filenames, err := vdmtest.Filenames(os.DirFS(dir), ".")
			if err != nil {
				t.Fatalf("SinglePatchWithPackage(): %v", err)
			}

			extra := make([]string, 0, len(filenames))
			for _, filename := range filenames {
				if !done[filename] {
					extra = append(extra, filename)
				}
			}

			if len(extra) != 0 {
				t.Fatalf("SinglePatchWithPackage(): extra files:\n  %s", strings.Join(extra, "\n  "))
			}
		})
	}
}

func TestExtractor_PatchWithPackage(t *testing.T) {
	tests := []struct {
		Name     string
		Path     string
		ZIP      string
		Manifest *ves.GoModuleManifest
		FS       fs.FS
		Patches  []string
		Want     string
		Error    string
	}{
		{
			Name:    "invalid-bad-filesystem",
			FS:      vdmtest.TestFS(t, vdmtest.WithErrors("open", "bad open")),
			Patches: []string{"open"},
			Error:   `failed to open patch path "open": bad open`,
		},
		{
			Name:    "invalid-bad-patch",
			FS:      os.DirFS("testdata"),
			Patches: []string{"patching/invalid.patch"},
			Error:   `unexpected patch header line: diff XXX`,
		},
		{
			Name: "invalid-bad-removal",
			ZIP:  filepath.FromSlash("testdata/zips/example.com/foo/@v/v1.2.3.zip"),
			Manifest: &ves.GoModuleManifest{
				Name:    "example.com/foo",
				Version: ves.S("v1.2.3"),
			},
			FS:      os.DirFS("testdata"),
			Patches: []string{"patching/remove1.patch"},
			Error:   `foo/redundant.txt: no such file or directory`,
		},
		{
			Name: "valid-constructed",
			ZIP:  filepath.FromSlash("testdata/zips/example.com/foo/@v/v1.2.3.zip"),
			Manifest: &ves.GoModuleManifest{
				Name:    "example.com/foo",
				Version: ves.S("v1.2.3"),
			},
			FS:      os.DirFS("testdata"),
			Patches: []string{"patching/constructed.patch"},
			Want:    filepath.FromSlash("testdata/zips/example.com/foo/@v/v1.2.3-patched.zip"),
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			x := new(extractor)
			var gotTarget, path string
			if test.ZIP != "" {
				gotTarget = filepath.Join(t.ArtifactDir(), "got")
				dst := filepath.Join(gotTarget, filepath.FromSlash(test.Manifest.Name))
				err := os.MkdirAll(dst, 0o777)
				if err != nil {
					t.Fatalf("PatchWithPackage(): failed to create %s: %v", dst, err)
				}

				err = x.Extract(dst, test.ZIP, test.Manifest)
				if err != nil {
					t.Fatalf("PatchWithPackage(): failed to extract %s: %v", test.Manifest.Name, err)
				}

				path = dst
			}

			if test.Path != "" {
				path = test.Path
			}

			err := x.PatchWithPackage(test.FS, path, test.Patches)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("PatchWithPackage(): unexpected lack of error")
				}

				e := err.Error()
				if !strings.Contains(e, test.Error) {
					t.Fatalf("PatchWithPackage(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("PatchWithPackage(): got unexpected error: %v", err)
			}

			wantTarget := filepath.Join(t.ArtifactDir(), "want")
			dst := filepath.Join(wantTarget, filepath.FromSlash(test.Manifest.Name))
			err = os.MkdirAll(dst, 0o777)
			if err != nil {
				t.Fatalf("PatchWithPackage(): failed to create %s: %v", dst, err)
			}

			err = x.Extract(dst, test.Want, test.Manifest)
			if err != nil {
				t.Fatalf("PatchWithPackage(): failed to extract %s: %v", test.Manifest.Name, err)
			}

			err = vdmtest.DiffTextDirectories(vdmtest.DirFS(t), "got", "want")
			if err != nil {
				t.Fatalf("PatchWithPackage(): %v", err)
			}
		})
	}
}

func TestExtractor_ApplyPatches(t *testing.T) {
	tests := []struct {
		Name     string
		Path     string
		ZIP      string
		Module   *ves.GoModule
		Manifest *ves.GoModuleManifest
		FS       fs.FS
		Patches  []string
		Want     string
		Error    string
	}{
		{
			Name: "valid-complex",
			ZIP:  filepath.FromSlash("testdata/zips/golang.org/x/arch/@v/v0.13.0.zip"),
			Module: &ves.GoModule{
				Name:    "golang.org/x/arch",
				Version: ves.S("v0.13.0"),
				Packages: []*ves.GoPackage{
					{Name: ves.S("golang.org/x/arch/x86/x86asm")},
				},
			},
			Manifest: &ves.GoModuleManifest{
				Name:    "golang.org/x/arch",
				Version: ves.S("v0.13.0"),
			},
			FS:      os.DirFS("testdata"),
			Patches: []string{"patching/golang.org_x_arch.patch"},
			Want:    filepath.FromSlash("testdata/zips/golang.org/x/arch/@v/v0.13.0-patched.zip"),
		},
	}

	for _, test := range tests {
		doTest := func(t *testing.T, usePackage bool) {
			x := new(extractor)
			var gotTarget, path string
			if test.ZIP != "" {
				gotTarget = filepath.Join(t.ArtifactDir(), "got")
				dst := filepath.Join(gotTarget, filepath.FromSlash(test.Manifest.Name))
				err := os.MkdirAll(dst, 0o777)
				if err != nil {
					t.Fatalf("ApplyPatches(): failed to create %s: %v", dst, err)
				}

				err = x.ExtractAndPrune(gotTarget, test.ZIP, test.Module, test.Manifest)
				if err != nil {
					t.Fatalf("ApplyPatches(): failed to extract %s: %v", test.Manifest.Name, err)
				}

				path = dst
			}

			if test.Path != "" {
				path = test.Path
			}

			err := x.ApplyPatches(test.FS, path, test.Patches, usePackage)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("ApplyPatches(): unexpected lack of error")
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("ApplyPatches(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("ApplyPatches(): got unexpected error: %v", err)
			}

			wantTarget := filepath.Join(t.ArtifactDir(), "want")
			dst := filepath.Join(wantTarget, filepath.FromSlash(test.Manifest.Name))
			err = os.MkdirAll(dst, 0o777)
			if err != nil {
				t.Fatalf("ApplyPatches(): failed to create %s: %v", dst, err)
			}

			err = x.ExtractAndPrune(wantTarget, test.Want, test.Module, test.Manifest)
			if err != nil {
				t.Fatalf("ApplyPatches(): failed to extract %s: %v", test.Manifest.Name, err)
			}

			err = vdmtest.DiffTextDirectories(vdmtest.DirFS(t), "got", "want")
			if err != nil {
				t.Fatalf("ApplyPatches(): %v", err)
			}
		}

		t.Run(test.Name, func(t *testing.T) {
			t.Run("Binary", func(t *testing.T) { doTest(t, false) })
			t.Run("Package", func(t *testing.T) { doTest(t, true) })
		})
	}
}

func BenchmarkExtractor_ApplyPatches(b *testing.B) {
	tests := []struct {
		Name     string
		ZIP      string
		Module   *ves.GoModule
		Manifest *ves.GoModuleManifest
		FS       fs.FS
		Patches  []string
		Error    string
	}{
		{
			Name: "valid-complex",
			ZIP:  filepath.FromSlash("testdata/zips/golang.org/x/arch/@v/v0.13.0.zip"),
			Module: &ves.GoModule{
				Name:    "golang.org/x/arch",
				Version: ves.S("v0.13.0"),
				Packages: []*ves.GoPackage{
					{Name: ves.S("golang.org/x/arch/x86/x86asm")},
				},
			},
			Manifest: &ves.GoModuleManifest{
				Name:    "golang.org/x/arch",
				Version: ves.S("v0.13.0"),
			},
			FS:      os.DirFS("testdata"),
			Patches: []string{"patching/golang.org_x_arch.patch"},
		},
	}

	for _, test := range tests {
		doTest := func(b *testing.B, useBinary bool) {
			for b.Loop() {
				b.StopTimer()
				x := new(extractor)
				var gotTarget, path string
				if test.ZIP != "" {
					gotTarget = filepath.Join(b.ArtifactDir(), "got")
					dst := filepath.Join(gotTarget, filepath.FromSlash(test.Manifest.Name))
					err := os.RemoveAll(dst)
					if err != nil {
						b.Fatalf("ApplyPatches(): failed to delete %s: %v", dst, err)
					}

					err = os.MkdirAll(dst, 0o777)
					if err != nil {
						b.Fatalf("ApplyPatches(): failed to create %s: %v", dst, err)
					}

					err = x.ExtractAndPrune(gotTarget, test.ZIP, test.Module, test.Manifest)
					if err != nil {
						b.Fatalf("ApplyPatches(): failed to extract %s: %v", test.Manifest.Name, err)
					}

					path = dst
				}

				b.StartTimer()

				err := x.ApplyPatches(test.FS, path, test.Patches, useBinary)
				if test.Error != "" {
					if err == nil {
						b.Fatalf("ApplyPatches(): unexpected lack of error")
					}

					e := err.Error()
					if e != test.Error {
						b.Fatalf("ApplyPatches(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
					}

					// All good.
					return
				}

				if err != nil {
					b.Fatalf("ApplyPatches(): got unexpected error: %v", err)
				}
			}
		}

		b.Run(test.Name, func(b *testing.B) {
			b.Run("Binary", func(b *testing.B) { doTest(b, true) })
			b.Run("Package", func(b *testing.B) { doTest(b, false) })
		})
	}
}
