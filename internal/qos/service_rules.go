package qos

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/kakeetopius/qosm/internal/db"
	"github.com/kakeetopius/qosm/internal/priority"
	"github.com/kakeetopius/qosm/internal/service"
)

func (m *QoSManager) AddServiceRule(serv service.Service, prioString string) (rule service.ServiceRule, err error) {
	defer func() {
		if err != nil {
			db.AddErrorLog(m.DB, err, "")
		} else {
			addRuleSuccessLog(m.DB, serv.String(), prioString)
		}
	}()
	prio, err := priority.PriorityFromString(prioString)
	if err != nil {
		return service.ServiceRule{}, err
	}

	exists, err := db.CheckServiceRuleExists(m.DB, serv)
	if err != nil {
		return rule, err
	}
	if exists {
		return rule, fmt.Errorf("rule for %v already exists", serv.String())
	}

	err = m.Classifier.AddServicesToPriority([]service.Service{serv}, prio)
	if err != nil {
		return service.ServiceRule{}, err
	}

	err = db.AddServiceToPriority(m.DB, serv, prio)
	if err != nil {
		return rule, err
	}

	serviceRule, err := db.GetServiceRule(m.DB, serv)
	if err != nil {
		return rule, err
	}

	return serviceRule, nil
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

	err = m.Classifier.DeleteServicesFromPriority([]service.Service{servRule.Service}, servRule.Priority)
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

	err = m.Classifier.DeleteServicesFromPriority([]service.Service{servRule.Service}, servRule.Priority)
	if err != nil {
		return err
	}

	return db.DeleteServiceRuleByID(m.DB, servRule.ID, servRule.Priority)
}

func (m *QoSManager) GetAllServiceRules() ([]service.ServiceRule, error) {
	return db.GetAllServiceRules(m.DB)
}

func (m *QoSManager) GetHighPriorityServices() ([]service.ServiceRule, error) {
	return db.GetHighPrioServices(m.DB)
}

func (m *QoSManager) GetLowPriorityServices() ([]service.ServiceRule, error) {
	return db.GetLowPrioServices(m.DB)
}
