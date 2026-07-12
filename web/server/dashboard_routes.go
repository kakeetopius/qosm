package server

import (
	"net/http"
	"slices"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/kakeetopius/qosm/internal/priority"
	"github.com/kakeetopius/qosm/internal/qos"
)

type DashBoardStats struct {
	HighPrioTargets int
	LowPrioTargets  int
	TotalTargets    int
	TotalDomains    int
	TotalIPs        int
	TotalServices   int
}

func (app *Server) DashboardPage(c *gin.Context) {
	session := sessions.Default(c)

	allRules, err := app.QoSManager.GetAllRules()
	if err != nil {
		c.Error(err)
		return
	}
	slices.SortFunc(allRules, func(a, b qos.Rule) int {
		return -a.CreatedAt.Compare(b.CreatedAt)
	})

	rulesToDisplay := allRules
	if len(allRules) > 5 {
		rulesToDisplay = allRules[:5]
	}

	c.HTML(http.StatusOK, "dashboard", gin.H{
		"Title":       "DashBoard - QoS Manager",
		"Heading":     "Dashboard",
		"Description": "Overview of network traffic and QoS policies",
		"User":        session.Get("username"),
		"Role":        session.Get("role"),
		"Rules":       rulesToDisplay,
		"Stats":       dashBoardStats(allRules),
		"Ifaces":      app.QoSManager.Ifaces,
	})
}

func dashBoardStats(rules []qos.Rule) DashBoardStats {
	stats := DashBoardStats{}

	for _, rule := range rules {
		switch rule.Type {
		case "domain":
			stats.TotalDomains++
		case "ip":
			stats.TotalIPs++
		case "service":
			stats.TotalServices++
		}

		switch rule.Priority {
		case priority.PRIORITYHIGH:
			stats.HighPrioTargets++
		case priority.PRIORITYLOW:
			stats.LowPrioTargets++
		}
	}

	stats.TotalTargets = len(rules)

	return stats
}
