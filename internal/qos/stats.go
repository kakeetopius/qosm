package qos

import (
	"net"

	"github.com/kakeetopius/qosm/internal/core/htb"
)

type QoSStats struct {
	TotalBytes      uint64
	TotalPackets    uint64
	TotalDrops      uint64
	TotalOverlimits uint64
	ManagedIfaces   int
	ActiveIfaces    int
	IfaceStats      []IfaceStats
}

type IfaceStats struct {
	Name          string
	Total         htb.HTBClassStats
	HighPrioClass htb.HTBClassStats
	LowPrioClass  htb.HTBClassStats
	DefaultClass  htb.HTBClassStats
}

func (m *QoSManager) GetStats() (QoSStats, error) {
	stats := QoSStats{
		IfaceStats: make([]IfaceStats, 0, len(m.Ifaces)),
	}

	for _, iface := range m.Ifaces {
		if !iface.QoSEnabled {
			continue
		}
		ifaceStats, err := m.GetIfaceStats(iface.Name)
		if err != nil {
			return QoSStats{}, err
		}
		stats.TotalBytes += ifaceStats.Total.Bytes
		stats.TotalPackets += uint64(ifaceStats.Total.Packets)
		stats.TotalDrops += uint64(ifaceStats.HighPrioClass.Drops + ifaceStats.LowPrioClass.Drops + ifaceStats.DefaultClass.Drops)
		stats.TotalOverlimits += uint64(ifaceStats.HighPrioClass.Overlimits + ifaceStats.LowPrioClass.Overlimits + ifaceStats.DefaultClass.Overlimits)

		stats.ActiveIfaces++
		stats.IfaceStats = append(stats.IfaceStats, ifaceStats)
	}

	stats.ManagedIfaces = len(m.Ifaces)

	return stats, nil
}

func (m *QoSManager) GetIfaceStats(ifaceName string) (IfaceStats, error) {
	iface, found := m.Ifaces[ifaceName]
	if !found {
		netIface, err := net.InterfaceByName(ifaceName)
		if err != nil {
			return IfaceStats{}, err
		}
		iface = Interface{Interface: *netIface}
	}

	ifaceQdisc, err := htb.GetQdisc(m.TcConn, iface.Index, m.Logger)
	if err != nil {
		return IfaceStats{}, err
	}

	m.Ifaces[ifaceName] = iface

	return IfaceStats{
		Name:          iface.Name,
		Total:         htb.GetClassStats(ifaceQdisc.ParentClass),
		HighPrioClass: htb.GetClassStats(ifaceQdisc.HighClass),
		LowPrioClass:  htb.GetClassStats(ifaceQdisc.LowClass),
		DefaultClass:  htb.GetClassStats(ifaceQdisc.DefaultClass),
	}, nil
}
