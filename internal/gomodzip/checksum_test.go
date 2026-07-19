// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package gomodzip

import (
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
	"time"

	"github.com/ProjectSerenity/vdm/internal/simplehttp"
	"github.com/ProjectSerenity/vdm/internal/ves"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"golang.org/x/mod/sumdb"
)

func TestDownloadDigest(t *testing.T) {
	tests := []struct {
		Name     string
		Path     string
		Manifest *ves.GoModuleManifest
		Want     string
		Error    string
	}{
		{
			Name: "pre-loaded",
			Manifest: &ves.GoModuleManifest{
				Download: s("sha256:checksum"),
			},
			Want: "sha256:checksum",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			err := DownloadDigest(test.Path, test.Manifest)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("DownloadDigest(): unexpected lack of error")
				}

				e := err.Error()
				if !strings.Contains(e, test.Error) {
					t.Fatalf("DownloadDigest(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("DownloadDigest(): got unexpected error: %v", err)
			}

			if test.Manifest.Download.Value != test.Want {
				t.Fatalf("DownloadDigest():\nGot:  %q\nWant: %q", test.Manifest.Download.Value, test.Want)
			}
		})
	}
}

func TestDigester_GOPATH(t *testing.T) {
	tests := []struct {
		Name string
		D    *digester
		Want string
	}{
		{
			Name: "explicit",
			D:    &digester{gopath: "foo/bar"},
			Want: "foo/bar",
		},
		{
			Name: "implicit",
			D:    &digester{},
			Want: func() string {
				home, _ := os.UserHomeDir()
				return filepath.Join(home, "go")
			}(),
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got := test.D.GOPATH()
			if got != test.Want {
				t.Fatalf("digester.GOPATH():\nGot:  %q\nWant: %q", got, test.Want)
			}
		})
	}
}

func TestDigester_ChecksumClient(t *testing.T) {
	tests := []struct {
		Name string
		D    *digester
		Want sumdb.ClientOps
	}{
		{
			Name: "explicit",
			D: &digester{
				sumdb:    checksumClient(io.Discard, "foo"),
				sumdbOps: clientOps(io.Discard, "foo"),
			},
			Want: clientOps(io.Discard, "foo"),
		},
		{
			Name: "implicit",
			D: &digester{
				sumdbOps: clientOps(io.Discard, "foo"),
			},
			Want: clientOps(io.Discard, "foo"),
		},
		{
			Name: "default",
			D: &digester{
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
			test.D.ChecksumClient()
			if diff := cmp.Diff(test.Want, test.D.sumdbOps, opts...); diff != "" {
				t.Errorf("digester.ChecksumClient(): client mismatch (-want, +got)\n%s", diff)
			}
		})
	}
}

func TestDigester_FindChecksum(t *testing.T) {
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

			manifest := &ves.GoModuleManifest{
				Name:    test.ModuleName,
				Version: ves.ParsedString{Value: test.ModuleVersion},
			}

			d := new(digester)
			err := d.FindChecksum(path, manifest, test.Lines)
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

func TestDigester_Digest(t *testing.T) {
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
		D             *digester
		MakeD         func(*testing.T) *digester
		Hash          []byte
		ModuleName    string
		ModuleVersion string
		Want          string
		WantHash      string
		Error         string
	}{
		{
			Name:  "invalid-bad-ziphash",
			D:     &digester{},
			Hash:  []byte("checksum"),
			Error: `invalid checksum "checksum": missing colon`,
		},
		{
			Name:     "valid-good-ziphash",
			D:        &digester{},
			Hash:     []byte("h1:checksum"),
			Want:     "sha256:checksum",
			WantHash: "h1:checksum",
		},
		{
			Name: "invalid-bad-ops",
			MakeD: func(t *testing.T) *digester {
				ops := clientOps(io.Discard, t.ArtifactDir())
				ops.publicKey = ""
				return &digester{sumdbOps: ops}
			},
			ModuleName:    "rsc.io/diff",
			ModuleVersion: "v0.0.0-20190621135850-fe3479844c3c",
			Error:         `failed to get checksum for rsc.io/diff: rsc.io/diff@v0.0.0-20190621135850-fe3479844c3c: initializing sumdb.Client: malformed verifier id`,
		},
		{
			Name: "valid-good-lookup",
			MakeD: func(t *testing.T) *digester {
				ops := clientOps(io.Discard, t.ArtifactDir())
				ops.host = u.Host
				ops.configDir = filepath.Join(t.ArtifactDir(), "config")
				ops.cacheDir = filepath.Join(t.ArtifactDir(), "cache")

				writeConfigs(t, ops.configDir)
				return &digester{sumdbOps: ops}
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

			manifest := &ves.GoModuleManifest{
				Name:    test.ModuleName,
				Version: ves.ParsedString{Value: test.ModuleVersion},
			}

			if test.D == nil && test.MakeD != nil {
				test.D = test.MakeD(t)
			}

			err := test.D.Digest(path, manifest)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("Digest(): unexpected lack of error")
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("Digest(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("Digest(): got unexpected error: %v", err)
			}

			got := manifest.Download.Value
			if got != test.Want {
				t.Fatalf("Digest():\nGot:  %q\nWant: %q", got, test.Want)
			}

			ziphash, err := os.ReadFile(path + "hash")
			if err != nil {
				t.Fatalf("Digest(): failed to read ziphash: %v", err)
			}

			if string(ziphash) != test.WantHash {
				t.Fatalf("Digest(): got wrong ziphash\nGot:  %q\nWant: %q", string(ziphash), test.WantHash)
			}
		})
	}
}
