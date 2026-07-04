package util

import (
	"net"
	"net/netip"
)

func NetIPtoNetIPPRefix(ips []net.IP) []netip.Prefix {
	addrs := make([]netip.Prefix, 0, len(ips))

	for _, ip := range ips {
		if ip == nil {
			continue
		}

		addr, ok := netip.AddrFromSlice(ip)
		if !ok {
			continue
		}

		var prefix netip.Prefix

		bits := 32
		if addr.Is6() {
			bits = 128
		}
		prefix = netip.PrefixFrom(addr, bits)
		addrs = append(addrs, prefix)
	}

	return addrs
}
