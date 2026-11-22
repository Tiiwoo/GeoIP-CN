package main

import (
	"net/netip"
	"slices"
	"testing"
)

func assertPrefixes(t *testing.T, entry *Entry, want ...string) {
	t.Helper()
	prefixes, err := entry.Prefixes()
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(prefixes))
	for _, p := range prefixes {
		got = append(got, p.String())
	}
	if !slices.Equal(got, want) {
		t.Fatalf("prefixes = %v, want %v", got, want)
	}
}

func TestEntryMappedPrefixes(t *testing.T) {
	for input, want := range map[string]string{
		"::ffff:1.2.3.0/120": "1.2.3.0/24",
		"::ffff:1.2.3.4/128": "1.2.3.4/32",
		"::ffff:0:0/96":      "0.0.0.0/0",
		"::ffff:1.2.3.4":     "1.2.3.4/32",
		"1.2.3.99/24":        "1.2.3.0/24",
		"2001:db8::1/32":     "2001:db8::/32",
	} {
		t.Run(input, func(t *testing.T) {
			entry := NewEntry(" cn ")
			if err := entry.AddPrefix(input); err != nil {
				t.Fatal(err)
			}
			assertPrefixes(t, entry, want)
		})
	}
}

func TestEntryRejectsBroadMappedPrefix(t *testing.T) {
	entry := NewEntry("cn")
	if err := entry.AddPrefix("::ffff:1.2.3.0/80"); err == nil {
		t.Fatal("ambiguous mapped prefix was accepted")
	}
}

func TestEntryMergeMappedPrefix(t *testing.T) {
	base, other := NewEntry("cn"), NewEntry("cn")
	if err := base.AddPrefix("1.1.1.0/24"); err != nil {
		t.Fatal(err)
	}
	if err := other.AddPrefix("::ffff:1.2.3.0/120"); err != nil {
		t.Fatal(err)
	}
	if err := base.Merge(other); err != nil {
		t.Fatal(err)
	}
	assertPrefixes(t, base, "1.1.1.0/24", "1.2.3.0/24")
}

func TestEntryMergesAdjacentAndOverlappingRanges(t *testing.T) {
	entry := NewEntry("cn")
	for _, p := range []string{"1.0.1.0/25", "1.0.1.128/25", "1.0.1.1/32", "2400:3200::/32"} {
		if err := entry.AddPrefix(p); err != nil {
			t.Fatal(err)
		}
	}
	assertPrefixes(t, entry, "1.0.1.0/24", "2400:3200::/32")
}

func TestEntryRejectsInvalidPrefix(t *testing.T) {
	entry := NewEntry("cn")
	if err := entry.addPrefix(netip.Prefix{}); err == nil {
		t.Fatal("invalid prefix was silently accepted")
	}
	assertPrefixes(t, entry)
}

func TestEntryMergePropagatesErrors(t *testing.T) {
	for _, family := range []string{"ipv4", "ipv6"} {
		t.Run(family, func(t *testing.T) {
			base, other := NewEntry("cn"), NewEntry("cn")
			if err := base.AddPrefix("1.1.1.0/24"); err != nil {
				t.Fatal(err)
			}
			if err := other.AddPrefix("1.2.3.0/24"); err != nil {
				t.Fatal(err)
			}
			if family == "ipv4" {
				other.ipv4.AddPrefix(netip.Prefix{})
			} else {
				other.ipv6.AddPrefix(netip.Prefix{})
			}
			if err := base.Merge(other); err == nil {
				t.Fatal("merge swallowed the builder error")
			}
			assertPrefixes(t, base, "1.1.1.0/24")
		})
	}
}

func TestEntryRemove(t *testing.T) {
	base, exclude := NewEntry("cn"), NewEntry("cn")
	for _, p := range []string{"223.120.0.0/14", "2400:3200::/32"} {
		if err := base.AddPrefix(p); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{"223.120.0.0/15", "2400:3200:1::/48"} {
		if err := exclude.AddPrefix(p); err != nil {
			t.Fatal(err)
		}
	}
	if err := base.Remove(exclude); err != nil {
		t.Fatal(err)
	}
	assertPrefixes(t, base, "223.122.0.0/15", "2400:3200::/48", "2400:3200:2::/47", "2400:3200:4::/46", "2400:3200:8::/45", "2400:3200:10::/44", "2400:3200:20::/43", "2400:3200:40::/42", "2400:3200:80::/41", "2400:3200:100::/40", "2400:3200:200::/39", "2400:3200:400::/38", "2400:3200:800::/37", "2400:3200:1000::/36", "2400:3200:2000::/35", "2400:3200:4000::/34", "2400:3200:8000::/33")
}

func TestEntryRemoveMappedPrefix(t *testing.T) {
	base, exclude := NewEntry("cn"), NewEntry("cn")
	if err := base.AddPrefix("223.120.0.0/14"); err != nil {
		t.Fatal(err)
	}
	if err := exclude.AddPrefix("::ffff:223.120.0.0/111"); err != nil {
		t.Fatal(err)
	}
	if err := base.Remove(exclude); err != nil {
		t.Fatal(err)
	}
	assertPrefixes(t, base, "223.122.0.0/15")
}

func TestEntryRemovePropagatesErrors(t *testing.T) {
	for _, family := range []string{"ipv4", "ipv6"} {
		t.Run(family, func(t *testing.T) {
			base, other := NewEntry("cn"), NewEntry("cn")
			if err := base.AddPrefix("1.1.1.0/24"); err != nil {
				t.Fatal(err)
			}
			if err := other.AddPrefix("1.1.1.0/24"); err != nil {
				t.Fatal(err)
			}
			if family == "ipv4" {
				other.ipv4.AddPrefix(netip.Prefix{})
			} else {
				other.ipv6.AddPrefix(netip.Prefix{})
			}
			if err := base.Remove(other); err == nil {
				t.Fatal("remove swallowed the builder error")
			}
			assertPrefixes(t, base, "1.1.1.0/24")
		})
	}
}
