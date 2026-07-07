// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package convert

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/ProjectSerenity/vdm/internal/starlark"
	"github.com/ProjectSerenity/vdm/internal/vendeps"

	"golang.org/x/tools/txtar"
	"rsc.io/diff"
)

func TestConvert(t *testing.T) {
	tests := []string{
		"complex",
	}

	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			ar, err := txtar.ParseFile(filepath.Join("testdata", test+".txtar"))
			if err != nil {
				t.Fatal(err)
			}

			if len(ar.Files) != 2 {
				t.Fatalf("got %d files, want 2", len(ar.Files))
			}

			if ar.Files[0].Name != "deps.bzl" || ar.Files[1].Name != "deps.vdm" {
				t.Fatalf("got wrong filenames:\nGot:  %q, %q\nWant: %q, %q", ar.Files[0].Name, ar.Files[1].Name, "deps.bzl", "deps.vdm")
			}

			var deps vendeps.Deps
			err = starlark.Unmarshal(vendeps.DepsBzl, ar.Files[0].Data, &deps)
			if err != nil {
				t.Fatalf("failed to parse input: %v", err)
			}

			converted := Convert(&deps)
			got := converted.Encode()
			if !bytes.Equal(got, ar.Files[1].Data) {
				t.Fatalf("bad conversion output:\n%s", diff.Format(string(ar.Files[1].Data), string(got)))
			}
		})
	}
}
