// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Command convert produces a deps.vdm equivalent to deps.bzl.
package convert

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ProjectSerenity/vdm/internal/starlark"
	"github.com/ProjectSerenity/vdm/internal/vdm"
	"github.com/ProjectSerenity/vdm/internal/vendeps"
)

var program = filepath.Base(os.Args[0])

func Main(ctx context.Context, w io.Writer, args []string) error {
	flags := flag.NewFlagSet("convert", flag.ExitOnError)

	var help bool
	flags.BoolVar(&help, "h", false, "Show this message and exit.")
	flags.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage\n  %s %s [OPTIONS]\n\n", program, flags.Name())
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()

		os.Exit(2)
	}

	err := flags.Parse(args)
	if err != nil || help {
		flags.Usage()
	}

	// Start by parsing the dependency manifest.
	fsys := os.DirFS(".")
	data, err := fs.ReadFile(fsys, vendeps.DepsBzl)
	if err != nil {
		return fmt.Errorf("failed to read %s: %v", vendeps.DepsBzl, err)
	}

	var deps vendeps.Deps
	err = starlark.Unmarshal(vendeps.DepsBzl, data, &deps)
	if err != nil {
		return err
	}

	out := Convert(&deps)

	err = os.WriteFile(vdm.DepsVDM, out.Encode(), 0o644)
	if err != nil {
		return fmt.Errorf("failed to write VDM data: %v", err)
	}

	return nil
}

// Convert copies data from the Vendeps format to the VDM format.
func Convert(deps *vendeps.Deps) *vdm.Deps {
	out := &vdm.Deps{
		GoModules: make([]*vdm.GoModule, len(deps.Go)),
	}

	for i, mod := range deps.Go {
		pkgs := make([]*vdm.GoPackage, len(mod.Packages))
		for j, pkg := range mod.Packages {
			pkgs[j] = &vdm.GoPackage{
				Name:          cloneParsedString(pkg.Name),
				BuildFile:     cloneParsedString(pkg.BuildFile),
				Deps:          cloneParsedStrings(pkg.Deps),
				Embed:         cloneParsedStrings(pkg.Embed),
				EmbedGlobs:    cloneParsedStrings(pkg.EmbedGlobs),
				Binary:        cloneParsedBool(pkg.Binary),
				BinaryDeps:    cloneParsedStrings(pkg.BinaryDeps),
				NoTests:       cloneParsedBoolComment(pkg.NoTests, pkg.NoTestsComment),
				TestSize:      cloneParsedString(pkg.TestSize),
				TestData:      cloneParsedStrings(pkg.TestData),
				TestDataGlobs: cloneParsedStrings(pkg.TestDataGlobs),
				TestDeps:      cloneParsedStrings(pkg.TestDeps),
				TestEnv:       cloneParsedStringsMap(pkg.TestEnv),
			}
		}

		out.GoModules[i] = &vdm.GoModule{
			Name:     mod.Name,
			Version:  cloneParsedString(mod.Version),
			Patches:  cloneParsedStrings(mod.Patches),
			Packages: pkgs,
		}
	}

	return out
}

func cloneParsedBool(b bool) vdm.ParsedBool {
	if b {
		return vdm.ParsedBool{Value: true, Pos: vdm.Pos{File: vendeps.DepsBzl}}
	}

	return vdm.ParsedBool{Value: false}
}

func cloneParsedBoolComment(b bool, comment string) vdm.ParsedBool {
	if b {
		return vdm.ParsedBool{Value: true, Pos: vdm.Pos{File: vendeps.DepsBzl}, Comment: comment}
	}

	return vdm.ParsedBool{Value: false, Comment: comment}
}

func cloneParsedString(s string) vdm.ParsedString {
	return vdm.ParsedString{Value: s}
}

func cloneParsedStrings(ss []string) []vdm.ParsedString {
	if ss == nil {
		return nil
	}

	out := make([]vdm.ParsedString, len(ss))
	for i, s := range ss {
		out[i].Value = s
	}

	return out
}

func cloneParsedStringsMap(mapping map[string]string) map[string]vdm.ParsedString {
	if mapping == nil {
		return nil
	}

	out := make(map[string]vdm.ParsedString, len(mapping))
	for k, v := range mapping {
		out[k] = vdm.ParsedString{Value: v}
	}

	return out
}
