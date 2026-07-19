// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package gomodzip

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/ProjectSerenity/vdm/internal/simplehttp"
	"github.com/ProjectSerenity/vdm/internal/ves"
)

func TestDownload(t *testing.T) {
	tests := []struct {
		Name     string
		Context  context.Context
		Path     string
		Manifest *ves.GoModuleManifest
		Error    string
	}{
		{
			Name: "invalid-bad-module-path",
			Path: filepath.Join(t.ArtifactDir(), "foo.zip"),
			Manifest: &ves.GoModuleManifest{
				Name:     "-!",
				Version:  ves.S("v1.2.3"),
				Download: ves.S("h1:checksum"),
			},
			Error: `failed to download Go module -!: invalid module path: malformed module path "-!": leading dash`,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			err := Download(test.Context, ModuleProxy, test.Path, test.Manifest)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("Download(): unexpected lack of error")
				}

				e := err.Error()
				if !strings.Contains(e, test.Error) {
					t.Fatalf("Download(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("Download(): got unexpected error: %v", err)
			}
		})
	}
}

type writeCloser struct {
	io.Writer
	err error
}

func (c writeCloser) Close() error { return c.err }

type readCloser struct {
	io.Reader
	err error
}

func (c readCloser) Close() error { return c.err }

func TestDownloader_WriteFile(t *testing.T) {
	tests := []struct {
		Name   string
		W      io.WriteCloser
		R      io.ReadCloser
		Module string
		Error  string
	}{
		{
			Name:   "invalid-bad-reader",
			W:      writeCloser{Writer: new(bytes.Buffer)},
			R:      readCloser{Reader: iotest.ErrReader(errors.New("bad reader"))},
			Module: "rsc.io/diff",
			Error:  "bad reader",
		},
		{
			Name:   "invalid-bad-write-closer",
			W:      writeCloser{Writer: new(bytes.Buffer), err: errors.New("bad write closer")},
			R:      readCloser{Reader: bytes.NewReader(nil)},
			Module: "rsc.io/diff",
			Error:  "bad write closer",
		},
		{
			Name:   "invalid-bad-read-closer",
			W:      writeCloser{Writer: new(bytes.Buffer)},
			R:      readCloser{Reader: bytes.NewReader(nil), err: errors.New("bad read closer")},
			Module: "rsc.io/diff",
			Error:  "bad read closer",
		},
		{
			Name:   "valid-simple",
			W:      writeCloser{Writer: new(bytes.Buffer)},
			R:      readCloser{Reader: bytes.NewReader(nil)},
			Module: "rsc.io/diff",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			dl := new(downloader)
			err := dl.WriteFile(test.W, test.R, test.Module)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("WriteFile(): unexpected lack of error")
				}

				e := err.Error()
				if !strings.Contains(e, test.Error) {
					t.Fatalf("WriteFile(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("WriteFile(): got unexpected error: %v", err)
			}
		})
	}
}

func hashFile(t *testing.T, name string) string {
	t.Helper()
	f, err := os.Open(name)
	if err != nil {
		t.Fatalf("failed to open %s: %v", name, err)
	}

	defer f.Close()

	h := sha256.New()
	_, err = io.Copy(h, f)
	if err != nil {
		t.Fatalf("failed to hash %s: %v", name, err)
	}

	err = f.Close()
	if err != nil {
		t.Fatalf("failed to close %s: %v", name, err)
	}

	sum := h.Sum(nil)
	return hex.EncodeToString(sum)
}

//go:embed testdata/zips/rsc.io/diff/@v/v0.0.0-20190621135850-fe3479844c3c.zip
var savedPackageZIP []byte

func TestDownloader_Download(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("/", http.NotFoundHandler()) // Default handler.
	mux.HandleFunc("GET /rsc.io/diff/@v/v0.0.0-20190621135850-fe3479844c3c.zip", func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "v0.0.0-20190621135850-fe3479844c3c.zip", time.Time{}, bytes.NewReader(savedPackageZIP))
	})

	srv := httptest.NewTLSServer(mux)
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	defer srv.Close()

	simplehttp.Client = srv.Client()
	simplehttp.UserAgent = "tests"
	simplehttp.SetInterval(time.Millisecond)

	tests := []struct {
		Name          string
		DL            *downloader
		PathSuffix    string
		Checksum      string
		Hash          []byte
		Context       context.Context
		ModuleName    string
		ModuleVersion string
		Want          string
		Error         string
	}{
		{
			Name:       "invalid-bad-module-name",
			DL:         &downloader{},
			Checksum:   "h1:checksum",
			ModuleName: "-!",
			Error:      `failed to download Go module -!: invalid module path: malformed module path "-!": leading dash`,
		},
		{
			Name:          "invalid-bad-context",
			DL:            &downloader{},
			Checksum:      "h1:checksum",
			Context:       nil,
			ModuleName:    "rsc.io/diff",
			ModuleVersion: "v0.0.0-20190621135850-fe3479844c3c",
			Error:         `failed to fetch Go module rsc.io/diff: net/http: nil Context`,
		},
		{
			Name:          "invalid-bad-module-proxy",
			DL:            &downloader{gomodproxy: "https://0.0.0.0:0"},
			Checksum:      "h1:checksum",
			Context:       context.Background(),
			ModuleName:    "rsc.io/diff",
			ModuleVersion: "v0.0.0-20190621135850-fe3479844c3c",
			Error:         `connect: connection refused`,
		},
		{
			Name:          "invalid-missing-directory",
			DL:            &downloader{gomodproxy: "https://example.com"},
			PathSuffix:    "nonexistant",
			Checksum:      "h1:checksum",
			Context:       context.Background(),
			ModuleName:    "rsc.io/diff",
			ModuleVersion: "v0.0.0-20190621135850-fe3479844c3c",
			Error:         `nonexistant: no such file or directory`,
		},
		{
			Name:          "valid-simple",
			DL:            &downloader{gomodproxy: "https://example.com"},
			Checksum:      "h1:checksum",
			Context:       context.Background(),
			ModuleName:    "rsc.io/diff",
			ModuleVersion: "v0.0.0-20190621135850-fe3479844c3c",
			Want:          filepath.FromSlash("zips/rsc.io/diff/@v/v0.0.0-20190621135850-fe3479844c3c.zip"),
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			dir := t.ArtifactDir()
			path := filepath.Join(dir, test.ModuleVersion+".zip")
			if test.PathSuffix != "" {
				path = filepath.Join(path, test.PathSuffix)
			}

			if test.Hash != nil {
				err := os.WriteFile(path+"hash", test.Hash, 0o644)
				if err != nil {
					t.Fatalf("failed to write ziphash: %v", err)
				}
			}

			manifest := &ves.GoModuleManifest{
				Name:     test.ModuleName,
				Version:  ves.S(test.ModuleVersion),
				Download: ves.S(test.Checksum),
			}

			err := test.DL.Download(test.Context, path, manifest)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("Download(): unexpected lack of error")
				}

				e := err.Error()
				if !strings.Contains(e, test.Error) {
					t.Fatalf("Download(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("Download(): got unexpected error: %v", err)
			}

			got := hashFile(t, path)
			want := hashFile(t, filepath.Join("testdata", test.Want))
			if got != want {
				t.Fatalf("Download():\nGot:  %q\nWant: %q", got, want)
			}
		})
	}
}
