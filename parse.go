package main

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	maxminddb "github.com/oschwald/maxminddb-golang/v2"
)

// Only country.iso_code is used for MaxMind-style sources: for CN routing what
// matters is where the IP is used. Falling back to registered_country would
// pull in CN-registered ranges that are operated from overseas.
func parseMaxmindMMDB(data []byte, want map[string]bool) (map[string]*Entry, error) {
	return parseMMDB(data, want, func(result maxminddb.Result) (string, error) {
		var record struct {
			Country struct {
				IsoCode string `maxminddb:"iso_code"`
			} `maxminddb:"country"`
		}
		if err := result.Decode(&record); err != nil {
			return "", err
		}
		return strings.TrimSpace(record.Country.IsoCode), nil
	})
}

// IPInfo Lite stores the ISO code in country_code; decode only that field.
func parseIPInfoMMDB(data []byte, want map[string]bool) (map[string]*Entry, error) {
	return parseMMDB(data, want, func(result maxminddb.Result) (string, error) {
		var record struct {
			CountryCode string `maxminddb:"country_code"`
		}
		if err := result.Decode(&record); err != nil {
			return "", err
		}
		return strings.TrimSpace(record.CountryCode), nil
	})
}

func parseMMDB(data []byte, want map[string]bool, extract func(maxminddb.Result) (string, error)) (map[string]*Entry, error) {
	db, err := maxminddb.OpenBytes(data)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	entries := make(map[string]*Entry, len(want))
	for result := range db.Networks() {
		name, err := extract(result)
		if err != nil {
			return nil, err
		}
		name = strings.ToUpper(name)
		if name == "" || (len(want) > 0 && !want[name]) {
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
	"64:ff9b::/96",
	"64:ff9b:1::/48",
	"100::/64",
	// 2001::/32 (Teredo) is an IPv4 alias network in the MMDB format and cannot
	// carry a PRIVATE record.
	"2001:1::1/128",
	"2001:1::2/128",
	"2001:2::/48",
	"2001:3::/32",
	"2001:4:112::/48",
	"2001:10::/28",
	"2001:20::/28",
	"2001:30::/28",
	"2001:db8::/32",
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

func removeEntries(container map[string]*Entry, entries map[string]*Entry) error {
	for _, entry := range entries {
		if err := removeEntry(container, entry); err != nil {
			return err
		}
	}
	return nil
}

func removeEntry(container map[string]*Entry, entry *Entry) error {
	existing, ok := container[entry.name]
	if !ok {
		return fmt.Errorf("remove target %s not found", entry.name)
	}
	return existing.Remove(entry)
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
