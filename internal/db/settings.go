// Package db is used to interface with the db.
package db

import (
	"database/sql"
	"errors"
	"fmt"
)

type Settings struct {
	DNSOverride    bool   `form:"dns_override"`
	PrimaryDNS     string `form:"primary_dns"`
	SessionTimeout int    `form:"session_timeout"`
}

func LoadSettings(db *sql.DB) (*Settings, error) {
	exists, err := CheckSettingsExists(db)
	if err != nil {
		return nil, err
	}

	if exists {
		return getSettingsRow(db)
	}

	defaultSettings := Settings{
		SessionTimeout: 5,
	}

	err = addSettingsRow(db, &defaultSettings)
	if err != nil {
		return nil, err
	}
	return &defaultSettings, nil
}

func UpdateSettings(db *sql.DB, s *Settings) error {
	_, err := db.Exec(
		`
    INSERT OR REPLACE INTO settings (
        id, 
        dns_override, primary_dns, session_timeout
    )
    VALUES (?, ?, ?, ?)
`,
		1,
		s.DNSOverride,
		s.PrimaryDNS,
		s.SessionTimeout,
	)

	return err
}

func UpdateSettingsField(db *sql.DB, field string, value any) error {
	allowed := map[string]struct{}{
		"dns_override":    {},
		"primary_dns":     {},
		"session_timeout": {},
	}
	if _, ok := allowed[field]; !ok {
		return fmt.Errorf("unknown field: %v", field)
	}

	query := fmt.Sprintf(`
		UPDATE settings
		SET %s = ?
		WHERE id = 1
	`, field)

	_, err := db.Exec(query, value)
	return err
}

func GetSettingsField(db *sql.DB, field string) (any, error) {
	allowed := map[string]struct{}{
		"dns_override":    {},
		"primary_dns":     {},
		"session_timeout": {},
	}
	if _, ok := allowed[field]; !ok {
		return nil, fmt.Errorf("unknown field: %v", field)
	}

	query := fmt.Sprintf(`
		SELECT %s 
		FROM settings
		WHERE id = 1
	`, field)

	stmt, err := db.Prepare(query)
	if err != nil {
		return nil, err
	}

	row := stmt.QueryRow()

	var value any
	err = row.Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotExists
	}

	return value, nil
}

func CheckSettingsExists(db *sql.DB) (bool, error) {
	var exists bool

	err := db.QueryRow(`
        SELECT EXISTS(
            SELECT 1
            FROM settings
            WHERE id = 1
        )
    `).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func addSettingsRow(db *sql.DB, settings *Settings) error {
	_, err := db.Exec(
		`
    INSERT OR IGNORE INTO settings (
        id, 
        dns_override, primary_dns, session_timeout
    )
    VALUES (?, ?, ?, ?)
`,
		1,
		settings.DNSOverride,
		settings.PrimaryDNS,
		settings.SessionTimeout,
	)

	return err
}

func getSettingsRow(db *sql.DB) (*Settings, error) {
	var (
		dnsOverride int
		s           Settings
	)

	row := db.QueryRow(`
        SELECT  dns_override, primary_dns,
                session_timeout
        FROM settings
        WHERE id = 1
    `)

	err := row.Scan(
		&dnsOverride,
		&s.PrimaryDNS,
		&s.SessionTimeout,
	)
	if err != nil {
		return nil, err
	}

	s.DNSOverride = dnsOverride == 1

	return &s, nil
}
