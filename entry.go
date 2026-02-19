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
	cidr, _, _ = strings.Cut(cidr, "#")
	cidr, _, _ = strings.Cut(cidr, "//")
	cidr = strings.TrimSpace(cidr)
	if cidr == "" {
		return nil
	}

	if strings.Contains(cidr, "/") {
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			return fmt.Errorf("invalid CIDR %q: %w", cidr, err)
		}
		e.addPrefix(prefix)
		return nil
	}

	addr, err := netip.ParseAddr(cidr)
	if err != nil {
		return fmt.Errorf("invalid IP %q: %w", cidr, err)
	}
	addr = addr.Unmap()
	bits := 32
	if addr.Is6() {
		bits = 128
	}
	e.addPrefix(netip.PrefixFrom(addr, bits))
	return nil
}

func (e *Entry) addPrefix(p netip.Prefix) {
	addr := p.Addr().Unmap()
	p = netip.PrefixFrom(addr, p.Bits())
	if addr.Is4() {
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

func (e *Entry) Merge(other *Entry) {
	if other.ipv4Builder != nil {
		if e.ipv4Builder == nil {
			e.ipv4Builder = new(netipx.IPSetBuilder)
		}
		s, _ := other.ipv4Builder.IPSet()
		if s != nil {
			for _, p := range s.Prefixes() {
				e.ipv4Builder.AddPrefix(p)
			}
		}
	}
	if other.ipv6Builder != nil {
		if e.ipv6Builder == nil {
			e.ipv6Builder = new(netipx.IPSetBuilder)
		}
		s, _ := other.ipv6Builder.IPSet()
		if s != nil {
			for _, p := range s.Prefixes() {
				e.ipv6Builder.AddPrefix(p)
			}
		}
	}
}
