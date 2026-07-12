package qos

import (
	"errors"
	"fmt"

	"github.com/kakeetopius/qosm/internal/core/htb"
	"github.com/kakeetopius/qosm/internal/db"
)

const DEFAULTRATE uint32 = 100

func (m *QoSManager) EnableTcOnInterface(ifaceName string, rate *uint32, classPercentages *htb.ClassPercentages) (err error) {
	if m.Classifier == nil && !m.DaemonMode {
		return ErrClassifierNotInitialised
	}

	if m.Ifaces == nil {
		m.Ifaces = make(map[string]Interface)
	}

	if rate == nil {
		r := DEFAULTRATE
		rate = &r
	}

	if *rate == 0 {
		return fmt.Errorf("invalid rate: %v", rate)
	}

	if classPercentages == nil {
		percentages := htb.DefaultClassPercentages()
		classPercentages = &percentages
	}

	err = classPercentages.Verify()
	if err != nil {
		return err
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
		return fmt.Errorf("unknown interface: %v", ifaceName)
	}
	defer func() {
		m.Ifaces[ifaceName] = iface
	}()

	qdiscExists, err := htb.HasHTBQdisc(m.TcConn, iface.Index)
	if err != nil {
		return err
	}
	if qdiscExists {
		return htb.ErrQdiscExists
	}

	if m.DaemonMode {
		err = m.sendEnableIfaceRequest(ifaceName, int32(iface.Index), *rate, *classPercentages)
	} else {
		err = htb.InitHTBOnIface(m.TcConn, iface.Index, *rate, *classPercentages, m.Logger)
		if err != nil {
			return err
		}

		err = m.Classifier.AddIfaces([]string{iface.Name})
	}
	if err != nil {
		return err
	}

	err = db.AddInterface(m.DB, db.DBInterface{
		Name:        iface.Name,
		IfaceIndex:  iface.Index,
		Enabled:     true,
		Rate:        *rate,
		Percentages: *classPercentages,
	})
	if err != nil {
		return err
	}

	iface.QoSEnabled = true
	iface.ShapingRate = *rate
	iface.Percentages = *classPercentages

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
		return fmt.Errorf("unknown interface: %v", ifaceName)
	}

	if m.DaemonMode {
		err = m.sendDisableIfaceRequest(ifaceName, int32(iface.Index))
	} else {
		err = htb.FlushQdiscFromIface(m.TcConn, iface.Index)
		if err != nil && !errors.Is(err, htb.ErrQdiscNotFound) {
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
		return fmt.Errorf("unknown interface: %v", ifaceName)
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
			err = m.sendEnableIfaceRequest(ifaceName, int32(iface.Index), rate, iface.Percentages)
		} else {
			err = htb.InitHTBOnIface(m.TcConn, iface.Index, rate, iface.Percentages, m.Logger)
			if err != nil && !errors.Is(err, htb.ErrQdiscExists) {
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

func (m *QoSManager) ChangeClassPercentages(ifaceName string, newPercentages htb.ClassPercentages) (err error) {
	if m.Classifier == nil && !m.DaemonMode {
		return ErrClassifierNotInitialised
	}

	defer func() {
		if err != nil {
			db.AddErrorLog(m.DB, err, "")
		} else {
			addClassPercentagesChangedLog(m.DB, ifaceName, newPercentages)
		}
	}()

	err = newPercentages.Verify()
	if err != nil {
		return err
	}

	iface, found := m.Ifaces[ifaceName]
	if !found {
		return fmt.Errorf("unknown interface: %v", ifaceName)
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
			err = m.sendEnableIfaceRequest(ifaceName, int32(iface.Index), iface.ShapingRate, newPercentages)
		} else {
			err = htb.InitHTBOnIface(m.TcConn, iface.Index, iface.ShapingRate, newPercentages, m.Logger)
			if err != nil && !errors.Is(err, htb.ErrQdiscExists) {
				return err
			}
		}
	}

	err = db.ChangeInterfaceClassPercentages(m.DB, ifaceName, newPercentages)
	if err != nil {
		return err
	}
	iface.Percentages = newPercentages
	m.Ifaces[ifaceName] = iface

	return nil
}

func (m *QoSManager) RestoreInterfaceStates() error {
	if m.Classifier == nil && !m.DaemonMode {
		return ErrClassifierNotInitialised
	}

	for _, iface := range m.Ifaces {
		if !iface.QoSEnabled {
			continue
		}
		err := m.EnableTcOnInterface(iface.Name, &iface.ShapingRate, &iface.Percentages)
		if err != nil && !errors.Is(err, htb.ErrQdiscExists) {
			return err
		}
	}

	return nil
}
