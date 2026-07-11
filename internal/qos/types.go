package qos

import (
	"database/sql"
	"errors"
	"log/slog"
	"net"
	"time"

	"github.com/florianl/go-tc"
	"github.com/kakeetopius/qosm/internal/core/nft"
	"github.com/kakeetopius/qosm/internal/priority"
)

var ErrClassifierNotInitialised = errors.New("nft classifier not initialised. either run in daemon mode or initialise the classifier which requires root priviledges")

type Options struct {
	DB         *sql.DB
	Logger     *slog.Logger
	DaemonMode bool // whether to send priviledged operations eg adding nft rules, adding htb qdisc to interface to the daemon
	DaemonSock string
}

type QoSManager struct {
	TcConn     *tc.Tc
	Ifaces     map[string]Interface
	Classifier *nft.NFT

	Options
}

type Interface struct {
	net.Interface
	QoSEnabled  bool
	LinkSpeed   uint32
	ShapingRate uint32
	// use LinkSpeed as ShapingRate
	AutoRate bool
}

type Rule struct {
	ID        int
	Target    string
	Type      string // ip or domain or service
	Priority  priority.Priority
	CreatedAt time.Time
}
