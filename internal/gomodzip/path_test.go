// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package gomodzip

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ProjectSerenity/vdm/internal/vdm"

	"github.com/google/go-cmp/cmp"
)

func TestModulePath(t *testing.T) {
	tests := []struct {
		Name       string
		GOMODCACHE string
		GOPATH     string
		Manifest   *vdm.GoModuleManifest
		Want       string
		Error      string
	}{
		{
			Name:       "GOMODCACHE",
			GOMODCACHE: filepath.FromSlash("/home/user/go/pkg/mod"),
			Manifest: &vdm.GoModuleManifest{
				Name:    "rsc.io/diff",
				Version: vdm.ParsedString{Value: "v1.2.3"},
			},
			Want: filepath.FromSlash("/home/user/go/pkg/mod/cache/download/rsc.io/diff/@v/v1.2.3.zip"),
		},
		{
			Name:   "GOPATH",
			GOPATH: filepath.FromSlash("/home/user/go"),
			Manifest: &vdm.GoModuleManifest{
				Name:    "rsc.io/diff",
				Version: vdm.ParsedString{Value: "v1.2.3"},
			},
			Want: filepath.FromSlash("/home/user/go/pkg/mod/cache/download/rsc.io/diff/@v/v1.2.3.zip"),
		},
		{
			Name: "fallback",
			Manifest: &vdm.GoModuleManifest{
				Name:    "rsc.io/diff",
				Version: vdm.ParsedString{Value: "v1.2.3"},
			},
			Want: filepath.Join(os.TempDir(), filepath.FromSlash("cache/download/rsc.io/diff/@v/v1.2.3.zip")),
		},
		{
			Name: "bad-module-name",
			Manifest: &vdm.GoModuleManifest{
				Name:    "-.",
				Version: vdm.ParsedString{Value: "v1.2.3"},
			},
			Error: `malformed module path "-.": leading dash`,
		},
		{
			Name: "bad-module-version",
			Manifest: &vdm.GoModuleManifest{
				Name:    "rsc.io/diff",
				Version: vdm.ParsedString{Value: "v1!"},
			},
			Error: `version "v1!" invalid: disallowed version string`,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got, err := modulePath(test.GOMODCACHE, test.GOPATH, test.Manifest)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("modulePath(): unexpected lack of error")
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("modulePath(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("modulePath(): got unexpected error: %v", err)
			}

			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Errorf("modulePath(): module mismatch (-want, +got)\n%s", diff)
			}

			// Check Path doesn't panic.
			Path(test.Manifest)
		})
	}
}
