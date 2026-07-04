package db

import (
	"database/sql"

	"github.com/kakeetopius/qosm/internal/priority"
	"github.com/kakeetopius/qosm/internal/service"
)

func CheckServiceRuleExists(db *sql.DB, service service.Service) (bool, error) {
	var exists bool

	err := db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM servicerules WHERE protocol = ? AND port = ?
		)
	`, service.Protocol, service.Port).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func GetHighPrioServices(db *sql.DB) ([]service.ServiceRule, error) {
	return getServicesOfPriority(db, priority.PRIORITYHIGH)
}

func GetLowPrioServices(db *sql.DB) ([]service.ServiceRule, error) {
	return getServicesOfPriority(db, priority.PRIORITYLOW)
}

func AddServiceToPriority(db *sql.DB, serv service.Service, priority priority.Priority) error {
	return addServiceRuleRow(db, service.ServiceRule{Service: serv, Priority: priority})
}

func AddServiceToHighPrio(db *sql.DB, serv service.Service) error {
	return addServiceRuleRow(db, service.ServiceRule{Service: serv, Priority: priority.PRIORITYHIGH})
}

func AddServiceToLowPrio(db *sql.DB, serv service.Service) error {
	return addServiceRuleRow(db, service.ServiceRule{Service: serv, Priority: priority.PRIORITYLOW})
}

func GetAllServiceRules(db *sql.DB) ([]service.ServiceRule, error) {
	rows, err := db.Query(`
		SELECT id, protocol, port, priority, created_at
		FROM servicerules
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []service.ServiceRule

	for rows.Next() {
		var rule service.ServiceRule
		err = rows.Scan(&rule.ID, &rule.Protocol, &rule.Port, &rule.Priority, &rule.CreatedAt)
		if err != nil {
			return nil, err
		}

		rules = append(rules, rule)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return rules, nil
}

func GetServiceRule(db *sql.DB, serv service.Service) (service.ServiceRule, error) {
	row := db.QueryRow(`
		SELECT id, protocol, port, priority, created_at 
		FROM servicerules
		WHERE protocol = ?
			AND port = ?
	`, serv.Protocol, serv.Port)

	var rule service.ServiceRule
	err := row.Scan(&rule.ID, &rule.Protocol, &rule.Port, &rule.Priority, &rule.CreatedAt)
	if err != nil {
		return service.ServiceRule{}, err
	}

	return rule, nil
}

func GetServiceRuleByID(db *sql.DB, id int) (service.ServiceRule, error) {
	row := db.QueryRow(`
		SELECT id, protocol, port, priority, created_at 
		FROM servicerules
		WHERE id = ?
	`, id)

	var rule service.ServiceRule
	err := row.Scan(&rule.ID, &rule.Protocol, &rule.Port, &rule.Priority, &rule.CreatedAt)
	if err != nil {
		return service.ServiceRule{}, err
	}

	return rule, nil
}

func DeleteServiceRule(db *sql.DB, service service.Service, priority priority.Priority) error {
	_, err := db.Exec(`
		DELETE FROM servicerules
		WHERE protocol = ?
		 AND port = ?
		 AND priority = ?
	`, service.Protocol, service.Port, priority)

	return err
}

func DeleteServiceRuleByID(db *sql.DB, id int, priority priority.Priority) error {
	_, err := db.Exec(`
		DELETE FROM servicerules
		WHERE id = ?
			AND priority = ?
	`, id, priority)

	return err
}

func FlushServiceRules(db *sql.DB) error {
	_, err := db.Exec(`
		DELETE FROM servicerules
	`)

	return err
}

func getServicesOfPriority(db *sql.DB, priority priority.Priority) ([]service.ServiceRule, error) {
	rows, err := db.Query(`
		SELECT id, protocol, port, priority, created_at
		FROM servicerules
		WHERE priority = ?
	`, priority)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []service.ServiceRule
	for rows.Next() {
		var rule service.ServiceRule
		err = rows.Scan(&rule.ID, &rule.Protocol, &rule.Port, &rule.Priority, &rule.CreatedAt)
		if err != nil {
			return nil, err
		}

		rules = append(rules, rule)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return rules, nil
}

func addServiceRuleRow(db *sql.DB, row service.ServiceRule) error {
	_, err := db.Exec(
		`
			INSERT OR IGNORE INTO servicerules (
			protocol,
			port,
			priority
		)
		VALUES (?, ?, ?)
	`,
		row.Protocol,
		row.Port,
		row.Priority,
	)

	return err
}
