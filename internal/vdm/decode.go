// Copyright 2026 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package vdm

import (
	"cmp"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

// ReadDeps opens the specified file in the filesystem
// and parses its contents as a dependency set.
func ReadDeps(fsys fs.FS, name string) (*Deps, error) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, err
	}

	return ParseDeps(name, string(data))
}

// ParseDeps parses the dependency set from a VDM file.
func ParseDeps(name, data string) (*Deps, error) {
	deps := new(Deps)
	p := &parser{
		Data: data,
		File: name,
		Line: 1,
	}

	for {
		keyword, err := p.FindColon(0)
		if err == io.EOF {
			return deps, nil
		}

		if err != nil {
			return nil, err
		}

		switch keyword {
		case "banned-go-packages":
			if len(deps.BannedGoPackages) != 0 {
				return nil, p.Errorf("duplicate banned Go package set, first found at %s", deps.BannedGoPackages[0].Pos)
			}

			deps.BannedGoPackages, err = p.FindQuotedStrings(1)
			if err == io.EOF {
				return nil, p.Errorf("expected a quoted string after banned-go-packages:, got EOF")
			}

			if err != nil {
				return nil, err
			}
		case "go-modules":
			if len(deps.GoModules) != 0 {
				return nil, p.Errorf("duplicate Go module set, first found at %s", deps.GoModules[0].Version.Pos)
			}

			keywordPos := p.Pos()
			for {
				module, err := p.ParseGoModule()
				if err == io.EOF {
					if len(deps.GoModules) == 0 {
						return nil, p.Errorf("no Go modules provided after %q keyword at %s", "go-modules", keywordPos)
					}

					return deps, nil
				}

				if err != nil {
					return nil, err
				}

				if module == nil {
					break
				}

				deps.GoModules = append(deps.GoModules, module)
			}
		default:
			return nil, p.Errorf("unrecognised keyword %q", keyword)
		}
	}
}

// ParseGoModule parses a single Go module.
func (p *parser) ParseGoModule() (*GoModule, error) {
	ok, err := p.FindKeyword(1, "module")
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, nil
	}

	moduleName, err := p.ParseQuotedString()
	if err == io.EOF {
		return nil, p.Errorf("expected module name, got EOF")
	}

	if err != nil {
		return nil, err
	}

	if !p.SkipSpaces() {
		return nil, p.Errorf("expected space after module name, got %q", p.Tokenise(p.Data))
	}

	version, err := p.ParseParsedString()
	if err == io.EOF {
		return nil, p.Errorf("expected module version, got EOF")
	}

	if err != nil {
		return nil, err
	}

	module := &GoModule{
		Name:    moduleName,
		Version: version,
	}

	// Iterate the remaining keywords.
	for {
		keyword, err := p.FindColon(2)
		if err == io.EOF {
			return module, nil
		}

		if err != nil {
			return nil, err
		}

		switch keyword {
		case "":
			// We're done.
			return module, nil
		case "patches":
			if len(module.Patches) > 0 {
				return nil, p.Errorf("duplicate patches, first found at %s", module.Patches[0].Pos)
			}

			module.Patches, err = p.FindQuotedStrings(3)
			if err != nil {
				return nil, err
			}
		case "packages":
			if len(module.Packages) > 0 {
				return nil, p.Errorf("duplicate Go package set, first found at %s", module.Packages[0].Name.Pos)
			}

			keywordPos := p.Pos()
			for {
				pkg, err := p.ParseGoPackage()
				if err == io.EOF {
					if len(module.Packages) == 0 {
						return nil, p.Errorf("no Go packages provided after %q keyword at %s", "packages", keywordPos)
					}

					return module, nil
				}

				if err != nil {
					return nil, err
				}

				if pkg == nil {
					break
				}

				module.Packages = append(module.Packages, pkg)
			}
		default:
			return nil, p.Errorf("unrecognised keyword %q", keyword)
		}
	}
}

// ParseGoPackage parses a single Go package.
func (p *parser) ParseGoPackage() (*GoPackage, error) {
	ok, err := p.FindKeyword(3, "package")
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, nil
	}

	packageName, err := p.ParseQuotedParsedString()
	if err == io.EOF {
		return nil, p.Errorf("expected package name, got EOF")
	}

	if err != nil {
		return nil, err
	}

	pkg := &GoPackage{
		Name: packageName,
	}

	// Iterate the remaining keywords.
	for {
		keyword, err := p.FindColon(4)
		if err == io.EOF {
			return pkg, nil
		}

		if err != nil {
			return nil, err
		}

		switch keyword {
		case "":
			// We're done.
			return pkg, nil
		case "build-file":
			if pkg.BuildFile.Value != "" {
				return nil, p.Errorf("duplicate build file, first found at %s", pkg.BuildFile.Pos)
			}

			if !p.SkipSpaces() {
				return nil, p.Errorf("expected a space after build-file:, got %q", p.Tokenise(p.Data))
			}

			pkg.BuildFile, err = p.ParseQuotedParsedString()
			if err == io.EOF {
				return nil, p.Errorf("expected a quoted string after build-file:, got EOF")
			}

			if err != nil {
				return nil, err
			}
		case "deps":
			if len(pkg.Deps) > 0 {
				return nil, p.Errorf("duplicate deps, first found at %s", pkg.Deps[0].Pos)
			}

			pkg.Deps, err = p.FindQuotedStrings(5)
			if err == io.EOF {
				return nil, p.Errorf("expected a quoted string after deps:, got EOF")
			}

			if err != nil {
				return nil, err
			}
		case "embed":
			if len(pkg.Embed) > 0 {
				return nil, p.Errorf("duplicate embed, first found at %s", pkg.Embed[0].Pos)
			}

			pkg.Embed, err = p.FindQuotedStrings(5)
			if err == io.EOF {
				return nil, p.Errorf("expected a quoted string after embed:, got EOF")
			}

			if err != nil {
				return nil, err
			}
		case "embed-globs":
			if len(pkg.EmbedGlobs) > 0 {
				return nil, p.Errorf("duplicate embed globs, first found at %s", pkg.EmbedGlobs[0].Pos)
			}

			pkg.EmbedGlobs, err = p.FindQuotedStrings(5)
			if err == io.EOF {
				return nil, p.Errorf("expected a quoted string after embed-globs:, got EOF")
			}

			if err != nil {
				return nil, err
			}
		case "directories":
			if len(pkg.Directories) > 0 {
				return nil, p.Errorf("duplicate directories, first found at %s", pkg.Directories[0].Pos)
			}

			pkg.Directories, err = p.FindQuotedStrings(5)
			if err == io.EOF {
				return nil, p.Errorf("expected a quoted string after directories:, got EOF")
			}

			if err != nil {
				return nil, err
			}
		case "binary":
			if pkg.Binary.Pos.File != "" {
				return nil, p.Errorf("duplicate binary, first found at %s", pkg.Binary.Pos)
			}

			if !p.SkipSpaces() {
				return nil, p.Errorf("expected a space after binary:, got %q", p.Tokenise(p.Data))
			}

			pkg.Binary, err = p.ParseParsedBool()
			if err == io.EOF {
				return nil, p.Errorf("expected a boolean after binary:, got EOF")
			}

			if err != nil {
				return nil, err
			}
		case "binary-deps":
			if len(pkg.BinaryDeps) > 0 {
				return nil, p.Errorf("duplicate binary deps, first found at %s", pkg.BinaryDeps[0].Pos)
			}

			pkg.BinaryDeps, err = p.FindQuotedStrings(5)
			if err == io.EOF {
				return nil, p.Errorf("expected a quoted string after binary-deps:, got EOF")
			}

			if err != nil {
				return nil, err
			}
		case "no-tests":
			if pkg.NoTests.Pos.File != "" {
				return nil, p.Errorf("duplicate no tests, first found at %s", pkg.NoTests.Pos)
			}

			if !p.SkipSpaces() {
				return nil, p.Errorf("expected a space after no-tests:, got %q", p.Tokenise(p.Data))
			}

			pkg.NoTests, err = p.ParseParsedBool()
			if err == io.EOF {
				return nil, p.Errorf("expected a boolean after no-tests:, got EOF")
			}

			if err != nil {
				return nil, err
			}
		case "test-size":
			if pkg.TestSize.Value != "" {
				return nil, p.Errorf("duplicate test size, first found at %s", pkg.TestSize.Pos)
			}

			if !p.SkipSpaces() {
				return nil, p.Errorf("expected a space after test-size:, got %q", p.Tokenise(p.Data))
			}

			pkg.TestSize, err = p.ParseParsedString()
			if err == io.EOF {
				return nil, p.Errorf("expected a string after test-size:, got EOF")
			}

			if err != nil {
				return nil, err
			}
		case "test-data":
			if len(pkg.TestData) > 0 {
				return nil, p.Errorf("duplicate test data, first found at %s", pkg.TestData[0].Pos)
			}

			pkg.TestData, err = p.FindQuotedStrings(5)
			if err == io.EOF {
				return nil, p.Errorf("expected a quoted string after test-data:, got EOF")
			}

			if err != nil {
				return nil, err
			}
		case "test-data-globs":
			if len(pkg.TestDataGlobs) > 0 {
				return nil, p.Errorf("duplicate test data globs, first found at %s", pkg.TestDataGlobs[0].Pos)
			}

			pkg.TestDataGlobs, err = p.FindQuotedStrings(5)
			if err == io.EOF {
				return nil, p.Errorf("expected a quoted string after test-data-globs:, got EOF")
			}

			if err != nil {
				return nil, err
			}
		case "test-deps":
			if len(pkg.TestDeps) > 0 {
				return nil, p.Errorf("duplicate test deps, first found at %s", pkg.TestDeps[0].Pos)
			}

			pkg.TestDeps, err = p.FindQuotedStrings(5)
			if err == io.EOF {
				return nil, p.Errorf("expected a quoted string after test-deps:, got EOF")
			}

			if err != nil {
				return nil, err
			}
		case "test-env":
			if len(pkg.TestEnv) > 0 {
				values := slices.Collect(maps.Values(pkg.TestEnv))
				slices.SortFunc(values, func(a, b ParsedString) int { return cmp.Compare(a.Pos.Line, b.Pos.Line) })
				return nil, p.Errorf("duplicate test env, first found at %s", values[0].Pos)
			}

			pkg.TestEnv, err = p.FindQuotedStringsMap(5)
			if err == io.EOF {
				return nil, p.Errorf("expected a quoted string after test-env:, got EOF")
			}

			if err != nil {
				return nil, err
			}
		default:
			return nil, p.Errorf("unrecognised keyword %q", keyword)
		}
	}
}

// ReadManifests opens the specified file in the filesystem
// and parses its contents as a dependency set manifest.
func ReadManifests(fsys fs.FS, name string) (*Manifests, error) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, err
	}

	return ParseManifests(name, string(data))
}

// ParseManifests parses the vendored dependency set from a VDM file.
func ParseManifests(name, data string) (*Manifests, error) {
	manifests := new(Manifests)
	p := &parser{
		Data: data,
		File: name,
		Line: 1,
	}

	for {
		keyword, err := p.FindColon(0)
		if err == io.EOF {
			return manifests, nil
		}

		if err != nil {
			return nil, err
		}

		switch keyword {
		case "go-modules":
			if len(manifests.GoModules) != 0 {
				return nil, p.Errorf("duplicate Go module set, first found at %s", manifests.GoModules[0].Version.Pos)
			}

			keywordPos := p.Pos()
			for {
				module, err := p.ParseGoModuleManifest()
				if err == io.EOF {
					if len(manifests.GoModules) == 0 {
						return nil, p.Errorf("no Go modules provided after %q keyword at %s", "go-modules", keywordPos)
					}

					return manifests, nil
				}

				if err != nil {
					return nil, err
				}

				if module == nil {
					break
				}

				manifests.GoModules = append(manifests.GoModules, module)
			}
		default:
			return nil, p.Errorf("unrecognised keyword %q", keyword)
		}
	}
}

// ParseGoModuleManifest parses a single Go module manifest.
func (p *parser) ParseGoModuleManifest() (*GoModuleManifest, error) {
	ok, err := p.FindKeyword(1, "module")
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, nil
	}

	moduleName, err := p.ParseQuotedString()
	if err == io.EOF {
		return nil, p.Errorf("expected module name, got EOF")
	}

	if err != nil {
		return nil, err
	}

	if !p.SkipSpaces() {
		return nil, p.Errorf("expected space after module name, got %q", p.Tokenise(p.Data))
	}

	version, err := p.ParseParsedString()
	if err == io.EOF {
		return nil, p.Errorf("expected module version, got EOF")
	}

	if err != nil {
		return nil, err
	}

	module := &GoModuleManifest{
		Name:    moduleName,
		Version: version,
	}

	// Iterate the remaining keywords.
	for {
		keyword, err := p.FindColon(2)
		if err == io.EOF {
			return module, nil
		}

		if err != nil {
			return nil, err
		}

		switch keyword {
		case "":
			// We're done.
			return module, nil
		case "download":
			if module.Download.Value != "" {
				return nil, p.Errorf("duplicate download, first found at %s", module.Download.Pos)
			}

			if !p.SkipSpaces() {
				return nil, p.Errorf("expected a space after download:, got %q", p.Tokenise(p.Data))
			}

			module.Download, err = p.ParseParsedString()
			if err == io.EOF {
				return nil, p.Errorf("expected a string after download:, got EOF")
			}

			if err != nil {
				return nil, err
			}
		case "vendored":
			if module.Vendored.Value != "" {
				return nil, p.Errorf("duplicate vendored, first found at %s", module.Vendored.Pos)
			}

			if !p.SkipSpaces() {
				return nil, p.Errorf("expected a space after vendored:, got %q", p.Tokenise(p.Data))
			}

			module.Vendored, err = p.ParseParsedString()
			if err == io.EOF {
				return nil, p.Errorf("expected a string after vendored:, got EOF")
			}

			if err != nil {
				return nil, err
			}
		case "patches":
			if module.Patches.Value != "" {
				return nil, p.Errorf("duplicate patches, first found at %s", module.Patches.Pos)
			}

			if !p.SkipSpaces() {
				return nil, p.Errorf("expected a space after patches:, got %q", p.Tokenise(p.Data))
			}

			module.Patches, err = p.ParseParsedString()
			if err == io.EOF {
				return nil, p.Errorf("expected a string after patches:, got EOF")
			}

			if err != nil {
				return nil, err
			}
		default:
			return nil, p.Errorf("unrecognised keyword %q", keyword)
		}
	}
}

type parser struct {
	Data string
	File string
	Line int
}

func (p *parser) Pos() Pos    { return Pos{File: p.File, Line: p.Line} }
func (p *parser) Empty() bool { return len(p.Data) == 0 }

func (p *parser) SkipSpaces() (skipped bool) {
	trimmed := strings.TrimLeftFunc(p.Data, func(r rune) bool { return r == ' ' })
	skipped = len(trimmed) < len(p.Data)
	p.Data = trimmed
	return skipped
}

func (p *parser) SkipNewlines() {
	trimmed := strings.TrimLeftFunc(p.Data, func(r rune) bool { return r == '\n' })
	p.Line += len(p.Data) - len(trimmed)
	p.Data = trimmed
}

func (p *parser) Errorf(format string, v ...any) error {
	msg := fmt.Sprintf(format, v...)
	return fmt.Errorf("%s:%d: %s", p.File, p.Line, msg)
}

func (p *parser) Tokenise(token string) string {
	noLeadingSpace := strings.TrimLeftFunc(token, unicode.IsSpace)
	leadingSpace := strings.TrimSuffix(token, noLeadingSpace)
	i := strings.IndexFunc(noLeadingSpace, unicode.IsSpace)
	if i > 0 {
		noLeadingSpace = noLeadingSpace[:i]
	}

	return leadingSpace + noLeadingSpace
}

func (p *parser) ValidateKeyword(keyword string) error {
	if keyword == "" {
		return p.Errorf("expected a keyword, got nothing")
	}

	for _, r := range keyword {
		if ('a' <= r && r <= 'z') || r == '-' {
			continue
		}

		return p.Errorf("expected a keyword, got invalid rune %q", r)
	}

	return nil
}

// ParseComment parses an optional comment, which must be
// prefixed with at least one space. If no comment is present,
// then only trailing whitespace is allowed.
//
// If a comment is found, the parser advances to the next
// line.
func (p *parser) ParseComment() (comment string, err error) {
	line, rest, ok := strings.Cut(p.Data, "\n")
	if line == "" {
		// Nothing left on this line, advance.
		p.Data = rest
		if ok {
			p.Line++
		}

		return "", nil
	}

	prefix, suffix, ok := strings.Cut(line, " // ")
	if !ok {
		// No comment, check it's just whitespace.
		if trimmed := strings.TrimLeft(line, " "); trimmed != "" {
			return "", p.Errorf("expected a newline or comment, got %q", trimmed)
		}

		// All good, advance.
		p.Data = rest
		p.Line++
		return "", nil
	}

	// Check that anything before the slashes is spaces.
	if trimmed := strings.TrimLeft(prefix, " "); trimmed != "" {
		return "", p.Errorf("expected a newline or comment, got %q", trimmed)
	}

	// Right, the comment is valid.
	comment = suffix
	p.Data = rest
	p.Line++

	return comment, nil
}

// ParseBool parses a boolean literal starting at the
// current byte. If successful, the parser is advanced
// to immediately after the final byte.
func (p *parser) ParseBool() (b bool, err error) {
	if p.Empty() {
		return false, io.EOF
	}

	var ok bool
	p.Data, ok = strings.CutPrefix(p.Data, "true")
	if ok {
		return true, nil
	}

	p.Data, ok = strings.CutPrefix(p.Data, "false")
	if ok {
		return false, nil
	}

	return false, p.Errorf("expected a boolean, got %q", p.Tokenise(p.Data))
}

// ParseQuotedString parses a quoted string starting at
// the current byte. If successful, the parser is advanced
// to immediately after the closing quotes.
func (p *parser) ParseQuotedString() (s string, err error) {
	if p.Empty() {
		return "", io.EOF
	}

	if p.Data[0] != '"' {
		return "", p.Errorf("expected a quoted string, got %q", p.Data[0])
	}

	escaped := false
	for i := 1; i < len(p.Data); i++ {
		switch b := p.Data[i]; b {
		case '"':
			if escaped {
				escaped = false
				continue
			}

			s, err := strconv.Unquote(p.Data[:i+1])
			if err != nil {
				return "", p.Errorf("expected a quoted string, got %v", err)
			}

			p.Data = p.Data[i+1:]
			return s, nil
		case '\\':
			escaped = !escaped
		case '\n':
			return "", p.Errorf("unterminated string")
		default:
			escaped = false
		}
	}

	// We haven't found the closing quotes.
	return "", p.Errorf("unterminated string")
}

// ParseString parses an unquoted string starting at
// the current byte. If successful, the parser is advanced
// to immediately after the final byte.
func (p *parser) ParseString() (s string, err error) {
	if p.Empty() {
		return "", io.EOF
	}

	for i := 0; i < len(p.Data); i++ {
		b := p.Data[i]
		if 'a' <= b && b <= 'z' ||
			'A' <= b && b <= 'Z' ||
			'0' <= b && b <= '9' {
			continue
		}

		switch b {
		case '.', '+', '-', '_', '/', ':', '=':
			continue
		case ' ', '\n':
			s = p.Data[:i]
			if s == "" {
				if b == ' ' {
					return "", p.Errorf("expected a string, got space")
				}

				return "", p.Errorf("expected a string, got nothing")
			}

			p.Data = p.Data[i:]
			return s, nil
		default:
			return "", p.Errorf("expected a string, got %q", b)
		}
	}

	s = p.Data
	p.Data = ""

	return s, nil
}

// ParseParsedBool parses an bool starting at the
// current byte. If successful, the parser is
// advanced to the next line.
func (p *parser) ParseParsedBool() (boo ParsedBool, err error) {
	boo.Value, err = p.ParseBool()
	if err != nil {
		return boo, err
	}

	boo.Pos = p.Pos()
	boo.Comment, err = p.ParseComment()
	if err != nil {
		return boo, err
	}

	return boo, nil
}

// ParseQuotedParsedString parses a quoted string
// starting at the current byte. If successful, the
// parser is advanced to the next line.
func (p *parser) ParseQuotedParsedString() (str ParsedString, err error) {
	str.Value, err = p.ParseQuotedString()
	if err != nil {
		return str, err
	}

	str.Pos = p.Pos()
	str.Comment, err = p.ParseComment()
	if err != nil {
		return str, err
	}

	return str, nil
}

// ParseParsedString parses an unquoted string
// starting at the current byte. If successful, the
// parser is advanced to the next line.
func (p *parser) ParseParsedString() (str ParsedString, err error) {
	str.Value, err = p.ParseString()
	if err != nil {
		return str, err
	}

	str.Pos = p.Pos()
	str.Comment, err = p.ParseComment()
	if err != nil {
		return str, err
	}

	return str, nil
}

// FindColon seeks ahead to the next keyword, followed
// by a colon. The keyword must be indented by exactly
// indents tabs.
//
// If a valid keyword is found, the parser is advanced
// to immediately after the colon. Otherwise, it only
// skips leading newlines.
func (p *parser) FindColon(indents int) (keyword string, err error) {
	p.SkipNewlines()
	if p.Empty() {
		return "", io.EOF
	}

	prefix := tabs[:indents]
	before, after, ok := strings.Cut(p.Data, ":")
	if !ok {
		if !strings.HasPrefix(p.Data, prefix) {
			return "", nil // The next token is less indented.
		}

		return "", p.Errorf("expected a keyword, got invalid token %q", p.Tokenise(before))
	}

	keyword, ok = strings.CutPrefix(before, prefix)
	if !ok {
		return "", nil // The next token is less indented.
	}

	if strings.HasPrefix(keyword, tabs[:1]) {
		return "", p.Errorf("expected a keyword, got excessive indentation")
	}

	if err := p.ValidateKeyword(keyword); err != nil {
		return "", err
	}

	p.Data = after

	return keyword, nil
}

// FindKeyword seeks ahead to the given keyword, followed
// by a space. The keyword must be indented by exactly
// indents tabs.
//
// If a valid keyword is found, the parser is advanced
// to immediately after the space. Otherwise, it only
// skips leading newlines.
func (p *parser) FindKeyword(indents int, keyword string) (ok bool, err error) {
	p.SkipNewlines()
	if p.Empty() {
		return false, io.EOF
	}

	prefix := tabs[:indents]
	before, after, ok := strings.Cut(p.Data, " ")
	if !ok {
		if !strings.HasPrefix(p.Data, prefix) {
			return false, nil // The next token is less indented.
		}

		return false, p.Errorf("expected keyword %q, got invalid token %q", keyword, p.Tokenise(before))
	}

	found, ok := strings.CutPrefix(before, prefix)
	if !ok {
		return false, nil // The next token is less indented.
	}

	if strings.HasPrefix(found, tabs[:1]) {
		return false, p.Errorf("expected keyword %q, got excessive indentation", keyword)
	}

	if found != keyword {
		return false, p.Errorf("expected keyword %q, got %q", keyword, found)
	}

	p.Data = after

	return true, nil
}

// FindQuotedString seeks ahead to a quoted string at the
// given indentation.
func (p *parser) FindQuotedString(indents int) (str ParsedString, err error) {
	p.SkipNewlines()
	if p.Empty() {
		return str, io.EOF
	}

	prefix := tabs[:indents]
	before, _, ok := strings.Cut(p.Data, `"`)
	if !ok {
		if !strings.HasPrefix(p.Data, prefix) {
			return str, nil // The next token is less indented.
		}

		return str, p.Errorf("expected a quoted string, got %q", p.Tokenise(before))
	}

	leading, ok := strings.CutPrefix(before, prefix)
	if !ok {
		return str, nil // The next token is less indented.
	}

	if strings.HasPrefix(leading, tabs[:1]) {
		return str, p.Errorf("expected a quoted string, got excessive indentation")
	}

	// Advance to the beginning of the string.
	p.Data = p.Data[len(before):]
	return p.ParseQuotedParsedString()
}

// FindQuotedStringKey seeks ahead to a quoted string at the
// given indentation.
func (p *parser) FindQuotedStringKey(indents int) (string, error) {
	p.SkipNewlines()
	if p.Empty() {
		return "", io.EOF
	}

	prefix := tabs[:indents]
	before, _, ok := strings.Cut(p.Data, `"`)
	if !ok {
		if !strings.HasPrefix(p.Data, prefix) {
			return "", nil // The next token is less indented.
		}

		return "", p.Errorf("expected a quoted string, got %q", p.Tokenise(before))
	}

	leading, ok := strings.CutPrefix(before, prefix)
	if !ok {
		return "", nil // The next token is less indented.
	}

	if strings.HasPrefix(leading, tabs[:1]) {
		return "", p.Errorf("expected a quoted string, got excessive indentation")
	}

	// Advance to the beginning of the string.
	p.Data = p.Data[len(before):]
	return p.ParseQuotedString()
}

// FindQuotedStrings seeks ahead to one or more quoted
// strings at the given indentation.
func (p *parser) FindQuotedStrings(indents int) (list []ParsedString, err error) {
	first, err := p.FindQuotedString(indents)
	if err != nil {
		return nil, err
	}

	if first.Value == "" {
		return nil, p.Errorf("expected a quoted string, got %q", p.Tokenise(p.Data))
	}

	list = append(list, first)
	for {
		next, err := p.FindQuotedString(indents)
		if err == io.EOF {
			return list, nil
		}

		if err != nil {
			return nil, err
		}

		if next.Value == "" {
			return list, nil
		}

		list = append(list, next)
	}
}

// FindQuotedStringsMap seeks ahead to one or more quoted
// strings mapped to other quoted strings at the given
// indentation.
func (p *parser) FindQuotedStringsMap(indents int) (mapping map[string]ParsedString, err error) {
	key, err := p.FindQuotedStringKey(indents)
	if err != nil {
		return nil, err
	}

	if key == "" {
		return nil, p.Errorf("expected a quoted string, got %q", p.Tokenise(p.Data))
	}

	p.SkipSpaces()
	var colon bool
	p.Data, colon = strings.CutPrefix(p.Data, ":")
	if !colon {
		return nil, p.Errorf("expected a colon after map key, got %q", p.Tokenise(p.Data))
	}

	p.SkipSpaces()
	value, err := p.ParseQuotedParsedString()
	if err == io.EOF {
		return nil, p.Errorf("expected a quoted string after map key, got EOF")
	}

	if err != nil {
		return nil, err
	}

	mapping = make(map[string]ParsedString)
	mapping[key] = value

	for {
		key, err := p.FindQuotedStringKey(indents)
		if err == io.EOF {
			return mapping, nil
		}

		if err != nil {
			return nil, err
		}

		if key == "" {
			return mapping, nil
		}

		p.SkipSpaces()
		p.Data, colon = strings.CutPrefix(p.Data, ":")
		if !colon {
			return nil, p.Errorf("expected a colon after map key, got %q", p.Tokenise(p.Data))
		}

		p.SkipSpaces()
		value, err := p.ParseQuotedParsedString()
		if err == io.EOF {
			return nil, p.Errorf("expected a quoted string after map key, got EOF")
		}

		if err != nil {
			return nil, err
		}

		if prev, ok := mapping[key]; ok {
			return nil, p.Errorf("duplicate map key %q, first found at %s", key, prev.Pos)
		}

		mapping[key] = value
	}
}
