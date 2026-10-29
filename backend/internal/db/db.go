package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const BaseSchema = `
CREATE TABLE IF NOT EXISTS processes (
    id INTEGER PRIMARY KEY,
    pid INTEGER NOT NULL,
    exe_name TEXT NOT NULL,
    exe_path TEXT,
    icon_path TEXT,
    first_seen INTEGER NOT NULL,
    last_seen INTEGER NOT NULL,
    UNIQUE(pid, exe_path)
);

CREATE TABLE IF NOT EXISTS traffic_samples (
    id INTEGER PRIMARY KEY,
    timestamp INTEGER NOT NULL,
    process_id INTEGER NOT NULL,
    bytes_in INTEGER NOT NULL DEFAULT 0,
    bytes_out INTEGER NOT NULL DEFAULT 0,
    remote_ip TEXT,
    remote_port INTEGER,
    local_ip TEXT,
    local_port INTEGER,
    protocol TEXT NOT NULL,
    estimated BOOLEAN NOT NULL DEFAULT 1,
    FOREIGN KEY(process_id) REFERENCES processes(id)
);

CREATE INDEX IF NOT EXISTS idx_traffic_time ON traffic_samples(timestamp);
CREATE INDEX IF NOT EXISTS idx_traffic_process ON traffic_samples(process_id);

CREATE TABLE IF NOT EXISTS hourly_aggregates (
    id INTEGER PRIMARY KEY,
    hour_start INTEGER NOT NULL,
    process_id INTEGER NOT NULL,
    country_code TEXT,
    bytes_in INTEGER NOT NULL,
    bytes_out INTEGER NOT NULL,
    connection_count INTEGER NOT NULL,
    FOREIGN KEY(process_id) REFERENCES processes(id),
    UNIQUE(hour_start, process_id, country_code)
);

CREATE INDEX IF NOT EXISTS idx_hourly_time ON hourly_aggregates(hour_start);
CREATE INDEX IF NOT EXISTS idx_hourly_process ON hourly_aggregates(process_id);

CREATE TABLE IF NOT EXISTS daily_aggregates (
    id INTEGER PRIMARY KEY,
    day_start INTEGER NOT NULL,
    process_id INTEGER NOT NULL,
    country_code TEXT,
    bytes_in INTEGER NOT NULL,
    bytes_out INTEGER NOT NULL,
    connection_count INTEGER NOT NULL,
    FOREIGN KEY(process_id) REFERENCES processes(id),
    UNIQUE(day_start, process_id, country_code)
);

CREATE INDEX IF NOT EXISTS idx_daily_time ON daily_aggregates(day_start);
CREATE INDEX IF NOT EXISTS idx_daily_process ON daily_aggregates(process_id);
`

func Open(dataDir string) (*sql.DB, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "traffic.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if _, err := db.Exec(BaseSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("run base schema: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return db, nil
}

func migrate(db *sql.DB) error {
	hasCountryCode, err := columnExists(db, "traffic_samples", "country_code")
	if err != nil {
		return fmt.Errorf("check column: %w", err)
	}

	if !hasCountryCode {
		if _, err := db.Exec(`ALTER TABLE traffic_samples ADD COLUMN country_code TEXT DEFAULT 'XX'`); err != nil {
			return fmt.Errorf("add country_code column: %w", err)
		}
	}

	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_traffic_country ON traffic_samples(country_code)`); err != nil {
		return fmt.Errorf("create country index: %w", err)
	}

	return nil
}

func columnExists(db *sql.DB, table, column string) (bool, error) {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dfltValue sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}