// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package update

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/ProjectSerenity/vdm/internal/simplehttp"

	"golang.org/x/time/rate"
)

func TestUpdateGoModule(t *testing.T) {
	tests := []struct {
		Name    string
		ModName string
		Version string
		Want    string
		Updated bool
		Error   string
	}{
		{
			Name:    "bad-module-name",
			ModName: "-!",
			Error:   `failed to look up Go module -!: invalid module path: malformed module path "-!": leading dash`,
		},
		{
			Name:    "valid-equal",
			ModName: "rsc.io/diff",
			Version: "v1.2.3",
			Want:    "v1.2.3",
			Updated: false,
		},
		{
			Name:    "valid-newer",
			ModName: "rsc.io/diff",
			Version: "v1.2.2",
			Want:    "v1.2.3",
			Updated: true,
		},
		{
			Name:    "valid-older",
			ModName: "rsc.io/diff",
			Version: "v1.2.4",
			Want:    "v1.2.4",
			Updated: false,
		},
	}

	ctx := context.Background()
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"Version":"v1.2.3"}`)
	}

	srv := httptest.NewTLSServer(http.HandlerFunc(handler))
	defer srv.Close()

	simplehttp.Client = srv.Client()
	simplehttp.UserAgent = "tests"
	simplehttp.RateLimit = rate.NewLimiter(rate.Every(time.Millisecond), 1)

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got := test.Version
			updated, err := UpdateGoModule(ctx, io.Discard, srv.URL, test.ModName, &got)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("UpdateGoModule(): unexpected lack of error")
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("UpdateGoModule(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("UpdateGoModule(): got unexpected error: %v", err)
			}

			if got != test.Want {
				t.Errorf("UpdateGoModule(): version mismatch\nGot:  %q\nWant: %q", got, test.Want)
			}

			if updated != test.Updated {
				t.Errorf("UpdateGoModule(): updated mismatch\nGot:  %v\nWant: %v", updated, test.Updated)
			}
		})
	}
}

func TestLatest(t *testing.T) {
	tests := []struct {
		Name    string
		Ctx     context.Context
		Addr    string
		ModName string
		Payload string
		Want    string
		Error   string
	}{
		{
			Name:    "bad-module-name",
			ModName: "-!",
			Error:   `failed to look up Go module -!: invalid module path: malformed module path "-!": leading dash`,
		},
		{
			Name:    "bad-context",
			Ctx:     nil,
			ModName: "rsc.io/diff",
			Error:   `failed to look up Go module rsc.io/diff: net/http: nil Context`,
		},
		{
			Name:    "bad-address",
			Ctx:     context.Background(),
			Addr:    "http://0.0.0.0:0",
			ModName: "rsc.io/diff",
			Error:   `failed to look up Go module rsc.io/diff: Get "http://0.0.0.0:0/rsc.io/diff/@latest": dial tcp 0.0.0.0:0: connect: connection refused`,
		},
		{
			Name:    "bad-payload",
			Ctx:     context.Background(),
			ModName: "rsc.io/diff",
			Payload: `!`,
			Error:   `failed to parse response for Go module rsc.io/diff: invalid character '!' looking for beginning of value`,
		},
		{
			Name:    "bad-version",
			Ctx:     context.Background(),
			ModName: "rsc.io/diff",
			Payload: `{"Version":"v!"}`,
			Error:   `failed to check Go module rsc.io/diff for updates: latest version "v!" is invalid`,
		},
		{
			Name:    "valid-version",
			Ctx:     context.Background(),
			ModName: "rsc.io/diff",
			Payload: `{"Version":"v1.2.3"}`,
			Want:    `v1.2.3`,
		},
	}

	mutex := new(sync.Mutex)
	payload := ""
	handler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mutex.Lock()
		send := payload
		mutex.Unlock()
		io.WriteString(w, send)
	}

	srv := httptest.NewTLSServer(http.HandlerFunc(handler))
	defer srv.Close()

	simplehttp.Client = srv.Client()
	simplehttp.UserAgent = "tests"
	simplehttp.RateLimit = rate.NewLimiter(rate.Every(time.Millisecond), 1)

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			mutex.Lock()
			payload = test.Payload
			mutex.Unlock()

			addr := srv.URL
			if test.Addr != "" {
				addr = test.Addr
			}

			got, err := Latest(test.Ctx, addr, test.ModName)
			if test.Error != "" {
				if err == nil {
					t.Fatalf("Latest(): unexpected lack of error")
				}

				e := err.Error()
				if e != test.Error {
					t.Fatalf("Latest(): got wrong error:\nGot:  %s\nWant: %s", e, test.Error)
				}

				// All good.
				return
			}

			if err != nil {
				t.Fatalf("Latest(): got unexpected error: %v", err)
			}

			if got != test.Want {
				t.Errorf("Latest(): version mismatch\nGot:  %q\nWant: %q", got, test.Want)
			}
		})
	}
}
