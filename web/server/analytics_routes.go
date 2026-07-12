// Package server contains code for the web server
package server

import (
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func (app *Server) AnalyticsPage(c *gin.Context) {
	session := sessions.Default(c)
	stats, err := app.QoSManager.GetStats()
	if err != nil {
		c.Error(err)
		return
	}
	c.HTML(http.StatusOK, "analytics", gin.H{
		"Title":       "Analytics - QoS Manager",
		"Heading":     "Analytics",
		"Description": "Network usage insights and QoS effectiveness",
		"User":        session.Get("username"),
		"Role":        session.Get("role"),
		"Stats":       stats,
	})
}

func (app *Server) AnalyticsRefresh(c *gin.Context) {
	stats, err := app.QoSManager.GetStats()
	if err != nil {
		c.Error(err)
		return
	}
	c.HTML(http.StatusOK, "analytics_refresh", gin.H{
		"Stats": stats,
	})
}
