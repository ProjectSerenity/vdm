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
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/iotest"
	"time"

	"github.com/ProjectSerenity/vdm/internal/simplehttp"
	"github.com/ProjectSerenity/vdm/internal/vdm"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/mod/sumdb"
)

func TestDownload(t *testing.T) {
	tests := []struct {
		Name     string
		Context  context.Context
		Path     string
		Manifest *vdm.GoModuleManifest
		Error    string
	}{
		{
			Name: "invalid-bad-module-path",
			Path: filepath.Join(t.ArtifactDir(), "foo.zip"),
			Manifest: &vdm.GoModuleManifest{
				Name:     "-!",
				Version:  vdm.ParsedString{Value: "v1.2.3"},
				Download: vdm.ParsedString{Value: "h1:checksum"},
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

func TestDownloader_GOPATH(t *testing.T) {
	tests := []struct {
		Name string
		DL   *downloader
		Want string
	}{
		{
			Name: "explicit",
			DL:   &downloader{gopath: "foo/bar"},
			Want: "foo/bar",
		},
		{
			Name: "implicit",
			DL:   &downloader{},
			Want: func() string {
				home, _ := os.UserHomeDir()
				return filepath.Join(home, "go")
			}(),
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got := test.DL.GOPATH()
			if got != test.Want {
				t.Fatalf("dl.GOPATH():\nGot:  %q\nWant: %q", got, test.Want)
			}
		})
	}
}

func TestDownloader_ChecksumClient(t *testing.T) {
	tests := []struct {
		Name string
		DL   *downloader
		Want sumdb.ClientOps
	}{
		{
			Name: "explicit",
			DL: &downloader{
				sumdb:    checksumClient(io.Discard, "foo"),
				sumdbOps: clientOps(io.Discard, "foo"),
			},
			Want: clientOps(io.Discard, "foo"),
		},
		{
			Name: "implicit",
			DL: &downloader{
				sumdbOps: clientOps(io.Discard, "foo"),
			},
			Want: clientOps(io.Discard, "foo"),
		},
		{
			Name: "default",
			DL: &downloader{
				logWriter: io.Discard,
				gopath:    "foo",
			},
			Want: clientOps(io.Discard, "foo"),
		},
	}

	opts := []cmp.Option{
		cmp.AllowUnexported(sumdb.Client{}, checksumClientOps{}),
		cmpopts.EquateComparable(sync.Once{}),
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			test.DL.ChecksumClient()
			if diff := cmp.Diff(test.Want, test.DL.sumdbOps, opts...); diff != "" {
				t.Errorf("client.ChecksumClient(): client mismatch (-want, +got)\n%s", diff)
			}
		})
	}
}

func TestDownloader_FindChecksum(t *testing.T) {
	tests := []struct {
		Name          string
		PathSuffix    string
		ModuleName    string
		ModuleVersion string
		Lines         []string
		Want          string
		WantHash      string
		Error         string
	}{
		{
			Name:          "invalid-bad-lines",
			ModuleName:    "rsc.io/diff",
			ModuleVersion: "v0.0.0-20190621135850-fe3479844c3c",
			Lines:         []string{"foo"},
			Error:         "failed to get checksum for rsc.io/diff v0.0.0-20190621135850-fe3479844c3c: no checksum in response:\n  foo",
		},
		{
			Name:          "invalid-bad-checksum",
			ModuleName:    "rsc.io/diff",
			ModuleVersion: "v0.0.0-20190621135850-fe3479844c3c",
			Lines: []string{
				"rsc.io/diff v0.0.0-20190621135850-fe3479844c3c checksum",
			},
			Error: `invalid checksum "checksum": missing colon`,
		},
		{
			Name:          "invalid-bad-write-path",
			PathSuffix:    "nonexistant",
			ModuleName:    "rsc.io/diff",
			ModuleVersion: "v0.0.0-20190621135850-fe3479844c3c",
			Lines:         recordedResponseLines,
			Error:         `failed to save checksum to module cache`,
		},
		{
			Name:          "valid-good-lines",
			ModuleName:    "rsc.io/diff",
			ModuleVersion: "v0.0.0-20190621135850-fe3479844c3c",
			Lines:         recordedResponseLines,
			Want:          "sha256:/WCDjRGIVDjKlhtSc1PEApp2fR58gfSVK62dr/yQNyQ=",
			WantHash:      "h1:/WCDjRGIVDjKlhtSc1PEApp2fR58gfSVK62dr/yQNyQ=",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			dir := t.ArtifactDir()
			path := filepath.Join(dir, "foo.zip")
			if test.PathSuffix != "" {
				path = filepath.Join(path, test.PathSuffix)
			}

			manifest := &vdm.GoModuleManifest{
				Name:    test.ModuleName,
				Version: vdm.ParsedString{Value: test.ModuleVersion},
			}

			dl := new(downloader)
			err := dl.FindChecksum(path, manifest, test.Lines)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("FindChecksum(): unexpected lack of error")
				}

				e := err.Error()
				if !strings.Contains(e, test.Error) {
					t.Fatalf("FindChecksum(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("FindChecksum(): got unexpected error: %v", err)
			}

			got := manifest.Download.Value
			if got != test.Want {
				t.Fatalf("FindChecksum():\nGot:  %q\nWant: %q", got, test.Want)
			}

			ziphash, err := os.ReadFile(path + "hash")
			if err != nil {
				t.Fatalf("FindChecksum(): failed to read ziphash: %v", err)
			}

			if string(ziphash) != test.WantHash {
				t.Fatalf("FindChecksum(): got wrong ziphash\nGot:  %q\nWant: %q", string(ziphash), test.WantHash)
			}
		})
	}
}

func TestDownloader_Checksum(t *testing.T) {
	mux := http.NewServeMux()
	addRecordingHandlers(mux)
	srv := httptest.NewTLSServer(mux)
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	defer srv.Close()

	u, err := url.Parse(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}

	simplehttp.Client = srv.Client()
	simplehttp.UserAgent = "tests"
	simplehttp.SetInterval(time.Millisecond)

	tests := []struct {
		Name          string
		DL            *downloader
		MakeDL        func(*testing.T) *downloader
		Hash          []byte
		ModuleName    string
		ModuleVersion string
		Want          string
		WantHash      string
		Error         string
	}{
		{
			Name:  "invalid-bad-ziphash",
			DL:    &downloader{},
			Hash:  []byte("checksum"),
			Error: `invalid checksum "checksum": missing colon`,
		},
		{
			Name:     "valid-good-ziphash",
			DL:       &downloader{},
			Hash:     []byte("h1:checksum"),
			Want:     "sha256:checksum",
			WantHash: "h1:checksum",
		},
		{
			Name: "invalid-bad-ops",
			MakeDL: func(t *testing.T) *downloader {
				ops := clientOps(io.Discard, t.ArtifactDir())
				ops.publicKey = ""
				return &downloader{sumdbOps: ops}
			},
			ModuleName:    "rsc.io/diff",
			ModuleVersion: "v0.0.0-20190621135850-fe3479844c3c",
			Error:         `failed to get checksum for rsc.io/diff: rsc.io/diff@v0.0.0-20190621135850-fe3479844c3c: initializing sumdb.Client: malformed verifier id`,
		},
		{
			Name: "valid-good-lookup",
			MakeDL: func(t *testing.T) *downloader {
				ops := clientOps(io.Discard, t.ArtifactDir())
				ops.host = u.Host
				ops.configDir = filepath.Join(t.ArtifactDir(), "config")
				ops.cacheDir = filepath.Join(t.ArtifactDir(), "cache")

				writeConfigs(t, ops.configDir)
				return &downloader{sumdbOps: ops}
			},
			ModuleName:    "rsc.io/diff",
			ModuleVersion: "v0.0.0-20190621135850-fe3479844c3c",
			Want:          "sha256:/WCDjRGIVDjKlhtSc1PEApp2fR58gfSVK62dr/yQNyQ=",
			WantHash:      "h1:/WCDjRGIVDjKlhtSc1PEApp2fR58gfSVK62dr/yQNyQ=",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			dir := t.ArtifactDir()
			path := filepath.Join(dir, "foo.zip")
			if test.Hash != nil {
				err := os.WriteFile(path+"hash", test.Hash, 0o644)
				if err != nil {
					t.Fatalf("failed to write ziphash: %v", err)
				}
			}

			manifest := &vdm.GoModuleManifest{
				Name:    test.ModuleName,
				Version: vdm.ParsedString{Value: test.ModuleVersion},
			}

			if test.DL == nil && test.MakeDL != nil {
				test.DL = test.MakeDL(t)
			}

			err := test.DL.Checksum(path, manifest)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("Checksum(): unexpected lack of error")
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("Checksum(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("Checksum(): got unexpected error: %v", err)
			}

			got := manifest.Download.Value
			if got != test.Want {
				t.Fatalf("Checksum():\nGot:  %q\nWant: %q", got, test.Want)
			}

			ziphash, err := os.ReadFile(path + "hash")
			if err != nil {
				t.Fatalf("Checksum(): failed to read ziphash: %v", err)
			}

			if string(ziphash) != test.WantHash {
				t.Fatalf("Checksum(): got wrong ziphash\nGot:  %q\nWant: %q", string(ziphash), test.WantHash)
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
			Name:       "invalid-bad-checksum",
			DL:         &downloader{},
			Hash:       []byte("checksum"),
			ModuleName: "-!",
			Error:      `invalid checksum "checksum": missing colon`,
		},
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

			manifest := &vdm.GoModuleManifest{
				Name:     test.ModuleName,
				Version:  vdm.ParsedString{Value: test.ModuleVersion},
				Download: vdm.ParsedString{Value: test.Checksum},
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
