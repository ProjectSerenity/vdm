// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package fix

import (
	"io"
	"io/fs"

	"github.com/ProjectSerenity/vdm/internal/ves"
)

// Fix identifies issues with the dependency set.
func Fix(w io.Writer, fsys fs.FS, deps *ves.Deps) (changed bool, err error) {
	changed, err = FindUnusedGoDependencies(w, fsys, deps)
	if err != nil {
		return false, err
	}

	return changed, nil
}
