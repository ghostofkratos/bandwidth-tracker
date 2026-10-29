package db

import (
	"context"
	"database/sql"
	"time"
)

type TrafficSample struct {
	ID         int64
	Timestamp  int64
	ProcessID  int64
	BytesIn    int64
	BytesOut   int64
	RemoteIP   string
	RemotePort int32
	LocalIP    string
	LocalPort  int32
	Protocol   string
	Estimated  bool
}

type AggregatedMetrics struct {
	Download int64
	Upload   int64
	Apps     []AppMetrics
}

type AppMetrics struct {
	PID         int32
	Name        string
	Download    int64
	Upload      int64
	Connections int
}

func InsertSample(ctx context.Context, db *sql.DB, s *TrafficSample) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO traffic_samples (timestamp, process_id, bytes_in, bytes_out, remote_ip, remote_port, local_ip, local_port, protocol, estimated)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, s.Timestamp, s.ProcessID, s.BytesIn, s.BytesOut, s.RemoteIP, s.RemotePort, s.LocalIP, s.LocalPort, s.Protocol, s.Estimated)
	return err
}

func GetSamplesSince(ctx context.Context, db *sql.DB, since int64) ([]TrafficSample, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, timestamp, process_id, bytes_in, bytes_out, remote_ip, remote_port, local_ip, local_port, protocol, estimated
		FROM traffic_samples
		WHERE timestamp >= ?
		ORDER BY timestamp ASC
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var samples []TrafficSample
	for rows.Next() {
		var s TrafficSample
		if err := rows.Scan(&s.ID, &s.Timestamp, &s.ProcessID, &s.BytesIn, &s.BytesOut, &s.RemoteIP, &s.RemotePort, &s.LocalIP, &s.LocalPort, &s.Protocol, &s.Estimated); err != nil {
			return nil, err
		}
		samples = append(samples, s)
	}
	return samples, rows.Err()
}

func GetAggregatedMetrics(ctx context.Context, db *sql.DB, windowSeconds int) (*AggregatedMetrics, error) {
	since := time.Now().UnixMilli() - int64(windowSeconds)*1000

	rows, err := db.QueryContext(ctx, `
		SELECT p.pid, p.exe_name, SUM(ts.bytes_in), SUM(ts.bytes_out), COUNT(DISTINCT ts.remote_ip || ':' || ts.remote_port)
		FROM traffic_samples ts
		JOIN processes p ON ts.process_id = p.id
		WHERE ts.timestamp >= ?
		GROUP BY p.pid, p.exe_name
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics AggregatedMetrics
	for rows.Next() {
		var a AppMetrics
		if err := rows.Scan(&a.PID, &a.Name, &a.Download, &a.Upload, &a.Connections); err != nil {
			return nil, err
		}
		metrics.Download += a.Download
		metrics.Upload += a.Upload
		metrics.Apps = append(metrics.Apps, a)
	}
	return &metrics, rows.Err()
}