// Package qos
package qos

import (
	"database/sql"
	"errors"
	"log/slog"
	"math"
	"net"
	"os"

	"github.com/florianl/go-tc"
	"github.com/kakeetopius/qosm/internal/core/htb"
	"github.com/kakeetopius/qosm/internal/core/nft"
	"github.com/kakeetopius/qosm/internal/db"
	"github.com/mdlayher/ethtool"
)

type Interface struct {
	net.Interface
	htb.HTBObjects
	QoSEnabled  bool
	LinkSpeed   uint32
	ShapingRate uint32
	// use LinkSpeed as ShapingRate
	AutoRate bool
}

type QoSManager struct {
	TcConn     *tc.Tc
	Ifaces     map[string]Interface
	DB         *sql.DB
	Classifier *nft.NFT
	Logger     *slog.Logger
}

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

func (m *QoSManager) Close() {
	m.TcConn.Close()
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
