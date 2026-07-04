package db

import (
	"database/sql"
	"net/netip"
	"time"

	"github.com/kakeetopius/qosm/internal/priority"
)

type IPRule struct {
	ID        int
	IP        string
	Priority  priority.Priority
	CreatedAt time.Time
}

func (r IPRule) AsPrefix() (netip.Prefix, error) {
	addr, err := netip.ParsePrefix(r.IP)
	if err != nil {
		return netip.Prefix{}, err
	}

	return addr, nil
}

func CheckIPRuleExists(db *sql.DB, ip string) (bool, error) {
	var exists bool

	err := db.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM iprules WHERE ip = ?
		)
	`, ip).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func GetHighPrioIPs(db *sql.DB) ([]IPRule, error) {
	return getIPsOfPriority(db, priority.PRIORITYHIGH)
}

func GetLowPrioIPs(db *sql.DB) ([]IPRule, error) {
	return getIPsOfPriority(db, priority.PRIORITYLOW)
}

func AddIPToPriority(db *sql.DB, ip string, priority priority.Priority) error {
	return addIPRuleRow(db, IPRule{IP: ip, Priority: priority})
}

func AddIPToHighPrio(db *sql.DB, ip string) error {
	return addIPRuleRow(db, IPRule{IP: ip, Priority: priority.PRIORITYHIGH})
}

func AddIPToLowPrio(db *sql.DB, ip string) error {
	return addIPRuleRow(db, IPRule{IP: ip, Priority: priority.PRIORITYLOW})
}

func GetAllIPRules(db *sql.DB) ([]IPRule, error) {
	rows, err := db.Query(`
		SELECT id, ip, priority, created_at
		FROM iprules
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []IPRule

	for rows.Next() {
		var rule IPRule
		err = rows.Scan(&rule.ID, &rule.IP, &rule.Priority, &rule.CreatedAt)
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

func GetIPRuleByName(db *sql.DB, name string) (IPRule, error) {
	row := db.QueryRow(`
		SELECT id, ip, priority, created_at 
		FROM iprules
		WHERE ip = ?
	`, name)

	var rule IPRule
	err := row.Scan(&rule.ID, &rule.IP, &rule.Priority, &rule.CreatedAt)
	if err != nil {
		return IPRule{}, err
	}

	return rule, nil
}

func GetIPRuleByID(db *sql.DB, id int) (IPRule, error) {
	row := db.QueryRow(`
		SELECT id, ip, priority, created_at 
		FROM iprules
		WHERE id = ?
	`, id)

	var rule IPRule
	err := row.Scan(&rule.ID, &rule.IP, &rule.Priority, &rule.CreatedAt)
	if err != nil {
		return IPRule{}, err
	}

	return rule, nil
}

func DeleteIPRuleByName(db *sql.DB, name string, priority priority.Priority) error {
	_, err := db.Exec(`
		DELETE FROM iprules
		WHERE ip = ?
		 AND priority = ?
	`, name, priority)

	return err
}

func DeleteIPRuleByID(db *sql.DB, id int, priority priority.Priority) error {
	_, err := db.Exec(`
		DELETE FROM iprules
		WHERE id = ?
			AND priority = ?
	`, id, priority)

	return err
}

func FlushIPRules(db *sql.DB) error {
	_, err := db.Exec(`
		DELETE FROM iprules
	`)

	return err
}

func getIPsOfPriority(db *sql.DB, priority priority.Priority) ([]IPRule, error) {
	rows, err := db.Query(`
		SELECT id, ip, priority, created_at
		FROM iprules
		WHERE priority = ?
	`, priority)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []IPRule
	for rows.Next() {
		var rule IPRule
		err = rows.Scan(&rule.ID, &rule.IP, &rule.Priority, &rule.CreatedAt)
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

func addIPRuleRow(db *sql.DB, row IPRule) error {
	_, err := db.Exec(
		`
		INSERT OR IGNORE INTO iprules (
			ip,
			priority
		)
		VALUES (?, ?)
	`,
		row.IP,
		row.Priority,
	)

	return err
}
