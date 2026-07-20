// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package depset

import (
	"iter"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestGoPackages(t *testing.T) {
	tests := []struct {
		Name           string
		ExpectMain     []string
		ExpectTest     []string
		UseMain        []string
		UseTest        []string
		Duplicated     []string
		UnusedMain     []string
		UnusedTest     []string
		UnexpectedMain []string
		UnexpectedTest []string
		BecameMain     []string
		BecameTest     []string
	}{
		{
			Name:       "exact",
			ExpectMain: []string{"a", "b", "c"},
			ExpectTest: []string{"d", "e"},
			UseMain:    []string{"a", "b", "c"},
			UseTest:    []string{"d", "e"},
		},
		{
			Name:       "repeated",
			ExpectMain: []string{"a", "b", "c"},
			ExpectTest: []string{"d", "e"},
			UseMain:    []string{"a", "b", "c", "b", "a"},
			UseTest:    []string{"d", "e", "d"},
		},
		{
			Name:       "duplicated",
			ExpectMain: []string{"a", "b", "c"},
			ExpectTest: []string{"d", "e", "b"},
			UseMain:    []string{"a", "b", "c", "b", "a"},
			UseTest:    []string{"d", "e", "d"},
			Duplicated: []string{"b"},
		},
		{
			Name:       "missing",
			ExpectMain: []string{"a", "b", "c"},
			ExpectTest: []string{"d", "e"},
			UseMain:    []string{"a"},
			UseTest:    []string{},
			UnusedMain: []string{"b", "c"},
			UnusedTest: []string{"d", "e"},
		},
		{
			Name:           "extras",
			ExpectMain:     []string{"a", "b", "c"},
			ExpectTest:     []string{"d", "e"},
			UseMain:        []string{"a", "b", "c", "f", "e", "d", "e"},
			UseTest:        []string{"d", "e", "g", "f", "g"},
			UnexpectedMain: []string{"f"},
			UnexpectedTest: []string{"f", "g"},
			BecameMain:     []string{"d", "e"},
		},
		{
			Name:       "upgraded",
			ExpectMain: []string{"a", "b", "c"},
			ExpectTest: []string{"d", "e"},
			UseMain:    []string{"a", "b", "c", "e"},
			UseTest:    []string{"d"},
			BecameMain: []string{"e"},
		},
		{
			Name:       "downgraded",
			ExpectMain: []string{"a", "b", "c"},
			ExpectTest: []string{"d", "e"},
			UseMain:    []string{"a", "c"},
			UseTest:    []string{"d", "b", "e"},
			BecameTest: []string{"b"},
		},
		{
			Name: "github.com/google/go-cmp/cmp/cmpopts",
			ExpectMain: []string{
				"github.com/google/go-cmp/cmp",
				"github.com/google/go-cmp/cmp/internal/function",
			},
			ExpectTest: []string{
				"github.com/google/go-cmp/cmp/internal/flags",
			},
			UseMain: []string{
				// equate.go
				"github.com/google/go-cmp/cmp",
				// ignore.go
				"github.com/google/go-cmp/cmp",
				"github.com/google/go-cmp/cmp/internal/function",
				// sort.go
				"github.com/google/go-cmp/cmp",
				"github.com/google/go-cmp/cmp/internal/function",
				// struct_filter.go
				"github.com/google/go-cmp/cmp",
				// xform.go
				"github.com/google/go-cmp/cmp",
			},
			UseTest: []string{
				// example_test.go
				"github.com/google/go-cmp/cmp",
				// "github.com/google/go-cmp/cmp/cmpopts",
				"github.com/google/go-cmp/cmp/internal/flags",
				// util_test.go
				"github.com/google/go-cmp/cmp",
			},
		},
	}

	add := func(fun func(string), deps []string) {
		for _, dep := range deps {
			fun(dep)
		}
	}

	check := func(t *testing.T, name string, got iter.Seq[string], want []string) {
		t.Helper()
		if diff := cmp.Diff(want, slices.Collect(got)); diff != "" {
			t.Errorf("%s(): dependencies mismatch (-want, +got)\n%s", name, diff)
		}
	}

	takeFirst := func(want iter.Seq[string]) {
		for range want {
			break
		}
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			pkgs := NewGoPackages()

			// Run each test twice to check Reset
			// works.
			tries := []string{"one", "two"}
			for _, try := range tries {
				t.Run(try, func(t *testing.T) {
					add(pkgs.ExpectMain, test.ExpectMain)
					add(pkgs.ExpectTest, test.ExpectTest)
					add(pkgs.UseMain, test.UseMain)
					add(pkgs.UseTest, test.UseTest)

					pkgs.Sort()

					check(t, "Duplicated", pkgs.Duplicated(), test.Duplicated)
					check(t, "UnusedMain", pkgs.UnusedMain(), test.UnusedMain)
					check(t, "UnusedTest", pkgs.UnusedTest(), test.UnusedTest)
					check(t, "UnexpectedMain", pkgs.UnexpectedMain(), test.UnexpectedMain)
					check(t, "UnexpectedTest", pkgs.UnexpectedTest(), test.UnexpectedTest)
					check(t, "BecameMain", pkgs.BecameMain(), test.BecameMain)
					check(t, "BecameTest", pkgs.BecameTest(), test.BecameTest)

					takeFirst(pkgs.Duplicated())
					takeFirst(pkgs.UnusedMain())
					takeFirst(pkgs.UnusedTest())
					takeFirst(pkgs.UnexpectedMain())
					takeFirst(pkgs.UnexpectedTest())
					takeFirst(pkgs.BecameMain())
					takeFirst(pkgs.BecameTest())

					pkgs.Reset()
				})
			}
		})
	}
}
