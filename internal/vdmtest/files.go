// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package vdmtest

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/google/go-cmp/cmp"
	"rsc.io/diff"
)

// Filenames retrieves the set of files within the given subset
// of a filesystem. The filenames returned are all relative to
// the chosen directory. This allows Filenames to be used to
// compare the filenames in two directories for similarity.
func Filenames(fsys fs.FS, dir string) ([]string, error) {
	var filenames []string
	err := fs.WalkDir(fsys, dir, func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d == nil || d.IsDir() {
			return nil
		}

		if name == dir {
			// The input was a filename, not a directory.
			return nil
		}

		filenames = append(filenames, strings.TrimPrefix(name, dir+"/"))
		return nil
	})

	return filenames, err
}

// DiffFilenames iterates through the two given directories.
// If the list of filenames are not identical, DiffFilenames
// returns an error.
func DiffFilenames(fsys fs.FS, got, want string) error {
	wantFiles, err := Filenames(fsys, want)
	if err != nil {
		return fmt.Errorf("failed to list want filenames: %v", err)
	}

	gotFiles, err := Filenames(fsys, got)
	if err != nil {
		return fmt.Errorf("failed to list got filenames: %v", err)
	}

	if diff := cmp.Diff(wantFiles, gotFiles); diff != "" {
		return fmt.Errorf("filenames mismatch (-want, +got)\n%s", diff)
	}

	return nil
}

// DiffTextFiles iterates through the set of filenames given
// in the two directories. If any files have different contents,
// the diff(s) are returned as an error.
//
// Note that DiffTextFiles expects file contents to be printable
// text. If any files have differing contents that are not text,
// the resulting error's text is undefined.
func DiffTextFiles(fsys fs.FS, got, want string, filenames []string) error {
	results := make([]error, len(filenames))
	for i, name := range filenames {
		dataWant, err := fs.ReadFile(fsys, path.Join(want, name))
		if err != nil {
			results[i] = fmt.Errorf("failed to open want file in %q: %v", name, err)
			continue
		}

		dataGot, err := fs.ReadFile(fsys, path.Join(got, name))
		if err != nil {
			results[i] = fmt.Errorf("failed to open got file in %q: %v", name, err)
			continue
		}

		if !bytes.Equal(dataGot, dataWant) {
			results[i] = fmt.Errorf("data mismatch in %q (-want, +got)\n%s", name, diff.Format(string(dataWant), string(dataGot)))
			continue
		}
	}

	return errors.Join(results...)
}

// DiffTextDirectories iterates through the two given directories.
// If any files have different contents, the diff(s) are returned
// as an error.
//
// Note that DiffTextDirectories expects file contents to be printable
// text. If any files have differing contents that are not text,
// the resulting error's text is undefined.
func DiffTextDirectories(fsys fs.FS, got, want string) error {
	wantFiles, err := Filenames(fsys, want)
	if err != nil {
		return fmt.Errorf("failed to list want filenames: %v", err)
	}

	gotFiles, err := Filenames(fsys, got)
	if err != nil {
		return fmt.Errorf("failed to list got filenames: %v", err)
	}

	if diff := cmp.Diff(wantFiles, gotFiles); diff != "" {
		return fmt.Errorf("filenames mismatch (-want, +got)\n%s", diff)
	}

	return DiffTextFiles(fsys, got, want, gotFiles)
}

// DiffTextFilesystems iterates through the two given filesystems.
// If any files have different contents, the diff(s) are returned
// as an error.
//
// Note that DiffTextFilesystems expects file contents to be printable
// text. If any files have differing contents that are not text,
// the resulting error's text is undefined.
func DiffTextFilesystems(got, want fs.FS, dir string) error {
	wantFiles, err := Filenames(want, dir)
	if err != nil {
		return fmt.Errorf("failed to list want filenames: %v", err)
	}

	gotFiles, err := Filenames(got, dir)
	if err != nil {
		return fmt.Errorf("failed to list got filenames: %v", err)
	}

	if diff := cmp.Diff(wantFiles, gotFiles); diff != "" {
		return fmt.Errorf("filenames mismatch (-want, +got)\n%s", diff)
	}

	results := make([]error, len(gotFiles))
	for i, name := range gotFiles {
		dataWant, err := fs.ReadFile(want, path.Join(dir, name))
		if err != nil {
			results[i] = fmt.Errorf("failed to open want file in %q: %v", name, err)
			continue
		}

		dataGot, err := fs.ReadFile(got, path.Join(dir, name))
		if err != nil {
			results[i] = fmt.Errorf("failed to open got file in %q: %v", name, err)
			continue
		}

		if !bytes.Equal(dataGot, dataWant) {
			results[i] = fmt.Errorf("data mismatch in %q (-want, +got)\n%s", name, diff.Format(string(dataWant), string(dataGot)))
			continue
		}
	}

	return errors.Join(results...)
}
