// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package vdmtest

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/tools/txtar"
)

// TxtarFS takes a path to a txtar archive and returns
// a filesystem containing the archive's files.
//
// The path should use forward slashes for separators,
// if any are needed.
//
// If an error is encountered, `t.Fatal` is called.
func TxtarFS(t testing.TB, path string) fs.FS {
	t.Helper()
	ar, err := txtar.ParseFile(filepath.FromSlash(path))
	if err != nil {
		t.Fatalf("failed to parse %s: %v", path, err)
	}

	fsys, err := txtar.FS(ar)
	if err != nil {
		t.Fatal(err)
	}

	return fsys
}

// ExtractTxtar takes a path to a txtar archive, opens
// it, and copies its contents to the target directory.
//
// The path should use forward slashes for separators,
// if any are needed.
//
// If an error is encountered, `t.Fatal` is called.
func ExtractTxtar(t testing.TB, target, path string) {
	t.Helper()
	ar, err := txtar.ParseFile(filepath.FromSlash(path))
	if err != nil {
		t.Fatalf("failed to parse %s: %v", path, err)
	}

	for _, file := range ar.Files {
		filename := filepath.FromSlash(file.Name)
		parent := filepath.Dir(filename)
		if parent != "." && parent != string(filepath.Separator) {
			err = os.MkdirAll(parent, 0o777)
			if err != nil {
				t.Fatalf("failed to create %s: %v", parent, err)
			}
		}

		err = os.WriteFile(filepath.Join(target, filename), file.Data, 0o666)
		if err != nil {
			t.Fatalf("failed to write %s: %v", file.Name, err)
		}
	}
}
