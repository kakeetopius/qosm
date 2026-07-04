package nft

// This file contains functions to manipulate the different nftables sets that are managed by qosm

import (
	"bytes"
	"errors"
	"fmt"
	"math/bits"
	"net/netip"
	"os"
	"slices"

	"github.com/google/nftables"
	"github.com/kakeetopius/qosm/internal/service"
	"golang.org/x/sys/unix"
)

func addServicesToServiceSet(conn *nftables.Conn, serviceSet *nftables.Set, services []service.Service) error {
	setElements := make([]nftables.SetElement, 0, len(services))
	for _, service := range services {
		setElement := nftables.SetElement{
			Key: service.NFTSetKey(),
		}
		setElements = append(setElements, setElement)
	}

	err := conn.SetAddElements(serviceSet, setElements)
	if err != nil {
		return err
	}
	err = conn.Flush()
	if err != nil {
		return err
	}
	return nil
}

func addIfaceToIfaceSet(conn *nftables.Conn, ifaceSet *nftables.Set, ifaceNames []string) error {
	ifNameSize := unix.IFNAMSIZ

	setElements := make([]nftables.SetElement, 0, len(ifaceNames))
	for _, ifaceName := range ifaceNames {
		if len(ifaceName) > ifNameSize {
			return fmt.Errorf("nft: invalid interface name: %v. Interface name is too long, it must be below %v characters", ifaceName, ifNameSize)
		}

		key := make([]byte, ifNameSize)
		copy(key, ifaceName)

		setElement := nftables.SetElement{
			Key: key,
		}
		setElements = append(setElements, setElement)
	}

	err := conn.SetAddElements(ifaceSet, setElements)
	if err != nil {
		return err
	}
	err = conn.Flush()
	if err != nil {
		return err
	}
	return nil
}

func addIPsToIPSet(conn *nftables.Conn, ip4Set *nftables.Set, ip6set *nftables.Set, ipRanges []netip.Prefix) error {
	rangeElements := make([]nftables.SetElement, 2)
	setToUse := ip4Set
	for _, ipRange := range ipRanges {
		networkAddr := ipRange.Masked().Addr()
		if ipRange.Addr() != networkAddr {
			return fmt.Errorf("invalid CIDR -> %s is not a correct network address", ipRange)
		}
		broadcast := ipIntervalEnd(ipRange)

		rangeElements[0] = nftables.SetElement{Key: networkAddr.AsSlice()}
		rangeElements[1] = nftables.SetElement{Key: broadcast.AsSlice(), IntervalEnd: true}

		if broadcast.Is6() {
			setToUse = ip6set
		} else {
			setToUse = ip4Set
		}

		err := conn.SetAddElements(setToUse, rangeElements)
		if err != nil {
			return err
		}
		err = conn.Flush()
		if err != nil {
			if errors.Is(err, os.ErrExist) {
				return fmt.Errorf("%s is already a subnet or supernet of one of the IP ranges", ipRange.String())
			}
			return err
		}
	}

	return nil
}

// getIPSetElements retrieves all IP addresses stored in the specified nftables IP set.
func getIPSetElements(conn *nftables.Conn, ip4Set *nftables.Set, ip6Set *nftables.Set) ([]netip.Prefix, error) {
	ip4s, err := conn.GetSetElements(ip4Set)
	if err != nil {
		return nil, err
	}
	ip6s, err := conn.GetSetElements(ip6Set)
	if err != nil {
		return nil, err
	}

	ips := make([]netip.Prefix, 0, len(ip4s)+len(ip6s))

	for i := 0; i < len(ip4s); i += 2 {
		if i+1 == len(ip4s) {
			return nil, fmt.Errorf("invald results from nftables")
		}
		prefix, err := reconstructNftIPRange(ip4s[i], ip4s[i+1])
		if err != nil {
			return nil, err
		}
		ips = append(ips, prefix)
	}

	for i := 0; i < len(ip6s); i += 2 {
		if i+1 == len(ip6s) {
			return nil, fmt.Errorf("invald results from nftables")
		}
		prefix, err := reconstructNftIPRange(ip6s[i], ip6s[i+1])
		if err != nil {
			return nil, err
		}
		ips = append(ips, prefix)
	}

	return ips, nil
}

func getServiceSetElements(conn *nftables.Conn, set *nftables.Set) ([]service.Service, error) {
	elements, err := conn.GetSetElements(set)
	if err != nil {
		return nil, err
	}

	services := make([]service.Service, 0, len(elements))

	for _, element := range elements {
		service, err := service.ServiceFromNftSetKey(element.Key)
		if err != nil {
			return nil, err
		}
		services = append(services, service)
	}

	return services, nil
}

func getIfaceSetElements(conn *nftables.Conn, set *nftables.Set) ([]string, error) {
	elements, err := conn.GetSetElements(set)
	if err != nil {
		return nil, err
	}

	ifaceNames := make([]string, 0, len(elements))

	for _, element := range elements {
		ifaceNames = append(ifaceNames, string(trimLeftPadding(element.Key)))
	}

	return ifaceNames, nil
}

func deleteIPsFromIPSet(conn *nftables.Conn, ip4Set *nftables.Set, ip6Set *nftables.Set, ipRanges []netip.Prefix) error {
	setToUse := ip4Set
	toDelete := make([]nftables.SetElement, 2)
	for _, ipRange := range ipRanges {
		if ipRange.Addr().Is6() {
			setToUse = ip6Set
		} else {
			setToUse = ip4Set
		}
		start := ipRange.Addr()
		end := ipIntervalEnd(ipRange)

		toDelete[0] = nftables.SetElement{
			Key: start.AsSlice(),
		}
		toDelete[1] = nftables.SetElement{
			Key:         end.AsSlice(),
			IntervalEnd: true,
		}
		err := conn.SetDeleteElements(setToUse, toDelete)
		if err != nil {
			return err
		}
		err = conn.Flush()
		if err != nil {
			return err
		}
	}

	return nil
}

func deleteServicesFromServiceSet(conn *nftables.Conn, serviceSet *nftables.Set, services []service.Service) error {
	toDelete := make([]nftables.SetElement, 0, len(services))

	currentElements, err := getServiceSetElements(conn, serviceSet)
	if err != nil {
		return err
	}

	for _, service := range services {
		if !slices.Contains(currentElements, service) {
			return ErrSetElementNotExists{Element: service.String(), SetName: serviceSet.Name}
		}

		toDelete = append(toDelete,
			nftables.SetElement{
				Key: service.NFTSetKey(),
			})
	}

	if len(toDelete) > 0 {
		err := conn.SetDeleteElements(serviceSet, toDelete)
		if err != nil {
			return err
		}
		return conn.Flush()
	}

	return nil
}

func deleteIfacesFromSet(conn *nftables.Conn, ifaceSet *nftables.Set, ifaceNames []string) error {
	toDelete := make([]nftables.SetElement, 0, len(ifaceNames))

	currentElements, err := getIfaceSetElements(conn, ifaceSet)
	if err != nil {
		return err
	}

	ifNameSize := unix.IFNAMSIZ
	for _, ifaceName := range ifaceNames {
		if !slices.Contains(currentElements, ifaceName) {
			return ErrSetElementNotExists{Element: ifaceName, SetName: ifaceSet.Name}
		}

		key := make([]byte, ifNameSize)
		copy(key, ifaceName)

		toDelete = append(toDelete,
			nftables.SetElement{
				Key: key,
			})
	}

	if len(toDelete) > 0 {
		err := conn.SetDeleteElements(ifaceSet, toDelete)
		if err != nil {
			return err
		}
		return conn.Flush()
	}

	return nil
}

func ipExistsInIPSets(conn *nftables.Conn, ip4Set *nftables.Set, ip6Set *nftables.Set, network netip.Prefix) (bool, error) {
	setElements, err := getIPSetElements(conn, ip4Set, ip6Set)
	if err != nil {
		return false, err
	}
	return slices.Contains(setElements, network), nil
}

func serviceExistsInPortSet(conn *nftables.Conn, set *nftables.Set, service service.Service) (bool, error) {
	setElements, err := getServiceSetElements(conn, set)
	if err != nil {
		return false, err
	}
	return slices.Contains(setElements, service), nil
}

func ifaceExistsInIfaceSet(conn *nftables.Conn, set *nftables.Set, ifaceName string) (bool, error) {
	setElements, err := getIfaceSetElements(conn, set)
	if err != nil {
		return false, err
	}
	return slices.Contains(setElements, ifaceName), nil
}

func ipIntervalEnd(prefix netip.Prefix) netip.Addr {
	if prefix.IsSingleIP() {
		return prefix.Addr().Next()
	}

	addr := prefix.Masked().Addr()

	var b []byte
	var bits int

	if addr.Is4() {
		a := addr.As4()
		b = a[:]
		bits = 32
	} else {
		a := addr.As16()
		b = a[:]
		bits = 128
	}

	hostBits := bits - prefix.Bits()

	// Starting from the least-significant byte, set all host bits to 1.
	for i := len(b) - 1; i >= 0 && hostBits > 0; i-- {
		if hostBits >= 8 {
			// setting 8 bits at a time until we have less than 8 bits
			b[i] = 0xff
			hostBits -= 8
		} else {
			// sets the remaining hostbits at once
			b[i] |= byte((1 << hostBits) - 1)
			hostBits = 0
		}
	}
	addr, _ = netip.AddrFromSlice(b)
	return addr
}

func reconstructNftIPRange(limit1, limit2 nftables.SetElement) (netip.Prefix, error) {
	var upper, lower []byte

	if limit1.IntervalEnd {
		upper = limit2.Key
		lower = limit1.Key
	} else {
		upper = limit1.Key
		lower = limit2.Key
	}

	start, ok := netip.AddrFromSlice(upper)
	if !ok {
		return netip.Prefix{}, fmt.Errorf("nft: invalid address")
	}

	end, ok := netip.AddrFromSlice(lower)
	if !ok {
		return netip.Prefix{}, fmt.Errorf("nft: invalid address")
	}

	bits := 32
	if start.Is6() {
		bits = 128
	}

	if start.Next().Compare(end) == 0 {
		return netip.PrefixFrom(start, bits), nil
	}

	startBytes := start.AsSlice()
	endBytes := end.AsSlice()

	prefix := longestCommonPrefix(startBytes, endBytes)

	return netip.PrefixFrom(start, prefix), nil
}

func trimLeftPadding(b []byte) []byte {
	b, _, _ = bytes.Cut(b, []byte{0})
	return b
}

func longestCommonPrefix(a []byte, b []byte) int {
	limit := min(len(a), len(b))
	prefixLen := 0

	for i := range limit {
		if a[i] == b[i] {
			prefixLen += 8
			continue
		}
		prefixLen += bits.LeadingZeros8(a[i] ^ b[i]) // if bytes are sth like 01101000 and 01110101 xor would be 00011101 so leading zeros of xor op tell us how many bits are common
	}

	return prefixLen
}
