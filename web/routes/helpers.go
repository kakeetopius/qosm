package routes

import (
	"database/sql"
	"log/slog"
	"net"

	"github.com/kakeetopius/qosm/internal/core/tc"
	"github.com/kakeetopius/qosm/web/db"
)

type Interface struct {
	net.Interface
	Enabled bool
}

type ServerCtx struct {
	DB       *sql.DB
	Logger   *slog.Logger
	Ifaces   map[string]Interface
	HTBCtx   *tc.HTBCtx
	Settings *db.Settings
}

type ServerError struct {
	StatusCode int
	Err        error
}

func (e ServerError) Error() string {
	return e.Err.Error()
}

func (app *ServerCtx) EnabledIfaces() []Interface {
	ifaces := make([]Interface, 0, 5)

	for _, iface := range app.Ifaces {
		if !iface.Enabled {
			continue
		}
		ifaces = append(ifaces, iface)
	}

	return ifaces
}
