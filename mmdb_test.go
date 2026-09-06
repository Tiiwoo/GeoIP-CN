package main

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/maxmind/mmdbwriter"
	"github.com/maxmind/mmdbwriter/mmdbtype"
	"go4.org/netipx"
)

func makeMMDB(t *testing.T, records map[string]mmdbtype.Map) []byte {
	t.Helper()
	tree, err := mmdbwriter.New(mmdbwriter.Options{DatabaseType: "test", Description: map[string]string{"en": "test database"}, IncludeReservedNetworks: true})
	if err != nil {
		t.Fatal(err)
	}
	for prefix, record := range records {
		if err := tree.Insert(netipx.PrefixIPNet(netip.MustParsePrefix(prefix)), record); err != nil {
			t.Fatal(err)
		}
	}
	var data bytes.Buffer
	if _, err := tree.WriteTo(&data); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func TestMMDBParsers(t *testing.T) {
	maxmindData := makeMMDB(t, map[string]mmdbtype.Map{
		"1.0.1.0/24":     {"country": mmdbtype.Map{"iso_code": mmdbtype.String(" cn ")}},
		"1.0.2.0/24":     {"country": mmdbtype.Map{"iso_code": mmdbtype.String("US")}, "registered_country": mmdbtype.Map{"iso_code": mmdbtype.String("CN")}},
		"1.0.3.0/24":     {"registered_country": mmdbtype.Map{"iso_code": mmdbtype.String("CN")}},
		"1.0.4.0/24":     {"represented_country": mmdbtype.Map{"iso_code": mmdbtype.String("CN")}},
		"2400:3200::/32": {"country": mmdbtype.Map{"iso_code": mmdbtype.String("CN")}},
	})
	ipinfoData := makeMMDB(t, map[string]mmdbtype.Map{
		"1.0.1.0/24":     {"country_code": mmdbtype.String(" cn "), "country": mmdbtype.String("China")},
		"1.0.2.0/24":     {"country_code": mmdbtype.String("US")},
		"1.0.3.0/24":     {"country": mmdbtype.String("CN")},
		"2400:3200::/32": {"country_code": mmdbtype.String("CN")},
	})
	for _, tc := range []struct {
		name  string
		data  []byte
		parse func([]byte, map[string]bool) (map[string]*Entry, error)
		want  []string
	}{
		{"maxmind", maxmindData, parseMaxmindMMDB, []string{"1.0.1.0/24", "1.0.3.0/24", "1.0.4.0/24", "2400:3200::/32"}},
		{"ipinfo", ipinfoData, parseIPInfoMMDB, []string{"1.0.1.0/24", "1.0.3.0/24", "2400:3200::/32"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries, err := tc.parse(tc.data, wantMap([]string{" cn "}))
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 || entries["CN"] == nil {
				t.Fatalf("unexpected entries: %v", entries)
			}
			assertPrefixes(t, entries["CN"], tc.want...)
			for _, wanted := range [][]string{{"private"}, {"cn", "private"}} {
				if _, err := tc.parse(tc.data, wantMap(wanted)); err == nil {
					t.Fatalf("missing requested category accepted: %v", wanted)
				}
			}
			if _, err := tc.parse(makeMMDB(t, nil), nil); err == nil {
				t.Fatal("empty database accepted")
			}
			if _, err := tc.parse([]byte("invalid"), nil); err == nil {
				t.Fatal("corrupt database accepted")
			}
		})
	}
}
