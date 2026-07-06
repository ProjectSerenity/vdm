// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Command update helps identify and perform updates to Firefly's dependencies.
package update

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/ProjectSerenity/vdm/internal/vdm"
	"github.com/ProjectSerenity/vdm/internal/vendeps"
)

var program = filepath.Base(os.Args[0])

func Main(ctx context.Context, w io.Writer, args []string) error {
	flags := flag.NewFlagSet("update", flag.ExitOnError)

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

	args = flags.Args()
	if len(args) != 0 {
		log.Printf("Unexpected arguments: %s\n", strings.Join(args, " "))
		flags.Usage()
	}

	return UpdateDependencies(ctx, w, vdm.DepsVDM)
}

// UpdateDependencies parses the given set of
// dependencies and checks each for an update,
// updating the document if possible.
//
// Note that UpdateDependencies does not modify
// the set of vendored dependencies, only the
// dependency specification.
func UpdateDependencies(ctx context.Context, w io.Writer, name string) error {
	// We parse the dependency configuration,
	// storing a set of modules, containing the
	// module name and a pointer to the version.
	//
	// We then iterate through these, checking
	// for updates, and writing out any changes
	// made.

	data, err := os.ReadFile(name)
	if err != nil {
		return err
	}

	deps, err := vdm.ParseDeps(vdm.DepsVDM, string(data))
	if err != nil {
		return err
	}

	modules := make([]*vendeps.UpdateDep, len(deps.GoModules))
	for i, mod := range deps.GoModules {
		modules[i] = &vendeps.UpdateDep{
			Name:    mod.Name,
			Version: &mod.Version.Value,
		}
	}

	anyUpdated := false
	for _, mod := range modules {
		updated, err := vendeps.UpdateGoModule(ctx, mod)
		if err != nil {
			return err
		}

		anyUpdated = anyUpdated || updated
	}

	if !anyUpdated {
		fmt.Fprintln(w, "No dependencies had updates available.")
		return nil
	}

	// We've updated the dependency set
	// so we format it and write it
	// back out.
	data = deps.Encode()
	err = os.WriteFile(name, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write updates back to %s: %v", name, err)
	}

	return nil
}
