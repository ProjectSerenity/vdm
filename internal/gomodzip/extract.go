// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package gomodzip

import (
	"fmt"
	"io"
	"io/fs"
	"iter"
	"maps"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"

	"github.com/ProjectSerenity/vdm/internal/vdm"

	"golang.org/x/mod/module"
	"golang.org/x/mod/sumdb/dirhash"
	"golang.org/x/mod/zip"
	"rsc.io/tmp/patch"
)

// Extract reads the Go module ZIP from the given path, writing
// its contents to the target directory. Before copying the moudle,
// Extract checks its checksum against the manifest.
//
// Once the module has been extracted, any patches are applied.
func Extract(target, path string, mod *vdm.GoModule, manifest *vdm.GoModuleManifest) error {
	var x extractor
	err := x.VerifyChecksum(path, manifest)
	if err != nil {
		return err
	}

	err = x.ExtractAndPrune(target, path, mod, manifest)
	if err != nil {
		return err
	}

	const (
		useBinary  = true
		usePackage = false
	)

	patches := make([]string, len(mod.Patches))
	for i, patch := range mod.Patches {
		patches[i] = patch.Value
	}

	err = x.ApplyPatches(os.DirFS("."), filepath.Join(target, filepath.FromSlash(mod.Name)), patches, usePackage)
	if err != nil {
		return err
	}

	return nil
}

type extractor struct {
	SavedError error
}

func (x *extractor) SaveError(err error) {
	if x.SavedError == nil {
		x.SavedError = err
	}
}

func (x *extractor) CompareChecksum(got string, manifest *vdm.GoModuleManifest) error {
	formatted, err := extractChecksum(got)
	if err != nil {
		return fmt.Errorf("failed to verify Go module %s: %v", manifest.Name, err)
	}

	if formatted != manifest.Download.Value {
		return fmt.Errorf("failed to verify Go module %s: got checksum %s, want %s", manifest.Name, formatted, manifest.Download.Value)
	}

	return nil
}

func (x *extractor) VerifyChecksum(path string, manifest *vdm.GoModuleManifest) error {
	got, err := dirhash.HashZip(path, dirhash.Hash1)
	if err != nil {
		return fmt.Errorf("failed to verify Go module %s: %v", manifest.Name, err)
	}

	return x.CompareChecksum(got, manifest)
}

func (x *extractor) Extract(dst, path string, manifest *vdm.GoModuleManifest) error {
	id := module.Version{
		Path:    manifest.Name,
		Version: manifest.Version.Value,
	}

	err := zip.Unzip(dst, id, path)
	if err != nil {
		return fmt.Errorf("failed to unzip Go module %s: %v", manifest.Name, err)
	}

	return nil
}

// parentDirectories returns an iterator over the parent
// directories of the given path.
func parentDirectories(dir string) iter.Seq[string] {
	return func(yield func(string) bool) {
		for parent := path.Dir(dir); parent != "."; parent = path.Dir(parent) {
			if !yield(parent) {
				return
			}
		}
	}
}

// Identify the set of directories that should be deleted.
func (x *extractor) IdentifyDeletions(fsys fs.FS, mod *vdm.GoModule) ([]string, error) {
	// Identify the set of directories to keep,
	// based on the packages specified.
	keep := make(map[string]bool)
	testdata := make(map[string]bool)
	for _, pkg := range mod.Packages {
		keep[pkg.Name.Value] = true
		testdata[pkg.Name.Value+"/testdata"] = true
		for _, dir := range pkg.Directories {
			full := path.Join(pkg.Name.Value, dir.Value)
			keep[full] = true
		}
	}

	// Delete directories we haven't marked to
	// keep.
	remove := make(map[string]bool)
	err := fs.WalkDir(fsys, mod.Name, func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Never delete the root directory.
		if name == mod.Name {
			return nil
		}

		// Ignore files.
		if !d.IsDir() {
			// Drop build files we wouldn't use.
			if path.Base(name) == "BUILD.bazel" {
				remove[name] = true
			}

			return nil
		}

		// Ignore testdata directories and
		// arbitrarily deep contents.
		if testdata[name] {
			return fs.SkipDir
		}

		// Ignore directories we should keep.
		if keep[name] {
			return nil
		}

		remove[name] = true
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to identify unused Go packages to delete in %s: %v", mod.Name, err)
	}

	// Don't remove any directories that are a
	// parent directory of a package we use.
	//
	// We have to do this separately so that we
	// remove packages that are siblings of
	// packages we want to keep.
	for _, pkg := range mod.Packages {
		for parent := range parentDirectories(pkg.Name.Value) {
			if remove[parent] {
				delete(remove, parent)
			}
		}
	}

	// There's no point removing child directories
	// of other directories we will remove.
	for dir := range remove {
		for parent := range parentDirectories(dir) {
			if remove[parent] {
				delete(remove, dir)
				break
			}
		}
	}

	deletions := slices.Collect(maps.Keys(remove))
	slices.Sort(deletions)

	return deletions, nil
}

func (x *extractor) ExtractAndPrune(target, path string, mod *vdm.GoModule, manifest *vdm.GoModuleManifest) error {
	dst := filepath.Join(target, manifest.Name)
	err := x.Extract(dst, path, manifest)
	if err != nil {
		return err
	}

	deletions, err := x.IdentifyDeletions(os.DirFS(target), mod)
	x.SaveError(err)
	for _, dir := range deletions {
		full := filepath.Join(target, dir)
		x.SaveError(os.RemoveAll(full))
	}

	return x.SavedError
}

func (x *extractor) CloseClosers(closers []io.Closer, names []string, context string) {
	for i, closer := range closers {
		err := closer.Close()
		if err != nil {
			x.SaveError(fmt.Errorf("failed to close %s %s: %v", context, names[i], err))
		}
	}
}

func (x *extractor) PatchWithBinary(w io.Writer, fsys fs.FS, path string, patches []string) error {
	// The patch binary is happiest if we pass the set
	// of patch files to stdin. Since we may have many
	// patch files and we don't want to concatenate
	// them in memory, we open them all as files, use
	// an io.MultiReader to concatenate them on the
	// fly, then pass that to stdin.
	readers := make([]io.Reader, len(patches))
	closers := make([]io.Closer, len(patches))
	for i, patch := range patches {
		f, err := fsys.Open(patch)
		if err != nil {
			return fmt.Errorf("failed to open patch path %q: %v", patch, err)
		}

		readers[i] = f
		closers[i] = f
	}

	cmd := exec.Command("patch", "-p1")
	cmd.Dir = path
	cmd.Stdin = io.MultiReader(readers...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		w.Write(out)
		x.SaveError(fmt.Errorf("failed to run patch: %v", err))
	}

	x.CloseClosers(closers, patches, "patch file")

	return x.SavedError
}

func (x *extractor) SinglePatchWithPackage(path string, file *patch.File) error {
	full := func(name string) string {
		local := filepath.FromSlash(name)
		return filepath.Join(path, local)
	}

	switch file.Verb {
	case patch.Add:
		patched, err := file.Diff.Apply(nil)
		if err != nil {
			return fmt.Errorf("failed to add %q: %v", file.Dst, err)
		}

		err = os.WriteFile(full(file.Dst), patched, os.FileMode(file.NewMode))
		if err != nil {
			return fmt.Errorf("failed to add %q: %v", file.Dst, err)
		}
	case patch.Delete:
		err := os.Remove(full(file.Src))
		if err != nil {
			return fmt.Errorf("failed to delete %q: %v", file.Src, err)
		}
	case patch.Edit:
		old, err := os.ReadFile(full(file.Src))
		if err != nil {
			return fmt.Errorf("failed to edit %q: %v", file.Dst, err)
		}

		info, err := os.Stat(full(file.Src))
		if err != nil {
			return fmt.Errorf("failed to edit %q: %v", file.Src, err)
		}

		patched, err := file.Diff.Apply(old)
		if err != nil {
			return fmt.Errorf("failed to edit %q: %v", file.Dst, err)
		}

		mode := os.FileMode(file.NewMode)
		if mode == 0 {
			mode = 0o644
		}

		if mode != info.Mode() {
			err = os.Chmod(full(file.Dst), mode)
			if err != nil {
				return fmt.Errorf("failed to edit %q: %v", file.Dst, err)
			}
		}

		err = os.WriteFile(full(file.Dst), patched, mode)
		if err != nil {
			return fmt.Errorf("failed to edit %q: %v", file.Dst, err)
		}
	case patch.Rename:
		err := os.Rename(full(file.Src), full(file.Dst))
		if err != nil {
			return fmt.Errorf("failed to rename %q to %q: %v", file.Src, file.Dst, err)
		}
	default:
		return fmt.Errorf("unexpected patch operation %#v", file.Verb)
	}

	return nil
}

func (x *extractor) PatchWithPackage(fsys fs.FS, path string, patches []string) error {
	for _, name := range patches {
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("failed to open patch path %q: %v", name, err)
		}

		set, err := patch.Parse(data)
		if err != nil {
			return fmt.Errorf("failed to parse patch path %q: %v", name, err)
		}

		for _, file := range set.File {
			err = x.SinglePatchWithPackage(path, file)
			if err != nil {
				return fmt.Errorf("failed to apply patch path %q: %v", name, err)
			}
		}
	}

	return nil
}

func (x *extractor) ApplyPatches(fsys fs.FS, path string, patches []string, useBinary bool) error {
	if !useBinary {
		return x.PatchWithPackage(fsys, path, patches)
	} else {
		return x.PatchWithBinary(os.Stderr, fsys, path, patches)
	}
}
