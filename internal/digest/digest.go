// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package digest

import (
	"crypto"
	_ "crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"io/fs"
	"iter"
	"path"
	"strings"
)

const (
	digestHash = crypto.SHA256
	digestName = "sha256"
)

// digestFile takes a filesystem, an optional path
// prefix, and an iterator of filenames and returns
// another iterator, which will yield either a digest
// for the previous filename or an error.
func digestFile(fsys fs.FS, prefix string, filenames iter.Seq2[string, error]) iter.Seq2[string, error] {
	hashBuf := make([]byte, digestHash.Size())
	return func(yield func(string, error) bool) {
		for filename, err := range filenames {
			if err != nil {
				if !yield("", err) {
					return
				}

				continue
			}

			if strings.Contains(filename, "\n") {
				if !yield("", fmt.Errorf("filenames with newlines are not allowed: found %q", prefix+filename)) {
					return
				}

				continue
			}

			f, err := fsys.Open(prefix + filename)
			if err != nil {
				if !yield("", fmt.Errorf("failed to open %s%s: %v", prefix, filename, err)) {
					return
				}

				continue
			}

			fh := digestHash.New()
			_, err = io.Copy(fh, f)
			if err != nil {
				if !yield("", fmt.Errorf("failed to read %s%s: %v", prefix, filename, err)) {
					return
				}

				continue
			}

			if err = f.Close(); err != nil {
				if !yield("", fmt.Errorf("failed to close %s%s: %v", prefix, filename, err)) {
					return
				}

				continue
			}

			if !yield(fmt.Sprintf("%x  %s\n", fh.Sum(hashBuf[:0]), filename), nil) {
				return
			}
		}
	}
}

// iterateFilenames is a simple iterator that returns
// each filename in sequence with a nil error.
func iterateFilenames(filenames []string) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		for _, filename := range filenames {
			if !yield(filename, nil) {
				return
			}
		}
	}
}

// Files produces the digest for a set of named
// files and their contents in a filesystem. This is
// performed by hashing one line of text for each file.
// Each line consists of the hexadecimal digest of the
// file's contents, two spaces (\x20), the relative
// filename, and a newline (\x0a).
//
// Filenames containing a newline (\x0a) are not allowed.
// The slice of filenames will be sorted to ensure consistent
// behaviour.
//
// The final digest is formatted as the hash algorithm
// name, a colon (\x3a), and the hexadecimal digest.
func Files(fsys fs.FS, filenames []string) (string, error) {
	h := digestHash.New()
	for line, err := range digestFile(fsys, "", iterateFilenames(filenames)) {
		if err != nil {
			return "", err
		}

		io.WriteString(h, line)
	}

	result := digestName + ":" + base64.StdEncoding.EncodeToString(h.Sum(nil))

	return result, nil
}

// iterateDir walks a directory, emitting the filenames
// below the given directory, ignoring any individual
// filenames specified. The prefix, if any, is trimmed
// from each filename.
func iterateDir(fsys fs.FS, dir, prefix string, ignore ...string) iter.Seq2[string, error] {
	// Make a lookup for the ignored files to reduce
	// the complexity.
	ignored := make(map[string]bool, len(ignore))
	for _, name := range ignore {
		ignored[name] = true
	}

	return func(yield func(string, error) bool) {
		// As we yield all errors, fs.WalkDir will
		// never return an error itself.
		fs.WalkDir(fsys, dir, func(name string, d fs.DirEntry, err error) error {
			if err != nil {
				if !yield("", err) {
					return fs.SkipAll
				}
			}

			if ignored[name] {
				return nil
			}

			if d != nil && !d.IsDir() {
				if !yield(strings.TrimPrefix(name, prefix), nil) {
					return fs.SkipAll
				}
			}

			return nil
		})
	}
}

// Directory produces the digest for a directory
// and its contents in a filesystem. This is performed
// by hashing one line of text for each file, with the
// files sorted into lexographical order. Each line
// consists of the hexadecimal digest of the file's
// contents, two spaces (\x20), the relative filename,
// and a newline (\x0a).
//
// Filenames containing a newline (\x0a) are not allowed.
//
// Any filenames listed in ignore are not included in
// the hashing process.
//
// The final digest is formatted as the hash algorithm
// name, a colon (\x3a), and the hexadecimal digest.
func Directory(fsys fs.FS, dir string, ignore ...string) (string, error) {
	h := digestHash.New()
	prefix := strings.TrimSuffix(dir, path.Base(dir))
	for line, err := range digestFile(fsys, prefix, iterateDir(fsys, dir, prefix, ignore...)) {
		if err != nil {
			return "", err
		}

		io.WriteString(h, line)
	}

	result := digestName + ":" + base64.StdEncoding.EncodeToString(h.Sum(nil))

	return result, nil
}
