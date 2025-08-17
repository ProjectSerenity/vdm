// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Command vdm implements the Vendoring Dependency Manager.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/ProjectSerenity/vdm/cmd/check"
	"github.com/ProjectSerenity/vdm/cmd/json"
	"github.com/ProjectSerenity/vdm/cmd/update"
	"github.com/ProjectSerenity/vdm/cmd/vendor"
	"github.com/ProjectSerenity/vdm/internal/simplehttp"
)

func init() {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)
	log.SetPrefix("")

	simplehttp.UserAgent = "Vendoring-Dependency-Manager/1 (github.com/ProjectSerenity/vdm)"
}

type Command struct {
	Name        string
	Description string
	Func        func(ctx context.Context, w io.Writer, args []string) error
}

var (
	commandsNames = make([]string, 0, 10)
	commandsMap   = make(map[string]*Command)

	program = filepath.Base(os.Args[0])
)

func RegisterCommand(name, description string, fun func(ctx context.Context, w io.Writer, args []string) error) {
	if commandsMap[name] != nil {
		panic("command " + name + " already registered")
	}

	if fun == nil {
		panic("command " + name + " registered with nil implementation")
	}

	commandsNames = append(commandsNames, name)
	commandsMap[name] = &Command{Name: name, Description: description, Func: fun}
}

func init() {
	RegisterCommand("check", "Check managed dependencies for vulnerabilities and other issues.", check.Main)
	RegisterCommand("json", "Print the current dependency set in JSON format.", json.Main)
	RegisterCommand("update", "Modify the dependency manifest to apply any available minor or patch updates to external dependencies.", update.Main)
	RegisterCommand("vendor", "Vendor the managed dependencies into the vendor directory.", vendor.Main)
}

func main() {
	sort.Strings(commandsNames)

	var help bool
	flag.BoolVar(&help, "h", false, "Show this message and exit.")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage\n  %s COMMAND [OPTIONS]\n\n", program)
		fmt.Fprintf(os.Stderr, "Commands:\n")
		maxWidth := 0
		for _, name := range commandsNames {
			if maxWidth < len(name) {
				maxWidth = len(name)
			}
		}

		for _, name := range commandsNames {
			cmd := commandsMap[name]
			fmt.Fprintf(os.Stderr, "  %-*s  %s\n", maxWidth, name, cmd.Description)
		}

		os.Exit(2)
	}

	flag.Parse()

	args := flag.Args()
	if help {
		flag.Usage()
	}

	if len(args) == 0 {
		flag.Usage()
	}

	name := args[0]
	cmd, ok := commandsMap[args[0]]
	if !ok {
		flag.Usage()
	}

	// If we're being run with `bazel run`, we're in
	// a semi-random build directory, and need to move
	// to the workspace root directory.
	//
	workspace := os.Getenv("BUILD_WORKSPACE_DIRECTORY")
	if workspace != "" {
		err := os.Chdir(workspace)
		if err != nil {
			log.Printf("Failed to change directory to %q: %v", workspace, err)
		}
	}

	log.SetPrefix(name + ": ")
	err := cmd.Func(context.Background(), os.Stdout, args[1:])
	if err != nil {
		log.Fatal(err)
	}
}
