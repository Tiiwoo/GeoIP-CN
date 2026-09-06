package main

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	maxminddb "github.com/oschwald/maxminddb-golang/v2"
)

func parseMaxmindMMDB(data []byte, want map[string]bool) (map[string]*Entry, error) {
	db, err := maxminddb.OpenBytes(data)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	entries := make(map[string]*Entry)
	for result := range db.Networks() {
		var record struct {
			Country struct {
				IsoCode string `maxminddb:"iso_code"`
			} `maxminddb:"country"`
			RegisteredCountry struct {
				IsoCode string `maxminddb:"iso_code"`
			} `maxminddb:"registered_country"`
			RepresentedCountry struct {
				IsoCode string `maxminddb:"iso_code"`
			} `maxminddb:"represented_country"`
		}
		if err := result.Decode(&record); err != nil {
			return nil, err
		}

		var name string
		switch {
		case strings.TrimSpace(record.Country.IsoCode) != "":
			name = strings.ToUpper(strings.TrimSpace(record.Country.IsoCode))
		case strings.TrimSpace(record.RegisteredCountry.IsoCode) != "":
			name = strings.ToUpper(strings.TrimSpace(record.RegisteredCountry.IsoCode))
		case strings.TrimSpace(record.RepresentedCountry.IsoCode) != "":
			name = strings.ToUpper(strings.TrimSpace(record.RepresentedCountry.IsoCode))
		}

		if name == "" {
			continue
		}
		if len(want) > 0 && !want[name] {
			continue
		}

		entry, ok := entries[name]
		if !ok {
			entry = NewEntry(name)
			entries[name] = entry
		}
		if err := entry.addPrefix(result.Prefix()); err != nil {
			return nil, err
		}
	}
	return entries, validateParsedEntries(entries, want)
}

func parseIPInfoMMDB(data []byte, want map[string]bool) (map[string]*Entry, error) {
	db, err := maxminddb.OpenBytes(data)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	entries := make(map[string]*Entry)
	for result := range db.Networks() {
		var record struct {
			Country     string `maxminddb:"country"`
			CountryCode string `maxminddb:"country_code"`
		}
		if err := result.Decode(&record); err != nil {
			return nil, err
		}

		name := strings.ToUpper(strings.TrimSpace(record.CountryCode))
		if name == "" {
			name = strings.ToUpper(strings.TrimSpace(record.Country))
		}
		if name == "" {
			continue
		}
		if len(want) > 0 && !want[name] {
			continue
		}

		entry, ok := entries[name]
		if !ok {
			entry = NewEntry(name)
			entries[name] = entry
		}
		if err := entry.addPrefix(result.Prefix()); err != nil {
			return nil, err
		}
	}
	return entries, validateParsedEntries(entries, want)
}

func parseText(data []byte, name string, onlyIPType string) (*Entry, error) {
	if onlyIPType != "" && onlyIPType != "ipv4" && onlyIPType != "ipv6" {
		return nil, fmt.Errorf("invalid onlyIPType %q", onlyIPType)
	}
	entry := NewEntry(name)
	if entry.name == "" {
		return nil, fmt.Errorf("text source name is required")
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	lineNumber, count := 0, 0
	for scanner.Scan() {
		lineNumber++
		prefix, err := parsePrefix(scanner.Text())
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNumber, err)
		}
		if !prefix.IsValid() {
			continue
		}

		if onlyIPType == "ipv4" && !prefix.Addr().Is4() {
			continue
		}
		if onlyIPType == "ipv6" && !prefix.Addr().Is6() {
			continue
		}

		if err := entry.addPrefix(prefix); err != nil {
			return nil, err
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, fmt.Errorf("text source %s contains no matching IP ranges", entry.name)
	}
	return entry, nil
}

func validateParsedEntries(entries map[string]*Entry, want map[string]bool) error {
	if len(entries) == 0 {
		return fmt.Errorf("MMDB source contains no matching IP ranges")
	}
	for name := range want {
		if _, ok := entries[name]; !ok {
			return fmt.Errorf("MMDB source is missing requested entry %s", name)
		}
	}
	return nil
}

var privateCIDRs = []string{
	"0.0.0.0/8",
	"10.0.0.0/8",
	"100.64.0.0/10",
	"127.0.0.0/8",
	"169.254.0.0/16",
	"172.16.0.0/12",
	"192.0.0.0/24",
	"192.0.2.0/24",
	"192.88.99.0/24",
	"192.168.0.0/16",
	"198.18.0.0/15",
	"198.51.100.0/24",
	"203.0.113.0/24",
	"224.0.0.0/4",
	"240.0.0.0/4",
	"255.255.255.255/32",
	"::/128",
	"::1/128",
	"fc00::/7",
	"ff00::/8",
	"fe80::/10",
}

func privateEntry() (*Entry, error) {
	entry := NewEntry("PRIVATE")
	for _, cidr := range privateCIDRs {
		if err := entry.AddPrefix(cidr); err != nil {
			return nil, err
		}
	}
	return entry, nil
}

func mergeEntries(container map[string]*Entry, entries map[string]*Entry) error {
	for _, entry := range entries {
		if err := mergeEntry(container, entry); err != nil {
			return err
		}
	}
	return nil
}

func mergeEntry(container map[string]*Entry, entry *Entry) error {
	name := entry.name
	if existing, ok := container[name]; ok {
		return existing.Merge(entry)
	}
	container[name] = entry
	return nil
}

func wantMap(list []string) map[string]bool {
	if len(list) == 0 {
		return nil
	}
	m := make(map[string]bool, len(list))
	for _, s := range list {
		m[strings.ToUpper(strings.TrimSpace(s))] = true
	}
	return m
}
