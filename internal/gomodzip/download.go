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

	"github.com/ProjectSerenity/vdm/internal/simplehttp"
	"github.com/ProjectSerenity/vdm/internal/ves"

	"golang.org/x/mod/module"
)

// Download fetches the given module's zip file and
// writes it to disk at the given path. The path
// is normally determined using [Path].
//
// The proxy is the URL scheme and host of the Go
// module proxy. In most cases, this will be the
// string [ModuleProxy].
//
// The download directory containing path must already
// exist.
//
// [module cache]: https://go.dev/ref/mod#module-cache
func Download(ctx context.Context, proxy, path string, manifest *ves.GoModuleManifest) error {
	dl := &downloader{
		logWriter:  os.Stdout,
		gomodproxy: proxy,
	}

	return dl.Download(ctx, path, manifest)
}

type downloader struct {
	logWriter  io.Writer
	gomodproxy string
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

func (dl *downloader) Download(ctx context.Context, path string, manifest *ves.GoModuleManifest) error {
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
