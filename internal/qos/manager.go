// Package qos contains code for the qos manager that controls and co-ordinates all qos operations like enabling tc on an interface, adding rules etc.
package qos

import (
	"log/slog"
	"net"
	"net/netip"

	"github.com/florianl/go-tc"
	"github.com/kakeetopius/qosm/internal/core/nft"
	"github.com/kakeetopius/qosm/internal/db"
	"github.com/kakeetopius/qosm/internal/priority"
	"github.com/kakeetopius/qosm/internal/service"
)

func NewManager(o Options) (*QoSManager, error) {
	tcnl, err := tc.Open(&tc.Config{})
	if err != nil {
		return nil, err
	}

	qosManager := QoSManager{
		Ifaces:  make(map[string]Interface),
		TcConn:  tcnl,
		Options: o,
	}

	err = qosManager.getNetInterfaces()
	if err != nil {
		return nil, err
	}

	return &qosManager, nil
}

func (m *QoSManager) WithLogger(l *slog.Logger) {
	m.Logger = l
}

func (m *QoSManager) InitQoSClassifier(createIfNotExists bool) error {
	if m.DaemonMode {
		return nil
	}
	nftCtx, err := nft.NewNFTCtx(nft.NFTOpts{
		CreateIfNotExists: createIfNotExists,
		Logger:            m.Logger,
	})
	if err != nil {
		return err
	}
	m.Classifier = &nftCtx

	return nil
}

func (m *QoSManager) InitSavedRules() error {
	if m.Classifier == nil && !m.DaemonMode {
		return ErrClassifierNotInitialised
	}
	ipRules, err := db.GetAllIPRules(m.DB)
	if err != nil {
		return err
	}
	highPrioIPs := make([]netip.Prefix, 0, 10)
	lowPrioIPs := make([]netip.Prefix, 0, 10)

	for _, rule := range ipRules {
		ip, ipErr := netip.ParsePrefix(rule.IP)
		if ipErr != nil {
			return ipErr
		}
		switch rule.Priority {
		case priority.PRIORITYHIGH:
			highPrioIPs = append(highPrioIPs, ip)
		case priority.PRIORITYLOW:
			lowPrioIPs = append(lowPrioIPs, ip)
		}
	}

	domainRules, err := db.GetAllDomainRules(m.DB)
	if err != nil {
		return err
	}

	for _, rule := range domainRules {
		ips, ipErr := rule.IPsAsPrefix()
		if ipErr != nil {
			return ipErr
		}
		switch rule.Priority {
		case priority.PRIORITYHIGH:
			highPrioIPs = append(highPrioIPs, ips...)
		case priority.PRIORITYLOW:
			lowPrioIPs = append(lowPrioIPs, ips...)
		}
	}

	if m.DaemonMode {
		err = m.sendAddHostsRequest(highPrioIPs, priority.PRIORITYHIGH)
		if err != nil {
			return err
		}
		err = m.sendAddHostsRequest(lowPrioIPs, priority.PRIORITYLOW)
	} else {
		err = m.Classifier.AddIPsToPriority(highPrioIPs, priority.PRIORITYHIGH)
		if err != nil {
			return err
		}
		err = m.Classifier.AddIPsToPriority(lowPrioIPs, priority.PRIORITYLOW)
	}
	if err != nil {
		return err
	}

	serviceRules, err := db.GetAllServiceRules(m.DB)
	if err != nil {
		return err
	}

	highPrioServ := make([]service.Service, 0, 10)
	lowPrioServ := make([]service.Service, 0, 10)
	for _, rule := range serviceRules {
		switch rule.Priority {
		case priority.PRIORITYHIGH:
			highPrioServ = append(highPrioServ, rule.Service)
		case priority.PRIORITYLOW:
			lowPrioServ = append(lowPrioServ, rule.Service)
		}
	}

	if m.DaemonMode {
		err = m.sendAddServicesRequest(highPrioServ, priority.PRIORITYHIGH)
		if err != nil {
			return err
		}
		err = m.sendAddServicesRequest(lowPrioServ, priority.PRIORITYLOW)
	} else {
		err = m.Classifier.AddServicesToPriority(highPrioServ, priority.PRIORITYHIGH)
		if err != nil {
			return err
		}
		err = m.Classifier.AddServicesToPriority(lowPrioServ, priority.PRIORITYLOW)
	}
	if err != nil {
		return err
	}
	return nil
}

func (m *QoSManager) Close() {
	m.TcConn.Close()
}

func (m *QoSManager) DeleteAllRules() error {
	if m.Classifier == nil && !m.DaemonMode {
		return ErrClassifierNotInitialised
	}
	var err error
	if m.DaemonMode {
		err = m.sendFlushAllRulesRequest()
	} else {
		err = m.Classifier.FlushAllRules()
	}
	if err != nil {
		return err
	}

	err = db.FlushDomainRules(m.DB)
	if err != nil {
		return err
	}

	err = db.FlushIPRules(m.DB)
	if err != nil {
		return err
	}

	err = db.FlushServiceRules(m.DB)
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
	serviceRules, err := db.GetAllServiceRules(m.DB)
	if err != nil {
		return nil, err
	}

	rules := joinIPAndDomainRules(ipRules, domainRules)
	rules = append(rules, serviceRulesToGenericRules(serviceRules)...)
	return rules, nil
}

func (m *QoSManager) GetHighPriorityRules() ([]Rule, error) {
	highPrioIPRules, err := db.GetHighPrioIPs(m.DB)
	if err != nil {
		return nil, err
	}
	highPrioDomainRules, err := db.GetHighPrioDomains(m.DB)
	if err != nil {
		return nil, err
	}
	serviceRules, err := db.GetAllServiceRules(m.DB)
	if err != nil {
		return nil, err
	}
	rules := joinIPAndDomainRules(highPrioIPRules, highPrioDomainRules)
	rules = append(rules, serviceRulesToGenericRules(serviceRules)...)
	return rules, nil
}

func (m *QoSManager) GetLowPriorityRules() ([]Rule, error) {
	lowPrioIPRules, err := db.GetLowPrioIPs(m.DB)
	if err != nil {
		return nil, err
	}
	lowPrioDomainRules, err := db.GetLowPrioDomains(m.DB)
	if err != nil {
		return nil, err
	}
	serviceRules, err := db.GetAllServiceRules(m.DB)
	if err != nil {
		return nil, err
	}
	rules := joinIPAndDomainRules(lowPrioIPRules, lowPrioDomainRules)
	rules = append(rules, serviceRulesToGenericRules(serviceRules)...)
	return rules, nil
}

func (m *QoSManager) getNetInterfaces() error {
	ifaces, err := net.Interfaces()
	if err != nil {
		return err
	}
	for _, iface := range ifaces {
		speed, err := getInterfaceSpeed(iface.Name)
		if err != nil {
			return err
		}

		exists, err := db.CheckInterfaceExists(m.DB, iface.Name)
		if err != nil {
			return err
		}

		var rate uint32
		if exists {
			dbrate, err := db.GetInterfaceField(m.DB, iface.Name, "rate")
			if err != nil {
				return err
			}
			rate64 := dbrate.(int64)
			rate = uint32(rate64)
		}

		m.Ifaces[iface.Name] = Interface{
			Interface:   iface,
			LinkSpeed:   speed,
			ShapingRate: rate,
		}
	}

	return nil
}
