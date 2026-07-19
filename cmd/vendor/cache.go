// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package vendor

import (
	"io/fs"
	"iter"
	"path"
	"strings"

	"github.com/ProjectSerenity/vdm/internal/digest"
	"github.com/ProjectSerenity/vdm/internal/vdm"
)

// StripCachedActions processes the action sequence,
// removing any actions that the cache can prove are
// unnecessary, returning the resulting action
// sequence.
//
// If no actions can be cached, or if there is no
// cache, the unmodified action sequence is returned.
func StripCachedActions(fsys fs.FS, actions []Action) []Action {
	// Start by loading the cache manifest. If we
	// fail to do so, we just return the unmodified
	// action sequence.
	cachedManifests, err := vdm.ReadManifests(fsys, path.Join(vdm.Vendor, vdm.ManifestsVDM))
	if err != nil {
		return actions
	}

	if len(cachedManifests.GoModules) == 0 {
		return actions
	}

	// Copy the manifest data into a map to make
	// lookups quicker.
	manifests := make(map[string]*vdm.GoModuleManifest)
	for _, manifest := range cachedManifests.GoModules {
		manifests[manifest.Name] = manifest
	}

	// Track which modules will be re-extracted
	// so we can detect cases where a module we
	// have cached will be deleted from the cache
	// when we re-extract a parent module.
	willExtract := make(map[string]bool)
	out := make([]Action, 0, len(actions))
	extract := func(dl DownloadGoModule) {
		out = append(out, dl)
		willExtract[dl.Module.Name] = true
	}

	// Search through the action sequence for crates
	// being downloaded and check whether the right
	// data is already present.
toNextAction:
	for _, action := range actions {
		switch dl := action.(type) {
		default:
			// For other actions, we leave them alone.
			out = append(out, action)
			continue
		case DownloadGoModule:
			// Check whether we have this module in the
			// cache. If the version in the cache isn't
			// the same, we ignore it.
			cached := manifests[dl.Module.Name]
			if cached == nil || dl.Module.Version.Value != cached.Version.Value {
				extract(dl)
				continue toNextAction
			}

			// We have already extracted the right
			// version, so now we check that the local
			// copy hasn't been modified. If the digest
			// in the filesystem doesn't matches, we
			// must ignore the cache contents.

			// First, build up the list of build files
			// to ignore; one for each package.
			ignore := make([]string, len(dl.Module.Packages))
			root := strings.TrimSuffix(dl.Path, dl.Module.Name)
			for i, pkg := range dl.Module.Packages {
				ignore[i] = path.Join(root, pkg.Name.Value, vdm.BuildBazel)
			}

			vendored, err := digest.Directory(fsys, dl.Path, ignore...)
			if err != nil || vendored != cached.Vendored.Value {
				extract(dl)
				continue toNextAction
			}

			// Next, we check that the set of packages
			// and directories produced by the current
			// configuration matches that in the cache.
			// Otherwise, we need to re-extract the module
			// to make sure we get the full contents for
			// every package/directory we keep.
			if dl.Manifest.Packages.Value != cached.Packages.Value {
				extract(dl)
				continue toNextAction
			}

			// We also invalidate the download cache
			// if our patches have changed, as we need
			// to apply the patches to the fresh data.
			if len(dl.Module.Patches) != 0 || cached.Patches.Value != "" {
				patches, err := digest.Files(fsys, cloneStrings(dl.Module.Patches))
				if err != nil || patches != cached.Patches.Value {
					extract(dl)
					continue toNextAction
				}
			}

			// Finally, we check whether any parent
			// modules will be extracted. If so,
			// then they will inadvertently delete
			// this module from the cache. As a
			// result, we will need to re-extract
			// this module too.
			for parent := range parentModules(dl.Module.Name) {
				if willExtract[parent] {
					extract(dl)
					continue toNextAction
				}
			}

			// We've got the right content, so we
			// drop this action. Before we do, we
			// update the stored manifest, so that
			// the cached manifests are persisted
			// when we rewrite the cache manifests.
			dl.Manifest.Download.Value = cached.Download.Value
			dl.Manifest.Vendored.Value = cached.Vendored.Value
			dl.Manifest.Patches.Value = cached.Patches.Value

			continue toNextAction
		}
	}

	return out
}

// parentModules returns an iterator over the parent modules
// of the given module.
func parentModules(mod string) iter.Seq[string] {
	return func(yield func(string) bool) {
		for parent := path.Dir(mod); parent != "."; parent = path.Dir(parent) {
			if !yield(parent) {
				return
			}
		}
	}
}

func cloneStrings(ss []vdm.ParsedString) []string {
	if ss == nil {
		return nil
	}

	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = s.Value
	}

	return out
}
