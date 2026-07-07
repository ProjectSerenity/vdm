// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Command stats prints numerical data about the dependency graph.
package stats

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"iter"
	"log"
	"os"
	"path"
	"path/filepath"
	"text/tabwriter"

	"github.com/ProjectSerenity/vdm/internal/vdm"
)

var program = filepath.Base(os.Args[0])

func Main(ctx context.Context, w io.Writer, args []string) error {
	flags := flag.NewFlagSet("stats", flag.ExitOnError)

	var help bool
	flags.BoolVar(&help, "h", false, "Show this message and exit.")

	flags.Usage = func() {
		log.Printf("Usage:\n  %s %s OPTIONS\n\n", program, flags.Name())
		flags.PrintDefaults()
		os.Exit(2)
	}

	err := flags.Parse(args)
	if err != nil || help {
		flags.Usage()
	}

	fsys := os.DirFS(".")
	stats, err := DependenciesStats(fsys)
	if err != nil {
		return err
	}

	tw := tabwriter.NewWriter(w, 0, 8, 1, ' ', 0)
	fmt.Fprintf(tw, "Go modules:\t%d\n", stats.GoModules)
	fmt.Fprintf(tw, "Go modules (roots):\t%d\n", stats.GoRootModules)
	fmt.Fprintf(tw, "Go moduels (sub):\t%d\n", stats.GoSubModules)
	fmt.Fprintf(tw, "Go packages:\t%d\n", stats.GoPackages)
	fmt.Fprintf(tw, "Go packages (tests):\t%d\n", stats.GoTestedPackages)

	return tw.Flush()
}

// DependencyStats contains statistical data
// describing a dependency graph.
type DependencyStats struct {
	GoModules        int // The total number of Go modules.
	GoRootModules    int // The number of top-level Go modules.
	GoSubModules     int // The number of Go modules that exist within another module.
	GoPackages       int // The number of Go packages across all modules.
	GoTestedPackages int // The number of Go packages with tests.
}

// DependenciesStats assesses the dependency set
// to produce statistics.
func DependenciesStats(fsys fs.FS) (*DependencyStats, error) {
	deps, err := vdm.ReadDeps(fsys, vdm.DepsVDM)
	if err != nil {
		return nil, err
	}

	var stats DependencyStats
	if len(deps.GoModules) == 0 {
		return &stats, nil
	}

	// Find all modules so we can determine which
	// are submodules.
	modules := make(map[string]bool, len(deps.GoModules))
	for _, mod := range deps.GoModules {
		stats.GoModules++
		modules[mod.Name] = true
		for _, pkg := range mod.Packages {
			stats.GoPackages++
			if !pkg.NoTests.Value {
				stats.GoTestedPackages++
			}
		}
	}

	// Determine which modules are submodules and
	// which are root modules.
	for _, mod := range deps.GoModules {
		root := true
		for parent := range parentModules(mod.Name) {
			if modules[parent] {
				root = false
				break
			}
		}

		if root {
			stats.GoRootModules++
		} else {
			stats.GoSubModules++
		}
	}

	return &stats, nil
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
