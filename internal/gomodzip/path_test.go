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

func TestModuleCacheFromEnv(t *testing.T) {
	tests := []struct {
		Name       string
		GOMODCACHE string
		GOPATH     string
		Want       *ModuleCache
		Error      string
	}{
		{
			Name:       "GOMODCACHE",
			GOMODCACHE: filepath.FromSlash("/home/user/go/pkg/mod"),
			Want:       &ModuleCache{base: "/home/user/go/pkg/mod"},
		},
		{
			Name:   "GOPATH",
			GOPATH: filepath.FromSlash("/home/user/go"),
			Want:   &ModuleCache{base: "/home/user/go/pkg/mod"},
		},
		{
			Name: "fallback",
			Want: &ModuleCache{base: os.TempDir()},
		},
	}

	// Check the full version doesn't panic.
	ModuleCacheFromEnv()

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got := moduleCacheFromEnv(test.GOMODCACHE, test.GOPATH)
			if diff := cmp.Diff(test.Want, got, cmp.AllowUnexported(ModuleCache{})); diff != "" {
				t.Errorf("moduleCacheFromEnv(): mismatch (-want, +got)\n%s", diff)
			}
		})
	}
}

func TestModuleCache_Path(t *testing.T) {
	tests := []struct {
		Name     string
		Base     string
		Manifest *vdm.GoModuleManifest
		Want     string
		Error    string
	}{
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
		{
			Name: "valid-simple",
			Base: filepath.FromSlash("/home/user/go/pkg/mod"),
			Manifest: &vdm.GoModuleManifest{
				Name:    "rsc.io/diff",
				Version: vdm.ParsedString{Value: "v1.2.3"},
			},
			Want: filepath.FromSlash("/home/user/go/pkg/mod/cache/download/rsc.io/diff/@v/v1.2.3.zip"),
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			cache := ModuleCacheFromBase(test.Base)
			got, err := cache.Path(test.Manifest)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("ModuleCache.Path(): unexpected lack of error")
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("ModuleCache.Path(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("ModuleCache.Path(): got unexpected error: %v", err)
			}

			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Errorf("ModuleCache.Path(): module mismatch (-want, +got)\n%s", diff)
			}
		})
	}
}
