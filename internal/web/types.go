package web

import (
	"database/sql"

	"github.com/kakeetopius/qosm/internal/core/tc"
)

type ServerCtx struct {
	db     sql.DB
	htbctx tc.HTBCtx
}

type ServerError struct {
	StatusCode int
	Err        error
}

func (e ServerError) Error() string {
	return e.Err.Error()
}

type ServerOptions struct {
	Port int
}
