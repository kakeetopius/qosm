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

func (m *QoSManager) AddDomainRule(domain string, prioString string) (rule Rule, err error) {
	if m.Classifier == nil && !m.DaemonMode {
		return Rule{}, ErrClassifierNotInitialised
	}
	defer func() {
		if err != nil {
			db.AddErrorLog(m.DB, err, "")
		} else {
			addRuleSuccessLog(m.DB, domain, prioString)
		}
	}()
	prio, err := priority.PriorityFromString(prioString)
	if err != nil {
		return Rule{}, err
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
		return Rule{}, fmt.Errorf("%v seems to be an IP address not a domain", domain)
	}

	ips, err := net.LookupIP(domain)
	if err != nil {
		util.Debug(m.Logger, "resolve_error", "domain_name", domain, "error", err.Error())
		return Rule{}, err
	}

	db.AddLog(m.DB, db.Log{
		EventType:   "DNS",
		Description: "Resolved domain " + domain + " to " + ipSliceToString(ips),
	})

	addrs := util.IPSlicestoNetIPPRefix(ips)

	if m.DaemonMode {
		err = m.sendAddHostsRequest(addrs, prio)
	} else {
		err = m.Classifier.AddIPsToPriority(addrs, prio)
	}
	if err != nil {
		return Rule{}, err
	}

	err = db.AddDomainToPriority(m.DB, domain, prio, addrs)
	if err != nil {
		return rule, err
	}

	domainRule, err := db.GetDomainRuleByName(m.DB, domain)
	if err != nil {
		return rule, err
	}

	return Rule{
		Type:      "domain",
		Priority:  prio,
		Target:    domainRule.DomainName,
		ID:        domainRule.ID,
		CreatedAt: domainRule.CreatedAt,
	}, nil
}

func (m *QoSManager) AddIPRule(ip string, prioString string) (rule Rule, err error) {
	if m.Classifier == nil && !m.DaemonMode {
		return Rule{}, ErrClassifierNotInitialised
	}
	defer func() {
		if err != nil {
			db.AddErrorLog(m.DB, err, "")
		} else {
			addRuleSuccessLog(m.DB, ip, prioString)
		}
	}()
	prio, err := priority.PriorityFromString(prioString)
	if err != nil {
		return Rule{}, err
	}

	addr, err := netip.ParsePrefix(ip)
	if err != nil {
		return Rule{}, fmt.Errorf("invalid IP address: %v", ip)
	}

	exists, err := db.CheckIPRuleExists(m.DB, addr.String())
	if err != nil {
		return rule, err
	}
	if exists {
		return rule, fmt.Errorf("rule for %v already exists", ip)
	}

	util.Debug(m.Logger, "add_rule", "target", ip, "priority", prioString)

	if m.DaemonMode {
		err = m.sendAddHostsRequest([]netip.Prefix{addr}, prio)
	} else {
		err = m.Classifier.AddIPsToPriority([]netip.Prefix{addr}, prio)
	}
	if err != nil {
		return Rule{}, err
	}

	ipString := addr.String()
	err = db.AddIPToPriority(m.DB, ipString, prio)
	if err != nil {
		return rule, err
	}

	ipRule, err := db.GetIPRuleByName(m.DB, ipString)
	if err != nil {
		return rule, err
	}

	return Rule{
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
	if m.Classifier == nil && !m.DaemonMode {
		return ErrClassifierNotInitialised
	}

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

	if m.DaemonMode {
		err = m.sendDeleteHostsRequest([]netip.Prefix{addr}, ipRule.Priority)
	} else {
		err = m.Classifier.DeleteIPsFromPriority([]netip.Prefix{addr}, ipRule.Priority)
	}
	if err != nil {
		return err
	}

	return db.DeleteIPRuleByID(m.DB, ipRuleID, ipRule.Priority)
}

func (m *QoSManager) DeleteIPRuleByName(ipRuleName string) error {
	if m.Classifier == nil && !m.DaemonMode {
		return ErrClassifierNotInitialised
	}
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

	if m.DaemonMode {
		err = m.sendDeleteHostsRequest([]netip.Prefix{addr}, ipRule.Priority)
	} else {
		err = m.Classifier.DeleteIPsFromPriority([]netip.Prefix{addr}, ipRule.Priority)
	}
	if err != nil {
		return err
	}

	return db.DeleteIPRuleByName(m.DB, ipRuleName, ipRule.Priority)
}

func (m *QoSManager) GetAllHostRules() ([]Rule, error) {
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

func (m *QoSManager) GetHighPriorityHostRules() ([]Rule, error) {
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

func (m *QoSManager) GetLowPriorityHostRules() ([]Rule, error) {
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
	if m.Classifier == nil && !m.DaemonMode {
		return ErrClassifierNotInitialised
	}
	addrs := make([]netip.Prefix, 0, len(domainRule.IPs))
	for _, addr := range domainRule.IPs {
		ip, iperr := netip.ParsePrefix(addr.IP)
		if iperr != nil {
			return iperr
		}
		addrs = append(addrs, ip)
	}

	var err error
	if m.DaemonMode {
		err = m.sendDeleteHostsRequest(addrs, domainRule.Priority)
	} else {
		m.Classifier.DeleteIPsFromPriority(addrs, domainRule.Priority)
	}

	return err
}
