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
	Go []*GoModule `bzl:"go/module"`
}

// GoModule contains the information necessary
// to vendor a Go module, specifying the set
// of packages within the module that are used.
type GoModule struct {
	// Dependency details.
	Name    string `bzl:"name"`
	Version string `bzl:"version"`

	// Patches to be applied to the
	// downloaded module, before the
	// BUILD file is copied/generated.
	PatchArgs []string `bzl:"patch_args"`
	Patches   []string `bzl:"patches"`

	// Packages that should be used.
	Packages []*GoPackage `bzl:"packages/package"`

	// Directories containing plain files.
	Directories []*TextFiles `bzl:"directories/files"`

	// Generation details.
	Digest      string `bzl:"digest"`
	PatchDigest string `bzl:"patch_digest"`
}

// GoPackage describes a package within
// a Go module.
type GoPackage struct {
	// Dependency details.
	Name  string `bzl:"name"`
	Files []string

	// Whether to use Bzlmod names
	// (eg rules_go, rather than io_bazel_rules_go).
	Bzlmod bool

	// Manually-managed BUILD file.
	BuildFile string `bzl:"build_file"`

	// Build configuration.
	Deps       []string `bzl:"deps"`
	Embed      []string `bzl:"embed"`
	EmbedGlobs []string `bzl:"embed_globs"`

	// Binary configuration.
	Binary     bool     `bzl:"binary"`
	BinaryDeps []string `bzl:"binary_deps"`

	// Test configuration.
	NoTests       bool `bzl:"no_tests"`
	TestFiles     []string
	TestSize      string            `bzl:"test_size"`
	TestData      []string          `bzl:"test_data"`
	TestDataGlobs []string          `bzl:"test_data_globs"`
	TestDeps      []string          `bzl:"test_deps"`
	TestEnv       map[string]string `bzl:"test_env"`
}

// TextFiles contains information necessary
// to manage text files.
type TextFiles struct {
	// Dependency details.
	Name string `bzl:"name"`

	// Export files publicly.
	ExportsFiles []string `bzl:"exports_files"`
}

// UpdateDeps includes a set of dependencies
// for the purposes of updating them.
type UpdateDeps struct {
	Go []*UpdateDep
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
