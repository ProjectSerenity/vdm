// Copyright 2026 The Firefly Authors.
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

	"github.com/ProjectSerenity/vdm/internal/vdm"
)

// The templates used to render build files and
// dependency cache manifests.
//
//go:embed templates/*.tmpl
var templatesFS embed.FS

var templates = template.Must(template.New("").Funcs(template.FuncMap{
	"binaryName":  binaryName,
	"packageName": packageName,
}).ParseFS(templatesFS, "templates/*.tmpl"))

// binaryName return the name in a form suitable
// for use as a Go binary's package name.
func binaryName(pkg *vdm.GoPackage) string {
	return path.Base(pkg.Name.Value)
}

// packageName return the name in a form suitable
// for use as a Go package name.
func packageName(pkg *vdm.GoPackage) string {
	base := path.Base(pkg.Name.Value)
	if pkg.Binary.Value {
		base += "_lib"
	}

	return base
}

// RenderGoPackageBuildFile generates a build file
// for the given Go package.
func (c *GenerateGoPackageBUILD) Render() ([]byte, error) {
	var buf bytes.Buffer
	err := templates.ExecuteTemplate(&buf, "go-BUILD.tmpl", c)
	if err != nil {
		return nil, fmt.Errorf("failed to render build file: %v", err)
	}

	return buf.Bytes(), nil
}
