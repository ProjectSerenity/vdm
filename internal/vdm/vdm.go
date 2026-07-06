// Package vdm provides functionality for recording a dependency set.
package vdm

import (
	"cmp"
	"slices"
	"strconv"
)

const (
	tabs   = "\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t"
	spaces = "                                "
)

// Pos stores a position in a file.
type Pos struct {
	File string // The filename.
	Line int    // The line number, starting at 1.
}

func (p Pos) String() string {
	file := p.File
	line := strconv.Itoa(p.Line)
	if file == "" {
		file = "???"
	}

	if p.Line == 0 {
		line = "?"
	}

	return file + ":" + line
}

// ParsedBool contains a bool, along with
// its location and any comment.
type ParsedBool struct {
	Value   bool
	Pos     Pos
	Comment string
}

// ParsedString contains a string, along with
// its location and any comment.
type ParsedString struct {
	Value   string
	Pos     Pos
	Comment string
}

// Deps describes a set of software dependencies.
type Deps struct {
	BannedGoPackages []ParsedString `json:"banned_go_packages,omitzero"`
	GoModules        []*GoModule    `json:"go_modules,omitzero"`
}

// GoModule contains the information necessary
// to vendor a Go module, specifying the set
// of packages within the module that are used.
type GoModule struct {
	// Dependency details.
	Name    string       `json:"name,omitzero"`
	Version ParsedString `json:"version,omitzero"`

	// Patches to be applied to the
	// downloaded module, before the
	// BUILD file is copied/generated.
	PatchArgs []ParsedString `json:"patch_args,omitzero"`
	Patches   []ParsedString `json:"patches,omitzero"`

	// Packages that should be used.
	Packages []*GoPackage `json:"packages,omitzero"`
}

// GoPackage describes a package within
// a Go module.
type GoPackage struct {
	// Dependency details.
	Name ParsedString `json:"name,omitzero"`

	// Manually-managed BUILD file.
	BuildFile ParsedString `json:"build_file,omitzero"`

	// Build configuration.
	Deps       []ParsedString `json:"deps,omitzero"`
	Embed      []ParsedString `json:"embed,omitzero"`
	EmbedGlobs []ParsedString `json:"embed_globs,omitzero"`

	// Binary configuration.
	Binary     ParsedBool     `json:"binary,omitzero"`
	BinaryDeps []ParsedString `json:"binary_deps,omitzero"`

	// Test configuration.
	NoTests       ParsedBool              `json:"no_tests,omitzero"`
	TestSize      ParsedString            `json:"test_size,omitzero"`
	TestData      []ParsedString          `json:"test_data,omitzero"`
	TestDataGlobs []ParsedString          `json:"test_data_globs,omitzero"`
	TestDeps      []ParsedString          `json:"test_deps,omitzero"`
	TestEnv       map[string]ParsedString `json:"test_env,omitzero"`
}

// Sort ensures that all order-insensitive data
// is sorted into alphabetical order.
func (d *Deps) Sort() {
	if d == nil {
		return
	}

	compareParsedStrings := func(a, b ParsedString) int { return cmp.Compare(a.Value, b.Value) }

	slices.SortFunc(d.BannedGoPackages, compareParsedStrings)
	slices.SortFunc(d.GoModules, func(a, b *GoModule) int { return cmp.Compare(a.Name, b.Name) })
	for _, mod := range d.GoModules {
		slices.SortFunc(mod.Patches, compareParsedStrings)
		slices.SortFunc(mod.Packages, func(a, b *GoPackage) int { return cmp.Compare(a.Name.Value, b.Name.Value) })
		for _, pkg := range mod.Packages {
			slices.SortFunc(pkg.Deps, compareParsedStrings)
			slices.SortFunc(pkg.Embed, compareParsedStrings)
			slices.SortFunc(pkg.EmbedGlobs, compareParsedStrings)
			slices.SortFunc(pkg.BinaryDeps, compareParsedStrings)
			slices.SortFunc(pkg.TestData, compareParsedStrings)
			slices.SortFunc(pkg.TestDataGlobs, compareParsedStrings)
			slices.SortFunc(pkg.TestDeps, compareParsedStrings)
			// The pkg.TestEnv map sorts itself in Encode.
		}
	}
}
