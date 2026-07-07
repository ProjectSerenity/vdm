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

	"github.com/ProjectSerenity/vdm/internal/gomodproxy"
	"github.com/ProjectSerenity/vdm/internal/vdm"

	"golang.org/x/mod/semver"
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

	deps, err := vdm.ReadDeps(os.DirFS("."), vdm.DepsVDM)
	if err != nil {
		return err
	}

	anyUpdated := false
	for _, mod := range deps.GoModules {
		updated, err := UpdateGoModule(ctx, w, mod.Name, &mod.Version.Value)
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
	data := deps.Encode()
	err = os.WriteFile(name, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write updates back to %s: %v", name, err)
	}

	return nil
}

// UpdateGoModule checks a Go module for updates,
// using the proxy.golang.org Go module proxy API.
func UpdateGoModule(ctx context.Context, w io.Writer, name string, version *string) (updated bool, err error) {
	latest, err := gomodproxy.Latest(ctx, name)
	if err != nil {
		return false, err
	}

	switch semver.Compare(*version, latest) {
	case 0:
		// Current is latest.
		return false, nil
	case -1:
		// There is a newer version.
		fmt.Fprintf(w, "Updated Go module %s from %s to %s.\n", name, *version, latest)
		*version = latest
		return true, nil
	default:
		fmt.Fprintf(w, "WARN: Go module %s has version %s, but latest is %s, which is older.\n", name, *version, latest)
		return false, nil
	}
}
