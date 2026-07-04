package qos

import (
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/netip"

	"github.com/kakeetopius/qosm/internal/db"
	"github.com/kakeetopius/qosm/internal/priority"
	"github.com/kakeetopius/qosm/internal/util"
)

func (m *QoSManager) AddDomainRule(domain string, prioString string) (rule HostRule, err error) {
	defer func() {
		if err != nil {
			db.AddErrorLog(m.DB, err, "")
		} else {
			addRuleSuccessLog(m.DB, domain, prioString)
		}
	}()
	prio, err := priority.PriorityFromString(prioString)
	if err != nil {
		return HostRule{}, err
	}

	exists, err := db.CheckDomainRuleExists(m.DB, domain)
	if err != nil {
		return rule, err
	}
	if exists {
		return rule, fmt.Errorf("rule for %v already exists", domain)
	}

	_, err = netip.ParseAddr(domain)
	if err == nil {
		return HostRule{}, fmt.Errorf("%v seems to be an IP address not a domain", domain)
	}

	util.Debug(m.Logger, "resolving_domain", "domain_name", domain)
	ips, err := net.LookupIP(domain)
	if err != nil {
		util.Debug(m.Logger, "resolve_error", "domain_name", domain, "error", err.Error())
		return HostRule{}, err
	}

	db.AddLog(m.DB, db.Log{
		EventType:   "DNS",
		Description: "Resolved domain " + domain + " to " + ipSliceToString(ips),
	})

	addrs := util.NetIPtoNetIPPRefix(ips)

	util.Debug(m.Logger, "add_rule", "target", domain, "priority", prioString)

	err = m.Classifier.AddIPsToPriority(addrs, prio)
	if err != nil {
		return HostRule{}, err
	}

	err = db.AddDomainToPriority(m.DB, domain, prio, addrs)
	if err != nil {
		return rule, err
	}

	domainRule, err := db.GetDomainRuleByName(m.DB, domain)
	if err != nil {
		return rule, err
	}

	return HostRule{
		Type:      "domain",
		Priority:  prio,
		Target:    domainRule.DomainName,
		ID:        domainRule.ID,
		CreatedAt: domainRule.CreatedAt,
	}, nil
}

func (m *QoSManager) AddIPRule(ip string, prioString string) (rule HostRule, err error) {
	defer func() {
		if err != nil {
			db.AddErrorLog(m.DB, err, "")
		} else {
			addRuleSuccessLog(m.DB, ip, prioString)
		}
	}()
	prio, err := priority.PriorityFromString(prioString)
	if err != nil {
		return HostRule{}, err
	}

	addrs, err := util.TargetsFromString(ip)
	if err != nil {
		return HostRule{}, fmt.Errorf("invalid IP address: %v", ip)
	}

	exists, err := db.CheckIPRuleExists(m.DB, addrs[0].String())
	if err != nil {
		return rule, err
	}
	if exists {
		return rule, fmt.Errorf("rule for %v already exists", ip)
	}

	util.Debug(m.Logger, "add_rule", "target", ip, "priority", prioString)

	err = m.Classifier.AddIPsToPriority(addrs, prio)
	if err != nil {
		return HostRule{}, err
	}

	ipString := addrs[0].String()
	err = db.AddIPToPriority(m.DB, ipString, prio)
	if err != nil {
		return rule, err
	}

	ipRule, err := db.GetIPRuleByName(m.DB, ipString)
	if err != nil {
		return rule, err
	}

	return HostRule{
		Type:      "ip",
		Priority:  prio,
		Target:    ipRule.IP,
		ID:        ipRule.ID,
		CreatedAt: ipRule.CreatedAt,
	}, nil
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
			addRuleDeletedLog(m.DB, domainRule.DomainName, domainRule.Priority.String())
		}
	}()

	err = m.deleteDomainAddrs(domainRule)
	if err != nil {
		return err
	}

	return db.DeleteDomainRuleByID(m.DB, domainRuleID, domainRule.Priority)
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
			addRuleDeletedLog(m.DB, domainRule.DomainName, domainRule.Priority.String())
		}
	}()

	err = m.deleteDomainAddrs(domainRule)
	if err != nil {
		return err
	}

	return db.DeleteDomainRuleByName(m.DB, name, domainRule.Priority)
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
			addRuleDeletedLog(m.DB, ipRule.IP, ipRule.Priority.String())
		}
	}()

	addr, err := netip.ParsePrefix(ipRule.IP)
	if err != nil {
		return err
	}

	err = m.Classifier.DeleteIPsFromPriority([]netip.Prefix{addr}, ipRule.Priority)
	if err != nil {
		return err
	}

	return db.DeleteIPRuleByID(m.DB, ipRuleID, ipRule.Priority)
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
			addRuleDeletedLog(m.DB, ipRule.IP, ipRule.Priority.String())
		}
	}()

	addr, err := netip.ParsePrefix(ipRule.IP)
	if err != nil {
		return err
	}

	err = m.Classifier.DeleteIPsFromPriority([]netip.Prefix{addr}, ipRule.Priority)
	if err != nil {
		return err
	}

	return db.DeleteIPRuleByName(m.DB, ipRuleName, ipRule.Priority)
}

func (m *QoSManager) GetAllRules() ([]HostRule, error) {
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

func (m *QoSManager) GetHighPriorityHostRules() ([]HostRule, error) {
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

func (m *QoSManager) GetLowPriorityHostRules() ([]HostRule, error) {
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

	return m.Classifier.DeleteIPsFromPriority(addrs, domainRule.Priority)
}
