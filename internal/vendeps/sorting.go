// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package vendeps

type SortedStrings []string

func (s SortedStrings) IsSorted() (sorted bool, firstUnsorted int) {
	for i := range s {
		if i == 0 {
			continue
		}

		if s[i-1] > s[i] {
			return false, i
		}
	}

	return true, 0
}

type SortedGoModules []*GoModule

func (s SortedGoModules) IsSorted() (sorted bool, firstUnsorted int) {
	for i := range s {
		if i == 0 {
			continue
		}

		if s[i-1].Name > s[i].Name {
			return false, i
		}
	}

	return true, 0
}

type SortedGoPackages []*GoPackage

func (s SortedGoPackages) IsSorted() (sorted bool, firstUnsorted int) {
	for i := range s {
		if i == 0 {
			continue
		}

		if s[i-1].Name > s[i].Name {
			return false, i
		}
	}

	return true, 0
}

type SortedTextFiles []*TextFiles

func (s SortedTextFiles) IsSorted() (sorted bool, firstUnsorted int) {
	for i := range s {
		if i == 0 {
			continue
		}

		if s[i-1].Name > s[i].Name {
			return false, i
		}
	}

	return true, 0
}

type SortedUpdateDeps []*UpdateDep

func (s SortedUpdateDeps) IsSorted() (sorted bool, firstUnsorted int) {
	for i := range s {
		if i == 0 {
			continue
		}

		if s[i-1].Name > s[i].Name {
			return false, i
		}
	}

	return true, 0
}
