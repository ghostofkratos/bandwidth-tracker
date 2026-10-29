package db

const Schema = `
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
    country_code TEXT DEFAULT 'XX',
    FOREIGN KEY(process_id) REFERENCES processes(id)
);

CREATE INDEX IF NOT EXISTS idx_traffic_time ON traffic_samples(timestamp);
CREATE INDEX IF NOT EXISTS idx_traffic_process ON traffic_samples(process_id);
CREATE INDEX IF NOT EXISTS idx_traffic_country ON traffic_samples(country_code);

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