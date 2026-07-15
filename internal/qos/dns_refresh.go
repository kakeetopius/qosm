package qos

import (
	"errors"
	"net/netip"

	"github.com/kakeetopius/qosm/internal/db"
	"github.com/kakeetopius/qosm/internal/util"
)

var ErrNoDomainIPs = errors.New("no domain ips to refresh")

func (m *QoSManager) RefreshAllDomains() error {
	if m.Classifier == nil && !m.DaemonMode {
		return ErrClassifierNotInitialised
	}

	domains, err := db.GetAllDomainRules(m.DB)
	if err != nil {
		return err
	}
	util.Debug(m.Logger, "dns: refreshing domains in database")

	if len(domains) == 0 {
		return ErrNoDomainIPs
	}

	for _, domain := range domains {
		util.Debug(m.Logger, "dns: refreshing domain ips", "domain_name", domain.DomainName)
		oldIPs, err := domain.IPsAsPrefix()
		if err != nil {
			return err
		}

		newIPs, err := util.LookupIPs(domain.DomainName)
		if err != nil {
			util.Error(m.Logger, "resolve_error", "domain_name", domain.DomainName, "error", err.Error())
			return err
		}

		err = m.clearOldIPs(&domain, oldIPs)
		if err != nil {
			return err
		}

		err = m.addNewIPs(&domain, newIPs)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *QoSManager) clearOldIPs(domain *db.DomainRule, oldIPs []netip.Prefix) error {
	var err error
	if m.DaemonMode {
		err = m.sendDeleteHostsRequest(oldIPs, domain.Priority)
	} else {
		err = m.Classifier.DeleteIPsFromPriority(oldIPs, domain.Priority)
	}
	if err != nil {
		return err
	}

	err = db.DeleteDomainIPsByDomainID(m.DB, domain.ID)
	if err != nil {
		return err
	}

	return nil
}

func (m *QoSManager) addNewIPs(domain *db.DomainRule, newIPs []netip.Prefix) error {
	var err error
	if m.DaemonMode {
		err = m.sendAddHostsRequest(newIPs, domain.Priority)
	} else {
		err = m.Classifier.AddIPsToPriority(newIPs, domain.Priority)
	}
	if err != nil {
		return err
	}

	err = db.AddDomainIPstoDB(m.DB, domain.DomainName, newIPs)
	if err != nil {
		return err
	}

	return nil
}
