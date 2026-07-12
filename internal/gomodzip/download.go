// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package gomodzip

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/ProjectSerenity/vdm/internal/simplehttp"
	"github.com/ProjectSerenity/vdm/internal/vdm"

	"golang.org/x/mod/module"
	"golang.org/x/mod/sumdb"
)

// Download fetches the given module's zip file and
// writes it to disk at the given path. The path
// is normally determined using [Path].
//
// Download then fetches the zip file's checksum.
//
// Download checks the [module cache] for an existing
// checksum. If one is not available, it fetches the
// checksum from the the Go sum database. The cheksum
// is then recorded in the manifest.
//
// The download directory containing path must already
// exist.
//
// [module cache]: https://go.dev/ref/mod#module-cache
func Download(ctx context.Context, path string, manifest *vdm.GoModuleManifest) error {
	dl := &downloader{
		logWriter:  os.Stdout,
		gopath:     os.Getenv("GOPATH"),
		gomodproxy: goModuleProxy,
	}

	return dl.Download(ctx, path, manifest)
}

type downloader struct {
	logWriter  io.Writer
	gopath     string
	gomodproxy string
	sumdb      *sumdb.Client
	sumdbOps   sumdb.ClientOps
}

func (dl *downloader) GOPATH() string {
	if dl.gopath == "" {
		home, _ := os.UserHomeDir()
		dl.gopath = filepath.Join(home, "go")
	}

	return dl.gopath
}

func (dl *downloader) ChecksumClient() *sumdb.Client {
	if dl.sumdb == nil {
		if dl.sumdbOps == nil {
			dl.sumdbOps = clientOps(dl.logWriter, dl.GOPATH())
		}

		dl.sumdb = sumdb.NewClient(dl.sumdbOps)
	}

	return dl.sumdb
}

func (dl *downloader) FindChecksum(path string, manifest *vdm.GoModuleManifest, lines []string) error {
	// Find the line consisting of "importpath version checksum".
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) == 3 && parts[0] == manifest.Name && parts[1] == manifest.Version.Value {
			manifest.Download.Value = parts[2]
			break
		}
	}

	if manifest.Download.Value == "" {
		content := strings.Join(lines, "\n  ")
		return fmt.Errorf("failed to get checksum for %s %s: no checksum in response:\n  %s", manifest.Name, manifest.Version.Value, content)
	}

	var err error
	orig := manifest.Download.Value
	manifest.Download.Value, err = extractChecksum(manifest.Download.Value)
	if err != nil {
		return err
	}

	err = os.WriteFile(path+"hash", []byte(orig), 0o644)
	if err != nil {
		return fmt.Errorf("failed to save checksum to module cache: %v", err)
	}

	return nil
}

func (dl *downloader) Checksum(path string, manifest *vdm.GoModuleManifest) error {
	// This is generally used in testing.
	if manifest.Download.Value != "" {
		return nil
	}

	// Try the module cache first.
	sumData, _ := os.ReadFile(path + "hash")
	if len(sumData) > 0 {
		var err error
		manifest.Download.Value, err = extractChecksum(string(sumData))
		if err != nil {
			return err
		}

		return nil
	}

	// Fall back to the checksum database.
	lines, err := dl.ChecksumClient().Lookup(manifest.Name, manifest.Version.Value)
	if err != nil {
		return fmt.Errorf("failed to get checksum for %s: %v", manifest.Name, err)
	}

	return dl.FindChecksum(path, manifest, lines)
}

func (dl *downloader) WriteFile(w io.WriteCloser, r io.ReadCloser, name string) error {
	_, err := io.Copy(w, r)
	if err != nil {
		w.Close()
		r.Close()
		return fmt.Errorf("failed to download Go module %s: %v", name, err)
	}

	if err = w.Close(); err != nil {
		r.Close()
		return fmt.Errorf("failed to close Go module %s's zip: %v", name, err)
	}

	if err = r.Close(); err != nil {
		return fmt.Errorf("failed to close response body for Go module %s: %v", name, err)
	}

	return nil
}

func (dl *downloader) Download(ctx context.Context, path string, manifest *vdm.GoModuleManifest) error {
	// Start by determining the checksum.
	err := dl.Checksum(path, manifest)
	if err != nil {
		return err
	}

	// Determine the request path.
	escaped, err := module.EscapePath(manifest.Name)
	if err != nil {
		return fmt.Errorf("failed to download Go module %s: invalid module path: %v", manifest.Name, err)
	}

	zipURL := fmt.Sprintf("%s/%s/@v/%s.zip", dl.gomodproxy, escaped, manifest.Version.Value)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, zipURL, nil)
	if err != nil {
		return fmt.Errorf("failed to fetch Go module %s: %v", manifest.Name, err)
	}

	res, err := simplehttp.Request(req)
	if err != nil {
		return fmt.Errorf("failed to fetch Go module %s: %v", manifest.Name, err)
	}

	f, err := os.Create(path)
	if err != nil {
		io.Copy(io.Discard, res.Body)
		res.Body.Close()
		return fmt.Errorf("failed to create temporary file for Go module %s's zip: %v", manifest.Name, err)
	}

	return dl.WriteFile(f, res.Body, manifest.Name)
}
