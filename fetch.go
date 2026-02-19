package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
)

func fetchAll(ctx context.Context, sources []Source) (map[int][]byte, error) {
	var mu sync.Mutex
	results := make(map[int][]byte)

	g, ctx := errgroup.WithContext(ctx)
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
		return os.ReadFile(url)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	return io.ReadAll(resp.Body)
}
