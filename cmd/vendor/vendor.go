// Copyright 2026 The Firefly Authors.
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
	"slices"
	"strings"

	"github.com/ProjectSerenity/vdm/internal/gomodzip"
	"github.com/ProjectSerenity/vdm/internal/ves"
)

// Vendor takes a filesystem, parses the set of software
// dependencies in deps.vdm, then produces the sequence of
// actions necessary to save those dependencies into the
// vendor directory.
//
// Note that Vendor does not perform any of these actions;
// it only reads data from fsys.
func Vendor(fsys fs.FS) (actions []Action, err error) {
	deps, err := ves.ReadDeps(fsys, ves.DepsVDM)
	if err != nil {
		return nil, err
	}

	if len(deps.GoModules) == 0 {
		actions = []Action{RemoveAll(ves.Vendor)}
		return actions, nil
	}

	// Check that the dependency graph is complete. Start
	// by making a mapping for packages to make them easier
	// to look up.
	packages := make(map[string]*ves.GoPackage)
	for _, module := range deps.GoModules {
		for _, pkg := range module.Packages {
			packages[pkg.Name.Value] = pkg
		}
	}

	var missingDeps bytes.Buffer
	for _, module := range deps.GoModules {
		for _, pkg := range module.Packages {
			for _, dep := range pkg.Deps {
				if packages[dep.Value] == nil {
					fmt.Fprintf(&missingDeps, "Go package %s depends on %s, which is not specified.\n", pkg.Name.Value, dep.Value)
				}
			}
		}
	}

	if missingDeps.Len() > 0 {
		return nil, fmt.Errorf("missing dependencies:\n%s", missingDeps.String())
	}

	// Start by checking whether the vendor folder exists.
	// If it does, we will need to check the cache later.
	info, err := fs.Stat(fsys, ves.Vendor)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("failed to stat %q: %v", ves.Vendor, err)
	}

	if info != nil && !info.IsDir() {
		return nil, fmt.Errorf("failed to vendor dependencies: %q exists and is not a directory", ves.Vendor)
	}

	// We proceed on the basis that the vendor directory
	// is dirty, so we start by removing any directories
	// that exist but wouldn't be created if we were to
	// start from scratch. These actions are never affected
	// by the cache.
	entries, err := fs.ReadDir(fsys, ves.Vendor)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("failed to read files in %q: %v", ves.Vendor, err)
	}

	for _, entry := range entries {
		name := entry.Name()
		full := path.Join(ves.Vendor, name)
		switch name {
		case ves.ManifestsVDM:
			// Never remove the cache manifest.
		default:
			if !entry.IsDir() {
				// Remove loose files.
				actions = append(actions, RemoveAll(full))
			}
		}
	}

	// Now, we iterate through the sets of dependencies,
	// assuming each dependency is dirty and should be
	// fully replaced. The caching layer may later strip
	// some of these actions out if it can prove that
	// they are unnecessary.
	var manifests *ves.Manifests
	if len(deps.GoModules) > 0 {
		actions, manifests, err = vendorGo(fsys, actions, deps)
		if err != nil {
			return nil, err
		}
	}

	actions = append(actions, BuildCacheManifest{Manifests: manifests, Path: path.Join(ves.Vendor, ves.ManifestsVDM)})

	return actions, nil
}

func vendorGo(fsys fs.FS, actions []Action, deps *ves.Deps) ([]Action, *ves.Manifests, error) {
	manifests := &ves.Manifests{
		GoModules: make([]*ves.GoModuleManifest, len(deps.GoModules)),
	}

	for i, mod := range deps.GoModules {
		manifests.GoModules[i] = &ves.GoModuleManifest{
			Name:     mod.Name,
			Version:  mod.Version,
			Packages: ves.ParsedString{Value: mod.Directories()},
		}
	}

	// Sanity-check each module and make
	// a mapping of module names to modules
	// to simplify looking up module paths.
	modulePaths := make(map[string]*ves.GoModule)
	for i, module := range deps.GoModules {
		modulePaths[module.Name] = module
		if module.Name == "" {
			return nil, nil, fmt.Errorf("Go module %d has no name", i)
		}

		if module.Version.Value == "" {
			return nil, nil, fmt.Errorf("Go module %s has no version", module.Name)
		}

		if len(module.Packages) == 0 {
			return nil, nil, fmt.Errorf("Go module %s has no packages", module.Name)
		}

		for i, pkg := range module.Packages {
			if pkg.Name.Value == "" {
				return nil, nil, fmt.Errorf("Go module %s has package %d with no import path", module.Name, i)
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
	err := fs.WalkDir(fsys, ves.Vendor, func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			// Don't delete folders containing a module
			// we're including, as we may want to retain
			// it as a cache.
			if modulePaths[strings.TrimPrefix(name, ves.Vendor+"/")] != nil {
				return fs.SkipDir
			}

			filepaths[name] = true
		}

		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, nil, fmt.Errorf("failed to walk %s: %v", ves.Vendor, err)
	}

	// Now, we eliminate any filepaths that are
	// a parent directory of, a module we'll be
	// creating.
	for _, module := range deps.GoModules {
		modname := path.Join(ves.Vendor, module.Name)
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

	slices.Sort(sortedFilepaths)

	for _, filepath := range sortedFilepaths {
		actions = append(actions, RemoveAll(filepath))
	}

	// Now, we download each module, which will include
	// deleting any contents previously there. The
	// cache may strip out the download action if it
	// can prove that the right data is already there.
	cache := gomodzip.ModuleCacheFromEnv()
	for i, module := range deps.GoModules {
		full := path.Join(ves.Vendor, module.Name)
		actions = append(actions, DownloadGoModule{
			Cache:       cache,
			Module:      module,
			Manifest:    manifests.GoModules[i],
			ModuleProxy: gomodzip.ModuleProxy,
			Dir:         ves.Vendor,
			Path:        full,
		})

		for _, pkg := range module.Packages {
			full = path.Join(ves.Vendor, pkg.Name.Value)
			if pkg.BuildFile.Value != "" {
				actions = append(actions, CopyBUILD{Source: pkg.BuildFile.Value, Path: path.Join(full, ves.BuildBazel)})
			} else {
				actions = append(actions, &GenerateGoPackageBUILD{Package: pkg, Dir: full, Path: path.Join(full, ves.BuildBazel)})
			}
		}
	}

	return actions, manifests, nil
}
