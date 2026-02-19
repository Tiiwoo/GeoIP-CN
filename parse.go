package main

import (
	"bufio"
	"bytes"
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
		entry.addPrefix(result.Prefix())
	}
	return entries, nil
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
		entry.addPrefix(result.Prefix())
	}
	return entries, nil
}

func parseText(data []byte, name string, onlyIPType string) (*Entry, error) {
	entry := NewEntry(name)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		if onlyIPType == "ipv4" && strings.Contains(line, ":") {
			continue
		}
		if onlyIPType == "ipv6" && !strings.Contains(line, ":") {
			continue
		}

		if err := entry.AddPrefix(line); err != nil {
			return nil, err
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entry, nil
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

func mergeEntries(container map[string]*Entry, entries map[string]*Entry) {
	for name, entry := range entries {
		if existing, ok := container[name]; ok {
			existing.Merge(entry)
		} else {
			container[name] = entry
		}
	}
}

func mergeEntry(container map[string]*Entry, entry *Entry) {
	name := entry.name
	if existing, ok := container[name]; ok {
		existing.Merge(entry)
	} else {
		container[name] = entry
	}
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
