package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func main() {
	configPath := flag.String("c", "config.json", "config file path")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := runContext(ctx, *configPath); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(configPath string) error {
	return runContext(context.Background(), configPath)
}

func runContext(ctx context.Context, configPath string) error {
	cfg, err := loadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log.Printf("fetching %d sources...", len(cfg.Sources))
	sourcesData, err := fetchAll(ctx, cfg.Sources)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	container := make(map[string]*Entry)

	for i, src := range cfg.Sources {
		switch src.Type {
		case "maxmind_mmdb":
			entries, err := parseMaxmindMMDB(sourcesData[i], wantMap(src.WantedList))
			if err != nil {
				return fmt.Errorf("parse maxmind_mmdb %s: %w", redactURL(src.URL), err)
			}
			if err := applyEntries(container, entries, src.Action); err != nil {
				return fmt.Errorf("apply %s: %w", redactURL(src.URL), err)
			}
			log.Printf("parsed maxmind_mmdb: %d entries%s", len(entries), actionSuffix(src.Action))

		case "text":
			entry, err := parseText(sourcesData[i], src.Name, src.OnlyIPType)
			if err != nil {
				return fmt.Errorf("parse text %s: %w", redactURL(src.URL), err)
			}
			if err := applyEntry(container, entry, src.Action); err != nil {
				return fmt.Errorf("apply %s: %w", redactURL(src.URL), err)
			}
			log.Printf("parsed text: %s%s", strings.ToUpper(src.Name), actionSuffix(src.Action))

		case "private":
			entry, err := privateEntry()
			if err != nil {
				return fmt.Errorf("generate private entry: %w", err)
			}
			if err := applyEntry(container, entry, src.Action); err != nil {
				return fmt.Errorf("apply private ranges: %w", err)
			}
			log.Printf("added private ranges%s", actionSuffix(src.Action))

		default:
			return fmt.Errorf("unknown source type: %s", src.Type)
		}
	}

	log.Printf("writing mmdb with %d wanted entries...", len(cfg.Output.WantedList))
	if err := writeMMDB(container, cfg.Output); err != nil {
		return fmt.Errorf("write mmdb: %w", err)
	}

	return nil
}

func applyEntries(container, entries map[string]*Entry, action string) error {
	if action == "remove" {
		return removeEntries(container, entries)
	}
	return mergeEntries(container, entries)
}

func applyEntry(container map[string]*Entry, entry *Entry, action string) error {
	if action == "remove" {
		return removeEntry(container, entry)
	}
	return mergeEntry(container, entry)
}

func actionSuffix(action string) string {
	if action == "remove" {
		return " (remove)"
	}
	return ""
}
