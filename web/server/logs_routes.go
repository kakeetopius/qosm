package server

import (
	"net/http"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/kakeetopius/qosm/internal/db"
)

func (app *Server) LogsPage(c *gin.Context) {
	session := sessions.Default(c)
	logs, stats, err := db.GetLogsWithStats(app.DB)
	if err != nil {
		c.Error(err)
		return
	}
	c.HTML(http.StatusOK, "logs", gin.H{
		"Title":       "Logs - QoS Manager",
		"Heading":     "Logs",
		"Description": "Real-time QoS engine and network activity logs",
		"User":        session.Get("username"),
		"Role":        session.Get("role"),
		"Logs":        logs,
		"Stats":       stats,
	})
}

func (app *Server) LogsFilter(c *gin.Context) {
	filter := c.Query("event_filter")
	if filter == "" {
		filter = "all"
	}
	var logs []db.Log
	var err error
	if filter == "all" {
		logs, err = db.GetLogs(app.DB)
	} else {
		logs, err = db.GetLogsOfEvent(app.DB, strings.ToUpper(filter))
	}

	if err != nil {
		c.Error(err)
		return
	}

	c.HTML(http.StatusOK, "logs_view", gin.H{
		"Logs": logs,
	})
}

func (app *Server) LogsDelete(c *gin.Context) {
	err := db.DeleteAllLogs(app.DB)
	if err != nil {
		c.Error(err)
		return
	}
}
