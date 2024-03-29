// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package vendor

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/ProjectSerenity/vdm/internal/starlark"
	"github.com/ProjectSerenity/vdm/internal/vendeps"
)

// Vendor takes a filesystem, parses the set of software
// dependencies in deps.bzl, then produces the sequence of
// actions necessary to vendor those dependencies into the
// vendor directory.
//
// Note that Vendor does not perform any of these actions;
// it only reads data from fsys.
func Vendor(fsys fs.FS, bzlmod bool) (actions []vendeps.Action, err error) {
	data, err := fs.ReadFile(fsys, vendeps.DepsBzl)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %v", vendeps.DepsBzl, err)
	}

	var deps vendeps.Deps
	err = starlark.Unmarshal(vendeps.DepsBzl, data, &deps)
	if err != nil {
		return nil, err
	}

	if len(deps.Go) == 0 {
		actions = []vendeps.Action{vendeps.RemoveAll(vendeps.Vendor)}
		return actions, nil
	}

	// Check that the dependency graph is complete. Start
	// by making a mapping for modules to make them easier
	// to look up.
	packages := make(map[string]*vendeps.GoPackage)
	for _, module := range deps.Go {
		for _, pkg := range module.Packages {
			packages[pkg.Name] = pkg
		}
	}

	var missingDeps bytes.Buffer
	for _, module := range deps.Go {
		for _, pkg := range module.Packages {
			for _, dep := range pkg.Deps {
				if packages[dep] == nil {
					fmt.Fprintf(&missingDeps, "Go package %s depends on %s, which is not specified.\n", pkg.Name, dep)
				}
			}
		}
	}

	if missingDeps.Len() > 0 {
		return nil, fmt.Errorf("missing dependencies:\n%s", missingDeps.String())
	}

	// Start by checking whether the vendor folder exists.
	// If it does, we will need to check the cache later.
	info, err := fs.Stat(fsys, vendeps.Vendor)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("failed to stat %q: %v", vendeps.Vendor, err)
	}

	if info != nil && !info.IsDir() {
		return nil, fmt.Errorf("failed to vendor dependencies: %q exists and is not a directory", vendeps.Vendor)
	}

	// We proceed on the basis that the vendor directory
	// is dirty, so we start by removing any directories
	// that exist but wouldn't be created if we were to
	// start from scratch. These actions are never affected
	// by the cache.
	entries, err := fs.ReadDir(fsys, vendeps.Vendor)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("failed to read files in %q: %v", vendeps.Vendor, err)
	}

	for _, entry := range entries {
		name := entry.Name()
		full := path.Join(vendeps.Vendor, name)
		switch name {
		case vendeps.ManifestBzl:
			// Never remove the cache manifest.
		default:
			if !entry.IsDir() {
				// Remove loose files.
				actions = append(actions, vendeps.RemoveAll(full))
			}
		}
	}

	// Now, we iterate through the sets of dependencies,
	// assuming each dependency is dirty and should be
	// fully replaced. The caching layer may later strip
	// some of these actions out if it can prove that
	// they are unnecessary.
	if len(deps.Go) > 0 {
		actions, err = vendorGo(fsys, actions, deps.Go, bzlmod)
		if err != nil {
			return nil, err
		}
	}

	actions = append(actions, BuildCacheManifest{Deps: &deps, Path: path.Join(vendeps.Vendor, vendeps.ManifestBzl)})

	return actions, nil
}

func vendorGo(fsys fs.FS, actions []vendeps.Action, modules []*vendeps.GoModule, bzlmod bool) ([]vendeps.Action, error) {
	// Sanity-check each module and make
	// a mapping of module names to modules
	// to simplify looking up module paths.
	modulePaths := make(map[string]*vendeps.GoModule)
	for i, module := range modules {
		modulePaths[module.Name] = module
		if module.Name == "" {
			return nil, fmt.Errorf("Go module %d has no name", i)
		}

		if module.Version == "" {
			return nil, fmt.Errorf("Go module %s has no version", module.Name)
		}

		if len(module.Packages) == 0 {
			return nil, fmt.Errorf("Go module %s has no packages", module.Name)
		}

		for i, pkg := range module.Packages {
			if pkg.Name == "" {
				return nil, fmt.Errorf("Go module %s has package %d with no import path", module.Name, i)
			}
		}
	}

	// Delete any modules we no longer include.
	// Sadly, this is more involved a process than
	// with Rust crates, as each module may have a
	// multi-part path segment, such as golang.org/x/crypto.
	// This makes detecting unwanted directories
	// more complex.
	//
	// First, we collect the set of all file paths
	// under that segment of the file tree.
	filepaths := make(map[string]bool)
	err := fs.WalkDir(fsys, vendeps.Vendor, func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			// Don't delete folders containing a module
			// we're including, as we may want to retain
			// it as a cache.
			if modulePaths[strings.TrimPrefix(name, vendeps.Vendor+"/")] != nil {
				return fs.SkipDir
			}

			filepaths[name] = true
		}

		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("failed to walk %s: %v", vendeps.Vendor, err)
	}

	// Now, we eliminate any filepaths that are
	// a parent directory of, a module we'll be
	// creating.
	for _, module := range modules {
		modname := path.Join(vendeps.Vendor, module.Name)
		for filepath := range filepaths {
			if strings.HasPrefix(modname, filepath+"/") {
				delete(filepaths, filepath)
			}
		}
	}

	// Finally, we reduce the remaining set of
	// filepaths (which should all be deleted)
	// to as small a set as possible by iterating
	// through them, ignoring any whose parent
	// directories also exist in the map.
	sortedFilepaths := make([]string, 0, len(filepaths))
	for filepath := range filepaths {
		if !filepaths[path.Dir(filepath)] {
			sortedFilepaths = append(sortedFilepaths, filepath)
		}
	}

	sort.Strings(sortedFilepaths)

	for _, filepath := range sortedFilepaths {
		actions = append(actions, vendeps.RemoveAll(filepath))
	}

	// Now, we download each module, which will include
	// deleting any contents previously there. The
	// cache may strip out the download action if it
	// can prove that the right data is already there.
	for _, module := range modules {
		full := path.Join(vendeps.Vendor, module.Name)
		actions = append(actions, vendeps.DownloadGoModule{Module: module, Path: full})
		for _, pkg := range module.Packages {
			pkg.Bzlmod = bzlmod
			full = path.Join(vendeps.Vendor, pkg.Name)
			if pkg.BuildFile != "" {
				actions = append(actions, vendeps.CopyBUILD{Source: pkg.BuildFile, Path: path.Join(full, vendeps.BuildBazel)})
			} else {
				actions = append(actions, GenerateGoPackageBUILD{Package: pkg, Path: path.Join(full, vendeps.BuildBazel)})
			}
		}

		for _, files := range module.Directories {
			full = path.Join(vendeps.Vendor, files.Name)
			actions = append(actions, GenerateTextFilesBUILD{Files: files, Path: path.Join(full, vendeps.BuildBazel)})
		}
	}

	return actions, nil
}
