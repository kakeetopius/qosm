package qos

import (
	"fmt"
	"net"

	"github.com/kakeetopius/qosm/internal/core/htb"
	"github.com/kakeetopius/qosm/internal/db"
)

func (m *QoSManager) EnableTcOnInterface(ifaceName string, rate uint32) (err error) {
	if m.Ifaces == nil {
		m.Ifaces = make(map[string]Interface)
	}

	if m.Classifier == nil {
		return fmt.Errorf("nft classifier not intialised")
	}

	if rate == 0 {
		return fmt.Errorf("invalid rate: %v", rate)
	}

	defer func() {
		if err != nil {
			db.AddErrorLog(m.DB, err, "")
		} else {
			addTCEnabledLog(m.DB, ifaceName)
		}
	}()

	iface, found := m.Ifaces[ifaceName]
	if !found {
		netIface, neterr := net.InterfaceByName(ifaceName)
		if neterr != nil {
			return neterr
		}
		iface.Interface = *netIface
	}

	htbObjs, err := htb.InitHTBOnIface(m.TcConn, iface.Index, m.Logger)
	if err != nil {
		return err
	}

	err = m.Classifier.AddIfaceRules(iface.Index)
	if err != nil {
		return err
	}

	err = db.AddInterface(m.DB, db.DBInterface{
		Name:       iface.Name,
		IfaceIndex: iface.Index,
		Enabled:    true,
		Rate:       rate,
	})
	if err != nil {
		return err
	}

	iface.HTBObjects = htbObjs
	iface.QoSEnabled = true
	iface.ShapingRate = rate
	m.Ifaces[iface.Name] = iface

	return nil
}

func (m *QoSManager) DisableTcOnInterface(ifaceName string) (err error) {
	if m.Classifier == nil {
		return fmt.Errorf("nft classifier not intialised")
	}

	defer func() {
		if err != nil {
			db.AddErrorLog(m.DB, err, "")
		} else {
			addTCDisabledLog(m.DB, ifaceName)
		}
	}()

	iface, found := m.Ifaces[ifaceName]
	if !found {
		netIface, netErr := net.InterfaceByName(ifaceName)
		if netErr != nil {
			return netErr
		}
		iface = Interface{
			Interface: *netIface,
		}
	}

	err = htb.FlushQdiscFromIface(m.TcConn, iface.Index)
	if err != nil {
		return err
	}

	if m.Classifier != nil {
		err = m.Classifier.DeleteIfaceRules(iface.Index)
		if err != nil {
			return err
		}
	}

	err = db.DisableInterface(m.DB, iface.Name)
	if err != nil {
		return err
	}

	iface.QoSEnabled = false
	m.Ifaces[ifaceName] = iface

	return nil
}

func (m *QoSManager) InitSavedInterfaceSettings() error {
	if m.Classifier == nil {
		return fmt.Errorf("nft filter not intialised")
	}
	enabledIfaces, err := db.GetEnabledInterfaces(m.DB)
	if err != nil {
		return err
	}

	for _, iface := range enabledIfaces {
		err = m.EnableTcOnInterface(iface.Name, iface.Rate)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *QoSManager) EnabledInterfaces() []Interface {
	enabled := make([]Interface, 0, len(m.Ifaces))
	for _, iface := range m.Ifaces {
		if iface.QoSEnabled {
			enabled = append(enabled, iface)
		}
	}

	return enabled
}
