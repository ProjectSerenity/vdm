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

// Path returns the path in the [module cache] where
// the module zip file for the given module is stored.
//
// [module cache]: https://go.dev/ref/mod#module-cache
func Path(manifest *vdm.GoModuleManifest) (string, error) {
	return modulePath(os.Getenv("GOMODCACHE"), os.Getenv("GOPATH"), manifest)
}

// modulePath returns the path in the [module cache]
// where the module zip file for the given module is
// stored.
//
// The arguments should be the values for $GOMODCACHE
// and $GOPATH, respectively.
//
// [module cache]: https://go.dev/ref/mod#module-cache
func modulePath(gomodcache, gopath string, manifest *vdm.GoModuleManifest) (string, error) {
	var base string
	switch {
	case gomodcache != "":
		// $GOMODCACHE overrides $GOPATH if present.
		base = gomodcache
	case gopath != "":
		// $GOPATH/pkg/mod is otherwise the default.
		base = filepath.Join(gopath, "pkg", "mod")
	default:
		// Fall back to a temporary directory.
		base = os.TempDir()
	}

	modName, err := module.EscapePath(manifest.Name)
	if err != nil {
		return "", err
	}

	modVersion, err := module.EscapeVersion(manifest.Version.Value)
	if err != nil {
		return "", err
	}

	full := filepath.Join(base, "cache", "download", filepath.FromSlash(modName), "@v", modVersion+".zip")

	return full, nil
}
