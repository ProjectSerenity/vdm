// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Command json uses package vendeps to vendor external dependencies into the repository.
package json

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ProjectSerenity/vdm/internal/starlark"
	"github.com/ProjectSerenity/vdm/internal/vendeps"
)

var program = filepath.Base(os.Args[0])

func Main(ctx context.Context, w io.Writer, args []string) error {
	flags := flag.NewFlagSet("json", flag.ExitOnError)

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

	data, err = json.MarshalIndent(&deps, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to encode JSON data: %v", err)
	}

	_, err = w.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write JSON data: %v", err)
	}

	return nil
}
