package nft

import (
	"fmt"
	"net/netip"

	"github.com/kakeetopius/qosm/internal/priority"
	"github.com/kakeetopius/qosm/internal/service"
)

func (c *NFT) AddIPsToPriority(ips []netip.Prefix, prio priority.Priority) error {
	switch prio {
	case priority.PRIORITYHIGH:
		return c.AddIPsToHighPriority(ips)
	case priority.PRIORITYLOW:
		return c.AddIPsToLowPriority(ips)
	default:
		return fmt.Errorf("unknown priority %v", prio)
	}
}

func (c *NFT) AddServicesToPriority(services []service.Service, prio priority.Priority) error {
	switch prio {
	case priority.PRIORITYHIGH:
		return c.AddServicesToHighPrioriy(services)
	case priority.PRIORITYLOW:
		return c.AddServicesToLowPriority(services)
	default:
		return fmt.Errorf("unknown priority %v", prio)
	}
}

func (c *NFT) DeleteIPsFromPriority(ips []netip.Prefix, prio priority.Priority) error {
	switch prio {
	case priority.PRIORITYHIGH:
		return c.DeleteIPsFromHighPriority(ips)
	case priority.PRIORITYLOW:
		return c.DeleteIPsFromLowPriority(ips)
	default:
		return fmt.Errorf("unknown priority %v", prio)
	}
}

func (c *NFT) AddIPsToHighPriority(ips []netip.Prefix) error {
	return addIPsToIPSet(c.conn, c.QosTable.IPSets.HighPrioIP4Set, c.QosTable.IPSets.HighPrioIP6Set, ips)
}

func (c *NFT) AddServicesToHighPrioriy(services []service.Service) error {
	return addServicesToServiceSet(c.conn, c.QosTable.ServiceSets.HighPrioServiceSet, services)
}

func (c *NFT) AddIPsToLowPriority(ips []netip.Prefix) error {
	return addIPsToIPSet(c.conn, c.QosTable.IPSets.LowPrioIP4Set, c.QosTable.IPSets.LowPrioIP6Set, ips)
}

func (c *NFT) AddServicesToLowPriority(services []service.Service) error {
	return addServicesToServiceSet(c.conn, c.QosTable.ServiceSets.LowPrioServiceSet, services)
}

func (c *NFT) DeleteIPsFromHighPriority(ips []netip.Prefix) error {
	return deleteIPsFromIPSet(c.conn, c.QosTable.IPSets.HighPrioIP4Set, c.QosTable.IPSets.HighPrioIP6Set, ips)
}

func (c *NFT) DeleteServicesFromHighPriority(services []service.Service) error {
	return deleteServicesFromServiceSet(c.conn, c.QosTable.ServiceSets.HighPrioServiceSet, services)
}

func (c *NFT) DeleteIPsFromLowPriority(ips []netip.Prefix) error {
	return deleteIPsFromIPSet(c.conn, c.QosTable.IPSets.LowPrioIP4Set, c.QosTable.IPSets.LowPrioIP6Set, ips)
}

func (c *NFT) DeleteServicesFromLowPriority(services []service.Service) error {
	return deleteServicesFromServiceSet(c.conn, c.QosTable.ServiceSets.LowPrioServiceSet, services)
}

func (c *NFT) GetHighPrioIPs() ([]netip.Prefix, error) {
	return getIPSetElements(c.conn, c.QosTable.IPSets.HighPrioIP4Set, c.QosTable.IPSets.HighPrioIP6Set)
}

func (c *NFT) GetHighPrioServices() ([]service.Service, error) {
	return getServiceSetElements(c.conn, c.QosTable.ServiceSets.HighPrioServiceSet)
}

func (c *NFT) GetLowPrioIPs() ([]netip.Prefix, error) {
	return getIPSetElements(c.conn, c.QosTable.IPSets.LowPrioIP4Set, c.QosTable.IPSets.LowPrioIP6Set)
}

func (c *NFT) GetLowPrioServices() ([]service.Service, error) {
	return getServiceSetElements(c.conn, c.QosTable.ServiceSets.LowPrioServiceSet)
}

func (c *NFT) IPIsHighPriority(ip netip.Prefix) (bool, error) {
	return ipExistsInIPSets(c.conn, c.QosTable.IPSets.HighPrioIP4Set, c.QosTable.IPSets.HighPrioIP6Set, ip)
}

func (c *NFT) ServiceIsHighPriority(service service.Service) (bool, error) {
	return serviceExistsInPortSet(c.conn, c.QosTable.ServiceSets.HighPrioServiceSet, service)
}

func (c *NFT) IPIsLowPriority(ip netip.Prefix) (bool, error) {
	return ipExistsInIPSets(c.conn, c.QosTable.IPSets.LowPrioIP4Set, c.QosTable.IPSets.LowPrioIP6Set, ip)
}

func (c *NFT) ServiceIsLowPriority(service service.Service) (bool, error) {
	return serviceExistsInPortSet(c.conn, c.QosTable.ServiceSets.LowPrioServiceSet, service)
}

func (c *NFT) AddIfaces(ifaceName []string) error {
	return addIfaceToIfaceSet(c.conn, c.QosTable.IfaceSet, ifaceName)
}

func (c *NFT) DeleteIfaces(ifaceName []string) error {
	return deleteIfacesFromSet(c.conn, c.QosTable.IfaceSet, ifaceName)
}

func (c *NFT) IfaceExistsInSet(ifaceName string) (bool, error) {
	return ifaceExistsInIfaceSet(c.conn, c.QosTable.IfaceSet, ifaceName)
}

// DeleteTable removes the qosm nftables table from the system. The context becomes invalid after this operation.
func (c *NFT) DeleteTable() error {
	fmt.Println("Deleting table")
	c.conn.DelTable(c.QosTable.Table)
	err := c.conn.Flush()
	if err != nil {
		return err
	}
	c = nil
	return nil
}
