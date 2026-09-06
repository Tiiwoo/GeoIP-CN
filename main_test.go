package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/maxmind/mmdbwriter/mmdbtype"
	maxminddb "github.com/oschwald/maxminddb-golang/v2"
)

func TestRunCombinesAllSourceTypes(t *testing.T) {
	dir := t.TempDir()
	maxmindPath, ipinfoPath, textPath := filepath.Join(dir, "maxmind.mmdb"), filepath.Join(dir, "ipinfo.mmdb"), filepath.Join(dir, "ips.txt")
	for path, data := range map[string][]byte{
		maxmindPath: makeMMDB(t, map[string]mmdbtype.Map{"1.0.1.0/24": {"country": mmdbtype.Map{"iso_code": mmdbtype.String("CN")}}}),
		ipinfoPath:  makeMMDB(t, map[string]mmdbtype.Map{"1.0.2.0/24": {"country_code": mmdbtype.String("CN")}}),
		textPath:    []byte("2400:3200::/32 # source: fixture\n1.0.1.0/24 // duplicate\n::ffff:1.0.3.0/120\n"),
	} {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	cfg := Config{Sources: []Source{
		{Type: "maxmind_mmdb", URL: maxmindPath, WantedList: []string{"cn"}},
		{Type: "ipinfo_mmdb", URL: ipinfoPath, WantedList: []string{"cn"}},
		{Type: "text", URL: textPath, Name: "cn"},
		{Type: "private"},
	}, Output: Output{File: "Country.mmdb", Dir: dir, WantedList: []string{"cn", "private"}}}
	configPath := filepath.Join(dir, "config.json")
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := run(configPath); err != nil {
		t.Fatal(err)
	}
	db, err := maxminddb.Open(filepath.Join(dir, "Country.mmdb"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.Verify(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ ip, want string }{{"1.0.1.1", "CN"}, {"1.0.2.1", "CN"}, {"1.0.3.1", "CN"}, {"2400:3200::1", "CN"}, {"10.0.0.1", "PRIVATE"}, {"8.8.8.8", ""}} {
		got, found := lookupCountry(t, db, tc.ip)
		if got != tc.want || found != (tc.want != "") {
			t.Errorf("lookup %s = %q/%t, want %q", tc.ip, got, found, tc.want)
		}
	}
	if err := os.WriteFile(ipinfoPath, makeMMDB(t, nil), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run(configPath); err == nil {
		t.Fatal("empty IPInfo source was hidden by the other CN sources")
	}
}
