package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

func main() {
	configPath := flag.String("c", "config.json", "config file path")
	flag.Parse()

	if err := run(*configPath); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(configPath string) error {
	cfg, err := loadConfig(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log.Printf("fetching %d sources...", len(cfg.Sources))
	dataMap, err := fetchAll(context.Background(), cfg.Sources)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	container := make(map[string]*Entry)

	for i, src := range cfg.Sources {
		switch src.Type {
		case "maxmind_mmdb":
			entries, err := parseMaxmindMMDB(dataMap[i], wantMap(src.WantedList))
			if err != nil {
				return fmt.Errorf("parse maxmind_mmdb %s: %w", src.URL, err)
			}
			mergeEntries(container, entries)
			log.Printf("parsed maxmind_mmdb: %d entries", len(entries))

		case "ipinfo_mmdb":
			entries, err := parseIPInfoMMDB(dataMap[i], wantMap(src.WantedList))
			if err != nil {
				return fmt.Errorf("parse ipinfo_mmdb %s: %w", src.URL, err)
			}
			mergeEntries(container, entries)
			log.Printf("parsed ipinfo_mmdb: %d entries", len(entries))

		case "text":
			entry, err := parseText(dataMap[i], src.Name, src.OnlyIPType)
			if err != nil {
				return fmt.Errorf("parse text %s: %w", src.URL, err)
			}
			mergeEntry(container, entry)
			log.Printf("parsed text: %s", strings.ToUpper(src.Name))

		case "private":
			entry, err := privateEntry()
			if err != nil {
				return fmt.Errorf("generate private entry: %w", err)
			}
			mergeEntry(container, entry)
			log.Printf("added private ranges")

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
