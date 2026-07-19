// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package vendor fetches the dependency set and stores it in the repository's vendor directory.
package vendor

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var program = filepath.Base(os.Args[0])

func Main(ctx context.Context, w io.Writer, args []string) error {
	flags := flag.NewFlagSet("vendor", flag.ExitOnError)

	var help, noCache, dryRun bool
	flags.BoolVar(&help, "h", false, "Show this message and exit.")
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
	actions, err := Vendor(fsys)
	if err != nil {
		return fmt.Errorf("failed to load dependency manifest: %v", err)
	}

	if !noCache {
		actions = StripCachedActions(fsys, actions)
	}

	// Perform/print the actions.
	for _, action := range actions {
		if dryRun {
			fmt.Println(action)
		} else {
			err = action.Do(ctx, fsys, w)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
