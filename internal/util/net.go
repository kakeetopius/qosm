// Package util contains some utility functions.
package util

import (
	"net/netip"

	"github.com/kakeetopius/qosm/internal/protobuf"
)

type ipSlice interface {
	~[]byte
}

func IPSlicestoNetIPPRefix[T ipSlice](ips []T) []netip.Prefix {
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

func IPPrefixesFromProtobufIPs(ips []*protobuf.IPPrefix) []netip.Prefix {
	addrs := make([]netip.Prefix, 0, len(ips))

	for _, ip := range ips {
		ipSlice := ip.GetIp()
		prefix := ip.GetPrefixLen()

		addr, ok := netip.AddrFromSlice(ipSlice)
		if !ok {
			continue
		}

		addrs = append(addrs, netip.PrefixFrom(addr, int(prefix)))
	}

	return addrs
}

func IPPrefixesToProtobufIPs(ips []netip.Prefix) []*protobuf.IPPrefix {
	protobufIPs := make([]*protobuf.IPPrefix, 0, len(ips))
	for _, ip := range ips {
		bits := ip.Bits()
		prefixLen := int32(bits)
		protoIP := protobuf.IPPrefix_builder{
			Ip:        ip.Addr().AsSlice(),
			PrefixLen: &prefixLen,
		}.Build()
		protobufIPs = append(protobufIPs, protoIP)
	}
	return protobufIPs
}
