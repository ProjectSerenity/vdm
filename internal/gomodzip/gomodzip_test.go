package gomodzip

import (
	"testing"

	"github.com/ProjectSerenity/vdm/internal/ves"
)

func s(str string) ves.ParsedString { return ves.ParsedString{Value: str} }

func TestExtractChecksum(t *testing.T) {
	tests := []struct {
		Name  string
		Sum   string
		Want  string
		Error string
	}{
		{
			Name:  "invalid-unformatted",
			Sum:   "checksum",
			Error: `invalid checksum "checksum": missing colon`,
		},
		{
			Name:  "invalid-unknown",
			Sum:   "zzz:checksum",
			Error: `invalid checksum "zzz:checksum": unrecognised format "zzz"`,
		},
		{
			Name: "valid-simple",
			Sum:  "h1:checksum",
			Want: "sha256:checksum",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got, err := extractChecksum(test.Sum)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("extractChecksum(%q): unexpected lack of error", test.Sum)
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("extractChecksum(%q): got wrong error:\nGot:  %s\nWant: %s", test.Sum, e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("extractChecksum(%q): got unexpected error: %v", test.Sum, err)
			}

			if got != test.Want {
				t.Fatalf("extractChecksum(%q):\nGot:  %q\nWant: %q", test.Sum, got, test.Want)
			}
		})
	}
}
