// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package vendor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ProjectSerenity/vdm/internal/digest"
	"github.com/ProjectSerenity/vdm/internal/gomodzip"
	"github.com/ProjectSerenity/vdm/internal/ves"
)

// Action represents a logical action that should be
// taken to progress the vendoring of a set of software
// dependencies.
//
// An action should contain any context necessary to
// perform its tasks.
type Action interface {
	// Do performs the action. Do should use the
	// given context and filesystem if possible.
	// The writer should be used for any log
	// messages.
	Do(ctx context.Context, fsys fs.FS, w io.Writer) error

	// Stringer is used to describe the action
	// without performing it.
	fmt.Stringer
}

// RemoveAll deletes a directory, along with any child
// nodes that exist. If the path does not exist, there
// is no effect.
type RemoveAll string

var _ Action = RemoveAll("")

func (r RemoveAll) Do(ctx context.Context, fsys fs.FS, w io.Writer) error {
	return os.RemoveAll(string(r))
}

func (r RemoveAll) String() string {
	return fmt.Sprintf("delete %s", string(r))
}

// DownloadModule indicates that the named module should
// be downloaded from the module proxy and extracted into
// the given path.
type DownloadGoModule struct {
	Cache       *gomodzip.ModuleCache
	Module      *ves.GoModule
	Manifest    *ves.GoModuleManifest
	ModuleProxy string
	Dir         string
	Path        string
}

var _ Action = DownloadGoModule{}

func (c DownloadGoModule) clearTarget() error {
	err := os.RemoveAll(c.Path)
	if err != nil {
		return err
	}

	err = os.MkdirAll(c.Path, 0o755)
	if err != nil {
		return err
	}

	return nil
}

func (c DownloadGoModule) Do(ctx context.Context, fsys fs.FS, w io.Writer) error {
	dl, err := c.Cache.Path(c.Manifest)
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(dl), 0o755)
	if err != nil {
		return err
	}

	err = gomodzip.DownloadDigest(dl, c.Manifest)
	if err != nil {
		return err
	}

	_, err = os.Stat(dl)
	if err != nil {
		fmt.Fprintf(w, "Downloading Go module %s.\n", c.Manifest.Name)
		err = gomodzip.Download(ctx, c.ModuleProxy, dl, c.Manifest)
		if err != nil {
			return err
		}
	}

	// Make room for the target.
	err = c.clearTarget()
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "Extracting Go module %s.\n", c.Manifest.Name)
	err = gomodzip.Extract(c.Dir, dl, c.Module, c.Manifest)
	if err != nil {
		return err
	}

	c.Manifest.Vendored.Value, err = digest.Directory(fsys, filepath.FromSlash(c.Path))
	if err != nil {
		return err
	}

	if len(c.Module.Patches) > 0 {
		patches := make([]string, len(c.Module.Patches))
		for i, patch := range c.Module.Patches {
			patches[i] = patch.Value
		}

		c.Manifest.Patches.Value, err = digest.Files(fsys, patches)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c DownloadGoModule) String() string {
	line := fmt.Sprintf("download module %s to %s", c.Module.Name, c.Path)
	if len(c.Module.Patches) > 0 {
		line += fmt.Sprintf(" with %d patches", len(c.Module.Patches))
	}

	return line
}

// CopyBUILD indicates that the named BUILD file
// should be copied to the given path.
type CopyBUILD struct {
	Source string
	Path   string
}

var _ Action = CopyBUILD{}

func (c CopyBUILD) Do(ctx context.Context, fsys fs.FS, w io.Writer) error {
	src, err := fsys.Open(c.Source)
	if err != nil {
		return fmt.Errorf("failed to open BUILD file %s: %v", c.Source, err)
	}

	// golang.org/x/mod/zip.Unzip, which we use to
	// extract Go modules, creates files that are
	// read-only, so if the module already contains
	// a BUILD file, we must make it writable before
	// we overwrite it.
	info, err := os.Stat(c.Path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		src.Close()
		return fmt.Errorf("failed to stat %s: %v", c.Path, err)
	}

	if info != nil && info.Mode().Perm()&0o200 == 0 {
		err = os.Chmod(c.Path, 0o644)
		if err != nil {
			src.Close()
			return fmt.Errorf("failed to make %s writable: %v", c.Path, err)
		}
	}

	dst, err := os.Create(c.Path)
	if err != nil {
		src.Close()
		return fmt.Errorf("failed to create BUILD file %s: %v", c.Path, err)
	}

	_, err = io.Copy(dst, src)
	if err != nil {
		src.Close()
		dst.Close()
		return fmt.Errorf("failed to copy BUILD file %s to %s: %v", c.Source, c.Path, err)
	}

	if err = src.Close(); err != nil {
		dst.Close()
		return fmt.Errorf("failed to close BUILD file %s: %v", c.Source, err)
	}

	if err = dst.Close(); err != nil {
		return fmt.Errorf("failed to close BUILD file %s: %v", c.Path, err)
	}

	return nil
}

func (c CopyBUILD) String() string {
	return fmt.Sprintf("copy BUILD file %s to %s", c.Source, c.Path)
}

// GeneratePackageBUILD indicates that the named package
// should have its BUILD file generated and written to
// the given path.
type GenerateGoPackageBUILD struct {
	Package   *ves.GoPackage
	Dir       string
	Files     []string
	TestFiles []string
	Path      string
}

var _ Action = (*GenerateGoPackageBUILD)(nil)

// recordGoPackageFiles notes the list of Go source
// files in a Go package for use in the Bazel
// BUILD file later.
func (c *GenerateGoPackageBUILD) recordGoPackageFiles(fsys fs.FS) error {
	entries, err := fs.ReadDir(fsys, c.Dir)
	if err != nil {
		return fmt.Errorf("failed to read directory containing %s: %v", c.Package.Name.Value, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if strings.HasSuffix(name, "_test.go") {
			c.TestFiles = append(c.TestFiles, name)
			continue
		}

		if strings.HasSuffix(name, ".go") || strings.HasSuffix(name, ".s") {
			c.Files = append(c.Files, name)
			continue
		}
	}

	return nil
}

func (c *GenerateGoPackageBUILD) Do(ctx context.Context, fsys fs.FS, w io.Writer) error {
	// Get the list of filenames.
	err := c.recordGoPackageFiles(fsys)
	if err != nil {
		return err
	}

	if len(c.TestFiles) == 0 {
		c.Package.NoTests.Value = true
	}

	// Render the build files.
	pretty, err := c.Render()
	if err != nil {
		return err
	}

	// golang.org/x/mod/zip.Unzip, which we use to
	// extract the module, creates files that are
	// read-only, so if the module already contains
	// a BUILD file, we must make it writable before
	// we overwrite it.
	info, err := os.Stat(c.Path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("failed to stat %s: %v", c.Path, err)
	}

	if info != nil && info.Mode().Perm()&0o200 == 0 {
		err = os.Chmod(c.Path, 0o644)
		if err != nil {
			return fmt.Errorf("failed to make %s writable: %v", c.Path, err)
		}
	}

	err = os.WriteFile(c.Path, pretty, 0o644)
	if err != nil {
		return fmt.Errorf("failed to write BUILD file to %s: %v", c.Path, err)
	}

	return nil
}

func (c *GenerateGoPackageBUILD) String() string {
	return fmt.Sprintf("generate BUILD file for Go package %s to %s", c.Package.Name.Value, c.Path)
}

// BuildCacheManifest indicates that the cache subsystem
// should scan the vendor filesystem, producing the
// information necessary to avoid unnecessary future work,
// writing it to the given path.
type BuildCacheManifest struct {
	Manifests *ves.Manifests
	Path      string
}

var _ Action = BuildCacheManifest{}

func (c BuildCacheManifest) Do(ctx context.Context, fsys fs.FS, w io.Writer) error {
	err := os.WriteFile(c.Path, c.Manifests.Encode(), 0644)
	if err != nil {
		return fmt.Errorf("failed to write cache manifest to %s: %v", c.Path, err)
	}

	return nil
}

func (c BuildCacheManifest) String() string {
	return fmt.Sprintf("generate cache manifest to %s", c.Path)
}
