package main

import (
	"fmt"
	"net/netip"
	"strings"

	"go4.org/netipx"
)

type Entry struct {
	name        string
	ipv4Builder *netipx.IPSetBuilder
	ipv6Builder *netipx.IPSetBuilder
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
		if e.ipv4Builder == nil {
			e.ipv4Builder = new(netipx.IPSetBuilder)
		}
		e.ipv4Builder.AddPrefix(p)
	} else {
		if e.ipv6Builder == nil {
			e.ipv6Builder = new(netipx.IPSetBuilder)
		}
		e.ipv6Builder.AddPrefix(p)
	}
	return nil
}

func (e *Entry) Prefixes() ([]netip.Prefix, error) {
	var out []netip.Prefix
	if e.ipv4Builder != nil {
		s, err := e.ipv4Builder.IPSet()
		if err != nil {
			return nil, err
		}
		out = append(out, s.Prefixes()...)
	}
	if e.ipv6Builder != nil {
		s, err := e.ipv6Builder.IPSet()
		if err != nil {
			return nil, err
		}
		out = append(out, s.Prefixes()...)
	}
	return out, nil
}

func (e *Entry) Merge(other *Entry) error {
	var v4, v6 *netipx.IPSet
	var err error
	if other.ipv4Builder != nil {
		v4, err = other.ipv4Builder.IPSet()
		if err != nil {
			return fmt.Errorf("merge %s IPv4: %w", other.name, err)
		}
	}
	if other.ipv6Builder != nil {
		v6, err = other.ipv6Builder.IPSet()
		if err != nil {
			return fmt.Errorf("merge %s IPv6: %w", other.name, err)
		}
	}
	if v4 != nil {
		if e.ipv4Builder == nil {
			e.ipv4Builder = new(netipx.IPSetBuilder)
		}
		e.ipv4Builder.AddSet(v4)
	}
	if v6 != nil {
		if e.ipv6Builder == nil {
			e.ipv6Builder = new(netipx.IPSetBuilder)
		}
		e.ipv6Builder.AddSet(v6)
	}
	return nil
}
