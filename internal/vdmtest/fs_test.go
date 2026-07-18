// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package vdmtest

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestTestFS_nonerrors(t *testing.T) {
	tests := []struct {
		Name  string
		Opt   Option
		Error string
	}{
		{"WithError", WithError(io.EOF), "WithError provided more than once for a single filesystem"},
		{"WithFiles", WithFiles(map[string]fs.File{}), "WithFiles provided more than once for a single filesystem"},
		{"WithGlob", WithGlob(map[string][]string{}), "WithGlob provided more than once for a single filesystem"},
		{"WithReadDir", WithReadDir(map[string][]fs.DirEntry{}), "WithReadDir provided more than once for a single filesystem"},
		{"WithReadFile", WithReadFile(map[string][]byte{}), "WithReadFile provided more than once for a single filesystem"},
		{"WithReadLink", WithReadLink(map[string]string{}), "WithReadLink provided more than once for a single filesystem"},
		{"WithLstat", WithLstat(map[string]fs.FileInfo{}), "WithLstat provided more than once for a single filesystem"},
		{"WithStat", WithStat(map[string]fs.FileInfo{}), "WithStat provided more than once for a single filesystem"},
		{"WithSub", WithSub(map[string]fs.FS{}), "WithSub provided more than once for a single filesystem"},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			t.Run("invalid-repeated", func(t *testing.T) {
				_, err := newTestFS(test.Opt, test.Opt)
				if test.Error != "" {
					if err == nil {
						t.Fatalf("newTestFS(): unexpected lack of error")
					}

					e := err.Error()
					if e != test.Error {
						t.Fatalf("newTestFS(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
					}

					// All good.
					return
				}

				if err != nil {
					t.Fatalf("newTestFS(): got unexpected error: %v", err)
				}
			})
			t.Run("valid", func(t *testing.T) {
				_, err := newTestFS(test.Opt)
				if err != nil {
					t.Fatalf("newTestFS(): got unexpected error: %v", err)
				}
			})
		})
	}
}

func TestTestFS_errors(t *testing.T) {
	tests := []struct {
		Name string
		Opt  func(...string) Option
	}{
		{"WithErrors", WithErrors},
		{"WithOpenErrors", WithOpenErrors},
		{"WithGlobErrors", WithGlobErrors},
		{"WithReadDirErrors", WithReadDirErrors},
		{"WithReadFileErrors", WithReadFileErrors},
		{"WithReadLinkErrors", WithReadLinkErrors},
		{"WithLstatErrors", WithLstatErrors},
		{"WithStatErrors", WithStatErrors},
		{"WithSubErrors", WithSubErrors},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			t.Run("invalid-repeated", func(t *testing.T) {
				_, err := newTestFS(test.Opt("one", "one error"), test.Opt("two", "two error"))
				want := fmt.Sprintf("%s provided more than once for a single filesystem", test.Name)
				if err == nil {
					t.Fatalf("newTestFS(): unexpected lack of error")
				}

				e := err.Error()
				if e != want {
					t.Fatalf("newTestFS(): got wrong error:\nGot:  %s\nWant: %s", e, want)
				}
			})
			t.Run("invalid-no-pairs", func(t *testing.T) {
				_, err := newTestFS(test.Opt())
				want := fmt.Sprintf("%s given no errors", test.Name)
				if err == nil {
					t.Fatalf("newTestFS(): unexpected lack of error")
				}

				e := err.Error()
				if e != want {
					t.Fatalf("newTestFS(): got wrong error:\nGot:  %s\nWant: %s", e, want)
				}
			})
			t.Run("invalid-incomplete-pairs", func(t *testing.T) {
				_, err := newTestFS(test.Opt("one"))
				want := fmt.Sprintf("%s given an incomplete pair", test.Name)
				if err == nil {
					t.Fatalf("newTestFS(): unexpected lack of error")
				}

				e := err.Error()
				if e != want {
					t.Fatalf("newTestFS(): got wrong error:\nGot:  %s\nWant: %s", e, want)
				}
			})
			t.Run("valid", func(t *testing.T) {
				_, err := newTestFS(test.Opt("one", "one error"))
				if err != nil {
					t.Fatalf("newTestFS(): got unexpected error: %v", err)
				}
			})
		})
	}
}

var (
	compareOptions = []cmp.Option{
		// The error types we're using have unexported
		// fields. Annoyingly, the least clumsy fix is
		// to write a custom comparer that just looks
		// at the error message.
		cmp.Comparer(func(a, b error) bool {
			if a == nil && b == nil {
				return true
			}

			if a == nil || b == nil {
				return false
			}

			return a.Error() == b.Error()
		}),

		cmp.AllowUnexported(testFS{}, testFile{}, testFileInfo{}, testDirEntry{}, bytes.Reader{}, strings.Reader{}),
	}

	testTime = time.Date(2006, time.January, 2, 15, 4, 5, 0, time.UTC)
)

func TestTestFS_Open(t *testing.T) {
	tests := []struct {
		Name  string
		Opts  []Option
		Path  string
		Want  fs.File
		Error string

		// File fields.
		Stat       fs.FileInfo
		StatError  error
		Read       []byte
		ReadError  error
		CloseError error
	}{
		{
			Name: "generic-error-hit",
			Opts: []Option{
				WithErrors("foo", "foo error"),
			},
			Path:  "foo",
			Error: "foo error",
		},
		{
			Name: "generic-error-miss",
			Opts: []Option{
				WithErrors("foo", "foo error"),
			},
			Path: "bar",
		},
		{
			Name: "specific-error-hit",
			Opts: []Option{
				WithOpenErrors("foo", "foo error"),
			},
			Path:  "foo",
			Error: "foo error",
		},
		{
			Name: "specific-error-miss",
			Opts: []Option{
				WithOpenErrors("foo", "foo error"),
			},
			Path: "bar",
		},
		{
			Name: "file-only",
			Opts: []Option{
				WithFiles(map[string]fs.File{
					"foo": TestFile(
						TestFileInfo("foo", 7, 0o644, testTime, true, io.EOF),
						errors.New("stat error"),
						strings.NewReader("foo bar baz"),
						errors.New("close error"),
					),
				}),
			},
			Path: "foo",
			Want: TestFile(
				TestFileInfo("foo", 7, 0o644, testTime, true, io.EOF),
				errors.New("stat error"),
				strings.NewReader("foo bar baz"),
				errors.New("close error"),
			),
			Stat:       TestFileInfo("foo", 7, 0o644, testTime, true, io.EOF),
			StatError:  errors.New("stat error"),
			Read:       []byte("foo bar baz"),
			CloseError: errors.New("close error"),
		},
		{
			Name: "file-and-error",
			Opts: []Option{
				WithFiles(map[string]fs.File{
					"foo": TestFile(
						TestFileInfo("foo", 7, 0o644, testTime, true, io.EOF),
						errors.New("stat error"),
						nil,
						errors.New("close error"),
					),
				}),
				WithOpenErrors("foo", "foo error"),
			},
			Path:  "foo",
			Error: "foo error",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			fsys := TestFS(t, test.Opts...)
			got, err := fsys.Open(test.Path)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("TestFS.Open(%q): unexpected lack of error", test.Path)
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("TestFS.Open(%q): got wrong error:\nGot:  %s\nWant: %s", test.Path, e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("TestFS.Open(%q): got unexpected error: %v", test.Path, err)
			}

			if diff := cmp.Diff(test.Want, got, compareOptions...); diff != "" {
				t.Errorf("TestFS.Open(%q): result mismatch (-want, +got)\n%s", test.Path, diff)
			}

			if got == nil {
				return
			}

			info, err := got.Stat()
			if diff := cmp.Diff(test.Stat, info, compareOptions...); diff != "" {
				t.Errorf("TestFS.Open(%q).Stat(): FileInfo result mismatch (-want, +got)\n%s", test.Path, diff)
			}

			if diff := cmp.Diff(test.StatError, err, compareOptions...); diff != "" {
				t.Errorf("TestFS.Open(%q).Stat(): error result mismatch (-want, +got)\n%s", test.Path, diff)
			}

			data, err := io.ReadAll(got)
			if diff := cmp.Diff(test.Read, data, compareOptions...); diff != "" {
				t.Errorf("TestFS.Open(%q).Read(): data result mismatch (-want, +got)\n%s", test.Path, diff)
			}

			if diff := cmp.Diff(test.ReadError, err, compareOptions...); diff != "" {
				t.Errorf("TestFS.Open(%q).Read(): error result mismatch (-want, +got)\n%s", test.Path, diff)
			}

			err = got.Close()
			if diff := cmp.Diff(test.CloseError, err, compareOptions...); diff != "" {
				t.Errorf("TestFS.Open(%q).Close(): error result mismatch (-want, +got)\n%s", test.Path, diff)
			}
		})
	}
}

func TestTestFS_Glob(t *testing.T) {
	tests := []struct {
		Name  string
		Opts  []Option
		Path  string
		Want  []string
		Error string
	}{
		{
			Name: "generic-error-hit",
			Opts: []Option{
				WithErrors("foo", "foo error"),
			},
			Path:  "foo",
			Error: "foo error",
		},
		{
			Name: "generic-error-miss",
			Opts: []Option{
				WithErrors("foo", "foo error"),
			},
			Path: "bar",
		},
		{
			Name: "specific-error-hit",
			Opts: []Option{
				WithGlobErrors("foo", "foo error"),
			},
			Path:  "foo",
			Error: "foo error",
		},
		{
			Name: "specific-error-miss",
			Opts: []Option{
				WithGlobErrors("foo", "foo error"),
			},
			Path: "bar",
		},
		{
			Name: "strings-only",
			Opts: []Option{
				WithGlob(map[string][]string{
					"foo": {"one", "two", "three"},
				}),
			},
			Path: "foo",
			Want: []string{"one", "two", "three"},
		},
		{
			Name: "strings-and-error",
			Opts: []Option{
				WithGlob(map[string][]string{
					"foo": {"one", "two", "three"},
				}),
				WithGlobErrors("foo", "foo error"),
			},
			Path:  "foo",
			Error: "foo error",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			fsys := TestFS(t, test.Opts...).(fs.GlobFS)
			got, err := fsys.Glob(test.Path)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("TestFS.Glob(%q): unexpected lack of error", test.Path)
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("TestFS.Glob(%q): got wrong error:\nGot:  %s\nWant: %s", test.Path, e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("TestFS.Glob(%q): got unexpected error: %v", test.Path, err)
			}

			if diff := cmp.Diff(test.Want, got, compareOptions...); diff != "" {
				t.Errorf("TestFS.Glob(%q): result mismatch (-want, +got)\n%s", test.Path, diff)
			}
		})
	}
}

func TestTestFS_ReadDir(t *testing.T) {
	tests := []struct {
		Name  string
		Opts  []Option
		Path  string
		Want  []fs.DirEntry
		Error string

		// DirEntry fields.
		EntryName      string
		EntryIsDir     bool
		EntryType      fs.FileMode
		EntryInfo      fs.FileInfo
		EntryInfoError error
	}{
		{
			Name: "generic-error-hit",
			Opts: []Option{
				WithErrors("foo", "foo error"),
			},
			Path:  "foo",
			Error: "foo error",
		},
		{
			Name: "generic-error-miss",
			Opts: []Option{
				WithErrors("foo", "foo error"),
			},
			Path: "bar",
		},
		{
			Name: "specific-error-hit",
			Opts: []Option{
				WithReadDirErrors("foo", "foo error"),
			},
			Path:  "foo",
			Error: "foo error",
		},
		{
			Name: "specific-error-miss",
			Opts: []Option{
				WithReadDirErrors("foo", "foo error"),
			},
			Path: "bar",
		},
		{
			Name: "entries-only",
			Opts: []Option{
				WithReadDir(map[string][]fs.DirEntry{
					"foo": {
						TestDirEntry("first", true, 0o755, TestFileInfo("first", 7, 0o755, testTime, true, io.EOF), errors.New("info error")),
						TestDirEntry("second", false, 0o644, TestFileInfo("second", 7, 0o644, testTime, false, io.EOF), errors.New("info error")),
					},
				}),
			},
			Path: "foo",
			Want: []fs.DirEntry{
				TestDirEntry("first", true, 0o755, TestFileInfo("first", 7, 0o755, testTime, true, io.EOF), errors.New("info error")),
				TestDirEntry("second", false, 0o644, TestFileInfo("second", 7, 0o644, testTime, false, io.EOF), errors.New("info error")),
			},
			EntryName:      "first",
			EntryIsDir:     true,
			EntryType:      0o755,
			EntryInfo:      TestFileInfo("first", 7, 0o755, testTime, true, io.EOF),
			EntryInfoError: errors.New("info error"),
		},
		{
			Name: "entries-and-error",
			Opts: []Option{
				WithReadDir(map[string][]fs.DirEntry{
					"foo": {
						TestDirEntry("first", true, 0o755, TestFileInfo("first", 7, 0o755, testTime, true, io.EOF), errors.New("info error")),
						TestDirEntry("second", false, 0o644, TestFileInfo("second", 7, 0o644, testTime, false, io.EOF), errors.New("info error")),
					},
				}),
				WithReadDirErrors("foo", "foo error"),
			},
			Path:  "foo",
			Error: "foo error",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			fsys := TestFS(t, test.Opts...).(fs.ReadDirFS)
			got, err := fsys.ReadDir(test.Path)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("TestFS.ReadDir(%q): unexpected lack of error", test.Path)
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("TestFS.ReadDir(%q): got wrong error:\nGot:  %s\nWant: %s", test.Path, e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("TestFS.ReadDir(%q): got unexpected error: %v", test.Path, err)
			}

			if diff := cmp.Diff(test.Want, got, compareOptions...); diff != "" {
				t.Errorf("TestFS.ReadDir(%q): result mismatch (-want, +got)\n%s", test.Path, diff)
			}

			if len(got) < 1 {
				return
			}

			entry := got[0]
			entryName := entry.Name()
			if entryName != test.EntryName {
				t.Errorf("TestFS.ReadDir(%q)[0].Name(): name mismatch:\nGot:  %q\nWant: %q", test.Path, entryName, test.EntryName)
			}

			entryIsDir := entry.IsDir()
			if entryIsDir != test.EntryIsDir {
				t.Errorf("TestFS.ReadDir(%q)[0].IsDir(): is dir mismatch:\nGot:  %v\nWant: %v", test.Path, entryIsDir, test.EntryIsDir)
			}

			entryType := entry.Type()
			if entryType != test.EntryType {
				t.Errorf("TestFS.ReadDir(%q)[0].Type(): type mismatch:\nGot:  %s\nWant: %s", test.Path, entryType, test.EntryType)
			}

			info, err := entry.Info()
			if diff := cmp.Diff(test.EntryInfo, info, compareOptions...); diff != "" {
				t.Errorf("TestFS.ReadDir(%q)[0].Info(): FileInfo result mismatch (-want, +got)\n%s", test.Path, diff)
			}

			if diff := cmp.Diff(test.EntryInfoError, err, compareOptions...); diff != "" {
				t.Errorf("TestFS.ReadDir(%q)[0].Info(): error result mismatch (-want, +got)\n%s", test.Path, diff)
			}
		})
	}
}

func TestTestFS_ReadFile(t *testing.T) {
	tests := []struct {
		Name  string
		Opts  []Option
		Path  string
		Want  []byte
		Error string
	}{
		{
			Name: "generic-error-hit",
			Opts: []Option{
				WithErrors("foo", "foo error"),
			},
			Path:  "foo",
			Error: "foo error",
		},
		{
			Name: "generic-error-miss",
			Opts: []Option{
				WithErrors("foo", "foo error"),
			},
			Path: "bar",
		},
		{
			Name: "specific-error-hit",
			Opts: []Option{
				WithReadFileErrors("foo", "foo error"),
			},
			Path:  "foo",
			Error: "foo error",
		},
		{
			Name: "specific-error-miss",
			Opts: []Option{
				WithReadFileErrors("foo", "foo error"),
			},
			Path: "bar",
		},
		{
			Name: "bytes-only",
			Opts: []Option{
				WithReadFile(map[string][]byte{
					"foo": []byte("bar"),
				}),
			},
			Path: "foo",
			Want: []byte("bar"),
		},
		{
			Name: "bytes-and-error",
			Opts: []Option{
				WithReadFile(map[string][]byte{
					"foo": []byte("bar"),
				}),
				WithReadFileErrors("foo", "foo error"),
			},
			Path:  "foo",
			Error: "foo error",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			fsys := TestFS(t, test.Opts...).(fs.ReadFileFS)
			got, err := fsys.ReadFile(test.Path)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("TestFS.ReadFile(%q): unexpected lack of error", test.Path)
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("TestFS.ReadFile(%q): got wrong error:\nGot:  %s\nWant: %s", test.Path, e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("TestFS.ReadFile(%q): got unexpected error: %v", test.Path, err)
			}

			if diff := cmp.Diff(test.Want, got, compareOptions...); diff != "" {
				t.Errorf("TestFS.ReadFile(%q): result mismatch (-want, +got)\n%s", test.Path, diff)
			}
		})
	}
}

func TestTestFS_ReadLink(t *testing.T) {
	tests := []struct {
		Name  string
		Opts  []Option
		Path  string
		Want  string
		Error string
	}{
		{
			Name: "generic-error-hit",
			Opts: []Option{
				WithErrors("foo", "foo error"),
			},
			Path:  "foo",
			Error: "foo error",
		},
		{
			Name: "generic-error-miss",
			Opts: []Option{
				WithErrors("foo", "foo error"),
			},
			Path: "bar",
		},
		{
			Name: "specific-error-hit",
			Opts: []Option{
				WithReadLinkErrors("foo", "foo error"),
			},
			Path:  "foo",
			Error: "foo error",
		},
		{
			Name: "specific-error-miss",
			Opts: []Option{
				WithReadLinkErrors("foo", "foo error"),
			},
			Path: "bar",
		},
		{
			Name: "string-only",
			Opts: []Option{
				WithReadLink(map[string]string{
					"foo": "bar",
				}),
			},
			Path: "foo",
			Want: "bar",
		},
		{
			Name: "string-and-error",
			Opts: []Option{
				WithReadLink(map[string]string{
					"foo": "bar",
				}),
				WithReadLinkErrors("foo", "foo error"),
			},
			Path:  "foo",
			Error: "foo error",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			fsys := TestFS(t, test.Opts...).(fs.ReadLinkFS)
			got, err := fsys.ReadLink(test.Path)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("TestFS.ReadLink(%q): unexpected lack of error", test.Path)
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("TestFS.ReadLink(%q): got wrong error:\nGot:  %s\nWant: %s", test.Path, e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("TestFS.ReadLink(%q): got unexpected error: %v", test.Path, err)
			}

			if diff := cmp.Diff(test.Want, got, compareOptions...); diff != "" {
				t.Errorf("TestFS.ReadLink(%q): result mismatch (-want, +got)\n%s", test.Path, diff)
			}
		})
	}
}

func TestTestFS_Lstat(t *testing.T) {
	tests := []struct {
		Name  string
		Opts  []Option
		Path  string
		Want  fs.FileInfo
		Error string
	}{
		{
			Name: "generic-error-hit",
			Opts: []Option{
				WithErrors("foo", "foo error"),
			},
			Path:  "foo",
			Error: "foo error",
		},
		{
			Name: "generic-error-miss",
			Opts: []Option{
				WithErrors("foo", "foo error"),
			},
			Path: "bar",
		},
		{
			Name: "specific-error-hit",
			Opts: []Option{
				WithLstatErrors("foo", "foo error"),
			},
			Path:  "foo",
			Error: "foo error",
		},
		{
			Name: "specific-error-miss",
			Opts: []Option{
				WithLstatErrors("foo", "foo error"),
			},
			Path: "bar",
		},
		{
			Name: "info-only",
			Opts: []Option{
				WithLstat(map[string]fs.FileInfo{
					"foo": TestFileInfo("foo", 7, 0o644, testTime, true, io.EOF),
				}),
			},
			Path: "foo",
			Want: TestFileInfo("foo", 7, 0o644, testTime, true, io.EOF),
		},
		{
			Name: "info-and-error",
			Opts: []Option{
				WithLstat(map[string]fs.FileInfo{
					"foo": TestFileInfo("foo", 7, 0o644, testTime, true, io.EOF),
				}),
				WithLstatErrors("foo", "foo error"),
			},
			Path:  "foo",
			Error: "foo error",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			fsys := TestFS(t, test.Opts...).(fs.ReadLinkFS)
			got, err := fsys.Lstat(test.Path)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("TestFS.Lstat(%q): unexpected lack of error", test.Path)
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("TestFS.Lstat(%q): got wrong error:\nGot:  %s\nWant: %s", test.Path, e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("TestFS.Lstat(%q): got unexpected error: %v", test.Path, err)
			}

			if diff := cmp.Diff(test.Want, got, compareOptions...); diff != "" {
				t.Errorf("TestFS.Lstat(%q): result mismatch (-want, +got)\n%s", test.Path, diff)
			}
		})
	}
}

func TestTestFS_Stat(t *testing.T) {
	tests := []struct {
		Name  string
		Opts  []Option
		Path  string
		Want  fs.FileInfo
		Error string

		// FileInfo fields.
		InfoName    string
		InfoSize    int64
		InfoMode    fs.FileMode
		InfoModTime time.Time
		InfoIsDir   bool
		InfoSys     any
	}{
		{
			Name: "generic-error-hit",
			Opts: []Option{
				WithErrors("foo", "foo error"),
			},
			Path:  "foo",
			Error: "foo error",
		},
		{
			Name: "generic-error-miss",
			Opts: []Option{
				WithErrors("foo", "foo error"),
			},
			Path: "bar",
		},
		{
			Name: "specific-error-hit",
			Opts: []Option{
				WithStatErrors("foo", "foo error"),
			},
			Path:  "foo",
			Error: "foo error",
		},
		{
			Name: "specific-error-miss",
			Opts: []Option{
				WithStatErrors("foo", "foo error"),
			},
			Path: "bar",
		},
		{
			Name: "info-only",
			Opts: []Option{
				WithStat(map[string]fs.FileInfo{
					"foo": TestFileInfo("foo", 7, 0o644, testTime, true, io.EOF),
				}),
			},
			Path:        "foo",
			Want:        TestFileInfo("foo", 7, 0o644, testTime, true, io.EOF),
			InfoName:    "foo",
			InfoSize:    7,
			InfoMode:    0o644,
			InfoModTime: testTime,
			InfoIsDir:   true,
			InfoSys:     io.EOF,
		},
		{
			Name: "info-and-error",
			Opts: []Option{
				WithStat(map[string]fs.FileInfo{
					"foo": TestFileInfo("foo", 7, 0o644, testTime, true, io.EOF),
				}),
				WithStatErrors("foo", "foo error"),
			},
			Path:        "foo",
			Error:       "foo error",
			InfoName:    "foo",
			InfoSize:    7,
			InfoMode:    0o644,
			InfoModTime: testTime,
			InfoIsDir:   true,
			InfoSys:     io.EOF,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			fsys := TestFS(t, test.Opts...).(fs.StatFS)
			got, err := fsys.Stat(test.Path)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("TestFS.Stat(%q): unexpected lack of error", test.Path)
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("TestFS.Stat(%q): got wrong error:\nGot:  %s\nWant: %s", test.Path, e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("TestFS.Stat(%q): got unexpected error: %v", test.Path, err)
			}

			if diff := cmp.Diff(test.Want, got, compareOptions...); diff != "" {
				t.Errorf("TestFS.Stat(%q): result mismatch (-want, +got)\n%s", test.Path, diff)
			}

			if got == nil {
				return
			}

			infoName := got.Name()
			if infoName != test.InfoName {
				t.Errorf("TestFS.Stat(%q).Name(): name mismatch:\nGot:  %q\nWant: %q", test.Path, infoName, test.InfoName)
			}

			infoSize := got.Size()
			if infoSize != test.InfoSize {
				t.Errorf("TestFS.Stat(%q).Size(): size mismatch:\nGot:  %d\nWant: %d", test.Path, infoSize, test.InfoSize)
			}

			infoMode := got.Mode()
			if infoMode != test.InfoMode {
				t.Errorf("TestFS.Stat(%q).Mode(): mode mismatch:\nGot:  %s\nWant: %s", test.Path, infoMode, test.InfoMode)
			}

			infoModTime := got.ModTime()
			if infoModTime != test.InfoModTime {
				t.Errorf("TestFS.Stat(%q).ModTime(): mod time mismatch:\nGot:  %s\nWant: %s", test.Path, infoModTime, test.InfoModTime)
			}

			infoIsDir := got.IsDir()
			if infoIsDir != test.InfoIsDir {
				t.Errorf("TestFS.Stat(%q).IsDir(): is dir mismatch:\nGot:  %v\nWant: %v", test.Path, infoIsDir, test.InfoIsDir)
			}

			infoSys := got.Sys()
			if diff := cmp.Diff(test.InfoSys, infoSys, compareOptions...); diff != "" {
				t.Errorf("TestFS.Stat(%q).Sys(): result mismatch (-want, +got)\n%s", test.Path, diff)
			}
		})
	}
}

func TestTestFS_Sub(t *testing.T) {
	tests := []struct {
		Name  string
		Opts  []Option
		Path  string
		Want  fs.FS
		Error string
	}{
		{
			Name: "generic-error-hit",
			Opts: []Option{
				WithErrors("foo", "foo error"),
			},
			Path:  "foo",
			Error: "foo error",
		},
		{
			Name: "generic-error-miss",
			Opts: []Option{
				WithErrors("foo", "foo error"),
			},
			Path: "bar",
		},
		{
			Name: "specific-error-hit",
			Opts: []Option{
				WithSubErrors("foo", "foo error"),
			},
			Path:  "foo",
			Error: "foo error",
		},
		{
			Name: "specific-error-miss",
			Opts: []Option{
				WithSubErrors("foo", "foo error"),
			},
			Path: "bar",
		},
		{
			Name: "fs-only",
			Opts: []Option{
				WithSub(map[string]fs.FS{
					"foo": TestFS(t),
				}),
			},
			Path: "foo",
			Want: TestFS(t),
		},
		{
			Name: "fs-and-error",
			Opts: []Option{
				WithSub(map[string]fs.FS{
					"foo": TestFS(t),
				}),
				WithSubErrors("foo", "foo error"),
			},
			Path:  "foo",
			Error: "foo error",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			fsys := TestFS(t, test.Opts...).(fs.SubFS)
			got, err := fsys.Sub(test.Path)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("TestFS.Sub(%q): unexpected lack of error", test.Path)
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("TestFS.Sub(%q): got wrong error:\nGot:  %s\nWant: %s", test.Path, e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("TestFS.Sub(%q): got unexpected error: %v", test.Path, err)
			}

			if diff := cmp.Diff(test.Want, got, compareOptions...); diff != "" {
				t.Errorf("TestFS.Sub(%q): result mismatch (-want, +got)\n%s", test.Path, diff)
			}
		})
	}
}
