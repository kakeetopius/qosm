package htb

import (
	"errors"
	"log/slog"
	"net"

	"github.com/florianl/go-tc"
)

func InitHTBOnIface(tcnl *tc.Tc, ifIndex int, rate uint32, logger *slog.Logger) error {
	_, err := findRootQdisc(tcnl, ifIndex)
	if err == nil {
		return ErrQdisExists
	} else if !errors.Is(err, ErrQdiscNotFound) {
		return err
	}

	_, err = CreateQdisc(tcnl, ifIndex, rate, logger)
	if err != nil {
		return err
	}

	return nil
}

func FlushQdiscFromIface(tcnl *tc.Tc, ifIndex int) error {
	root, err := findRootQdisc(tcnl, ifIndex)
	if err != nil {
		return err
	}
	return deleteQdisc(tcnl, root)
}

func HasHTBQdisc(iface *net.Interface) (bool, error) {
	tcnl, err := tc.Open(&tc.Config{})
	if err != nil {
		return false, err
	}

	_, err = findRootQdisc(tcnl, iface.Index)
	if err != nil {
		if errors.Is(err, ErrQdiscNotFound) {
			return false, nil
		}
		return false, err
	} else {
		return true, nil
	}
}

func FindHTBEnabledIfaces() ([]net.Interface, error) {
	devs, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	htbEnabledIfaces := make([]net.Interface, 0, len(devs))
	for _, dev := range devs {
		htbEnabled, err := HasHTBQdisc(&dev)
		if err != nil {
			return nil, err
		}
		if htbEnabled {
			htbEnabledIfaces = append(htbEnabledIfaces, dev)
		}
	}

	return htbEnabledIfaces, nil
}
