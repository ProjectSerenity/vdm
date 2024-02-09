// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Command vendor uses package vendeps to vendor external dependencies into the repository.
package vendor

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ProjectSerenity/vdm/internal/vendeps"
)

var program = filepath.Base(os.Args[0])

func Main(ctx context.Context, w io.Writer, args []string) error {
	flags := flag.NewFlagSet("vendor", flag.ExitOnError)

	var help, noCache, dryRun, bzlmod bool
	flags.BoolVar(&help, "h", false, "Show this message and exit.")
	flags.BoolVar(&bzlmod, "bzlmod", true, "Use module names as used with bzlmod (eg rules_go, rather than io_bazel_rules_go)")
	flags.BoolVar(&noCache, "no-cache", false, "Ignore any locally cached dependency data.")
	flags.BoolVar(&dryRun, "dry-run", false, "Print the set of actions that would be performed, without performing them.")
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
	actions, err := Vendor(fsys, bzlmod)
	if err != nil {
		return fmt.Errorf("failed to load dependency manifest: %v", err)
	}

	if !noCache {
		actions = vendeps.StripCachedActions(fsys, actions)
	}

	// Perform/print the actions.
	for _, action := range actions {
		if dryRun {
			fmt.Println(action)
		} else {
			err = action.Do(fsys)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
