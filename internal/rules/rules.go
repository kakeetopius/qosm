// Package rules is used to manipulate traffic control rules.
package rules

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"time"

	"github.com/kakeetopius/qosm/internal/core/htb"
	"github.com/kakeetopius/qosm/internal/core/nft"
	"github.com/kakeetopius/qosm/internal/db"
	"github.com/kakeetopius/qosm/internal/util"
)

type Rule struct {
	ID        int
	Target    string
	Type      string
	Priority  string
	CreatedAt time.Time
}

func AddDomainRule(dbCon *sql.DB, htbCtx *htb.HTBCtx, domain string, priority string, logger *slog.Logger) (Rule, error) {
	if htbCtx == nil {
		return Rule{}, fmt.Errorf("htb context not intialised")
	}
	rule := Rule{}
	var err error
	exists, err := db.CheckDomainRuleExists(dbCon, domain)
	if err != nil {
		return rule, err
	}
	if exists {
		return rule, fmt.Errorf("rule for %v already exists", domain)
	}

	_, err = netip.ParseAddr(domain)
	if err == nil {
		return Rule{}, fmt.Errorf("%v seems to be an IP address not a domain", domain)
	}

	util.Debug(logger, "resolving_domain", "domain_name", domain)
	ips, err := net.LookupIP(domain)
	if err != nil {
		util.Error(logger, "resolve_error", "domain_name", domain, "error", err.Error())
		return Rule{}, err
	}

	addrs := util.NetIPtoNetIPPRefix(ips)

	util.Debug(logger, "add_rule", "target", domain, "priority", priority)
	err = addDomainRuleToNft(addrs, priority, htbCtx, logger)
	if err != nil {
		return Rule{}, err
	}

	err = db.AddDomainToPriority(dbCon, domain, priority, addrs)
	if err != nil {
		return rule, err
	}

	domainRule, err := db.GetDomainRuleNameByWithoutIPs(dbCon, domain)
	if err != nil {
		return rule, err
	}

	return Rule{
		Type:      "domain",
		Priority:  domainRule.Priority,
		Target:    domainRule.DomainName,
		ID:        domainRule.ID,
		CreatedAt: domainRule.CreatedAt,
	}, nil
}

func AddIPRule(dbCon *sql.DB, htbCtx *htb.HTBCtx, ip string, priority string, logger *slog.Logger) (Rule, error) {
	if htbCtx == nil {
		return Rule{}, fmt.Errorf("htb context not intialised")
	}
	rule := Rule{}
	exists, err := db.CheckIPRuleExists(dbCon, ip)
	if err != nil {
		return rule, err
	}
	if exists {
		return rule, fmt.Errorf("rule for %v already exists", ip)
	}

	addrs, err := util.TargetsFromString(ip)
	if err != nil {
		return Rule{}, fmt.Errorf("invalid IP address: %v", ip)
	}

	util.Debug(logger, "add_rule", "target", ip, "priority", priority)

	err = addIPRuleToNft(addrs[0], priority, htbCtx, logger)
	if err != nil {
		return Rule{}, err
	}

	ipString := addrs[0].String()
	err = db.AddIPToPriority(dbCon, ipString, priority)
	if err != nil {
		return rule, err
	}

	ipRule, err := db.GetIPRuleByName(dbCon, ipString)
	if err != nil {
		return rule, err
	}

	return Rule{
		Type:      "ip",
		Priority:  ipRule.Priority,
		Target:    ipRule.IP,
		ID:        ipRule.ID,
		CreatedAt: ipRule.CreatedAt,
	}, nil
}

func InitSavedRules(dbCon *sql.DB, htbCtx *htb.HTBCtx, logger *slog.Logger) error {
	ipRules, err := db.GetAllIPRules(dbCon)
	if err != nil {
		return err
	}

	for _, rule := range ipRules {
		ip, ipErr := netip.ParsePrefix(rule.IP)
		if ipErr != nil {
			return ipErr
		}
		ipErr = addIPRuleToNft(ip, rule.Priority, htbCtx, logger)
		if ipErr != nil {
			return ipErr
		}
	}

	domainRules, err := db.GetAllDomainRules(dbCon)
	if err != nil {
		return err
	}

	for _, rule := range domainRules {
		ips, err := rule.IPsAsPrefix()
		if err != nil {
			return err
		}
		err = addDomainRuleToNft(ips, rule.Priority, htbCtx, logger)
		if err != nil {
			return err
		}
	}

	return nil
}

func DeleteDomainRuleByID(dbConn *sql.DB, htbCtx *htb.HTBCtx, domainRuleID int) error {
	domainRule, err := db.GetDomainRuleByID(dbConn, domainRuleID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("no rules to delete for domain with ID %v", domainRuleID)
		}
		return err
	}

	err = db.DeleteDomainRuleByID(dbConn, domainRuleID, domainRule.Priority)
	if err != nil {
		return err
	}

	return deleteDomainRulesFromNft(domainRule, htbCtx)
}

func DeleteDomainRuleByName(dbConn *sql.DB, htbCtx *htb.HTBCtx, name string) error {
	domainRule, err := db.GetDomainRuleByName(dbConn, name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("no rules to delete for domain %v", name)
		}
		return err
	}

	err = db.DeleteDomainRuleByName(dbConn, name, domainRule.Priority)
	if err != nil {
		return err
	}

	return deleteDomainRulesFromNft(domainRule, htbCtx)
}

func DeleteIPRuleByID(dbConn *sql.DB, htbCtx *htb.HTBCtx, ipRuleID int) error {
	ipRule, err := db.GetIPRuleByID(dbConn, ipRuleID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("no rules to delete for IP rule with ID %v", ipRuleID)
		}
		return err
	}

	err = db.DeleteIPRuleByID(dbConn, ipRuleID, ipRule.Priority)
	if err != nil {
		return err
	}

	return deleteIPRuleFromNft(htbCtx, ipRule)
}

func DeleteIPRuleByName(dbConn *sql.DB, htbCtx *htb.HTBCtx, ipRuleName string) error {
	ipRule, err := db.GetIPRuleByName(dbConn, ipRuleName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("no rules to delete for ip %v", ipRuleName)
		}
		return err
	}

	err = db.DeleteIPRuleByName(dbConn, ipRuleName, ipRule.Priority)
	if err != nil {
		return err
	}

	return deleteIPRuleFromNft(htbCtx, ipRule)
}

func DeleteAll(dbConn *sql.DB, htbCtx *htb.HTBCtx) error {
	var err error
	if htbCtx != nil && htbCtx.NFTFilter != nil {
		err = htbCtx.NFTFilter.DeleteTable()
	} else {
		err = nft.DeleteTable()
	}

	if err != nil {
		if !errors.Is(err, nft.ErrTableNotFound) {
			return err
		}
	}

	err = db.FlushDomainRules(dbConn)
	if err != nil {
		return err
	}

	err = db.FlushIPRules(dbConn)
	if err != nil {
		return err
	}

	return nil
}

func GetAll(dbCon *sql.DB) ([]Rule, error) {
	ipRules, err := db.GetAllIPRules(dbCon)
	if err != nil {
		return nil, err
	}
	domainRules, err := db.GetAllDomainRulesWithoutIPs(dbCon)
	if err != nil {
		return nil, err
	}

	return joinIPAndDomainRules(ipRules, domainRules), nil
}

func GetHighPriority(dbCon *sql.DB) ([]Rule, error) {
	highPrioIPRules, err := db.GetHighPrioIPs(dbCon)
	if err != nil {
		return nil, err
	}
	highPrioDomainRules, err := db.GetHighPrioDomains(dbCon)
	if err != nil {
		return nil, err
	}

	return joinIPAndDomainRules(highPrioIPRules, highPrioDomainRules), nil
}

func GetLowPriority(dbCon *sql.DB) ([]Rule, error) {
	lowPrioIPRules, err := db.GetLowPrioIPs(dbCon)
	if err != nil {
		return nil, err
	}
	lowPrioDomainRules, err := db.GetLowPrioDomains(dbCon)
	if err != nil {
		return nil, err
	}

	return joinIPAndDomainRules(lowPrioIPRules, lowPrioDomainRules), nil
}

func joinIPAndDomainRules(ipRules []db.IPRule, domainRules []db.DomainRule) []Rule {
	allRules := make([]Rule, 0, len(ipRules)+len(domainRules))
	for _, rule := range ipRules {
		allRules = append(allRules, Rule{
			ID:        rule.ID,
			Priority:  rule.Priority,
			Target:    rule.IP,
			Type:      "ip",
			CreatedAt: rule.CreatedAt,
		})
	}

	for _, rule := range domainRules {
		allRules = append(allRules, Rule{
			ID:        rule.ID,
			Priority:  rule.Priority,
			Target:    rule.DomainName,
			Type:      "domain",
			CreatedAt: rule.CreatedAt,
		})
	}

	return allRules
}

func deleteDomainRulesFromNft(domainRule db.DomainRule, htbCtx *htb.HTBCtx) error {
	if htbCtx == nil {
		return fmt.Errorf("htb context not intialised")
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
		return htbCtx.NFTFilter.DeleteTargetFromHighPriority(addrs)
	case "low":
		return htbCtx.NFTFilter.DeleteTargetFromLowPriority(addrs)
	default:
		return fmt.Errorf("unknown priority: %v", domainRule.Priority)
	}
}

func deleteIPRuleFromNft(htbCtx *htb.HTBCtx, ipRule db.IPRule) error {
	if htbCtx == nil {
		return fmt.Errorf("htb context not intialised")
	}
	addr, err := netip.ParsePrefix(ipRule.IP)
	if err != nil {
		return err
	}

	switch ipRule.Priority {
	case "high":
		return htbCtx.NFTFilter.DeleteTargetFromHighPriority([]netip.Prefix{addr})
	case "low":
		return htbCtx.NFTFilter.DeleteTargetFromLowPriority([]netip.Prefix{addr})
	default:
		return fmt.Errorf("unknown priority: %v", ipRule.Priority)
	}
}

func addDomainRuleToNft(domainIPs []netip.Prefix, priority string, htbCtx *htb.HTBCtx, logger *slog.Logger) error {
	var prio htb.Priority
	switch priority {
	case "high":
		prio = htb.PRIORITYHIGH
	case "low":
		prio = htb.PRIORITYLOW
	default:
		return fmt.Errorf("unknown priority: %s", priority)
	}

	err := htbCtx.AddRule(domainIPs, prio)
	if err != nil {
		util.Error(logger, "tc_error", "error", err.Error())
		return err
	}

	return nil
}

func addIPRuleToNft(ip netip.Prefix, priority string, htbCtx *htb.HTBCtx, logger *slog.Logger) error {
	var prio htb.Priority
	switch priority {
	case "high":
		prio = htb.PRIORITYHIGH
	case "low":
		prio = htb.PRIORITYLOW
	default:
		return fmt.Errorf("unknown priority: %s", priority)
	}

	err := htbCtx.AddRule([]netip.Prefix{ip}, prio)
	if err != nil {
		util.Error(logger, "tc_error", "error", err.Error())
		return err
	}

	return nil
}
