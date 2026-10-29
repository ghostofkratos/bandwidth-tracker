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
    FOREIGN KEY(process_id) REFERENCES processes(id)
);

CREATE INDEX IF NOT EXISTS idx_traffic_time ON traffic_samples(timestamp);
CREATE INDEX IF NOT EXISTS idx_traffic_process ON traffic_samples(process_id);
`