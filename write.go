package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/maxmind/mmdbwriter"
	"github.com/maxmind/mmdbwriter/mmdbtype"
)

func writeMMDB(entries map[string]*Entry, output Output) error {
	writer, err := mmdbwriter.New(mmdbwriter.Options{
		DatabaseType:            "GeoLite2-Country",
		Description:             map[string]string{"en": "GeoIP-CN Country database"},
		RecordSize:              28,
		IncludeReservedNetworks: true,
	})
	if err != nil {
		return fmt.Errorf("create mmdb writer: %w", err)
	}

	for _, name := range output.WantedList {
		name = strings.ToUpper(strings.TrimSpace(name))
		entry, ok := entries[name]
		if !ok {
			log.Printf("warning: entry %s not found, skipping", name)
			continue
		}

		record := mmdbtype.Map{
			"country": mmdbtype.Map{
				"iso_code": mmdbtype.String(name),
			},
		}

		prefixes, err := entry.Prefixes()
		if err != nil {
			return fmt.Errorf("get prefixes for %s: %w", name, err)
		}

		for _, prefix := range prefixes {
			_, network, err := net.ParseCIDR(prefix.String())
			if err != nil {
				return fmt.Errorf("parse CIDR %s: %w", prefix, err)
			}
			if err := writer.Insert(network, record); err != nil {
				return fmt.Errorf("insert %s: %w", prefix, err)
			}
		}

		log.Printf("added %s: %d prefixes", name, len(prefixes))
	}

	if err := os.MkdirAll(output.Dir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	outPath := filepath.Join(output.Dir, output.File)
	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer f.Close()

	if _, err := writer.WriteTo(f); err != nil {
		return fmt.Errorf("write mmdb: %w", err)
	}

	log.Printf("wrote %s", outPath)
	return nil
}
