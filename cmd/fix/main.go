// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package fix resolves errors in the dependency set.
package fix

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ProjectSerenity/vdm/internal/ves"
)

var program = filepath.Base(os.Args[0])

func Main(ctx context.Context, w io.Writer, args []string) error {
	flags := flag.NewFlagSet("fix", flag.ExitOnError)

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

	if flags.NArg() != 0 {
		flags.Usage()
	}

	fsys := os.DirFS(".")
	deps, err := ves.ReadDeps(fsys, ves.DepsVDM)
	if err != nil {
		return err
	}

	deps.Sort()

	changed, err := Fix(w, fsys, deps)
	if err != nil {
		return err
	}

	if changed {
		deps.Sort()
		err = os.WriteFile(ves.DepsVDM, deps.Encode(), 0o644)
		if err != nil {
			return err
		}
	}

	return nil
}
