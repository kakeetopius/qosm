package routes

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kakeetopius/qosm/internal/core/tc"
	"github.com/kakeetopius/qosm/internal/core/util"
	"github.com/kakeetopius/qosm/web/db"
)

type PostForm struct {
	RuleType string `form:"type"`
	Target   string `form:"target"`
	Priority string `form:"priority"`
}

type Rule struct {
	ID        int
	Target    string
	Type      string
	Priority  string
	CreatedAt time.Time
}

func (app *ServerCtx) PostRules(c *gin.Context) {
	var form PostForm

	if err := c.ShouldBind(&form); err != nil {
		c.Error(fmt.Errorf("invalid form fields"))
		return
	}

	var err error
	var rule Rule
	switch form.RuleType {
	case "ip":
		rule, err = addIPRule(app, form.Target, form.Priority)
	case "domain":
		rule, err = addDomainRule(app, form.Target, form.Priority)
	default:
		err = fmt.Errorf("unknown rule type: %s", form.RuleType)
	}

	if err != nil {
		c.Error(err)
		return
	}

	c.Header("HX-Trigger", `{"toast":"Rule added"}`)
	SendNewRuleRow(c, rule)
}

func (app *ServerCtx) DeleteRule(c *gin.Context) {
	ruleType := c.Param("type")
	ruleID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.Error(fmt.Errorf("invalid id given"))
		return
	}

	switch ruleType {
	case "domain":
		err = deleteDomainRule(app, ruleID)
	case "ip":
		err = deleteIPRule(app, ruleID)
	}

	if err != nil {
		c.Error(err)
		return
	}

	c.Header("HX-Trigger", `{"toast":"Rule deleted successfully"}`)
}

func SendNewRuleRow(c *gin.Context, rule Rule) {
	c.HTML(http.StatusOK, "rule_table_row", gin.H{
		"Rule": rule,
	})
}

func addDomainRule(app *ServerCtx, domain string, priority string) (Rule, error) {
	exists, err := db.CheckDomainRuleExists(app.DB, domain)
	if err != nil {
		return Rule{}, err
	}
	if exists {
		return Rule{}, fmt.Errorf("rule for %v already exists", domain)
	}

	var prio tc.Priority
	switch priority {
	case "high":
		prio = tc.PRIORITYHIGH
	case "low":
		prio = tc.PRIORITYLOW
	default:
		return Rule{}, fmt.Errorf("unknown priority: %s", priority)
	}

	_, err = netip.ParseAddr(domain)
	if err == nil {
		return Rule{}, fmt.Errorf("%v seems to be an IP address not a domain", domain)
	}

	app.Logger.Info("resolving_domain", "domain", domain)
	ips, err := net.LookupIP(domain)
	if err != nil {
		app.Logger.Error("resolve_error", "domain", domain, "error", err.Error())
		return Rule{}, err
	}
	addrs := util.NetIPtoNetIPPRefix(ips)

	app.Logger.Info("add_rule", "target", domain, "priority", priority)

	err = app.HTBCtx.AddRule(addrs, prio)
	if err != nil {
		app.Logger.Error("tc_error", "error", err.Error())
		return Rule{}, err
	}
	err = db.AddDomainToPriority(app.DB, domain, priority, addrs)
	if err != nil {
		return Rule{}, err
	}

	rule, err := db.GetDomainRuleNameByWithoutIPs(app.DB, domain)
	if err != nil {
		return Rule{}, err
	}

	return Rule{
		Type:      "domain",
		Priority:  rule.Priority,
		Target:    rule.DomainName,
		ID:        rule.ID,
		CreatedAt: rule.CreatedAt,
	}, nil
}

func addIPRule(app *ServerCtx, ip string, priority string) (Rule, error) {
	exists, err := db.CheckIPRuleExists(app.DB, ip)
	if err != nil {
		return Rule{}, err
	}
	if exists {
		return Rule{}, fmt.Errorf("rule for %v already exists", ip)
	}

	var prio tc.Priority
	switch priority {
	case "high":
		prio = tc.PRIORITYHIGH
	case "low":
		prio = tc.PRIORITYLOW
	default:
		return Rule{}, fmt.Errorf("unknown priority: %s", priority)
	}

	addrs, err := util.TargetsFromString(ip)
	if err != nil {
		return Rule{}, fmt.Errorf("invalid IP address: %v", ip)
	}

	app.Logger.Info("add_rule", "target", ip, "priority", priority)

	err = app.HTBCtx.AddRule(addrs, prio)
	if err != nil {
		app.Logger.Error("tc_error", "error", err.Error())
		return Rule{}, err
	}

	ipString := addrs[0].String()
	err = db.AddIPToPriority(app.DB, ipString, priority)
	if err != nil {
		return Rule{}, err
	}

	rule, err := db.GetIPRuleByName(app.DB, ipString)
	if err != nil {
		return Rule{}, err
	}

	return Rule{
		Type:      "ip",
		Priority:  rule.Priority,
		Target:    rule.IP,
		ID:        rule.ID,
		CreatedAt: rule.CreatedAt,
	}, nil
}

func deleteDomainRule(app *ServerCtx, domainRuleID int) error {
	domainRule, err := db.GetDomainRuleByID(app.DB, domainRuleID)
	if err != nil {
		return err
	}

	addrs := make([]netip.Prefix, 0, len(domainRule.IPs))
	for _, addr := range domainRule.IPs {
		ip, iperr := netip.ParsePrefix(addr.IP)
		if iperr != nil {
			return iperr
		}
		addrs = append(addrs, ip)
	}

	switch domainRule.Priority {
	case "high":
		err = app.HTBCtx.NFTFilter.DeleteTargetFromHighPriority(addrs)
	case "low":
		err = app.HTBCtx.NFTFilter.DeleteTargetFromLowPriority(addrs)
	default:
		return fmt.Errorf("unknown priority: %v", domainRule.Priority)
	}
	if err != nil {
		return err
	}

	return db.DeleteDomainRuleByID(app.DB, domainRuleID)
}

func deleteIPRule(app *ServerCtx, ipRuleID int) error {
	ipRule, err := db.GetIPRuleByID(app.DB, ipRuleID)
	if err != nil {
		return err
	}
	addr, err := netip.ParsePrefix(ipRule.IP)
	if err != nil {
		return err
	}

	switch ipRule.Priority {
	case "high":
		err = app.HTBCtx.NFTFilter.DeleteTargetFromHighPriority([]netip.Prefix{addr})
	case "low":
		err = app.HTBCtx.NFTFilter.DeleteTargetFromLowPriority([]netip.Prefix{addr})
	default:
		return fmt.Errorf("unknown priority: %v", ipRule.Priority)
	}
	if err != nil {
		return err
	}

	return db.DeleteIPRuleByID(app.DB, ipRuleID)
}

func getAllRules(app *ServerCtx) ([]Rule, error) {
	ipRules, err := db.GetAllIPRules(app.DB)
	if err != nil {
		return nil, err
	}
	domainRules, err := db.GetAllDomainRulesWithoutIPs(app.DB)
	if err != nil {
		return nil, err
	}

	rules := make([]Rule, 0, len(ipRules)+len(domainRules))
	for _, rule := range ipRules {
		rules = append(rules, Rule{
			ID:        rule.ID,
			Priority:  rule.Priority,
			Target:    rule.IP,
			Type:      "ip",
			CreatedAt: rule.CreatedAt,
		})
	}

	for _, rule := range domainRules {
		rules = append(rules, Rule{
			ID:        rule.ID,
			Priority:  rule.Priority,
			Target:    rule.DomainName,
			Type:      "domain",
			CreatedAt: rule.CreatedAt,
		})
	}

	return rules, nil
}
