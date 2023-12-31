// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package vendor

import (
	"bytes"
	"embed"
	"fmt"
	"path"
	"text/template"

	"github.com/ProjectSerenity/vdm/internal/starlark"
	"github.com/ProjectSerenity/vdm/internal/vendeps"
)

// The templates used to render build files and
// dependency cache manifests.
//
//go:embed templates/*.txt
var templatesFS embed.FS

var templates = template.Must(template.New("").Funcs(template.FuncMap{
	"binaryName":  binaryName,
	"packageName": packageName,
}).ParseFS(templatesFS, "templates/*.txt"))

// binaryName return the name in a form suitable
// for use as a Go binary's package name.
func binaryName(pkg *vendeps.GoPackage) string {
	return path.Base(pkg.Name)
}

// packageName return the name in a form suitable
// for use as a Go package name.
func packageName(pkg *vendeps.GoPackage) string {
	base := path.Base(pkg.Name)
	if pkg.Binary {
		base += "_lib"
	}

	return base
}

// RenderGoPackageBuildFile generates a build file
// for the given Go package.
func RenderGoPackageBuildFile(name string, pkg *vendeps.GoPackage) ([]byte, error) {
	var buf bytes.Buffer
	err := templates.ExecuteTemplate(&buf, "go-BUILD.txt", pkg)
	if err != nil {
		return nil, fmt.Errorf("failed to render build file: %v", err)
	}

	return starlark.Format(name, buf.Bytes())
}

// RenderTextFilesBuildFile generates a build file
// for the given text files.
func RenderTextFilesBuildFile(name string, files *vendeps.TextFiles) ([]byte, error) {
	var buf bytes.Buffer
	err := templates.ExecuteTemplate(&buf, "files-BUILD.txt", files)
	if err != nil {
		return nil, fmt.Errorf("failed to render build file: %v", err)
	}

	return starlark.Format(name, buf.Bytes())
}

// RenderManifest generates a dependency manifest
// from the given set of dependencies.
func RenderManifest(name string, manifest *vendeps.Deps) ([]byte, error) {
	var buf bytes.Buffer
	err := templates.ExecuteTemplate(&buf, "manifest.txt", manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to render cache manifest: %v", err)
	}

	return starlark.Format(name, buf.Bytes())
}
