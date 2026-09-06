package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestFetchHTTPRetriesTransientFailures(t *testing.T) {
	for _, status := range []int{408, 429, 500, 503, 404, 401} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) == 1 {
					w.WriteHeader(status)
					return
				}
				fmt.Fprint(w, "1.2.3.0/24")
			}))
			defer server.Close()
			data, err := fetchHTTP(t.Context(), server.Client(), server.URL, 1024)
			if status == 401 || status == 404 {
				if err == nil || calls.Load() != 1 {
					t.Fatalf("permanent failure: calls=%d, error=%v", calls.Load(), err)
				}
			} else if err != nil || string(data) != "1.2.3.0/24" || calls.Load() != 2 {
				t.Fatalf("retry: calls=%d, data=%q, error=%v", calls.Load(), data, err)
			}
		})
	}
}

func TestFetchHTTPEnforcesSizeLimit(t *testing.T) {
	for _, chunked := range []bool{false, true} {
		t.Run(fmt.Sprint(chunked), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if chunked {
					w.(http.Flusher).Flush()
				}
				fmt.Fprint(w, strings.Repeat("x", 17))
			}))
			defer server.Close()
			if _, err := fetchHTTP(t.Context(), server.Client(), server.URL, 16); !errors.Is(err, errSourceTooLarge) {
				t.Fatalf("error = %v, want size limit", err)
			}
			if calls.Load() != 1 {
				t.Fatal("oversized response was retried")
			}
		})
	}
	data, err := readSource(strings.NewReader("1234567890123456"), 16)
	if err != nil || len(data) != 16 {
		t.Fatalf("exact limit rejected: data=%q, error=%v", data, err)
	}
}

func TestFetchHTTPBodyTimeoutAndRetryLimit(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer server.Close()
	client := server.Client()
	client.Timeout = 100 * time.Millisecond
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	_, err := fetchHTTP(ctx, client, server.URL, 1024)
	if err == nil {
		t.Fatal("stalled response did not time out")
	}
	if calls.Load() != fetchAttempts {
		t.Fatalf("calls = %d, want %d", calls.Load(), fetchAttempts)
	}
	if ctx.Err() != nil {
		t.Fatalf("parent timeout, not client timeout, stopped download: %v", ctx.Err())
	}
	if sourceHTTPClient.Timeout <= 0 {
		t.Fatal("production client has no timeout")
	}
}

func TestFetchHTTPCancellationStopsBackoff(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	if _, err := fetchHTTP(ctx, server.Client(), server.URL, 1024); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("calls = %d, retry continued after cancellation", calls.Load())
	}
}

func TestFetchHTTPDiscardsTruncatedAttempt(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Content-Length", "100")
			fmt.Fprint(w, "partial")
			return
		}
		fmt.Fprint(w, "complete")
	}))
	defer server.Close()
	data, err := fetchHTTP(t.Context(), server.Client(), server.URL, 1024)
	if err != nil || string(data) != "complete" || calls.Load() != 2 {
		t.Fatalf("calls=%d, data=%q, error=%v", calls.Load(), data, err)
	}
}

func TestFetchAllAndLocalSizeLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.txt")
	if err := os.WriteFile(path, []byte("local"), 0600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "remote") }))
	defer server.Close()
	data, err := fetchAll(t.Context(), []Source{{Type: "text", URL: path}, {Type: "private"}, {Type: "text", URL: server.URL}})
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 2 || string(data[0]) != "local" || string(data[2]) != "remote" {
		t.Fatalf("source ordering lost: %v", data)
	}
	if err := os.Truncate(path, maxSourceBytes+1); err != nil {
		t.Fatal(err)
	}
	if _, err := fetchOne(t.Context(), path); !errors.Is(err, errSourceTooLarge) {
		t.Fatalf("oversized local file: %v", err)
	}
}

func TestFetchAllCancelsOtherDownloads(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/slow" {
			w.(http.Flusher).Flush()
			close(started)
			<-r.Context().Done()
			return
		}
		select {
		case <-started:
		case <-r.Context().Done():
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	_, err := fetchAll(ctx, []Source{{Type: "text", URL: server.URL + "/slow"}, {Type: "text", URL: server.URL + "/missing"}})
	if err == nil || !strings.Contains(err.Error(), "HTTP 404") {
		t.Fatalf("error = %v, want source failure", err)
	}
	if ctx.Err() != nil {
		t.Fatal("peer download required parent timeout to stop")
	}
}
