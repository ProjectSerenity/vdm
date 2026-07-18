// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package vdmtest

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"testing"
	"testing/iotest"
	"time"
)

// DirFS returns an [os.DirFS] for the
// test's [testing.T.ArtifactDir].
func DirFS(t *testing.T) fs.FS {
	return os.DirFS(t.ArtifactDir())
}

// ErrReader returns an [io.Reader] that
// returns 0, errors.New(msg) from all
// Read calls.
func ErrReader(msg string) io.Reader {
	return iotest.ErrReader(errors.New(msg))
}

type testFile struct {
	stat      fs.FileInfo
	statError error
	read      io.Reader
	close     error
}

var _ fs.File = (*testFile)(nil)

func (t *testFile) Stat() (fs.FileInfo, error) { return t.stat, t.statError }
func (t *testFile) Read(b []byte) (int, error) { return t.read.Read(b) }
func (t *testFile) Close() error               { return t.close }

// TestFile can be used to implement a configurable test file. TestFile
// is designed to be used in tests to check that code handles unexpected
// behaviour from a file correctly. For example, it can be used to return
// errors or synthetic data from certain operations.
func TestFile(stat fs.FileInfo, statError error, read io.Reader, close error) fs.File {
	f := &testFile{
		stat:      stat,
		statError: statError,
		read:      read,
		close:     close,
	}

	if f.read == nil {
		f.read = ErrReader("Read not configured")
	}

	return f
}

type testFileInfo struct {
	name    string
	size    int64
	mode    fs.FileMode
	modTime time.Time
	isDir   bool
	sys     any
}

var _ fs.FileInfo = (*testFileInfo)(nil)

func (i *testFileInfo) Name() string       { return i.name }
func (i *testFileInfo) Size() int64        { return i.size }
func (i *testFileInfo) Mode() fs.FileMode  { return i.mode }
func (i *testFileInfo) ModTime() time.Time { return i.modTime }
func (i *testFileInfo) IsDir() bool        { return i.isDir }
func (i *testFileInfo) Sys() any           { return i.sys }

// TestFileInfo contains the information returned by an [fs.FileInfo],
// which simplifies the production of test responses.
func TestFileInfo(name string, size int64, mode fs.FileMode, modTime time.Time, isDir bool, sys any) fs.FileInfo {
	return &testFileInfo{
		name:    name,
		size:    size,
		mode:    mode,
		modTime: modTime,
		isDir:   isDir,
		sys:     sys,
	}
}

type testDirEntry struct {
	name      string
	isDir     bool
	typ       fs.FileMode
	info      fs.FileInfo
	infoError error
}

var _ fs.DirEntry = (*testDirEntry)(nil)

func (t *testDirEntry) Name() string               { return t.name }
func (t *testDirEntry) IsDir() bool                { return t.isDir }
func (t *testDirEntry) Type() fs.FileMode          { return t.typ }
func (t *testDirEntry) Info() (fs.FileInfo, error) { return t.info, t.infoError }

// TestDirEntry contains the information returned by an [fs.DirEntry],
// which simplifies the production of test responses.
func TestDirEntry(name string, isDir bool, typ fs.FileMode, info fs.FileInfo, infoError error) fs.DirEntry {
	return &testDirEntry{
		name:      name,
		isDir:     isDir,
		typ:       typ,
		info:      info,
		infoError: infoError,
	}
}

type testFS struct {
	baseError      error
	errors         map[string]error
	files          map[string]fs.File
	openErrors     map[string]error
	glob           map[string][]string
	globErrors     map[string]error
	readDir        map[string][]fs.DirEntry
	readDirErrors  map[string]error
	readFile       map[string][]byte
	readFileErrors map[string]error
	readLink       map[string]string
	readLinkErrors map[string]error
	lstat          map[string]fs.FileInfo
	lstatErrors    map[string]error
	stat           map[string]fs.FileInfo
	statErrors     map[string]error
	sub            map[string]fs.FS
	subErrors      map[string]error
}

var (
	_ fs.FS         = (*testFS)(nil)
	_ fs.GlobFS     = (*testFS)(nil)
	_ fs.ReadDirFS  = (*testFS)(nil)
	_ fs.ReadFileFS = (*testFS)(nil)
	_ fs.ReadLinkFS = (*testFS)(nil)
	_ fs.StatFS     = (*testFS)(nil)
	_ fs.SubFS      = (*testFS)(nil)
)

// TestFS can be used to implement a configurable test filesystem. TestFS
// is designed to be used in tests to check that code handles unexpected
// behaviour from a filesystem correctly. For example, it can be used to
// return errors or synthetic data from certain operations.
//
// testFS implements all the interfaces in package [io/fs], including:
//
//   - [fs.FS]
//   - [fs.GlobFS]
//   - [fs.ReadDirFS]
//   - [fs.ReadFileFS]
//   - [fs.ReadLinkFS]
//   - [fs.StatFS]
//   - [fs.SubFS]
func TestFS(tb testing.TB, options ...Option) fs.FS {
	fsys, err := newTestFS(options...)
	if err != nil {
		tb.Helper()
		tb.Fatal(err)
	}

	return fsys
}

func newTestFS(options ...Option) (*testFS, error) {
	fsys := &testFS{}
	for _, option := range options {
		err := option(fsys)
		if err != nil {
			return nil, err
		}
	}

	return fsys, nil
}

func (fsys *testFS) err(special map[string]error, name string) error {
	err, ok := special[name]
	if !ok {
		err, ok = fsys.errors[name]
	}

	if !ok {
		err = fsys.baseError
	}

	return err
}

func (fsys *testFS) Open(name string) (fs.File, error) {
	file := fsys.files[name]
	err := fsys.err(fsys.openErrors, name)

	return file, err
}

func (fsys *testFS) Glob(pattern string) ([]string, error) {
	entries := slices.Clone(fsys.glob[pattern])
	err := fsys.err(fsys.globErrors, pattern)

	return entries, err
}

func (fsys *testFS) ReadDir(name string) ([]fs.DirEntry, error) {
	entries := fsys.readDir[name]
	err := fsys.err(fsys.readDirErrors, name)

	return entries, err
}

func (fsys *testFS) ReadFile(name string) ([]byte, error) {
	data := fsys.readFile[name]
	err := fsys.err(fsys.readFileErrors, name)

	return data, err
}

func (fsys *testFS) ReadLink(name string) (string, error) {
	link := fsys.readLink[name]
	err := fsys.err(fsys.readLinkErrors, name)

	return link, err
}

func (fsys *testFS) Lstat(name string) (fs.FileInfo, error) {
	info := fsys.lstat[name]
	err := fsys.err(fsys.lstatErrors, name)

	return info, err
}

func (fsys *testFS) Stat(name string) (fs.FileInfo, error) {
	info := fsys.stat[name]
	err := fsys.err(fsys.statErrors, name)

	return info, err
}

func (fsys *testFS) Sub(dir string) (fs.FS, error) {
	out := fsys.sub[dir]
	err := fsys.err(fsys.subErrors, dir)

	return out, err
}

// Option is a function that configures the behaviour
// of a [TestFS].
type Option func(*testFS) error

// WithError configures the filesystem to use the given
// error in response to any operations. If an error is
// also configured using a more specific method error,
// then that error is returned instead.
//
// For example, an error configured using [WithErrors]
// or [WithReadDirErrors] will take precedence for calls
// to [fs.ReadDir].
func WithError(err error) Option {
	return func(fsys *testFS) error {
		if fsys.baseError != nil {
			return fmt.Errorf("WithError provided more than once for a single filesystem")
		}

		fsys.baseError = err

		return nil
	}
}

// WithErrors configures the filesystem to use the given
// error in response to any operation using the given
// name. If an error is also configured using a more
// specific method error, then that error is returned
// instead.
//
// For example, an error configured using [WithReadDirErrors]
// will take precedence for calls to [fs.ReadDir].
//
// The arguments consist of one or more pairs of strings.
// Within each pair, the first string is the name of the
// path that should return an error. The second string is
// the error that will be returned.
func WithErrors(pairs ...string) Option {
	return func(fsys *testFS) error {
		if fsys.errors != nil {
			return fmt.Errorf("WithErrors provided more than once for a single filesystem")
		}

		if len(pairs) == 0 {
			return fmt.Errorf("WithErrors given no errors")
		}

		if len(pairs)%2 != 0 {
			return fmt.Errorf("WithErrors given an incomplete pair")
		}

		fsys.errors = make(map[string]error, len(pairs)/2)
		for i := 0; i < len(pairs); i += 2 {
			fsys.errors[pairs[i+0]] = errors.New(pairs[i+1])
		}

		return nil
	}
}

// WithFiles configures the filesystem to use the given
// files in responses to [fs.FS.Open].
func WithFiles(files map[string]fs.File) Option {
	return func(fsys *testFS) error {
		if fsys.files != nil {
			return fmt.Errorf("WithFiles provided more than once for a single filesystem")
		}

		fsys.files = files
		return nil
	}
}

// WithOpenErrors configures the filesystem to use the
// given errors in responses to [fs.FS.Open].
//
// The arguments consist of one or more pairs of strings.
// Within each pair, the first string is the name of the
// path that should return an error. The second string is
// the error that will be returned.
func WithOpenErrors(pairs ...string) Option {
	return func(fsys *testFS) error {
		if fsys.openErrors != nil {
			return fmt.Errorf("WithOpenErrors provided more than once for a single filesystem")
		}

		if len(pairs) == 0 {
			return fmt.Errorf("WithOpenErrors given no errors")
		}

		if len(pairs)%2 != 0 {
			return fmt.Errorf("WithOpenErrors given an incomplete pair")
		}

		fsys.openErrors = make(map[string]error, len(pairs)/2)
		for i := 0; i < len(pairs); i += 2 {
			fsys.openErrors[pairs[i*2+0]] = errors.New(pairs[i*2+1])
		}

		return nil
	}
}

// WithGlob configures the filesystem to use the
// given glob in responses to [fs.Glob].
func WithGlob(glob map[string][]string) Option {
	return func(fsys *testFS) error {
		if fsys.glob != nil {
			return fmt.Errorf("WithGlob provided more than once for a single filesystem")
		}

		fsys.glob = glob
		return nil
	}
}

// WithGlobErrors configures the filesystem to use the
// given errors in responses to [fs.Glob].
//
// The arguments consist of one or more pairs of strings.
// Within each pair, the first string is the name of the
// path that should return an error. The second string is
// the error that will be returned.
func WithGlobErrors(pairs ...string) Option {
	return func(fsys *testFS) error {
		if fsys.globErrors != nil {
			return fmt.Errorf("WithGlobErrors provided more than once for a single filesystem")
		}

		if len(pairs) == 0 {
			return fmt.Errorf("WithGlobErrors given no errors")
		}

		if len(pairs)%2 != 0 {
			return fmt.Errorf("WithGlobErrors given an incomplete pair")
		}

		fsys.globErrors = make(map[string]error, len(pairs)/2)
		for i := 0; i < len(pairs); i += 2 {
			fsys.globErrors[pairs[i*2+0]] = errors.New(pairs[i*2+1])
		}

		return nil
	}
}

// WithReadDir configures the filesystem to use the
// given directory entries in responses to [fs.ReadDir].
func WithReadDir(readDir map[string][]fs.DirEntry) Option {
	return func(fsys *testFS) error {
		if fsys.readDir != nil {
			return fmt.Errorf("WithReadDir provided more than once for a single filesystem")
		}

		fsys.readDir = readDir
		return nil
	}
}

// WithReadDirErrors configures the filesystem to use the
// given errors in responses to [fs.ReadDir].
//
// The arguments consist of one or more pairs of strings.
// Within each pair, the first string is the name of the
// path that should return an error. The second string is
// the error that will be returned.
func WithReadDirErrors(pairs ...string) Option {
	return func(fsys *testFS) error {
		if fsys.readDirErrors != nil {
			return fmt.Errorf("WithReadDirErrors provided more than once for a single filesystem")
		}

		if len(pairs) == 0 {
			return fmt.Errorf("WithReadDirErrors given no errors")
		}

		if len(pairs)%2 != 0 {
			return fmt.Errorf("WithReadDirErrors given an incomplete pair")
		}

		fsys.readDirErrors = make(map[string]error, len(pairs)/2)
		for i := 0; i < len(pairs); i += 2 {
			fsys.readDirErrors[pairs[i*2+0]] = errors.New(pairs[i*2+1])
		}

		return nil
	}
}

// WithReadFile configures the filesystem to use the
// given file data in responses to [fs.ReadFile].
func WithReadFile(readFile map[string][]byte) Option {
	return func(fsys *testFS) error {
		if fsys.readFile != nil {
			return fmt.Errorf("WithReadFile provided more than once for a single filesystem")
		}

		fsys.readFile = readFile
		return nil
	}
}

// WithReadFileErrors configures the filesystem to use the
// given errors in responses to [fs.ReadFile].
//
// The arguments consist of one or more pairs of strings.
// Within each pair, the first string is the name of the
// path that should return an error. The second string is
// the error that will be returned.
func WithReadFileErrors(pairs ...string) Option {
	return func(fsys *testFS) error {
		if fsys.readFileErrors != nil {
			return fmt.Errorf("WithReadFileErrors provided more than once for a single filesystem")
		}

		if len(pairs) == 0 {
			return fmt.Errorf("WithReadFileErrors given no errors")
		}

		if len(pairs)%2 != 0 {
			return fmt.Errorf("WithReadFileErrors given an incomplete pair")
		}

		fsys.readFileErrors = make(map[string]error, len(pairs)/2)
		for i := 0; i < len(pairs); i += 2 {
			fsys.readFileErrors[pairs[i*2+0]] = errors.New(pairs[i*2+1])
		}

		return nil
	}
}

// WithReadLink configures the filesystem to use the
// given link path in responses to [fs.ReadLink].
func WithReadLink(readLink map[string]string) Option {
	return func(fsys *testFS) error {
		if fsys.readLink != nil {
			return fmt.Errorf("WithReadLink provided more than once for a single filesystem")
		}

		fsys.readLink = readLink
		return nil
	}
}

// WithReadLinkErrors configures the filesystem to use the
// given errors in responses to [fs.ReadLink].
//
// The arguments consist of one or more pairs of strings.
// Within each pair, the first string is the name of the
// path that should return an error. The second string is
// the error that will be returned.
func WithReadLinkErrors(pairs ...string) Option {
	return func(fsys *testFS) error {
		if fsys.readLinkErrors != nil {
			return fmt.Errorf("WithReadLinkErrors provided more than once for a single filesystem")
		}

		if len(pairs) == 0 {
			return fmt.Errorf("WithReadLinkErrors given no errors")
		}

		if len(pairs)%2 != 0 {
			return fmt.Errorf("WithReadLinkErrors given an incomplete pair")
		}

		fsys.readLinkErrors = make(map[string]error, len(pairs)/2)
		for i := 0; i < len(pairs); i += 2 {
			fsys.readLinkErrors[pairs[i*2+0]] = errors.New(pairs[i*2+1])
		}

		return nil
	}
}

// WithLstat configures the filesystem to use the
// given link info in responses to [fs.Lstat].
func WithLstat(lstat map[string]fs.FileInfo) Option {
	return func(fsys *testFS) error {
		if fsys.lstat != nil {
			return fmt.Errorf("WithLstat provided more than once for a single filesystem")
		}

		fsys.lstat = lstat
		return nil
	}
}

// WithLstatErrors configures the filesystem to use the
// given errors in responses to [fs.Lstat].
//
// The arguments consist of one or more pairs of strings.
// Within each pair, the first string is the name of the
// path that should return an error. The second string is
// the error that will be returned.
func WithLstatErrors(pairs ...string) Option {
	return func(fsys *testFS) error {
		if fsys.lstatErrors != nil {
			return fmt.Errorf("WithLstatErrors provided more than once for a single filesystem")
		}

		if len(pairs) == 0 {
			return fmt.Errorf("WithLstatErrors given no errors")
		}

		if len(pairs)%2 != 0 {
			return fmt.Errorf("WithLstatErrors given an incomplete pair")
		}

		fsys.lstatErrors = make(map[string]error, len(pairs)/2)
		for i := 0; i < len(pairs); i += 2 {
			fsys.lstatErrors[pairs[i*2+0]] = errors.New(pairs[i*2+1])
		}

		return nil
	}
}

// WithStat configures the filesystem to use the
// given file info in responses to [fs.Stat].
func WithStat(stat map[string]fs.FileInfo) Option {
	return func(fsys *testFS) error {
		if fsys.stat != nil {
			return fmt.Errorf("WithStat provided more than once for a single filesystem")
		}

		fsys.stat = stat
		return nil
	}
}

// WithStatErrors configures the filesystem to use the
// given errors in responses to [fs.Stat].
//
// The arguments consist of one or more pairs of strings.
// Within each pair, the first string is the name of the
// path that should return an error. The second string is
// the error that will be returned.
func WithStatErrors(pairs ...string) Option {
	return func(fsys *testFS) error {
		if fsys.statErrors != nil {
			return fmt.Errorf("WithStatErrors provided more than once for a single filesystem")
		}

		if len(pairs) == 0 {
			return fmt.Errorf("WithStatErrors given no errors")
		}

		if len(pairs)%2 != 0 {
			return fmt.Errorf("WithStatErrors given an incomplete pair")
		}

		fsys.statErrors = make(map[string]error, len(pairs)/2)
		for i := 0; i < len(pairs); i += 2 {
			fsys.statErrors[pairs[i*2+0]] = errors.New(pairs[i*2+1])
		}

		return nil
	}
}

// WithSub configures the filesystem to use the
// given filesystem in responses to [fs.Sub].
func WithSub(sub map[string]fs.FS) Option {
	return func(fsys *testFS) error {
		if fsys.sub != nil {
			return fmt.Errorf("WithSub provided more than once for a single filesystem")
		}

		fsys.sub = sub
		return nil
	}
}

// WithSubErrors configures the filesystem to use the
// given errors in responses to [fs.Sub].
//
// The arguments consist of one or more pairs of strings.
// Within each pair, the first string is the name of the
// path that should return an error. The second string is
// the error that will be returned.
func WithSubErrors(pairs ...string) Option {
	return func(fsys *testFS) error {
		if fsys.subErrors != nil {
			return fmt.Errorf("WithSubErrors provided more than once for a single filesystem")
		}

		if len(pairs) == 0 {
			return fmt.Errorf("WithSubErrors given no errors")
		}

		if len(pairs)%2 != 0 {
			return fmt.Errorf("WithSubErrors given an incomplete pair")
		}

		fsys.subErrors = make(map[string]error, len(pairs)/2)
		for i := 0; i < len(pairs); i += 2 {
			fsys.subErrors[pairs[i*2+0]] = errors.New(pairs[i*2+1])
		}

		return nil
	}
}
