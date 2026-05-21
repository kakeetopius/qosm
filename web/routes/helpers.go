package routes

import (
	"database/sql"
	"log/slog"

	"github.com/kakeetopius/qosm/web/db"
)

type ServerCtx struct {
	DB       *sql.DB
	Logger   *slog.Logger
	Ifaces   map[string]db.Interface
	Settings *db.Settings
}

type ServerError struct {
	StatusCode int
	Err        error
}

func (e ServerError) Error() string {
	return e.Err.Error()
}

func (app *ServerCtx) EnabledIfaces() []db.Interface {
	ifaces := make([]db.Interface, 0, 5)

	for _, iface := range app.Ifaces {
		if !iface.Enabled {
			continue
		}
		ifaces = append(ifaces, iface)
	}

	return ifaces
}
