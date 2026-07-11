package server

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/kakeetopius/qosm/internal/qos"
)

type PostForm struct {
	RuleType string `form:"type"`
	Target   string `form:"target"`
	Priority string `form:"priority"`
}

func (app *Server) PostHostRules(c *gin.Context) {
	var form PostForm

	if err := c.ShouldBind(&form); err != nil {
		c.Error(fmt.Errorf("invalid form fields"))
		return
	}

	var err error
	var rule qos.Rule
	switch form.RuleType {
	case "ip":
		rule, err = app.QoSManager.AddIPRule(form.Target, form.Priority)
	case "domain":
		rule, err = app.QoSManager.AddDomainRule(form.Target, form.Priority)
	case "service":
		rule, err = app.QoSManager.AddServiceRule(form.Target, form.Priority)
	default:
		err = fmt.Errorf("unknown rule type: %s", form.RuleType)
	}

	if err != nil {
		c.Error(err)
		return
	}

	SendNewRuleRow(c, rule)
}

func (app *Server) DeleteHostRule(c *gin.Context) {
	ruleType := c.Param("type")
	ruleID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.Error(fmt.Errorf("invalid id given"))
		return
	}

	switch ruleType {
	case "domain":
		err = app.QoSManager.DeleteDomainRuleByID(ruleID)
	case "ip":
		err = app.QoSManager.DeleteIPRuleByID(ruleID)
	case "service":
		err = app.QoSManager.DeleteServiceRuleByID(ruleID)
	default:
		err = fmt.Errorf("unknown rule type: %s", ruleType)
	}

	if err != nil {
		c.Error(err)
		return
	}

	SendSuccessMessage(c, "Successfully deleted rule.")
}

func SendNewRuleRow(c *gin.Context, rule qos.Rule) {
	c.HTML(http.StatusOK, "rule_table_row", gin.H{
		"Message": "Successfully added rule for " + rule.Target,
		"Rule":    rule,
	})
}
