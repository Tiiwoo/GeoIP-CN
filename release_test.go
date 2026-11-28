package main

import (
	"fmt"
	"os"
	"testing"

	maxminddb "github.com/oschwald/maxminddb-golang/v2"
)

const (
	maxDropRatio   = 0.15
	maxGrowthRatio = 0.50
)

// CI sets GEOIP_MMDB_FILE after generation. Ordinary tests stay offline.
func TestReleaseDatabase(t *testing.T) {
	path := os.Getenv("GEOIP_MMDB_FILE")
	if path == "" {
		t.Skip("set GEOIP_MMDB_FILE to check a generated release database")
	}
	counts := countDatabase(t, path)
	for name := range counts {
		if name != "CN" && name != "PRIVATE" {
			t.Fatalf("unexpected country classification %q", name)
		}
	}
	for _, name := range []string{"CN", "PRIVATE"} {
		count := counts[name]
		if count[0] == 0 || count[1] == 0 {
			t.Errorf("%s is missing IPv4 or IPv6 coverage: %v", name, count)
		}
	}
	if counts["CN"][0] < 1000 || counts["CN"][1] < 500 {
		t.Errorf("CN coverage unexpectedly low [IPv4 IPv6]: %v", counts["CN"])
	}
	t.Logf("prefix counts [IPv4 IPv6]: %v", counts)

	if baselinePath := os.Getenv("GEOIP_BASELINE_MMDB"); baselinePath != "" {
		if err := compareCounts(countDatabase(t, baselinePath), counts); err != nil {
			t.Error(err)
		}
	}

	db, err := maxminddb.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, tc := range []struct{ ip, want string }{
		{"223.5.5.5", "CN"}, {"119.29.29.29", "CN"}, {"2400:3200::1", "CN"},
		{"10.0.0.1", "PRIVATE"}, {"127.0.0.1", "PRIVATE"}, {"::1", "PRIVATE"}, {"fc00::1", "PRIVATE"},
		{"8.8.8.8", ""}, {"1.1.1.1", ""}, {"2001:4860:4860::8888", ""},
		{"47.52.0.1", ""}, {"43.132.0.1", ""},
		{"223.118.0.1", ""}, {"223.119.0.1", ""}, {"223.120.0.1", ""}, {"123.254.104.1", ""},
		{"140.205.1.1", "CN"}, {"162.14.0.1", "CN"},
	} {
		got, found := lookupCountry(t, db, tc.ip)
		if got != tc.want || found != (tc.want != "") {
			t.Errorf("lookup %s = %q/%t, want %q", tc.ip, got, found, tc.want)
		}
	}
}

func countDatabase(t *testing.T, path string) map[string][2]int {
	t.Helper()
	db, err := maxminddb.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Verify(); err != nil {
		t.Fatal(err)
	}
	counts := make(map[string][2]int)
	for result := range db.Networks() {
		var record struct {
			Country struct {
				ISO string `maxminddb:"iso_code"`
			} `maxminddb:"country"`
		}
		if err := result.Decode(&record); err != nil {
			t.Fatal(err)
		}
		count := counts[record.Country.ISO]
		family := 1
		if addr := result.Prefix().Addr(); addr.Is4() || addr.Is4In6() {
			family = 0
		}
		count[family]++
		counts[record.Country.ISO] = count
	}
	return counts
}

// compareCounts rejects a release whose prefix counts swing far away from the
// previous one: a large drop means missing data, while a large CN jump usually
// means a bad upstream range. PRIVATE is a fixed special-use list, so only its
// drops are suspicious; intentional additions are allowed.
func compareCounts(baseline, current map[string][2]int) error {
	for name, base := range baseline {
		cur, ok := current[name]
		if !ok {
			return fmt.Errorf("classification %s is missing from the new database", name)
		}
		for family, label := range [...]string{"IPv4", "IPv6"} {
			if base[family] == 0 {
				continue
			}
			ratio := float64(cur[family]-base[family]) / float64(base[family])
			if ratio < -maxDropRatio {
				return fmt.Errorf("%s %s dropped %.0f%% (%d -> %d)", name, label, -ratio*100, base[family], cur[family])
			}
			if name == "CN" && ratio > maxGrowthRatio {
				return fmt.Errorf("%s %s grew %.0f%% (%d -> %d)", name, label, ratio*100, base[family], cur[family])
			}
		}
	}
	return nil
}

func TestCompareCounts(t *testing.T) {
	baseline := map[string][2]int{
		"CN":      {1000, 500},
		"PRIVATE": {10, 5},
	}
	for _, tc := range []struct {
		name    string
		current map[string][2]int
		wantErr bool
	}{
		{"within thresholds", map[string][2]int{"CN": {900, 600}, "PRIVATE": {10, 5}}, false},
		{"drop", map[string][2]int{"CN": {800, 500}, "PRIVATE": {10, 5}}, true},
		{"growth", map[string][2]int{"CN": {1000, 800}, "PRIVATE": {10, 5}}, true},
		{"private growth allowed", map[string][2]int{"CN": {1000, 500}, "PRIVATE": {10, 40}}, false},
		{"missing classification", map[string][2]int{"CN": {1000, 500}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := compareCounts(baseline, tc.current)
			if (err != nil) != tc.wantErr {
				t.Fatalf("compareCounts() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
