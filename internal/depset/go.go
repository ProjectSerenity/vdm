// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package depset

import (
	"iter"
	"slices"
)

// GoPackages records a set of Go package dependencies. It
// records the expected and seen set of packages, identifying
// those that were expected but not seen and those seen but
// not expected. It also differentiates between test and
// main (non-test) dependencies.
//
// GoPackages assumes that main dependencies are implicitly
// available as test dependencies, as is the case in Bazel.
// That is, a Go package main dependency does not need to
// be declared as a test dependency too.
//
// This property cascades. If a test dependency is found as
// main dependency, it is reported by [GoPackages.BecameMain],
// rather than [GoPackages.UnexpectedMain].
//
// Its results are provided via various iterators.
//
// The zero value of GoPackages is useful, after calling
// [GoPackages.Reset].
type GoPackages struct {
	allExpectedMainDeps   []string
	allExpectedTestDeps   []string
	allUnexpectedMainDeps []string
	allUnexpectedTestDeps []string

	isDuplicated     map[string]bool
	isExpectedMain   map[string]bool
	isExpectedTest   map[string]bool
	isUsedMain       map[string]bool
	isUsedTest       map[string]bool
	isUnexpectedMain map[string]bool
	isUnexpectedTest map[string]bool
}

// NewGoPackages prepares an empty set of Go package
// dependencies.
func NewGoPackages() *GoPackages {
	pkgs := new(GoPackages)
	pkgs.Reset()

	return pkgs
}

func (pkgs *GoPackages) Reset() {
	// Delete the previous data from the slices.
	clear(pkgs.allExpectedMainDeps)
	clear(pkgs.allExpectedTestDeps)
	clear(pkgs.allUnexpectedMainDeps)
	clear(pkgs.allUnexpectedTestDeps)

	// Reset the slices to length zero, while
	// keeping any previously allocated space.
	pkgs.allExpectedMainDeps = pkgs.allExpectedMainDeps[:0]
	pkgs.allExpectedTestDeps = pkgs.allExpectedTestDeps[:0]
	pkgs.allUnexpectedMainDeps = pkgs.allUnexpectedMainDeps[:0]
	pkgs.allUnexpectedTestDeps = pkgs.allUnexpectedTestDeps[:0]

	if pkgs.isDuplicated != nil {
		// Delete the previous data from the maps,
		// resetting them to length zero, while
		// keeping any previously allocated space.
		clear(pkgs.isDuplicated)
		clear(pkgs.isExpectedMain)
		clear(pkgs.isExpectedTest)
		clear(pkgs.isUsedMain)
		clear(pkgs.isUsedTest)
		clear(pkgs.isUnexpectedMain)
		clear(pkgs.isUnexpectedTest)
	} else {
		// Allocate the maps so they're ready for
		// use.
		pkgs.isDuplicated = make(map[string]bool)
		pkgs.isExpectedMain = make(map[string]bool)
		pkgs.isExpectedTest = make(map[string]bool)
		pkgs.isUsedMain = make(map[string]bool)
		pkgs.isUsedTest = make(map[string]bool)
		pkgs.isUnexpectedMain = make(map[string]bool)
		pkgs.isUnexpectedTest = make(map[string]bool)
	}
}

// Sort arranges the data into alphabetical order.
func (pkgs *GoPackages) Sort() {
	slices.Sort(pkgs.allExpectedMainDeps)
	slices.Sort(pkgs.allExpectedTestDeps)
	slices.Sort(pkgs.allUnexpectedMainDeps)
	slices.Sort(pkgs.allUnexpectedTestDeps)
}

// ExpectMain records that the given dependency is
// expected as a main dependency.
func (pkgs *GoPackages) ExpectMain(name string) {
	pkgs.allExpectedMainDeps = append(pkgs.allExpectedMainDeps, name)
	pkgs.isExpectedMain[name] = true

	// Identify packages that are expected as
	// both a test and a non-test dependency.
	if pkgs.isExpectedTest[name] {
		pkgs.isDuplicated[name] = true
	}
}

// ExpectTest records that the given dependency
// is expected as a test dependency.
func (pkgs *GoPackages) ExpectTest(name string) {
	pkgs.allExpectedTestDeps = append(pkgs.allExpectedTestDeps, name)
	pkgs.isExpectedTest[name] = true

	// Identify packages that are expected as
	// both a test and a non-test dependency.
	if pkgs.isExpectedMain[name] {
		pkgs.isDuplicated[name] = true
	}
}

// UseMain records that the given dependency has
// been used in a non-test file.
func (pkgs *GoPackages) UseMain(name string) {
	pkgs.isUsedMain[name] = true
	if !pkgs.isUnexpectedMain[name] && !pkgs.isExpectedMain[name] {
		pkgs.isUnexpectedMain[name] = true
		pkgs.allUnexpectedMainDeps = append(pkgs.allUnexpectedMainDeps, name)
	}
}

// UseTest records that the given dependency has
// been used in a test file.
func (pkgs *GoPackages) UseTest(name string) {
	pkgs.isUsedTest[name] = true
	if !pkgs.isUnexpectedTest[name] && !pkgs.isExpectedTest[name] {
		pkgs.isUnexpectedTest[name] = true
		pkgs.allUnexpectedTestDeps = append(pkgs.allUnexpectedTestDeps, name)
	}
}

// Duplicated yields dependencies that are expected
// as both main and test dependencies.
//
// This is independent of whether each package was
// seen in either context.
func (pkgs *GoPackages) Duplicated() iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, dep := range pkgs.allExpectedTestDeps {
			if pkgs.isDuplicated[dep] && !yield(dep) {
				return
			}
		}
	}
}

// UnusedMain yields the expected main dependencies
// that were not used in any context.
func (pkgs *GoPackages) UnusedMain() iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, dep := range pkgs.allExpectedMainDeps {
			if !pkgs.isUsedMain[dep] && !pkgs.isUsedTest[dep] && !yield(dep) {
				return
			}
		}
	}
}

// UnusedTest yields the expected test dependencies
// that were not used in any context.
func (pkgs *GoPackages) UnusedTest() iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, dep := range pkgs.allExpectedTestDeps {
			if !pkgs.isUsedTest[dep] && !pkgs.isUsedMain[dep] && !yield(dep) {
				return
			}
		}
	}
}

// UnexpectedMain yields the main dependencies that
// were recorded as used but not as expected in either
// context.
func (pkgs *GoPackages) UnexpectedMain() iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, dep := range pkgs.allUnexpectedMainDeps {
			if !pkgs.isExpectedTest[dep] && !yield(dep) {
				return
			}
		}
	}
}

// UnexpectedTest yields the test dependencies that
// were recorded as used but not as expected in either
// context.
func (pkgs *GoPackages) UnexpectedTest() iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, dep := range pkgs.allUnexpectedTestDeps {
			if !pkgs.isExpectedMain[dep] && !yield(dep) {
				return
			}
		}
	}
}

// BecameMain yields the dependencies that were recorded
// as expected test dependencies but were used as main
// dependencies.
func (pkgs *GoPackages) BecameMain() iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, dep := range pkgs.allUnexpectedMainDeps {
			if pkgs.isExpectedTest[dep] && !pkgs.isExpectedMain[dep] && !yield(dep) {
				return
			}
		}
	}
}

// BecameTest yields the dependencies that were recorded
// as expected main dependencies but were used as test
// dependencies.
func (pkgs *GoPackages) BecameTest() iter.Seq[string] {
	return func(yield func(string) bool) {
		for _, dep := range pkgs.allUnexpectedTestDeps {
			if pkgs.isExpectedMain[dep] && !pkgs.isUsedMain[dep] && !pkgs.isExpectedTest[dep] && !yield(dep) {
				return
			}
		}
	}
}
