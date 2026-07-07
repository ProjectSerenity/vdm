// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package stats

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParentModules(t *testing.T) {
	tests := []struct {
		Module string
		Want   []string
	}{
		{"github.com/SlyMarbo/marbo", []string{"github.com/SlyMarbo", "github.com"}},
		{"github.com/SlyMarbo/marbo/foo", []string{"github.com/SlyMarbo/marbo", "github.com/SlyMarbo", "github.com"}},
	}

	for _, test := range tests {
		got := slices.Collect(parentModules(test.Module))
		if diff := cmp.Diff(test.Want, got); diff != "" {
			t.Errorf("parentModules(%q): modules mismatch (-want, +got)\n%s", test.Module, diff)
		}
	}
}
