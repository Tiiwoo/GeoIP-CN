package main

import "testing"

func TestParseTextFamilyWithComment(t *testing.T) {
	data := []byte("# source: fixture\n1.2.3.0/24 # source: fixture\n2400:3200::/32 // source: fixture\n::ffff:1.2.4.1 // mapped address\n")
	for _, tc := range []struct {
		family string
		want   []string
	}{
		{"ipv4", []string{"1.2.3.0/24", "1.2.4.1/32"}},
		{"ipv6", []string{"2400:3200::/32"}},
		{"", []string{"1.2.3.0/24", "1.2.4.1/32", "2400:3200::/32"}},
	} {
		t.Run(tc.family, func(t *testing.T) {
			entry, err := parseText(data, "cn", tc.family)
			if err != nil {
				t.Fatal(err)
			}
			assertPrefixes(t, entry, tc.want...)
		})
	}
}

func TestParseTextRejectsEmptyOrInvalidSource(t *testing.T) {
	for _, tc := range []struct{ name, text, family string }{
		{"empty", "", ""},
		{"comments only", "# source: fixture\n// ignored\n", ""},
		{"no matching family", "1.2.3.0/24 # source: fixture\n", "ipv6"},
		{"invalid address", "not-an-address\n", "ipv6"},
		{"invalid family", "1.2.3.0/24\n", "ip4"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseText([]byte(tc.text), "cn", tc.family); err == nil {
				t.Fatal("invalid or empty source was accepted")
			}
		})
	}
}
