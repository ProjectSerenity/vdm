// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package digest

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io/fs"
	"iter"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/ProjectSerenity/vdm/internal/vdmtest"

	"github.com/google/go-cmp/cmp"
)

func TestDigestFile(t *testing.T) {
	tests := []struct {
		Name    string
		FS      fs.FS
		Prefix  string
		Entries []testEntry
		Want    []testEntry
	}{
		{
			Name: "straightforward",
			FS:   vdmtest.TxtarFS(t, "testdata/subdirectories.txtar"),
			Entries: []testEntry{
				{E: errors.New("input error")},
				{S: "nonexistant.txt"},
				{S: "foo/bar/a/b/c.txt"},
				{S: "foo/bar/baz.txt"},
			},
			Want: []testEntry{
				{E: errors.New("input error")},
				{E: errors.New("failed to open nonexistant.txt: open nonexistant.txt: file does not exist")},
				{S: "0c47cda934d53d7ca29d822a59531dcf6d36cbd9740a4fd0b867a0343910a715  foo/bar/a/b/c.txt\n"},
				{S: "181210f8f9c779c26da1d9b2075bde0127302ee0e3fca38c9a83f5b1dd8e5d3b  foo/bar/baz.txt\n"},
			},
		},
		{
			Name: "weird",
			FS: fstest.MapFS{
				"newline\nfile.txt": {
					Data:    []byte{1, 2, 3},
					Mode:    0o644,
					ModTime: time.Now(),
				},
			},
			Entries: []testEntry{
				{S: "newline\nfile.txt"},
			},
			Want: []testEntry{
				{E: errors.New(`filenames with newlines are not allowed: found "newline\nfile.txt"`)},
			},
		},
		{
			Name: "errors",
			FS: vdmtest.TestFS(t,
				vdmtest.WithOpenErrors("open", "bad open"),
				vdmtest.WithFiles(map[string]fs.File{
					"read":  vdmtest.TestFile(nil, nil, vdmtest.ErrReader("bad read"), nil),
					"close": vdmtest.TestFile(nil, nil, strings.NewReader("foo"), errors.New("bad close")),
				})),
			Entries: []testEntry{
				{S: "open"},
				{S: "read"},
				{S: "close"},
			},
			Want: []testEntry{
				{E: errors.New("failed to open open: bad open")},
				{E: errors.New("failed to read read: bad read")},
				{E: errors.New("failed to close close: bad close")},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			// Do the entries as a group.
			got := collect(digestFile(test.FS, test.Prefix, iterate(test.Entries)))
			if diff := cmp.Diff(test.Want, got, compareErrors); diff != "" {
				t.Fatalf("digestFile(): mismatch (-want, +got)\n%s", diff)
			}

			// Do the entries individually.
			for i, entry := range test.Entries {
				got := collectSingle(digestFile(test.FS, test.Prefix, iterateSingle(entry)))
				if diff := cmp.Diff(test.Want[i], got, compareErrors); diff != "" {
					t.Fatalf("digestFile(%d): mismatch (-want, +got)\n%s", i, diff)
				}
			}
		})
	}
}

func TestFiles(t *testing.T) {
	tests := []struct {
		Name  string
		FS    fs.FS
		Files []string
		Want  string
		Error string
	}{
		{
			Name:  "nonexistant",
			FS:    vdmtest.TxtarFS(t, "testdata/small.txtar"),
			Files: []string{"nonexistant.txt"},
			Error: "failed to open nonexistant.txt: open nonexistant.txt: file does not exist",
		},
		{
			Name:  "small",
			FS:    vdmtest.TxtarFS(t, "testdata/small.txtar"),
			Files: []string{"foo/bar.txt"},
			Want:  "sha256:nXbUeK5RNiJs1oQQWJa3D5t0HpyVd6lqNMUcYaIS5hQ=",
		},
		{
			Name: "subdirectories",
			FS:   vdmtest.TxtarFS(t, "testdata/subdirectories.txtar"),
			Files: []string{
				"foo/bar/a/b/c.txt",
				"foo/bar/baz.txt",
			},
			Want: "sha256:XjdtpK2cfWjxTP9RvL5f7/YJaeAncW+MQB1SrUD9JIY=",
		},
		{
			Name: "unsorted",
			FS:   vdmtest.TxtarFS(t, "testdata/subdirectories.txtar"),
			Files: []string{
				"foo/bar/baz.txt",
				"foo/bar/a/b/c.txt",
			},
			Want: "sha256:EMDAMuiLe5TMiYMznf26bfMsvyWyEgzSeKcVCkHfLDY=", // The order matters, so the digest is different from above.
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			final, err := Files(test.FS, test.Files)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("Files(): unexpected lack of error")
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("Files(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("Files(): %v", err)
			}

			if final != test.Want {
				t.Fatalf("Files(): digest mismatch:\nGot:  %s\nWant: %s", final, test.Want)
			}
		})
	}
}

func TestIterateDir(t *testing.T) {
	tests := []struct {
		Name   string
		FS     fs.FS
		Dir    string
		Prefix string
		Ignore []string
		Want   []testEntry
	}{
		{
			Name: "small",
			FS:   vdmtest.TxtarFS(t, "testdata/small.txtar"),
			Dir:  "foo",
			Want: []testEntry{
				{S: "foo/bar.txt"},
			},
		},
		{
			Name:   "subdirectories",
			FS:     vdmtest.TxtarFS(t, "testdata/subdirectories.txtar"),
			Dir:    "foo/bar",
			Prefix: "foo/",
			Ignore: []string{
				"foo/bar/ignored.txt",
			},
			Want: []testEntry{
				{S: "bar/a/b/c.txt"},
				{S: "bar/baz.txt"},
			},
		},
		{
			Name: "errors",
			FS:   vdmtest.TestFS(t, vdmtest.WithErrors(".", "file does not exist")),
			Dir:  ".",
			Ignore: []string{
				"read",
				"close",
			},
			Want: []testEntry{
				{E: errors.New("file does not exist")},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			// Do the entries as a group.
			got := collect(iterateDir(test.FS, test.Dir, test.Prefix, test.Ignore...))
			if diff := cmp.Diff(test.Want, got, compareErrors); diff != "" {
				t.Fatalf("iterateDir(): mismatch (-want, +got)\n%s", diff)
			}

			// Do the first individually.
			first := collectSingle(iterateDir(test.FS, test.Dir, test.Prefix, test.Ignore...))
			if diff := cmp.Diff(test.Want[0], first, compareErrors); diff != "" {
				t.Fatalf("iterateDir(first): mismatch (-want, +got)\n%s", diff)
			}
		})
	}
}

func TestDirectory(t *testing.T) {
	tests := []struct {
		Name   string
		FS     fs.FS
		Dir    string
		Ignore []string
		Want   string
		Error  string
	}{
		{
			Name:  "nonexistant",
			FS:    vdmtest.TxtarFS(t, "testdata/small.txtar"),
			Dir:   "bar",
			Error: "open bar: file does not exist",
		},
		{
			Name: "small",
			FS:   vdmtest.TxtarFS(t, "testdata/small.txtar"),
			Dir:  "foo",
			Want: "sha256:nXbUeK5RNiJs1oQQWJa3D5t0HpyVd6lqNMUcYaIS5hQ=",
		},
		{
			Name: "subdirectories",
			FS:   vdmtest.TxtarFS(t, "testdata/subdirectories.txtar"),
			Dir:  "foo/bar",
			Ignore: []string{
				"foo/bar/ignored.txt",
			},
			Want: "sha256:69LmaQn9kFxSV2OfnxzyvV4l8iKCB9EnSVuDkjTnn9I=",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got, err := Directory(test.FS, test.Dir, test.Ignore...)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("Directory(): unexpected lack of error")
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("Directory(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("Directory(): %v", err)
			}

			if got != test.Want {
				t.Fatalf("Directory(): digest mismatch:\nGot:  %s\nWant: %s", got, test.Want)
			}
		})
	}
}

func TestLines(t *testing.T) {
	tests := []struct {
		Name  string
		Lines []string
		Want  string
		Error string
	}{
		{
			Name: "invalid-newline",
			Lines: []string{
				"foo",
				"b\nar",
				"baz",
			},
			Error: `lines with newlines are not allowed: found "b\nar"`,
		},
		{
			Name: "valid",
			Lines: []string{
				"foo",
				"bar",
				"baz",
			},
			Want: func() string {
				sum := sha256.Sum256([]byte("foo\nbar\nbaz\n"))
				return "sha256:" + base64.StdEncoding.EncodeToString(sum[:])
			}(),
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got, err := Lines(slices.Values(test.Lines))
			if test.Error != "" {
				if err == nil {
					t.Fatalf("Lines(): unexpected lack of error")
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("Lines(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("Lines(): %v", err)
			}

			if got != test.Want {
				t.Fatalf("Lines(): digest mismatch:\nGot:  %s\nWant: %s", got, test.Want)
			}
		})
	}
}

// The error types we're using have unexported
// fields. Annoyingly, the least clumsy fix is
// to write a custom comparer that just looks
// at the error message.
var compareErrors = cmp.Comparer(func(a, b error) bool {
	if a == nil && b == nil {
		return true
	}

	if a == nil || b == nil {
		return false
	}

	return a.Error() == b.Error()
})

type testEntry struct {
	S string
	E error
}

func iterate(entries []testEntry) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		for _, entry := range entries {
			if !yield(entry.S, entry.E) {
				return
			}
		}
	}
}

func iterateSingle(entry testEntry) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		yield(entry.S, entry.E)
	}
}

func collect(entries iter.Seq2[string, error]) []testEntry {
	var out []testEntry
	for s, e := range entries {
		out = append(out, testEntry{S: s, E: e})
	}

	return out
}

func collectSingle(entries iter.Seq2[string, error]) testEntry {
	for s, e := range entries {
		return testEntry{S: s, E: e}
	}

	panic("got no values from iterator")
}
