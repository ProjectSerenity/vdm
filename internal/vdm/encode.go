// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package vdm

import (
	"bytes"
	"fmt"
	"maps"
	"slices"
)

// Encode writes out the dependency set.
func (d *Deps) Encode() []byte {
	var buf bytes.Buffer
	if d == nil {
		return nil
	}

	if len(d.BannedGoPackages) > 0 {
		encodeParsedStringsQuoted(&buf, 0, "banned-go-packages", d.BannedGoPackages)
	}

	if len(d.GoModules) > 0 {
		if len(d.BannedGoPackages) > 0 {
			buf.WriteByte('\n')
		}

		fmt.Fprintf(&buf, "go-modules:\n")
		for _, mod := range d.GoModules {
			fmt.Fprintf(&buf, "\tmodule %q %s\n", mod.Name, mod.Version.string())
			encodeParsedStringsQuoted(&buf, 2, "patch-args", mod.PatchArgs)
			encodeParsedStringsQuoted(&buf, 2, "patches", mod.Patches)
			if len(mod.Packages) == 0 {
				continue
			}

			buf.WriteString("\t\tpackages:\n")
			for _, pkg := range mod.Packages {
				pkg.encode(&buf)
			}
		}
	}

	return buf.Bytes()
}

func (pkg *GoPackage) encode(buf *bytes.Buffer) {
	fmt.Fprintf(buf, "\t\t\tpackage %s\n", pkg.Name.quoted())
	encodeParsedStringQuoted(buf, 4, "build-file", pkg.BuildFile)
	encodeParsedStringsQuoted(buf, 4, "deps", pkg.Deps)
	encodeParsedStringsQuoted(buf, 4, "embed", pkg.Embed)
	encodeParsedStringsQuoted(buf, 4, "embed-globs", pkg.EmbedGlobs)
	encodeParsedBoolString(buf, 4, "binary", pkg.Binary)
	encodeParsedStringsQuoted(buf, 4, "binary-deps", pkg.BinaryDeps)
	encodeParsedBoolString(buf, 4, "no-tests", pkg.NoTests)
	encodeParsedStringString(buf, 4, "test-size", pkg.TestSize)
	encodeParsedStringsQuoted(buf, 4, "test-data", pkg.TestData)
	encodeParsedStringsQuoted(buf, 4, "test-data-globs", pkg.TestDataGlobs)
	encodeParsedStringsQuoted(buf, 4, "test-deps", pkg.TestDeps)
	if len(pkg.TestEnv) != 0 {
		keys := slices.Collect(maps.Keys(pkg.TestEnv))
		slices.Sort(keys)

		longestKey := 0
		for _, key := range keys {
			if longestKey < len(key) {
				longestKey = len(key)
			}
		}

		fmt.Fprintf(buf, "\t\t\t\ttest-env:\n")
		for _, key := range keys {
			fmt.Fprintf(buf, "\t\t\t\t\t%q: %s%s\n", key, spaces[:longestKey-len(key)], pkg.TestEnv[key].quoted())
		}
	}
}

// Encode writes out the manifest set.
func (m *Manifests) Encode() []byte {
	var buf bytes.Buffer
	if m == nil {
		return nil
	}

	if len(m.GoModules) > 0 {
		fmt.Fprintf(&buf, "go-modules:\n")
		for _, mod := range m.GoModules {
			fmt.Fprintf(&buf, "\tmodule %q %s\n", mod.Name, mod.Version.string())
			encodeParsedStringString(&buf, 2, "download", mod.Download)
			encodeParsedStringString(&buf, 2, "vendored", mod.Vendored)
			encodeParsedStringString(&buf, 2, "patches", mod.Patches)
		}
	}

	return buf.Bytes()
}

func (p ParsedBool) string() string {
	if p.Comment == "" {
		return fmt.Sprintf("%v", p.Value)
	}

	return fmt.Sprintf("%v  // %s", p.Value, p.Comment)
}

func (p ParsedString) string() string {
	if p.Comment == "" {
		return fmt.Sprintf("%v", p.Value)
	}

	return fmt.Sprintf("%v  // %s", p.Value, p.Comment)
}

func (p ParsedString) quoted() string {
	if p.Comment == "" {
		return fmt.Sprintf("%q", p.Value)
	}

	return fmt.Sprintf("%q  // %s", p.Value, p.Comment)
}

func encodeParsedBoolString(buf *bytes.Buffer, indent int, name string, boo ParsedBool) {
	if boo.Pos.File == "" {
		return
	}

	buf.WriteString(tabs[:indent])
	buf.WriteString(name)
	buf.WriteByte(':')
	buf.WriteByte(' ')
	buf.WriteString(boo.string())
	buf.WriteByte('\n')
}

func encodeParsedStringString(buf *bytes.Buffer, indent int, name string, str ParsedString) {
	if str.Value == "" {
		return
	}

	buf.WriteString(tabs[:indent])
	buf.WriteString(name)
	buf.WriteByte(':')
	buf.WriteByte(' ')
	buf.WriteString(str.string())
	buf.WriteByte('\n')
}

func encodeParsedStringQuoted(buf *bytes.Buffer, indent int, name string, str ParsedString) {
	if str.Value == "" {
		return
	}

	buf.WriteString(tabs[:indent])
	buf.WriteString(name)
	buf.WriteByte(':')
	buf.WriteByte(' ')
	buf.WriteString(str.quoted())
	buf.WriteByte('\n')
}

func encodeParsedStringsQuoted(buf *bytes.Buffer, indent int, name string, list []ParsedString) {
	if len(list) == 0 {
		return
	}

	buf.WriteString(tabs[:indent])
	buf.WriteString(name)
	buf.WriteByte(':')
	buf.WriteByte('\n')

	indent++
	for _, str := range list {
		buf.WriteString(tabs[:indent])
		buf.WriteString(str.quoted())
		buf.WriteByte('\n')
	}
}
