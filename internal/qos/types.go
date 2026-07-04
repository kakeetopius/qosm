package qos

import (
	"database/sql"
	"log/slog"
	"net"
	"time"

	"github.com/florianl/go-tc"
	"github.com/kakeetopius/qosm/internal/core/htb"
	"github.com/kakeetopius/qosm/internal/core/nft"
	"github.com/kakeetopius/qosm/internal/priority"
)

type QoSManager struct {
	TcConn     *tc.Tc
	Ifaces     map[string]Interface
	DB         *sql.DB
	Classifier *nft.NFT
	Logger     *slog.Logger
}

type Interface struct {
	net.Interface
	htb.HTBObjects
	QoSEnabled  bool
	LinkSpeed   uint32
	ShapingRate uint32
	// use LinkSpeed as ShapingRate
	AutoRate bool
}

type HostRule struct {
	ID        int
	Target    string
	Type      string // ip or domain
	Priority  priority.Priority
	CreatedAt time.Time
}
