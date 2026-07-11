package qos

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/kakeetopius/qosm/internal/db"
	"github.com/kakeetopius/qosm/internal/priority"
	"github.com/kakeetopius/qosm/internal/service"
)

func (m *QoSManager) AddServiceRule(servStr string, prioString string) (rule Rule, err error) {
	serv, err := service.ServiceFromString(servStr)
	if err != nil {
		return Rule{}, err
	}
	defer func() {
		if err != nil {
			db.AddErrorLog(m.DB, err, "")
		} else {
			addRuleSuccessLog(m.DB, serv.String(), prioString)
		}
	}()
	prio, err := priority.PriorityFromString(prioString)
	if err != nil {
		return Rule{}, err
	}

	exists, err := db.CheckServiceRuleExists(m.DB, serv)
	if err != nil {
		return rule, err
	}
	if exists {
		return rule, fmt.Errorf("rule for %v already exists", serv.String())
	}

	if m.DaemonMode {
		err = m.sendAddServicesRequest([]service.Service{serv}, prio)
	} else {
		err = m.Classifier.AddServicesToPriority([]service.Service{serv}, prio)
	}
	if err != nil {
		return Rule{}, err
	}

	err = db.AddServiceToPriority(m.DB, serv, prio)
	if err != nil {
		return rule, err
	}

	serviceRule, err := db.GetServiceRule(m.DB, serv)
	if err != nil {
		return rule, err
	}

	return Rule{
		ID:        serviceRule.ID,
		Target:    serviceRule.String(),
		Type:      "service",
		Priority:  serviceRule.Priority,
		CreatedAt: serviceRule.CreatedAt,
	}, nil
}

func (m *QoSManager) DeleteServiceRuleByID(servID int) (err error) {
	servRule, err := db.GetServiceRuleByID(m.DB, servID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("no rules to delete for service with ID %v", servID)
		}
		return err
	}

	defer func() {
		if err != nil {
			db.AddErrorLog(m.DB, err, "")
		} else {
			addRuleDeletedLog(m.DB, servRule.String(), servRule.Priority.String())
		}
	}()

	if m.DaemonMode {
		err = m.sendDeleteServiceRequest([]service.Service{servRule.Service}, servRule.Priority)
	} else {
		err = m.Classifier.DeleteServicesFromPriority([]service.Service{servRule.Service}, servRule.Priority)
	}
	if err != nil {
		return err
	}

	return db.DeleteServiceRuleByID(m.DB, servRule.ID, servRule.Priority)
}

func (m *QoSManager) DeleteServiceRule(serv service.Service) error {
	servRule, err := db.GetServiceRule(m.DB, serv)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("no rules to delete for service %v", serv.String())
		}
		return err
	}

	defer func() {
		if err != nil {
			db.AddErrorLog(m.DB, err, "")
		} else {
			addRuleDeletedLog(m.DB, servRule.String(), servRule.Priority.String())
		}
	}()

	if m.DaemonMode {
		err = m.sendDeleteServiceRequest([]service.Service{serv}, servRule.Priority)
	} else {
		err = m.Classifier.DeleteServicesFromPriority([]service.Service{servRule.Service}, servRule.Priority)
	}
	if err != nil {
		return err
	}

	return db.DeleteServiceRuleByID(m.DB, servRule.ID, servRule.Priority)
}

func (m *QoSManager) GetAllServiceRules() ([]Rule, error) {
	servRules, err := db.GetAllServiceRules(m.DB)
	if err != nil {
		return nil, err
	}

	return serviceRulesToGenericRules(servRules), nil
}

func (m *QoSManager) GetHighPriorityServices() ([]Rule, error) {
	servRules, err := db.GetHighPrioServices(m.DB)
	if err != nil {
		return nil, err
	}
	return serviceRulesToGenericRules(servRules), nil
}

func (m *QoSManager) GetLowPriorityServices() ([]Rule, error) {
	servRules, err := db.GetLowPrioServices(m.DB)
	if err != nil {
		return nil, err
	}
	return serviceRulesToGenericRules(servRules), nil
}

func serviceRulesToGenericRules(servRules []service.ServiceRule) []Rule {
	rules := make([]Rule, 0, len(servRules))
	for _, servRule := range servRules {
		rules = append(rules,
			Rule{
				ID:        servRule.ID,
				Target:    servRule.String(),
				Type:      "service",
				Priority:  servRule.Priority,
				CreatedAt: servRule.CreatedAt,
			})
	}

	return rules
}
