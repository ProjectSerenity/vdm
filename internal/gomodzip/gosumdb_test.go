package gomodzip

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ProjectSerenity/vdm/internal/simplehttp"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/mod/sumdb"
)

func TestChecksumClient(t *testing.T) {
	// We record the data produced by querying
	// the checksum database in gen-sumdb-recordings.go
	// and store the data in testdata/. In this
	// test, we replay the recorded data so we
	// can test the functionality without any
	// connectivity to the real Go checksum
	// database.
	//
	// We do the test twice, once with the
	// recorded cache and once without.

	mux := http.NewServeMux()
	addRecordingHandlers(mux)
	srv := httptest.NewTLSServer(mux)
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	defer srv.Close()

	u, err := url.Parse(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}

	simplehttp.Client = srv.Client()
	simplehttp.UserAgent = "tests"
	simplehttp.SetInterval(time.Millisecond)

	t.Run("without-cache", func(t *testing.T) {
		ops := clientOps(io.Discard, t.ArtifactDir())
		ops.host = u.Host
		ops.configDir = filepath.Join(t.ArtifactDir(), "config")
		ops.cacheDir = filepath.Join(t.ArtifactDir(), "cache")

		writeConfigs(t, ops.configDir)

		client := sumdb.NewClient(ops)
		got, err := client.Lookup("rsc.io/diff", "v0.0.0-20190621135850-fe3479844c3c")
		if err != nil {
			t.Fatalf("client.Lookup(): unexpected error: %v", err)
		}

		if diff := cmp.Diff(recordedResponseLines, got); diff != "" {
			t.Errorf("client.Lookup(): lines mismatch (-want, +got)\n%s", diff)
		}
	})

	t.Run("with-cache", func(t *testing.T) {
		ops := clientOps(io.Discard, t.ArtifactDir())
		ops.host = u.Host
		ops.configDir = filepath.Join(t.ArtifactDir(), "config")
		ops.cacheDir = filepath.Join(t.ArtifactDir(), "cache")

		writeConfigs(t, ops.configDir)
		writeCache(t, ops.cacheDir)

		client := sumdb.NewClient(ops)
		got, err := client.Lookup("rsc.io/diff", "v0.0.0-20190621135850-fe3479844c3c")
		if err != nil {
			t.Fatalf("client.Lookup(): unexpected error: %v", err)
		}

		if diff := cmp.Diff(recordedResponseLines, got); diff != "" {
			t.Errorf("client.Lookup(): lines mismatch (-want, +got)\n%s", diff)
		}
	})
}

func TestChecksumClientOps_ReadConfig(t *testing.T) {
	tests := []struct {
		Name  string
		File  string
		Data  []byte
		Want  []byte
		Error string
	}{
		{
			Name: "simple",
			File: "file.txt",
			Data: []byte("foo"),
			Want: []byte("foo"),
		},
		{
			Name: "key",
			File: "key",
			Want: []byte(goChecksumKey),
		},
		{
			Name:  "nonexistant",
			File:  "nonexistant",
			Error: "no such file or directory",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			dir := t.ArtifactDir()
			name := filepath.Join(dir, test.File)
			if test.Data != nil {
				err := os.WriteFile(name, test.Data, 0o755)
				if err != nil {
					t.Fatalf("failed to write %q: %v", name, err)
				}
			}

			ops := &checksumClientOps{
				publicKey: goChecksumKey,
				configDir: dir,
			}

			got, err := ops.ReadConfig(test.File)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("ReadConfig(): unexpected lack of error")
				}

				e := err.Error()
				if !strings.Contains(e, test.Error) {
					t.Fatalf("ReadConfig(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("ReadConfig(): got unexpected error: %v", err)
			}

			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Errorf("ReadConfig(): data mismatch (-want, +got)\n%s", diff)
			}
		})
	}
}

func TestChecksumClientOps_WriteConfig(t *testing.T) {
	tests := []struct {
		Name  string
		File  string
		Data  []byte
		Old   []byte
		New   []byte
		Want  []byte
		Error string
	}{
		{
			Name: "simple",
			File: "file.txt",
			Data: []byte("foo"),
			Old:  []byte("foo"),
			New:  []byte("bar"),
			Want: []byte("bar"),
		},
		{
			Name:  "conflict",
			File:  "file.txt",
			Data:  []byte("foo"),
			Old:   []byte("bar"),
			New:   []byte("baz"),
			Error: "write conflict",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			dir := t.ArtifactDir()
			name := filepath.Join(dir, test.File)
			if test.Data != nil {
				err := os.WriteFile(name, test.Data, 0o755)
				if err != nil {
					t.Fatalf("failed to write %q: %v", name, err)
				}
			}

			ops := &checksumClientOps{
				publicKey: goChecksumKey,
				configDir: dir,
			}

			err := ops.WriteConfig(test.File, test.Old, test.New)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("WriteConfig(): unexpected lack of error")
				}

				e := err.Error()
				if !strings.Contains(e, test.Error) {
					t.Fatalf("WriteConfig(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("WriteConfig(): got unexpected error: %v", err)
			}

			got, err := ops.ReadConfig(test.File)
			if err != nil {
				t.Fatalf("ReadConfig(): got unexpected error: %v", err)
			}

			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Errorf("WriteConfig(): data mismatch (-want, +got)\n%s", diff)
			}
		})
	}
}

func TestChecksumClientOps_Log(t *testing.T) {
	tests := []struct {
		Name string
		Msg  string
		Want string
	}{
		{
			Name: "newline",
			Msg:  "foo\n",
			Want: "foo\n",
		},
		{
			Name: "without-newline",
			Msg:  "foo",
			Want: "foo\n",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			t.Run("Log", func(t *testing.T) {
				var buf bytes.Buffer
				ops := &checksumClientOps{log: &buf}
				ops.Log(test.Msg)
				got := buf.String()
				if got != test.Want {
					t.Fatalf("Log(%q): got wrong message:\nGot:  %q\nWant: %q", test.Msg, got, test.Want)
				}
			})

			t.Run("SecurityError", func(t *testing.T) {
				var buf bytes.Buffer
				ops := &checksumClientOps{log: &buf}
				ops.SecurityError(test.Msg)
				got := buf.String()
				if got != test.Want {
					t.Fatalf("Log(%q): got wrong message:\nGot:  %q\nWant: %q", test.Msg, got, test.Want)
				}
			})
		})
	}
}
