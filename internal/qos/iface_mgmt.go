package qos

import (
	"errors"
	"fmt"
	"net"

	"github.com/kakeetopius/qosm/internal/core/htb"
	"github.com/kakeetopius/qosm/internal/db"
)

func (m *QoSManager) EnableTcOnInterface(ifaceName string, rate uint32) (err error) {
	if m.Classifier == nil && !m.DaemonMode {
		return ErrClassifierNotInitialised
	}

	if m.Ifaces == nil {
		m.Ifaces = make(map[string]Interface)
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
	iface.ShapingRate = rate

	if m.DaemonMode {
		err = m.sendEnableIfaceRequest(ifaceName, int32(iface.Index), rate)
	} else {
		err = htb.InitHTBOnIface(m.TcConn, iface.Index, rate, m.Logger)
		if err != nil && !errors.Is(err, htb.ErrQdisExists) {
			return err
		}

		err = m.Classifier.AddIfaces([]string{iface.Name})
	}

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

	iface.QoSEnabled = true
	m.Ifaces[iface.Name] = iface

	return nil
}

func (m *QoSManager) DisableTcOnInterface(ifaceName string) (err error) {
	if m.Classifier == nil && !m.DaemonMode {
		return ErrClassifierNotInitialised
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

	if m.DaemonMode {
		err = m.sendDisableIfaceRequest(ifaceName, int32(iface.Index))
	} else {
		err = htb.FlushQdiscFromIface(m.TcConn, iface.Index)
		if err != nil {
			return err
		}

		err = m.Classifier.DeleteIfaces([]string{iface.Name})
	}
	if err != nil {
		return err
	}

	err = db.DisableInterface(m.DB, iface.Name)
	if err != nil {
		return err
	}

	iface.QoSEnabled = false
	m.Ifaces[ifaceName] = iface

	return nil
}

func (m *QoSManager) ChangeInterfaceRate(ifaceName string, rate uint32) (err error) {
	if m.Classifier == nil && !m.DaemonMode {
		return ErrClassifierNotInitialised
	}

	defer func() {
		if err != nil {
			db.AddErrorLog(m.DB, err, "")
		} else {
			addRateChangedLog(m.DB, ifaceName, rate)
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

	if iface.QoSEnabled {
		// first remove qdisc from interface
		if m.DaemonMode {
			err = m.sendDisableIfaceRequest(ifaceName, int32(iface.Index))
		} else {
			err = htb.FlushQdiscFromIface(m.TcConn, iface.Index)
			if err != nil {
				return err
			}
		}
		if err != nil {
			return err
		}

		// create new qdisc with new rate
		if m.DaemonMode {
			err = m.sendEnableIfaceRequest(ifaceName, int32(iface.Index), rate)
		} else {
			err = htb.InitHTBOnIface(m.TcConn, iface.Index, rate, m.Logger)
			if err != nil && !errors.Is(err, htb.ErrQdisExists) {
				return err
			}
		}
	}

	err = db.ChangeInterfaceRate(m.DB, ifaceName, rate)
	if err != nil {
		return err
	}
	iface.ShapingRate = rate // change the rate to the new one
	m.Ifaces[ifaceName] = iface

	return nil
}

func (m *QoSManager) InitSavedInterfaceSettings() error {
	if m.Classifier == nil && !m.DaemonMode {
		return ErrClassifierNotInitialised
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
