package main

import (
	"fmt"
	"math/big"
	"os"
	"testing"

	maxminddb "github.com/oschwald/maxminddb-golang/v2"
)

const (
	maxAddressDropRatio     = 0.02
	maxCNAddressGrowthRatio = 0.20
)

type familyStats struct {
	prefixes  int
	addresses *big.Int
}

type databaseStats map[string][2]familyStats

// CI sets GEOIP_MMDB_FILE after generation. Ordinary tests stay offline.
func TestReleaseDatabase(t *testing.T) {
	path := os.Getenv("GEOIP_MMDB_FILE")
	if path == "" {
		t.Skip("set GEOIP_MMDB_FILE to check a generated release database")
	}
	stats := databaseStatsFor(t, path)
	for name := range stats {
		if name != "CN" && name != "PRIVATE" {
			t.Fatalf("unexpected country classification %q", name)
		}
	}
	for _, name := range []string{"CN", "PRIVATE"} {
		for family, label := range [...]string{"IPv4", "IPv6"} {
			if stats[name][family].prefixes == 0 {
				t.Errorf("%s is missing %s coverage", name, label)
			}
		}
	}
	if stats["CN"][0].prefixes < 1000 || stats["CN"][1].prefixes < 500 {
		t.Errorf("CN coverage unexpectedly low: v4=%d v6=%d prefixes", stats["CN"][0].prefixes, stats["CN"][1].prefixes)
	}
	logStats(t, "current", stats)

	if baselinePath := os.Getenv("GEOIP_BASELINE_MMDB"); baselinePath != "" {
		baseline := databaseStatsFor(t, baselinePath)
		logStats(t, "baseline", baseline)
		if err := compareStats(baseline, stats); err != nil {
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

func databaseStatsFor(t *testing.T, path string) databaseStats {
	t.Helper()
	db, err := maxminddb.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Verify(); err != nil {
		t.Fatal(err)
	}
	stats := make(databaseStats)
	for result := range db.Networks() {
		var record struct {
			Country struct {
				ISO string `maxminddb:"iso_code"`
			} `maxminddb:"country"`
		}
		if err := result.Decode(&record); err != nil {
			t.Fatal(err)
		}
		prefix := result.Prefix()
		family, bits, prefixLen := 1, 128, prefix.Bits()
		if addr := prefix.Addr(); addr.Is4() || addr.Is4In6() {
			family, bits = 0, 32
			if addr.Is4In6() {
				prefixLen = prefix.Bits() - 96
			}
		}
		st := stats[record.Country.ISO]
		st[family].prefixes++
		if st[family].addresses == nil {
			st[family].addresses = new(big.Int)
		}
		st[family].addresses.Add(st[family].addresses, new(big.Int).Lsh(big.NewInt(1), uint(bits-prefixLen)))
		stats[record.Country.ISO] = st
	}
	return stats
}

func logStats(t *testing.T, label string, stats databaseStats) {
	t.Helper()
	for _, name := range []string{"CN", "PRIVATE"} {
		st, ok := stats[name]
		if !ok {
			continue
		}
		t.Logf("%s %s: v4 %d prefixes / %s addrs, v6 %d prefixes / %s addrs",
			label, name,
			st[0].prefixes, formatAddresses(st[0].addresses),
			st[1].prefixes, formatAddresses(st[1].addresses))
	}
}

func formatAddresses(v *big.Int) string {
	if v == nil || v.Sign() == 0 {
		return "0"
	}
	return new(big.Float).SetInt(v).Text('g', 4)
}

// compareStats compares address-space coverage, which is what matters for
// routing: prefix counts change whenever a source is added or replaced.
// PRIVATE is a fixed special-use list, so only its drops are suspicious.
func compareStats(baseline, current databaseStats) error {
	for name, base := range baseline {
		cur, ok := current[name]
		if !ok {
			return fmt.Errorf("classification %s is missing from the new database", name)
		}
		for family, label := range [...]string{"IPv4", "IPv6"} {
			b := base[family].addresses
			if b == nil || b.Sign() == 0 {
				continue
			}
			c := cur[family].addresses
			if c == nil {
				c = new(big.Int)
			}
			ratio, _ := new(big.Float).Quo(new(big.Float).SetInt(c), new(big.Float).SetInt(b)).Float64()
			change := ratio - 1
			if change < -maxAddressDropRatio {
				return fmt.Errorf("%s %s address space dropped %.2f%%", name, label, -change*100)
			}
			if name == "CN" && change > maxCNAddressGrowthRatio {
				return fmt.Errorf("%s %s address space grew %.2f%%", name, label, change*100)
			}
		}
	}
	return nil
}

func TestCompareStats(t *testing.T) {
	baseline := statsFixture(1000, 500, 10, 5)
	for _, tc := range []struct {
		name    string
		current databaseStats
		wantErr bool
	}{
		{"within thresholds", statsFixture(990, 505, 10, 5), false},
		{"drop", statsFixture(950, 500, 10, 5), true},
		{"private drop", statsFixture(1000, 500, 5, 5), true},
		{"growth", statsFixture(1000, 700, 10, 5), true},
		{"private growth allowed", statsFixture(1000, 500, 10, 50), false},
		{"missing classification", databaseStats{"CN": baseline["CN"]}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := compareStats(baseline, tc.current)
			if (err != nil) != tc.wantErr {
				t.Fatalf("compareStats() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func statsFixture(cn4, cn6, private4, private6 int64) databaseStats {
	return databaseStats{
		"CN": {
			{prefixes: 1, addresses: big.NewInt(cn4)},
			{prefixes: 1, addresses: big.NewInt(cn6)},
		},
		"PRIVATE": {
			{prefixes: 1, addresses: big.NewInt(private4)},
			{prefixes: 1, addresses: big.NewInt(private6)},
		},
	}
}
