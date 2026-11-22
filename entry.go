package main

import (
	"fmt"
	"net/netip"
	"strings"

	"go4.org/netipx"
)

type Entry struct {
	name string
	ipv4 netipx.IPSetBuilder
	ipv6 netipx.IPSetBuilder
}

func NewEntry(name string) *Entry {
	return &Entry{name: strings.ToUpper(strings.TrimSpace(name))}
}

func (e *Entry) AddPrefix(cidr string) error {
	p, err := parsePrefix(cidr)
	if err != nil || !p.IsValid() {
		return err
	}
	return e.addPrefix(p)
}

// parsePrefix strips comments and returns a zero prefix for a blank line.
func parsePrefix(cidr string) (netip.Prefix, error) {
	cidr, _, _ = strings.Cut(cidr, "#")
	cidr, _, _ = strings.Cut(cidr, "//")
	cidr = strings.TrimSpace(cidr)
	if cidr == "" {
		return netip.Prefix{}, nil
	}

	if strings.Contains(cidr, "/") {
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			return netip.Prefix{}, fmt.Errorf("invalid CIDR %q: %w", cidr, err)
		}
		return normalizePrefix(prefix)
	}

	addr, err := netip.ParseAddr(cidr)
	if err != nil {
		return netip.Prefix{}, fmt.Errorf("invalid IP %q: %w", cidr, err)
	}
	if addr.Zone() != "" {
		return netip.Prefix{}, fmt.Errorf("scoped IP %q is not supported", cidr)
	}
	return normalizePrefix(netip.PrefixFrom(addr, addr.BitLen()))
}

func normalizePrefix(p netip.Prefix) (netip.Prefix, error) {
	if !p.IsValid() {
		return netip.Prefix{}, fmt.Errorf("invalid prefix")
	}
	if p.Addr().Is4In6() {
		// Only prefixes contained in ::ffff:0:0/96 map to IPv4 ranges.
		if p.Bits() < 96 {
			return netip.Prefix{}, fmt.Errorf("mapped IPv4 prefix %s is broader than /96", p)
		}
		p = netip.PrefixFrom(p.Addr().Unmap(), p.Bits()-96)
	}
	return p.Masked(), nil
}

func (e *Entry) addPrefix(p netip.Prefix) error {
	p, err := normalizePrefix(p)
	if err != nil {
		return err
	}
	if p.Addr().Is4() {
		e.ipv4.AddPrefix(p)
	} else {
		e.ipv6.AddPrefix(p)
	}
	return nil
}

func (e *Entry) Prefixes() ([]netip.Prefix, error) {
	v4, err := e.ipv4.IPSet()
	if err != nil {
		return nil, err
	}
	v6, err := e.ipv6.IPSet()
	if err != nil {
		return nil, err
	}
	p4, p6 := v4.Prefixes(), v6.Prefixes()
	out := make([]netip.Prefix, 0, len(p4)+len(p6))
	out = append(out, p4...)
	out = append(out, p6...)
	return out, nil
}

func (e *Entry) Merge(other *Entry) error {
	v4, err := other.ipv4.IPSet()
	if err != nil {
		return fmt.Errorf("merge %s IPv4: %w", other.name, err)
	}
	v6, err := other.ipv6.IPSet()
	if err != nil {
		return fmt.Errorf("merge %s IPv6: %w", other.name, err)
	}
	e.ipv4.AddSet(v4)
	e.ipv6.AddSet(v6)
	return nil
}

func (e *Entry) Remove(other *Entry) error {
	v4, err := other.ipv4.IPSet()
	if err != nil {
		return fmt.Errorf("remove %s IPv4: %w", other.name, err)
	}
	v6, err := other.ipv6.IPSet()
	if err != nil {
		return fmt.Errorf("remove %s IPv6: %w", other.name, err)
	}
	e.ipv4.RemoveSet(v4)
	e.ipv6.RemoveSet(v6)
	return nil
}
