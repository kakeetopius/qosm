package qos

import (
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"time"

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

func (m *QoSManager) AddDomainRule(domain string, priority string) (rule Rule, err error) {
	defer func() {
		if err != nil {
			db.AddErrorLog(m.DB, err, "")
		} else {
			addRuleSuccessLog(m.DB, domain, priority)
		}
	}()

	exists, err := db.CheckDomainRuleExists(m.DB, domain)
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

	util.Debug(m.Logger, "resolving_domain", "domain_name", domain)
	ips, err := net.LookupIP(domain)
	if err != nil {
		util.Debug(m.Logger, "resolve_error", "domain_name", domain, "error", err.Error())
		return Rule{}, err
	}

	db.AddLog(m.DB, db.Log{
		EventType:   "DNS",
		Description: "Resolved domain " + domain + " to " + ipSliceToString(ips),
	})

	addrs := util.NetIPtoNetIPPRefix(ips)

	util.Debug(m.Logger, "add_rule", "target", domain, "priority", priority)

	err = m.Classifier.AddTargetsToPriority(addrs, priority)
	if err != nil {
		return Rule{}, err
	}

	err = db.AddDomainToPriority(m.DB, domain, priority, addrs)
	if err != nil {
		return rule, err
	}

	domainRule, err := db.GetDomainRuleByName(m.DB, domain)
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

func (m *QoSManager) AddIPRule(ip string, priority string) (rule Rule, err error) {
	defer func() {
		if err != nil {
			db.AddErrorLog(m.DB, err, "")
		} else {
			addRuleSuccessLog(m.DB, ip, priority)
		}
	}()

	addrs, err := util.TargetsFromString(ip)
	if err != nil {
		return Rule{}, fmt.Errorf("invalid IP address: %v", ip)
	}

	exists, err := db.CheckIPRuleExists(m.DB, addrs[0].String())
	if err != nil {
		return rule, err
	}
	if exists {
		return rule, fmt.Errorf("rule for %v already exists", ip)
	}

	util.Debug(m.Logger, "add_rule", "target", ip, "priority", priority)

	err = m.Classifier.AddTargetsToPriority(addrs, priority)
	if err != nil {
		return Rule{}, err
	}

	ipString := addrs[0].String()
	err = db.AddIPToPriority(m.DB, ipString, priority)
	if err != nil {
		return rule, err
	}

	ipRule, err := db.GetIPRuleByName(m.DB, ipString)
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

func (m *QoSManager) InitSavedRules() error {
	ipRules, err := db.GetAllIPRules(m.DB)
	if err != nil {
		return err
	}

	for _, rule := range ipRules {
		ip, ipErr := netip.ParsePrefix(rule.IP)
		if ipErr != nil {
			return ipErr
		}
		ipErr = m.Classifier.AddTargetsToPriority([]netip.Prefix{ip}, rule.Priority)
		if ipErr != nil {
			return ipErr
		}
	}

	domainRules, err := db.GetAllDomainRules(m.DB)
	if err != nil {
		return err
	}

	for _, rule := range domainRules {
		ips, err := rule.IPsAsPrefix()
		if err != nil {
			return err
		}
		err = m.Classifier.AddTargetsToPriority(ips, rule.Priority)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *QoSManager) DeleteDomainRuleByID(domainRuleID int) (err error) {
	domainRule, err := db.GetDomainRuleByID(m.DB, domainRuleID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("no rules to delete for domain with ID %v", domainRuleID)
		}
		return err
	}

	defer func() {
		if err != nil {
			db.AddErrorLog(m.DB, err, "")
		} else {
			addRuleDeletedLog(m.DB, domainRule.DomainName, domainRule.Priority)
		}
	}()

	err = db.DeleteDomainRuleByID(m.DB, domainRuleID, domainRule.Priority)
	if err != nil {
		return err
	}

	return m.deleteDomainAddrs(domainRule)
}

func (m *QoSManager) DeleteDomainRuleByName(name string) error {
	domainRule, err := db.GetDomainRuleByName(m.DB, name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("no rules to delete for domain %v", name)
		}
		return err
	}

	defer func() {
		if err != nil {
			db.AddErrorLog(m.DB, err, "")
		} else {
			addRuleDeletedLog(m.DB, domainRule.DomainName, domainRule.Priority)
		}
	}()

	err = db.DeleteDomainRuleByName(m.DB, name, domainRule.Priority)
	if err != nil {
		return err
	}

	return m.deleteDomainAddrs(domainRule)
}

func (m *QoSManager) DeleteIPRuleByID(ipRuleID int) error {
	ipRule, err := db.GetIPRuleByID(m.DB, ipRuleID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("no rules to delete for IP rule with ID %v", ipRuleID)
		}
		return err
	}

	defer func() {
		if err != nil {
			db.AddErrorLog(m.DB, err, "")
		} else {
			addRuleDeletedLog(m.DB, ipRule.IP, ipRule.Priority)
		}
	}()

	err = db.DeleteIPRuleByID(m.DB, ipRuleID, ipRule.Priority)
	if err != nil {
		return err
	}

	addr, err := netip.ParsePrefix(ipRule.IP)
	if err != nil {
		return err
	}

	return m.Classifier.DeleteTargetsFromPriority([]netip.Prefix{addr}, ipRule.Priority)
}

func (m *QoSManager) DeleteIPRuleByName(ipRuleName string) error {
	ipRule, err := db.GetIPRuleByName(m.DB, ipRuleName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("no rules to delete for ip %v", ipRuleName)
		}
		return err
	}

	defer func() {
		if err != nil {
			db.AddErrorLog(m.DB, err, "")
		} else {
			addRuleDeletedLog(m.DB, ipRule.IP, ipRule.Priority)
		}
	}()

	err = db.DeleteIPRuleByName(m.DB, ipRuleName, ipRule.Priority)
	if err != nil {
		return err
	}

	addr, err := netip.ParsePrefix(ipRule.IP)
	if err != nil {
		return err
	}
	return m.Classifier.DeleteTargetsFromPriority([]netip.Prefix{addr}, ipRule.Priority)
}

func (m *QoSManager) DeleteAllRules() error {
	var err error
	if m.Classifier != nil {
		err = m.Classifier.DeleteTable()
	} else {
		err = nft.DeleteTable()
	}

	if err != nil {
		if !errors.Is(err, nft.ErrTableNotFound) {
			return err
		}
	}

	err = db.FlushDomainRules(m.DB)
	if err != nil {
		return err
	}

	err = db.FlushIPRules(m.DB)
	if err != nil {
		return err
	}

	return nil
}

func (m *QoSManager) GetAllRules() ([]Rule, error) {
	ipRules, err := db.GetAllIPRules(m.DB)
	if err != nil {
		return nil, err
	}
	domainRules, err := db.GetAllDomainRules(m.DB)
	if err != nil {
		return nil, err
	}

	return joinIPAndDomainRules(ipRules, domainRules), nil
}

func (m *QoSManager) GetHighPriority() ([]Rule, error) {
	highPrioIPRules, err := db.GetHighPrioIPs(m.DB)
	if err != nil {
		return nil, err
	}
	highPrioDomainRules, err := db.GetHighPrioDomains(m.DB)
	if err != nil {
		return nil, err
	}

	return joinIPAndDomainRules(highPrioIPRules, highPrioDomainRules), nil
}

func (m *QoSManager) GetLowPriority() ([]Rule, error) {
	lowPrioIPRules, err := db.GetLowPrioIPs(m.DB)
	if err != nil {
		return nil, err
	}
	lowPrioDomainRules, err := db.GetLowPrioDomains(m.DB)
	if err != nil {
		return nil, err
	}

	return joinIPAndDomainRules(lowPrioIPRules, lowPrioDomainRules), nil
}

func (m *QoSManager) deleteDomainAddrs(domainRule db.DomainRule) error {
	addrs := make([]netip.Prefix, 0, len(domainRule.IPs))
	for _, addr := range domainRule.IPs {
		ip, iperr := netip.ParsePrefix(addr.IP)
		if iperr != nil {
			return iperr
		}
		addrs = append(addrs, ip)
	}

	return m.Classifier.DeleteTargetsFromPriority(addrs, domainRule.Priority)
}
