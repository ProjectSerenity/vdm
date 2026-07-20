// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package fix

import (
	"fmt"
	"go/build"
	"io"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/ProjectSerenity/vdm/internal/depset"
	"github.com/ProjectSerenity/vdm/internal/ves"
)

// isGoStdlibPackage returns whether the given import string
// references a package in the Go standard library.
func isGoStdlibPackage(s string) bool {
	first, _, _ := strings.Cut(s, "/")
	return !strings.Contains(first, ".")
}

func buildGoPackage(set *depset.GoPackages, pkg *ves.GoPackage, dir string, contexts []*build.Context) error {
	for _, ctx := range contexts {
		parsed, err := ctx.ImportDir(dir, 0)
		if err != nil {
			return fmt.Errorf("failed to parse %s for %s: %v", dir, ctx.GOARCH, err)
		}

		for _, dep := range parsed.Imports {
			if !isGoStdlibPackage(dep) {
				set.UseMain(dep)
			}
		}

		if pkg.NoTests.Value {
			// Ignore test dependencies if we've disabled tests.
			continue
		}

		for _, dep := range parsed.TestImports {
			if !isGoStdlibPackage(dep) {
				set.UseTest(dep)
			}
		}

		for _, dep := range parsed.XTestImports {
			if !isGoStdlibPackage(dep) && dep != pkg.Name.Value {
				set.UseTest(dep)
			}
		}
	}

	return nil
}

func makeBuildContext(fsys fs.FS, goarch, goos string) *build.Context {
	return &build.Context{
		GOARCH:      goarch,
		GOOS:        goos,
		CgoEnabled:  false,
		Compiler:    "gc",
		ToolTags:    slices.Clone(build.Default.ToolTags),
		ReleaseTags: slices.Clone(build.Default.ReleaseTags),
		JoinPath:    path.Join, // Always use Linux-compatible paths, irrespective of this OS.
		SplitPathList: func(list string) []string {
			if list == "" {
				return []string{}
			}

			return strings.Split(list, ":")
		},
		IsAbsPath: path.IsAbs,
		IsDir: func(path string) bool {
			info, _ := fs.Stat(fsys, path)
			if info != nil && info.IsDir() {
				return true
			}

			return false
		},
		ReadDir: func(dir string) ([]fs.FileInfo, error) {
			entries, err := fs.ReadDir(fsys, dir)
			if err != nil {
				return nil, err
			}

			infos := make([]fs.FileInfo, len(entries))
			for i, entry := range entries {
				info, err := entry.Info()
				if err != nil {
					return nil, err
				}

				infos[i] = info
			}

			return infos, nil
		},
		OpenFile: func(path string) (io.ReadCloser, error) {
			return fsys.Open(path)
		},
	}
}

// FindUnusedGoDependencies scans a Go package for dependencies
// that are not used in practice.
func FindUnusedGoDependencies(w io.Writer, fsys fs.FS, deps *ves.Deps) (changed bool, err error) {
	var set depset.GoPackages
	contexts := []*build.Context{
		makeBuildContext(fsys, "amd64", "linux"),
		makeBuildContext(fsys, "arm64", "linux"),
	}

	for _, mod := range deps.GoModules {
		for _, pkg := range mod.Packages {
			if pkg.BuildFile.Value != "" {
				// All bets are off, leave it as-is.
				continue
			}

			set.Reset()
			for _, dep := range pkg.Deps {
				set.ExpectMain(dep.Value)
			}

			if pkg.NoTests.Value {
				if len(pkg.TestDeps) > 0 {
					changed = true
					pkg.TestDeps = pkg.TestDeps[:0]
				}
			} else {
				for _, dep := range pkg.TestDeps {
					set.ExpectTest(dep.Value)
				}
			}

			// Read each file.
			dir := path.Join(ves.Vendor, pkg.Name.Value)
			err := buildGoPackage(&set, pkg, dir, contexts)
			if err != nil {
				return false, fmt.Errorf("failed to scan Go package %s for dependencies: %v", pkg.Name.Value, err)
			}

			set.Sort()

			prefix := path.Join(ves.Vendor, pkg.Name.Value)
			for dep := range set.Duplicated() {
				// Keep it in the main deps and remove from
				// the test deps.
				changed = true
				pkg.TestDeps = slices.DeleteFunc(pkg.TestDeps, func(s ves.ParsedString) bool { return s.Value == dep })
			}

			for dep := range set.UnusedMain() {
				// Remove unused dependency.
				changed = true
				pkg.Deps = slices.DeleteFunc(pkg.Deps, func(s ves.ParsedString) bool { return s.Value == dep })
			}

			for dep := range set.UnusedTest() {
				// Remove unused dependency.
				changed = true
				pkg.TestDeps = slices.DeleteFunc(pkg.TestDeps, func(s ves.ParsedString) bool { return s.Value == dep })
			}

			for dep := range set.UnexpectedMain() {
				fmt.Fprintf(w, "%s: dependency %s seen unexpectedly.\n", prefix, dep)
			}

			for dep := range set.UnexpectedTest() {
				fmt.Fprintf(w, "%s: test dependency %s seen unexpectedly.\n", prefix, dep)
			}

			for dep := range set.BecameMain() {
				// Move to main dependencies only.
				changed = true
				pkg.Deps = slices.DeleteFunc(pkg.Deps, func(s ves.ParsedString) bool { return s.Value == dep })
				pkg.TestDeps = append(pkg.TestDeps, ves.S(dep))
			}

			for dep := range set.BecameTest() {
				// Remove from main deps and add to
				// test deps if it's not already
				// there.
				changed = true
				pkg.Deps = slices.DeleteFunc(pkg.Deps, func(s ves.ParsedString) bool { return s.Value == dep })
				if !slices.ContainsFunc(pkg.TestDeps, func(s ves.ParsedString) bool { return s.Value == dep }) {
					pkg.TestDeps = append(pkg.TestDeps, ves.S(dep))
				}
			}
		}
	}

	return changed, nil
}
