// Package db is used to interface with the db.
package db

import (
	"database/sql"
	"fmt"
	"slices"
)

type Settings struct {
	LoggingLevel   string `form:"logging_level"`
	MaxBandwidth   int    `form:"max_bandwidth"`
	IfaceName      string `form:"interface"`
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
		return GetSettingsRow(db)
	}

	defaultSettings := Settings{
		SessionTimeout: 5,
		LoggingLevel:   "Info",
		MaxBandwidth:   1000,
	}

	err = AddSettingsRow(db, &defaultSettings)
	if err != nil {
		return nil, err
	}
	return &defaultSettings, nil
}

func UpdateSettings(db *sql.DB, s *Settings) error {
	_, err := db.Exec(
		`
    INSERT OR REPLACE INTO settings (
        id, logging_level, max_bandwidth,
        dns_override, primary_dns, session_timeout
    )
    VALUES (?, ?, ?, ?, ?, ?)
`,
		1,
		s.LoggingLevel,
		s.MaxBandwidth,
		s.DNSOverride,
		s.PrimaryDNS,
		s.SessionTimeout,
	)

	return err
}

func UpdateSettingField(db *sql.DB, field string, value any) error {
	allowed := []string{"logging_level", "max_bandwidth", "dns_override", "primary_dns", "session_timeout"}
	if !slices.Contains(allowed, field) {
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

func AddSettingsRow(db *sql.DB, settings *Settings) error {
	_, err := db.Exec(
		`
    INSERT OR IGNORE INTO settings (
        id,  logging_level, max_bandwidth,
        dns_override, primary_dns, session_timeout
    )
    VALUES (?, ?, ?, ?, ?, ?)
`,
		1,
		settings.LoggingLevel,
		settings.MaxBandwidth,
		settings.DNSOverride,
		settings.PrimaryDNS,
		settings.SessionTimeout,
	)

	return err
}

func GetSettingsRow(db *sql.DB) (*Settings, error) {
	var (
		dnsOverride int
		s           Settings
	)

	row := db.QueryRow(`
        SELECT  logging_level, max_bandwidth,
                dns_override, primary_dns,
                session_timeout
        FROM settings
        WHERE id = 1
    `)

	err := row.Scan(
		&s.LoggingLevel,
		&s.MaxBandwidth,
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
