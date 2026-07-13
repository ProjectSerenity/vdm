// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package gomodzip provides helper functionality for
// handling Go module zip files.
package gomodzip

import (
	"fmt"
	"strings"
)

// ModuleProxy is the address of the standard Go
// module proxy. Most callers of [Download] should
// supply this string.
const ModuleProxy = "https://proxy.golang.org"

func extractChecksum(sum string) (string, error) {
	format, checksum, ok := strings.Cut(sum, ":")
	if !ok {
		return "", fmt.Errorf("invalid checksum %q: missing colon", sum)
	}

	switch format {
	case "h1":
		return "sha256:" + checksum, nil
	default:
		return "", fmt.Errorf("invalid checksum %q: unrecognised format %q", sum, format)
	}
}
