// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package vdmtest

import (
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestFilenames(t *testing.T) {
	tests := []struct {
		Name  string
		FS    fs.FS
		Dir   string
		Want  []string
		Error string
	}{
		{
			Name:  "bad dir",
			FS:    TxtarFS(t, "testdata/files/identical.txtar"),
			Dir:   "nonexistant",
			Error: `open nonexistant: file does not exist`,
		},
		{
			Name:  "bad FS",
			FS:    TestFS(t, WithErrors("dir", "bad read")),
			Dir:   "dir",
			Error: `bad read`,
		},
		{
			Name: "valid-dir",
			FS:   TxtarFS(t, "testdata/files/identical.txtar"),
			Dir:  "a",
			Want: []string{
				"bar.txt",
				"foo/baz.txt",
				"foo/third.txt",
			},
		},
		{
			Name: "valid-file",
			FS:   TxtarFS(t, "testdata/files/identical.txtar"),
			Dir:  "a/bar.txt",
			Want: []string(nil),
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got, err := Filenames(test.FS, test.Dir)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("Filenames(): unexpected lack of error")
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("Filenames(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("Filenames(): got unexpected error: %v", err)
			}

			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Errorf("Filenames(): filenames mismatch (-want, +got)\n%s", diff)
			}
		})
	}
}

func TestDiffFilenames(t *testing.T) {
	tests := []struct {
		Name  string
		FS    fs.FS
		Want  string
		Got   string
		Error []string
	}{
		{
			Name:  "bad-want-dir",
			FS:    TxtarFS(t, "testdata/files/identical.txtar"),
			Want:  "nonexistant",
			Got:   "b",
			Error: []string{`failed to list want filenames: open nonexistant: file does not exist`},
		},
		{
			Name:  "bad-got-dir",
			FS:    TxtarFS(t, "testdata/files/identical.txtar"),
			Want:  "a",
			Got:   "nonexistant",
			Error: []string{`failed to list got filenames: open nonexistant: file does not exist`},
		},
		{
			Name: "invalid-different-filenames",
			FS:   TxtarFS(t, "testdata/files/different-filenames.txtar"),
			Want: "a",
			Got:  "b",
			Error: []string{
				`filenames mismatch (-want, +got)`,
				`  []string{`,
				`  	"bar.txt",`,
				`  	"foo/baz.txt",`,
				`- 	"foo/third.txt",`,
				`+ 	"foo/other.txt",`,
				`  }`,
				``,
			},
		},
		{
			Name: "valid-different-contents",
			FS:   TxtarFS(t, "testdata/files/different-contents.txtar"),
			Want: "a",
			Got:  "b",
		},
		{
			Name: "valid-identical",
			FS:   TxtarFS(t, "testdata/files/identical.txtar"),
			Want: "a",
			Got:  "b",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			err := DiffFilenames(test.FS, test.Got, test.Want)
			if test.Error != nil {
				if err == nil {
					t.Fatalf("DiffFilenames(): unexpected lack of error")
				}

				e := fixError(err.Error())
				want := strings.Join(test.Error, "\n")
				if e != want {
					t.Fatalf("DiffFilenames(): got wrong error:\nGot:  %s\nWant: %s", e, want)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("DiffFilenames(): got unexpected error: %v", err)
			}
		})
	}
}

func TestDiffTextFiles(t *testing.T) {
	tests := []struct {
		Name  string
		FS    fs.FS
		Want  string
		Got   string
		Names []string
		Error []string
	}{
		{
			Name:  "invalid-bad-got-read",
			FS:    TestFS(t, WithReadFileErrors("a/foo.txt", "bad read file")),
			Want:  "a",
			Got:   "b",
			Names: []string{"foo.txt"},
			Error: []string{`failed to open want file in "foo.txt": bad read file`},
		},
		{
			Name: "invalid-bad-want-read",
			FS: TestFS(t,
				WithReadFile(map[string][]byte{"a/foo.txt": []byte("foo")}),
				WithReadFileErrors("b/foo.txt", "bad read file"),
			),
			Want:  "a",
			Got:   "b",
			Names: []string{"foo.txt"},
			Error: []string{`failed to open got file in "foo.txt": bad read file`},
		},
		{
			Name: "valid-identical",
			FS:   TxtarFS(t, "testdata/files/identical.txtar"),
			Want: "a",
			Got:  "b",
			Names: []string{
				"bar.txt",
				"foo/baz.txt",
				"foo/third.txt",
			},
		},
		{
			Name: "valid-different",
			FS:   TxtarFS(t, "testdata/files/different-contents.txtar"),
			Want: "a",
			Got:  "b",
			Names: []string{
				"bar.txt",
				"foo/baz.txt",
				"foo/third.txt",
			},
			Error: []string{
				`data mismatch in "foo/baz.txt" (-want, +got)`,
				`-This is the second file.`,
				`+This is the second file, but with new content.`,
				``,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			err := DiffTextFiles(test.FS, test.Got, test.Want, test.Names)
			if test.Error != nil {
				if err == nil {
					t.Fatalf("DiffTextFiles(): unexpected lack of error")
				}

				e := err.Error()
				want := strings.Join(test.Error, "\n")
				if e != want {
					t.Fatalf("DiffTextFiles(): got wrong error:\nGot:  %s\nWant: %s", e, want)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("DiffTextFiles(): got unexpected error: %v", err)
			}
		})
	}
}

func TestDiffTextDirectories(t *testing.T) {
	tests := []struct {
		Name  string
		FS    fs.FS
		Want  string
		Got   string
		Error []string
	}{
		{
			Name:  "bad-want-dir",
			FS:    TxtarFS(t, "testdata/files/identical.txtar"),
			Want:  "nonexistant",
			Got:   "b",
			Error: []string{`failed to list want filenames: open nonexistant: file does not exist`},
		},
		{
			Name:  "bad-got-dir",
			FS:    TxtarFS(t, "testdata/files/identical.txtar"),
			Want:  "a",
			Got:   "nonexistant",
			Error: []string{`failed to list got filenames: open nonexistant: file does not exist`},
		},
		{
			Name: "invalid-different-filenames",
			FS:   TxtarFS(t, "testdata/files/different-filenames.txtar"),
			Want: "a",
			Got:  "b",
			Error: []string{
				`filenames mismatch (-want, +got)`,
				`  []string{`,
				`  	"bar.txt",`,
				`  	"foo/baz.txt",`,
				`- 	"foo/third.txt",`,
				`+ 	"foo/other.txt",`,
				`  }`,
				``,
			},
		},
		{
			Name: "invalid-different-contents",
			FS:   TxtarFS(t, "testdata/files/different-contents.txtar"),
			Want: "a",
			Got:  "b",
			Error: []string{
				`data mismatch in "foo/baz.txt" (-want, +got)`,
				`-This is the second file.`,
				`+This is the second file, but with new content.`,
				``,
			},
		},
		{
			Name: "valid-identical",
			FS:   TxtarFS(t, "testdata/files/identical.txtar"),
			Want: "a",
			Got:  "b",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			err := DiffTextDirectories(test.FS, test.Got, test.Want)
			if test.Error != nil {
				if err == nil {
					t.Fatalf("DiffTextDirectories(): unexpected lack of error")
				}

				e := fixError(err.Error())
				want := strings.Join(test.Error, "\n")
				if e != want {
					t.Fatalf("DiffTextDirectories(): got wrong error:\nGot:  %s\nWant: %s", e, want)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("DiffTextDirectories(): got unexpected error: %v", err)
			}
		})
	}
}

func TestDiffTextFilesystems(t *testing.T) {
	tests := []struct {
		Name  string
		Want  fs.FS
		Got   fs.FS
		Dir   string
		Error []string
	}{
		{
			Name:  "bad-want-dir",
			Want:  TestFS(t, WithError(errors.New("bad FS"))),
			Got:   TxtarFS(t, "testdata/filesystems/base.txtar"),
			Dir:   ".",
			Error: []string{`failed to list want filenames: bad FS`},
		},
		{
			Name:  "bad-got-dir",
			Want:  TxtarFS(t, "testdata/filesystems/base.txtar"),
			Got:   TestFS(t, WithError(errors.New("bad FS"))),
			Dir:   ".",
			Error: []string{`failed to list got filenames: bad FS`},
		},
		{
			Name: "invalid-different-filenames",
			Want: TxtarFS(t, "testdata/filesystems/base.txtar"),
			Got:  TxtarFS(t, "testdata/filesystems/different-filenames.txtar"),
			Dir:  ".",
			Error: []string{
				`filenames mismatch (-want, +got)`,
				`  []string{`,
				`  	"bar.txt",`,
				`  	"foo/baz.txt",`,
				`- 	"foo/third.txt",`,
				`+ 	"foo/other.txt",`,
				`  }`,
				``,
			},
		},
		{
			Name: "invalid-bad-got-read",
			Want: TestFS(t,
				WithStat(map[string]fs.FileInfo{
					".": TestFileInfo(".", 0, 0o755, testTime, true, nil),
				}),
				WithReadDir(map[string][]fs.DirEntry{
					".": {TestDirEntry("foo.txt", false, 0o644, TestFileInfo("foo.txt", 1, 0o644, testTime, false, nil), nil)},
				}),
				WithReadFileErrors("foo.txt", "bad read file"),
			),
			Got:   TxtarFS(t, "testdata/filesystems/foo.txtar"),
			Dir:   ".",
			Error: []string{`failed to open want file in "foo.txt": bad read file`},
		},
		{
			Name: "invalid-bad-want-read",
			Want: TxtarFS(t, "testdata/filesystems/foo.txtar"),
			Got: TestFS(t,
				WithStat(map[string]fs.FileInfo{
					".": TestFileInfo(".", 0, 0o755, testTime, true, nil),
				}),
				WithReadDir(map[string][]fs.DirEntry{
					".": {TestDirEntry("foo.txt", false, 0o644, TestFileInfo("foo.txt", 1, 0o644, testTime, false, nil), nil)},
				}),
				WithReadFileErrors("foo.txt", "bad read file"),
			),
			Dir:   ".",
			Error: []string{`failed to open got file in "foo.txt": bad read file`},
		},
		{
			Name: "invalid-different-contents",
			Want: TxtarFS(t, "testdata/filesystems/base.txtar"),
			Got:  TxtarFS(t, "testdata/filesystems/different-contents.txtar"),
			Dir:  ".",
			Error: []string{
				`data mismatch in "foo/baz.txt" (-want, +got)`,
				`-This is the second file.`,
				`+This is the second file, but with new content.`,
				``,
			},
		},
		{
			Name: "valid-identical",
			Want: TxtarFS(t, "testdata/filesystems/base.txtar"),
			Got:  TxtarFS(t, "testdata/filesystems/base.txtar"),
			Dir:  ".",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			err := DiffTextFilesystems(test.Got, test.Want, test.Dir)
			if test.Error != nil {
				if err == nil {
					t.Fatalf("DiffTextFilesystems(): unexpected lack of error")
				}

				e := fixError(err.Error())
				want := strings.Join(test.Error, "\n")
				if e != want {
					t.Fatalf("DiffTextFilesystems(): got wrong error:\nGot:  %s\nWant: %s", e, want)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("DiffTextFilesystems(): got unexpected error: %v", err)
			}
		})
	}
}

// rsc.io/diff can be non-deterministic in its
// output, sometimes using `\u00a0` in place of
// leading spaces. To keep the tests consistent,
// we ensure they're always spaces.
func fixError(s string) string {
	canonicalise := func(r rune) rune {
		if r == '\u00a0' {
			return ' '
		}

		return r
	}

	return strings.Map(canonicalise, s)
}
