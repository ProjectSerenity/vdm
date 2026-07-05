package vdm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func lines(lines ...string) string {
	return strings.Join(lines, "\n")
}

var fuzzSeeds = []string{
	// From TestParseDeps.
	`not-a-keyword`,
	lines(
		`banned-go-packages:`,
		`	"example.com/foo"`,
		`banned-go-packages:`,
	),
	lines(

		`banned-go-packages:`,
		``,
	),
	lines(
		`banned-go-packages:`,
		`	"first"`,
		`		"second"`,
	),
	lines(
		`banned-go-packages:`,
		`	"first"`,
		`	"second" // Check comments work.`,
	),
	lines(
		`go-modules:`,
		`	module "example.com/foo" v1.2.3`,
		`go-modules:`,
	),
	lines(
		`go-modules:`,
		``,
	),
	lines(
		`go-modules:`,
		`	"first"`,
		`		"second"`,
	),
	lines(
		`go-modules:`,
		`	module "first" v1.2.3`,
	),
	lines(
		``,
		`something-bizarre:`,
	),
}

func FuzzParseDeps(f *testing.F) {
	testFiles := []string{
		"empty",
		"simple",
		"commented",
		"complex",
		"complex-sorted",
	}

	for _, testFile := range testFiles {
		data, err := os.ReadFile(filepath.Join("testdata", "fuzz-seeds", testFile+".vdm"))
		if err != nil {
			f.Fatalf("failed to open input %s: %v", testFile, err)
		}

		f.Add(string(data))
	}

	for _, seed := range fuzzSeeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data string) {
		ParseDeps("deps.vdm", data)
	})
}
