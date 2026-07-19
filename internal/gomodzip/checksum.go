// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package gomodzip

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ProjectSerenity/vdm/internal/vdm"

	"golang.org/x/mod/sumdb"
)

// DownloadDigest fetches the expected digest for
// the given module's zip file and stores it in
// the given module manifest.
//
// DownloadDigest checks the [module cache] for an
// existing checksum. If one is not available, it fetches
// the checksum from the the Go sum database. The checksum
// is then recorded in the manifest.
//
// The download directory containing path must already
// exist.
//
// [module cache]: https://go.dev/ref/mod#module-cache
func DownloadDigest(path string, manifest *vdm.GoModuleManifest) error {
	d := &digester{
		logWriter: os.Stdout,
		gopath:    os.Getenv("GOPATH"),
	}

	return d.Digest(path, manifest)
}

type digester struct {
	logWriter io.Writer
	gopath    string
	sumdb     *sumdb.Client
	sumdbOps  sumdb.ClientOps
}

func (d *digester) GOPATH() string {
	if d.gopath == "" {
		home, _ := os.UserHomeDir()
		d.gopath = filepath.Join(home, "go")
	}

	return d.gopath
}

func (d *digester) ChecksumClient() *sumdb.Client {
	if d.sumdb == nil {
		if d.sumdbOps == nil {
			d.sumdbOps = clientOps(d.logWriter, d.GOPATH())
		}

		d.sumdb = sumdb.NewClient(d.sumdbOps)
	}

	return d.sumdb
}

func (d *digester) FindChecksum(path string, manifest *vdm.GoModuleManifest, lines []string) error {
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

func (d *digester) Digest(path string, manifest *vdm.GoModuleManifest) error {
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
	lines, err := d.ChecksumClient().Lookup(manifest.Name, manifest.Version.Value)
	if err != nil {
		return fmt.Errorf("failed to get checksum for %s: %v", manifest.Name, err)
	}

	return d.FindChecksum(path, manifest, lines)
}
