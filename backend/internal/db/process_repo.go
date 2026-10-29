package db

import (
	"context"
	"database/sql"
	"time"
)

type Process struct {
	ID        int64
	PID       int32
	ExeName   string
	ExePath   string
	IconPath  sql.NullString
	FirstSeen int64
	LastSeen  int64
}

func UpsertProcess(ctx context.Context, db *sql.DB, pid int32, exeName, exePath string) (*Process, error) {
	now := time.Now().UnixMilli()

	var p Process
	err := db.QueryRowContext(ctx, `
		INSERT INTO processes (pid, exe_name, exe_path, first_seen, last_seen)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(pid, exe_path) DO UPDATE SET
			exe_name = excluded.exe_name,
			last_seen = excluded.last_seen
		RETURNING id, pid, exe_name, exe_path, icon_path, first_seen, last_seen
	`, pid, exeName, exePath, now, now).Scan(&p.ID, &p.PID, &p.ExeName, &p.ExePath, &p.IconPath, &p.FirstSeen, &p.LastSeen)

	if err != nil {
		return nil, err
	}
	return &p, nil
}

func GetProcessByPID(ctx context.Context, db *sql.DB, pid int32) (*Process, error) {
	var p Process
	err := db.QueryRowContext(ctx, `
		SELECT id, pid, exe_name, exe_path, icon_path, first_seen, last_seen
		FROM processes
		WHERE pid = ?
		ORDER BY last_seen DESC
		LIMIT 1
	`, pid).Scan(&p.ID, &p.PID, &p.ExeName, &p.ExePath, &p.IconPath, &p.FirstSeen, &p.LastSeen)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &p, err
}

func GetProcessByID(ctx context.Context, db *sql.DB, id int64) (*Process, error) {
	var p Process
	err := db.QueryRowContext(ctx, `
		SELECT id, pid, exe_name, exe_path, icon_path, first_seen, last_seen
		FROM processes
		WHERE id = ?
	`, id).Scan(&p.ID, &p.PID, &p.ExeName, &p.ExePath, &p.IconPath, &p.FirstSeen, &p.LastSeen)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &p, err
}