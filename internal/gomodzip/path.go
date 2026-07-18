// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package gomodzip

import (
	"os"
	"path/filepath"

	"github.com/ProjectSerenity/vdm/internal/vdm"

	"golang.org/x/mod/module"
)

// ModuleCache is used to help identify the Go module
// cache, if present. It can be used to determine where
// to download modulze zips as efficiently as possible.
type ModuleCache struct {
	base string // The path to the module cache, if any.
}

// ModuleCacheFromEnv uses environment variables to
// determine the location of the module cache.
func ModuleCacheFromEnv() *ModuleCache {
	return moduleCacheFromEnv(os.Getenv("GOMODCACHE"), os.Getenv("GOPATH"))
}

func moduleCacheFromEnv(gomodcache, gopath string) *ModuleCache {
	if gomodcache != "" {
		return &ModuleCache{base: gomodcache}
	}

	if gopath != "" {
		return &ModuleCache{base: filepath.Join(gopath, "pkg", "mod")}
	}

	return &ModuleCache{base: os.TempDir()}
}

// ModuleCacheFromBase uses the specified base path
// as the root of the module cache.
//
// The base path must be syntactically valid for the
// current operating system.
func ModuleCacheFromBase(base string) *ModuleCache {
	return &ModuleCache{base: base}
}

// Path returns the path to the zip for the given
// module within the module cache.
//
// The path may not exist.
func (c *ModuleCache) Path(manifest *vdm.GoModuleManifest) (string, error) {
	modName, err := module.EscapePath(manifest.Name)
	if err != nil {
		return "", err
	}

	modVersion, err := module.EscapeVersion(manifest.Version.Value)
	if err != nil {
		return "", err
	}

	full := filepath.Join(c.base, "cache", "download", filepath.FromSlash(modName), "@v", modVersion+".zip")

	return full, nil
}
