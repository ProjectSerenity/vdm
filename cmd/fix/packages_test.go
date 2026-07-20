// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package fix

import (
	"testing"
)

func TestIsGoStdlibPackage(t *testing.T) {
	tests := []struct {
		Name string
		Path string
		Want bool
	}{
		{
			Name: "short-stdlib",
			Path: "bytes",
			Want: true,
		},
		{
			Name: "long-stdlib",
			Path: "net/http/httptest",
			Want: true,
		},
		{
			Name: "short-external",
			Path: "rsc.io/diff",
			Want: false,
		},
		{
			Name: "long-external",
			Path: "github.com/google/go-cmp/cmpopts",
			Want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got := isGoStdlibPackage(test.Path)
			if got != test.Want {
				t.Fatalf("isGoStdlibPackage(%q): got %v, want %v", test.Path, got, test.Want)
			}
		})
	}
}
