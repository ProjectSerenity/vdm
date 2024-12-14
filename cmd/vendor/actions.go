// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package vendor

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ProjectSerenity/vdm/internal/vendeps"
)

// GeneratePackageBUILD indicates that the named package
// should have its BUILD file generated and written to
// the given path.
type GenerateGoPackageBUILD struct {
	Package *vendeps.GoPackage
	Path    string
}

var _ vendeps.Action = GenerateGoPackageBUILD{}

// recordGoPackageFiles notes the list of Go source
// files in a Go package for use in the Bazel
// BUILD file later.
func (c GenerateGoPackageBUILD) recordGoPackageFiles() error {
	entries, err := os.ReadDir(filepath.Dir(c.Path))
	if err != nil {
		return fmt.Errorf("failed to read directory containing %s: %v", c.Package.Name, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if strings.HasSuffix(name, "_test.go") {
			c.Package.TestFiles = append(c.Package.TestFiles, name)
			continue
		}

		if strings.HasSuffix(name, ".go") || strings.HasSuffix(name, ".s") {
			c.Package.Files = append(c.Package.Files, name)
			continue
		}
	}

	return nil
}

func (c GenerateGoPackageBUILD) Do(fsys fs.FS) error {
	// Get the list of filenames.
	err := c.recordGoPackageFiles()
	if err != nil {
		return err
	}

	// Render the build files.
	pretty, err := RenderGoPackageBuildFile(c.Path, c.Package)
	if err != nil {
		return err
	}

	// golang.org/x/mod/zip.Unzip, which we use to
	// extract the module, creates files that are
	// read-only, so if the module already contains
	// a BUILD file, we must make it writable before
	// we overwrite it.
	info, err := fs.Stat(fsys, c.Path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("failed to stat %s: %v", c.Path, err)
	}

	if info != nil && info.Mode().Perm()&0200 == 0 {
		err = os.Chmod(c.Path, 0644)
		if err != nil {
			return fmt.Errorf("failed to make %s writable: %v", c.Path, err)
		}
	}

	if errors.Is(err, fs.ErrNotExist) {
		parent := filepath.Dir(c.Path)
		err = os.MkdirAll(parent, 0755)
		if err != nil {
			return fmt.Errorf("failed to make %s to write %s: %v", parent, c.Path, err)
		}
	}

	err = os.WriteFile(c.Path, pretty, 0644)
	if err != nil {
		return fmt.Errorf("failed to write build file to %s: %v", c.Path, err)
	}

	return nil
}

func (c GenerateGoPackageBUILD) String() string {
	return fmt.Sprintf("generate BUILD file for Go package %s to %s", c.Package.Name, c.Path)
}

// GenerateTextFilesBUILD indicates that the named directory
// should have its BUILD file generated and written to
// the given path.
type GenerateTextFilesBUILD struct {
	Files *vendeps.TextFiles
	Path  string
}

var _ vendeps.Action = GenerateTextFilesBUILD{}

func (c GenerateTextFilesBUILD) Do(fsys fs.FS) error {
	// Render the build files.
	pretty, err := RenderTextFilesBuildFile(c.Path, c.Files)
	if err != nil {
		return err
	}

	err = os.WriteFile(c.Path, pretty, 0644)
	if err != nil {
		return fmt.Errorf("failed to write build file to %s: %v", c.Path, err)
	}

	return nil
}

func (c GenerateTextFilesBUILD) String() string {
	return fmt.Sprintf("generate BUILD file for text files %s to %s", c.Files.Name, c.Path)
}

// BuildCacheManifest indicates that the cache subsystem
// should scan the vendor filesystem, producing the
// information necessary to avoid unnecessary future work,
// writing it to the given path.
type BuildCacheManifest struct {
	Deps *vendeps.Deps
	Path string
}

var _ vendeps.Action = BuildCacheManifest{}

func (c BuildCacheManifest) Do(fsys fs.FS) error {
	manifest, err := vendeps.GenerateCacheManifest(fsys, c.Deps)
	if err != nil {
		return fmt.Errorf("failed to build cache manifest: %v", err)
	}

	pretty, err := RenderManifest(c.Path, manifest)
	if err != nil {
		return err
	}

	err = os.WriteFile(c.Path, pretty, 0644)
	if err != nil {
		return fmt.Errorf("failed to write cache manifest to %s: %v", c.Path, err)
	}

	return nil
}

func (c BuildCacheManifest) String() string {
	return fmt.Sprintf("generate cache manifest to %s", c.Path)
}
