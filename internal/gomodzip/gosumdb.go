// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package gomodzip

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/ProjectSerenity/vdm/internal/simplehttp"

	"golang.org/x/mod/sumdb"
)

const (
	// Public key for sum.golang.org. See go/src/cmd/go/internal/modfetch/key.go
	goChecksumHost = "sum.golang.org"
	goChecksumKey  = "sum.golang.org+033de0ae+Ac4zctda0e5eza+HJyk9SxEdh+s3Ux18htTTAD8OuAn8"
)

func clientOps(logWriter io.Writer, gopath string) *checksumClientOps {
	configDir := gopath
	if configDir == "" {
		configDir = filepath.Join(os.TempDir(), "config")
	} else {
		configDir = filepath.Join(configDir, "pkg", "sumdb")
	}

	return &checksumClientOps{
		log:       logWriter,
		host:      goChecksumHost,
		name:      goChecksumHost,
		publicKey: goChecksumKey,
		configDir: configDir,
		cacheDir:  filepath.Join(os.TempDir(), "cache"),
	}
}

func checksumClient(logWriter io.Writer, gopath string) *sumdb.Client {
	return sumdb.NewClient(clientOps(logWriter, gopath))
}

type checksumClientOps struct {
	log       io.Writer
	host      string
	name      string
	publicKey string
	configDir string
	cacheDir  string
}

func (c *checksumClientOps) ReadRemote(path string) ([]byte, error) {
	fullURL := (&url.URL{Scheme: "https", Host: c.host, Path: path}).String()
	req, err := http.NewRequest(http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}

	res, err := simplehttp.Request(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		io.Copy(io.Discard, res.Body)
		res.Body.Close()
		return nil, fmt.Errorf("%s returned status code %d", c.host, res.StatusCode)
	}

	data, err := io.ReadAll(res.Body)
	res.Body.Close()

	return data, err
}

func (c *checksumClientOps) configPath(file string) string {
	fullPath := filepath.Join(c.configDir, file)
	dir := filepath.Dir(fullPath)
	os.MkdirAll(dir, 0777)
	return fullPath
}

func (c *checksumClientOps) ReadConfig(file string) ([]byte, error) {
	fullPath := c.configPath(file)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		if file == "key" {
			return []byte(c.publicKey), nil
		}

		return nil, err
	}

	return data, nil
}

func (c *checksumClientOps) WriteConfig(file string, old, new []byte) error {
	fullPath := c.configPath(file)
	f, err := os.OpenFile(fullPath, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return err
	}

	current, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	if !bytes.Equal(current, old) {
		return sumdb.ErrWriteConflict
	}

	if err = f.Truncate(0); err != nil {
		return err
	}

	if _, err = f.Seek(0, 0); err != nil {
		return err
	}

	_, err = f.Write(new)
	if err != nil {
		return err
	}

	return f.Close()
}

func (c *checksumClientOps) cachePath(file string) string {
	fullPath := filepath.Join(c.cacheDir, file)
	dir := filepath.Dir(fullPath)
	os.MkdirAll(dir, 0777)

	return fullPath
}

func (c *checksumClientOps) ReadCache(file string) ([]byte, error) {
	fullPath := c.cachePath(file)
	return os.ReadFile(fullPath)
}

func (c *checksumClientOps) WriteCache(file string, data []byte) {
	fullPath := c.cachePath(file)
	os.WriteFile(fullPath, data, 0666)
}

func (c *checksumClientOps) Log(msg string) {
	if strings.HasSuffix(msg, "\n") {
		io.WriteString(c.log, msg)
	} else {
		fmt.Fprintln(c.log, msg)
	}
}

func (c *checksumClientOps) SecurityError(msg string) {
	if strings.HasSuffix(msg, "\n") {
		io.WriteString(c.log, msg)
	} else {
		fmt.Fprintln(c.log, msg)
	}
}
