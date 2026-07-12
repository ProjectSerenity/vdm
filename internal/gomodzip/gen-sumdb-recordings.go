//go:build script

// This is a helper script for generating test data for the gomodzip package.
//
// It performs a sumdb lookup for rsc.io/diff@v0.0.0-20190621135850-fe3479844c3c,
// recording the HTTP response and config entry data to disk. This can then be
// replayed in tests to check the package's behaviour without needing connectivity
// to the real Go checksum database.

package main

import (
	"bytes"
	"fmt"
	"go/format"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"text/template"

	"golang.org/x/mod/sumdb"
)

func init() {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)
	log.SetPrefix(filepath.Base(os.Args[0]) + ": ")
}

func main() {
	tmpl, err := template.New("recordings_test.go").Parse(goTemplate)
	if err != nil {
		log.Fatalf("Failed to parse template: %v", err)
	}

	ops := &ChecksumClientOps{
		host:      goChecksumHost,
		publicKey: goChecksumKey,
	}

	client := sumdb.NewClient(ops)

	// Make the recording.
	lines, err := client.Lookup("rsc.io/diff", "v0.0.0-20190621135850-fe3479844c3c")
	if err != nil {
		log.Fatalf("Failed to lookup module: %v", err)
	}

	output := &Output{
		Lines:     lines,
		Configs:   ops.configs,
		Cache:     ops.cache,
		Responses: ops.responses,
	}

	// Clear out any old data.
	dir := filepath.Join("testdata", "recordings")
	err = os.RemoveAll(dir)
	if err != nil {
		log.Fatalf("Failed to remove %s: %v", dir, err)
	}

	// Store our recordings.
	err = os.MkdirAll(dir, 0o777)
	if err != nil {
		log.Fatalf("Failed to create %s: %v", dir, err)
	}

	for _, config := range output.Configs {
		name := filepath.Join(dir, fmt.Sprintf("config%d", config.Number))
		err = os.WriteFile(name, config.Payload, 0o644)
		if err != nil {
			log.Fatalf("Failed to write config%d (%q): %v", config.Number, config.Path, err)
		}
	}

	for _, cache := range output.Cache {
		name := filepath.Join(dir, fmt.Sprintf("cache%d", cache.Number))
		err = os.WriteFile(name, cache.Payload, 0o644)
		if err != nil {
			log.Fatalf("Failed to write cache%d (%q): %v", cache.Number, cache.Path, err)
		}
	}

	for _, response := range output.Responses {
		name := filepath.Join(dir, fmt.Sprintf("response%d", response.Number))
		err = os.WriteFile(name, response.Payload, 0o644)
		if err != nil {
			log.Fatalf("Failed to write response%d (%q): %v", response.Number, response.Path, err)
		}
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, output)
	if err != nil {
		log.Fatalf("Failed to generate recordings_test.go: %v", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		log.Printf("Failed to format recordings_test.go: %v", err)
		formatted = buf.Bytes()
	}

	err = os.WriteFile("recordings_test.go", formatted, 0644)
	if err != nil {
		log.Fatalf("Failed to write recordings_test.go: %v", err)
	}
}

type Output struct {
	Lines     []string
	Configs   []*SavedFile
	Cache     []*SavedFile
	Responses []*SavedResponse
}

const (
	// Public key for sum.golang.org. See go/src/cmd/go/internal/modfetch/key.go
	goChecksumHost = "sum.golang.org"
	goChecksumKey  = "sum.golang.org+033de0ae+Ac4zctda0e5eza+HJyk9SxEdh+s3Ux18htTTAD8OuAn8"
)

type ChecksumClientOps struct {
	host      string
	publicKey string

	mutex     sync.Mutex
	configs   []*SavedFile
	cache     []*SavedFile
	responses []*SavedResponse
}

type SavedFile struct {
	Number  int
	Path    string
	Payload []byte
}

type SavedResponse struct {
	Number  int
	Path    string
	Headers http.Header
	Payload []byte
}

func (c *ChecksumClientOps) ReadRemote(path string) ([]byte, error) {
	fullURL := (&url.URL{Scheme: "https", Host: c.host, Path: path}).String()
	req, err := http.NewRequest(http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		io.Copy(io.Discard, res.Body)
		res.Body.Close()
		return nil, fmt.Errorf("%s returned status code %d", c.host, res.StatusCode)
	}

	data, err := io.ReadAll(res.Body)
	res.Body.Close()

	header := make(http.Header)
	fields := []string{
		"Content-Type",
	}

	for _, field := range fields {
		if values, ok := res.Header[field]; ok {
			header[field] = slices.Clone(values)
		}
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.responses = append(c.responses, &SavedResponse{
		Number:  len(c.responses) + 1,
		Path:    path,
		Headers: header,
		Payload: bytes.Clone(data),
	})

	return data, err
}

func (c *ChecksumClientOps) ReadConfig(file string) ([]byte, error) {
	if file == "key" {
		return []byte(c.publicKey), nil
	}

	if file == c.host+"/latest" {
		return []byte{}, nil
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	for _, f := range c.configs {
		if f.Path == file {
			return bytes.Clone(f.Payload), nil
		}
	}

	return nil, fmt.Errorf("no such config: %s", file)
}

func (c *ChecksumClientOps) WriteConfig(file string, old, new []byte) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for _, f := range c.configs {
		if f.Path == file {
			if bytes.Equal(f.Payload, old) {
				f.Payload = new
				return nil
			}

			return sumdb.ErrWriteConflict
		}
	}

	c.configs = append(c.configs, &SavedFile{
		Number:  len(c.configs) + 1,
		Path:    file,
		Payload: new,
	})

	return nil
}

func (c *ChecksumClientOps) ReadCache(file string) ([]byte, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for _, f := range c.cache {
		if f.Path == file {
			return bytes.Clone(f.Payload), nil
		}
	}

	return nil, fmt.Errorf("no such cache: %s", file)
}

func (c *ChecksumClientOps) WriteCache(file string, data []byte) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for _, f := range c.cache {
		if f.Path == file {
			f.Payload = data
			return
		}
	}

	c.cache = append(c.cache, &SavedFile{
		Number:  len(c.cache) + 1,
		Path:    file,
		Payload: data,
	})
}

func (c *ChecksumClientOps) Log(msg string) {
	log.Print(msg)
}

func (c *ChecksumClientOps) SecurityError(msg string) {
	log.Print(msg)
}

const goTemplate = `// Code generated by gen-sumdb-recordings.go. DO NOT EDIT.

package gomodzip

import (
	_ "embed"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

var recordedResponseLines = []string{ {{- range .Lines }}
	{{ printf "%q" . }},{{ end }}
}{{ range .Configs }}

//go:embed testdata/recordings/config{{ .Number }}
var config{{ .Number }} []byte{{ end }}{{ range .Cache }}

//go:embed testdata/recordings/cache{{ .Number }}
var cache{{ .Number }} []byte{{ end }}{{ range .Responses }}

//go:embed testdata/recordings/response{{ .Number }}
var response{{ .Number }} []byte{{ end }}

func copyFile(t *testing.T, fromDir, from, toDir, to string) {
	t.Helper()

	to = filepath.Join(toDir, filepath.FromSlash(to))
	from = filepath.Join(fromDir, filepath.FromSlash(from))

	toDir = filepath.Dir(to)
	err := os.MkdirAll(toDir, 0o777)
	if err != nil {
		t.Fatalf("failed to create %q: %v", toDir, err)
	}

	w, err := os.Create(to)
	if err != nil {
		t.Fatalf("failed to create %q: %v", to, err)
	}

	defer w.Close()

	r, err := os.Open(from)
	if err != nil {
		t.Fatalf("failed to open %q: %v", from, err)
	}

	_, err = io.Copy(w, r)
	if err != nil {
		t.Fatalf("failed to copy %q to %q: %v", from, to, err)
	}

	err = w.Close()
	if err != nil {
		t.Fatalf("failed to close %q: %v", to, err)
	}

	err = r.Close()
	if err != nil {
		t.Fatalf("failed to close %q: %v", from, err)
	}
}

func writeConfigs(t *testing.T, dir string) {
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		t.Fatalf("failed to create %s: %v", dir, err)
	}

	from := filepath.Join("testdata", "recordings"){{ range .Configs }}
	copyFile(t, from, "config{{ .Number }}", dir, {{ printf "%q" .Path }}){{ end }}
}

func writeCache(t *testing.T, dir string) {
	err := os.MkdirAll(dir, 0o755)
	if err != nil {
		t.Fatalf("failed to create %s: %v", dir, err)
	}

	from := filepath.Join("testdata", "recordings"){{ range .Cache }}
	copyFile(t, from, "cache{{ .Number }}", dir, {{ printf "%q" .Path }}){{ end }}
}

func addHandler(mux *http.ServeMux, pattern string, payload []byte, header http.Header) {
	mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		for key, values := range header {
			h[key] = slices.Clone(values)
		}

		w.Write(payload)
	})
}

func addRecordingHandlers(mux *http.ServeMux) { {{- range .Responses }}
	addHandler(mux, "GET {{ .Path }}", response{{ .Number }}, http.Header{ {{- range $key, $value := .Headers }}
		{{ printf "%q" $key }}: []string{ {{- range $value }}{{ printf "%q, " . }}{{ end -}} },{{ end }}
	}){{ end }}
}
`
