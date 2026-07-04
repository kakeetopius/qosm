// Package qos
package qos

import (
	"database/sql"
	"errors"
	"log/slog"
	"math"
	"net"
	"net/netip"
	"os"

	"github.com/florianl/go-tc"
	"github.com/kakeetopius/qosm/internal/core/nft"
	"github.com/kakeetopius/qosm/internal/db"
	"github.com/kakeetopius/qosm/internal/service"
	"github.com/mdlayher/ethtool"
)

func NewManager(dbCon *sql.DB) (*QoSManager, error) {
	tcnl, err := tc.Open(&tc.Config{})
	if err != nil {
		return nil, err
	}

	qosManager := QoSManager{
		Ifaces: make(map[string]Interface),
		TcConn: tcnl,
		DB:     dbCon,
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, iface := range ifaces {
		speed, err := getInterfaceSpeed(iface.Name)
		if err != nil {
			return nil, err
		}

		exists, err := db.CheckInterfaceExists(dbCon, iface.Name)
		if err != nil {
			return nil, err
		}

		var rate uint32
		if exists {
			dbrate, err := db.GetInterfaceField(dbCon, iface.Name, "rate")
			if err != nil {
				return nil, err
			}
			rate64 := dbrate.(int64)
			rate = uint32(rate64)
		}

		qosManager.Ifaces[iface.Name] = Interface{
			Interface:   iface,
			LinkSpeed:   speed,
			ShapingRate: rate,
		}
	}

	return &qosManager, nil
}

func (m *QoSManager) WithLogger(l *slog.Logger) {
	m.Logger = l
}

func (m *QoSManager) InitQoSClassifier(createIfNotExists bool) error {
	nftCtx, err := nft.NewNFTCtx(nft.NFTOpts{
		CreateIfNotExists: createIfNotExists,
		Logger:            m.Logger,
	})
	if err != nil {
		return err
	}
	m.Classifier = &nftCtx

	return nil
}

func (m *QoSManager) InitSavedRules() error {
	ipRules, err := db.GetAllIPRules(m.DB)
	if err != nil {
		return err
	}

	for _, rule := range ipRules {
		ip, ipErr := netip.ParsePrefix(rule.IP)
		if ipErr != nil {
			return ipErr
		}
		ipErr = m.Classifier.AddIPsToPriority([]netip.Prefix{ip}, rule.Priority)
		if ipErr != nil {
			return ipErr
		}
	}

	domainRules, err := db.GetAllDomainRules(m.DB)
	if err != nil {
		return err
	}

	for _, rule := range domainRules {
		ips, ipErr := rule.IPsAsPrefix()
		if ipErr != nil {
			return ipErr
		}
		err = m.Classifier.AddIPsToPriority(ips, rule.Priority)
		if err != nil {
			return err
		}
	}

	serviceRules, err := db.GetAllServiceRules(m.DB)
	if err != nil {
		return err
	}

	for _, rule := range serviceRules {
		err = m.Classifier.AddServicesToPriority([]service.Service{rule.Service}, rule.Priority)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *QoSManager) Close() {
	m.TcConn.Close()
}

func (m *QoSManager) DeleteAllRules() error {
	var err error
	if m.Classifier != nil {
		err = m.Classifier.DeleteTable()
	} else {
		err = nft.DeleteTable()
	}

	if err != nil {
		if !errors.Is(err, nft.ErrTableNotFound) {
			return err
		}
	}

	err = db.FlushDomainRules(m.DB)
	if err != nil {
		return err
	}

	err = db.FlushIPRules(m.DB)
	if err != nil {
		return err
	}

	err = db.FlushServiceRules(m.DB)
	if err != nil {
		return err
	}

	return nil
}

func getInterfaceSpeed(ifName string) (uint32, error) {
	client, err := ethtool.New()
	if err != nil {
		return 0, err
	}

	linkMode, err := client.LinkMode(ethtool.Interface{Name: ifName})
	if err != nil {
		if errors.Is(err, os.ErrNotExist) { // returned if the interface is not an ethernet interface.
			return 0, nil
		}
		return 0, err
	}

	speed := linkMode.SpeedMegabits
	if speed == math.MaxUint32 { // returned if the interface has speed of -1 meaning speed is not known to kernel
		speed = 0
	}

	return uint32(speed), nil
}
