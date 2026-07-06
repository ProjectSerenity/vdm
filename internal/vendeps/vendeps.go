// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package vendeps provides functionality for managing vendored external dependencies.
package vendeps

import (
	"github.com/ProjectSerenity/vdm/internal/simplehttp"
)

const (
	BuildBazel  = "BUILD.bazel"
	DepsBzl     = "deps.bzl"
	ManifestBzl = "manifest.bzl"
	Vendor      = "vendor"
)

// Deps describes a set of software dependencies.
type Deps struct {
	Go SortedGoModules `bzl:"go/module" json:"go,omitzero"`
}

// GoModule contains the information necessary
// to vendor a Go module, specifying the set
// of packages within the module that are used.
type GoModule struct {
	// Dependency details.
	Name    string `bzl:"name" json:"name,omitzero"`
	Version string `bzl:"version" json:"version,omitzero"`

	// Patches to be applied to the
	// downloaded module, before the
	// BUILD file is copied/generated.
	PatchArgs []string `bzl:"patch_args" json:"patch_args,omitzero"`
	Patches   []string `bzl:"patches" json:"patches,omitzero"`

	// Packages that should be used.
	Packages SortedGoPackages `bzl:"packages/package" json:"packages,omitzero"`

	// Directories containing plain files.
	Directories SortedTextFiles `bzl:"directories/files" json:"directories,omitzero"`

	// Generation details.
	Digest      string `bzl:"digest" json:"digest,omitzero"`
	PatchDigest string `bzl:"patch_digest" json:"patch_digest,omitzero"`
}

// GoPackage describes a package within
// a Go module.
type GoPackage struct {
	// Dependency details.
	Name  string   `bzl:"name" json:"name,omitzero"`
	Files []string `json:"-"`

	// Whether to use Bzlmod names
	// (eg rules_go, rather than io_bazel_rules_go).
	Bzlmod bool `json:"-"`

	// Manually-managed BUILD file.
	BuildFile string `bzl:"build_file" json:"build_file,omitzero"`

	// Build configuration.
	Deps       SortedStrings `bzl:"deps" json:"deps,omitzero"`
	Embed      SortedStrings `bzl:"embed" json:"embed,omitzero"`
	EmbedGlobs []string      `bzl:"embed_globs" json:"embed_globs,omitzero"`

	// Binary configuration.
	Binary     bool          `bzl:"binary" json:"binary,omitzero"`
	BinaryDeps SortedStrings `bzl:"binary_deps" json:"binary_deps,omitzero"`

	// Test configuration.
	NoTests       bool              `bzl:"no_tests" json:"no_tests,omitzero"`
	TestFiles     []string          `json:"-"`
	TestSize      string            `bzl:"test_size" json:"test_size,omitzero"`
	TestData      SortedStrings     `bzl:"test_data" json:"test_data,omitzero"`
	TestDataGlobs []string          `bzl:"test_data_globs" json:"test_data_globs,omitzero"`
	TestDeps      SortedStrings     `bzl:"test_deps" json:"test_deps,omitzero"`
	TestEnv       map[string]string `bzl:"test_env" json:"test_env,omitzero"`
}

// TextFiles contains information necessary
// to manage text files.
type TextFiles struct {
	// Dependency details.
	Name string `bzl:"name" json:"name,omitzero"`

	// Export files publicly.
	ExportsFiles SortedStrings `bzl:"exports_files" json:"exports_files,omitzero"`
}

// UpdateDeps includes a set of dependencies
// for the purposes of updating them.
type UpdateDeps struct {
	Go SortedUpdateDeps
}

// UpdateDep describes the least information
// necessary to determine a third-party
// software library. This is used when
// determining whether updates are available.
type UpdateDep struct {
	Name    string
	Version *string
}

func init() {
	simplehttp.UserAgent = "vendoring-dependency-manager/1 (github.com/ProjectSerenity/vdm)"
}
