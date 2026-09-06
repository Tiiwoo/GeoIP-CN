package main

import (
	"os"
	"testing"

	maxminddb "github.com/oschwald/maxminddb-golang/v2"
)

// CI sets GEOIP_MMDB_FILE after generation. Ordinary tests stay offline.
func TestReleaseDatabase(t *testing.T) {
	path := os.Getenv("GEOIP_MMDB_FILE")
	if path == "" {
		t.Skip("set GEOIP_MMDB_FILE to check a generated release database")
	}
	db, err := maxminddb.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Verify(); err != nil {
		t.Fatal(err)
	}
	counts := map[string][2]int{"CN": {}, "PRIVATE": {}}
	for result := range db.Networks() {
		var record struct {
			Country struct {
				ISO string `maxminddb:"iso_code"`
			} `maxminddb:"country"`
		}
		if err := result.Decode(&record); err != nil {
			t.Fatal(err)
		}
		name := record.Country.ISO
		count, ok := counts[name]
		if !ok {
			t.Fatalf("unexpected country classification %q", name)
		}
		family := 1
		if result.Prefix().Addr().Is4() {
			family = 0
		}
		count[family]++
		counts[name] = count
	}
	for name, count := range counts {
		if count[0] == 0 || count[1] == 0 {
			t.Errorf("%s is missing IPv4 or IPv6 coverage: %v", name, count)
		}
	}
	t.Logf("prefix counts [IPv4 IPv6]: %v", counts)
	for _, tc := range []struct{ ip, want string }{
		{"223.5.5.5", "CN"}, {"119.29.29.29", "CN"}, {"2400:3200::1", "CN"},
		{"10.0.0.1", "PRIVATE"}, {"127.0.0.1", "PRIVATE"}, {"::1", "PRIVATE"}, {"fc00::1", "PRIVATE"},
		{"8.8.8.8", ""}, {"1.1.1.1", ""}, {"2001:4860:4860::8888", ""},
	} {
		got, found := lookupCountry(t, db, tc.ip)
		if got != tc.want || found != (tc.want != "") {
			t.Errorf("lookup %s = %q/%t, want %q", tc.ip, got, found, tc.want)
		}
	}
}
