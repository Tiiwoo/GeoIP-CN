package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	maxSourceBytes = 512 << 20
	fetchAttempts  = 3
	fetchTimeout   = 2 * time.Minute
)

var (
	sourceHTTPClient  = &http.Client{Timeout: fetchTimeout}
	errSourceTooLarge = errors.New("source size limit exceeded")
)

func fetchAll(ctx context.Context, sources []Source) (map[int][]byte, error) {
	var mu sync.Mutex
	results := make(map[int][]byte)

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(4)
	for i, src := range sources {
		if src.Type == "private" {
			continue
		}
		g.Go(func() error {
			data, err := fetchOne(ctx, src.URL)
			if err != nil {
				return fmt.Errorf("fetch %s: %w", src.URL, err)
			}
			mu.Lock()
			results[i] = data
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}

func fetchOne(ctx context.Context, url string) ([]byte, error) {
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		f, err := os.Open(url)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			return nil, err
		}
		if info.Size() > maxSourceBytes {
			return nil, errSourceTooLarge
		}
		return readSource(f, maxSourceBytes)
	}
	return fetchHTTP(ctx, sourceHTTPClient, url, maxSourceBytes)
}

// Retry transient HTTP and transport failures, including interrupted bodies.
func fetchHTTP(ctx context.Context, client *http.Client, url string, maxBytes int64) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < fetchAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if attempt > 0 {
			timer := time.NewTimer(250 * time.Millisecond << (attempt - 1))
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
			if resp.StatusCode != http.StatusRequestTimeout && resp.StatusCode != http.StatusTooManyRequests &&
				(resp.StatusCode < 500 || resp.StatusCode > 599) {
				return nil, lastErr
			}
			continue
		}
		if resp.ContentLength > maxBytes {
			resp.Body.Close()
			return nil, errSourceTooLarge
		}
		data, err := readSource(resp.Body, maxBytes)
		resp.Body.Close()
		if err == nil {
			return data, nil
		}
		if errors.Is(err, errSourceTooLarge) {
			return nil, err
		}
		lastErr = err
	}
	return nil, fmt.Errorf("download failed after %d attempts: %w", fetchAttempts, lastErr)
}

func readSource(r io.Reader, maxBytes int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, errSourceTooLarge
	}
	return data, nil
}
