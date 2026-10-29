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
	CountryCode string
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
	Country     string
}

type HistoryResponse struct {
	Range       string            `json:"range"`
	Granularity string            `json:"granularity"`
	Series      []HistoryPoint    `json:"series"`
	Apps        []AppHistoryPoint `json:"apps"`
}

type HistoryPoint struct {
	Timestamp int64 `json:"timestamp"`
	Download  int64 `json:"download"`
	Upload    int64 `json:"upload"`
}

type AppHistoryPoint struct {
	PID       int32  `json:"pid"`
	Name      string `json:"name"`
	Download  int64  `json:"download"`
	Upload    int64  `json:"upload"`
	Country   string `json:"country"`
}

type GeoIPStats struct {
	Countries []CountryStats `json:"countries"`
}

type CountryStats struct {
	CountryCode string `json:"country_code"`
	Download    int64  `json:"download"`
	Upload      int64  `json:"upload"`
	Total       int64  `json:"total"`
}

func InsertSample(ctx context.Context, db *sql.DB, s *TrafficSample) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO traffic_samples (timestamp, process_id, bytes_in, bytes_out, remote_ip, remote_port, local_ip, local_port, protocol, estimated, country_code)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, s.Timestamp, s.ProcessID, s.BytesIn, s.BytesOut, s.RemoteIP, s.RemotePort, s.LocalIP, s.LocalPort, s.Protocol, s.Estimated, s.CountryCode)
	return err
}

func GetSamplesSince(ctx context.Context, db *sql.DB, since int64) ([]TrafficSample, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, timestamp, process_id, bytes_in, bytes_out, remote_ip, remote_port, local_ip, local_port, protocol, estimated, country_code
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
		if err := rows.Scan(&s.ID, &s.Timestamp, &s.ProcessID, &s.BytesIn, &s.BytesOut, &s.RemoteIP, &s.RemotePort, &s.LocalIP, &s.LocalPort, &s.Protocol, &s.Estimated, &s.CountryCode); err != nil {
			return nil, err
		}
		samples = append(samples, s)
	}
	return samples, rows.Err()
}

func GetAggregatedMetrics(ctx context.Context, db *sql.DB, windowSeconds int) (*AggregatedMetrics, error) {
	since := time.Now().UnixMilli() - int64(windowSeconds)*1000

	rows, err := db.QueryContext(ctx, `
		SELECT p.pid, p.exe_name, SUM(ts.bytes_in), SUM(ts.bytes_out), COUNT(DISTINCT ts.remote_ip || ':' || ts.remote_port), ts.country_code
		FROM traffic_samples ts
		JOIN processes p ON ts.process_id = p.id
		WHERE ts.timestamp >= ?
		GROUP BY p.pid, p.exe_name, ts.country_code
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics AggregatedMetrics
	for rows.Next() {
		var a AppMetrics
		if err := rows.Scan(&a.PID, &a.Name, &a.Download, &a.Upload, &a.Connections, &a.Country); err != nil {
			return nil, err
		}
		metrics.Download += a.Download
		metrics.Upload += a.Upload
		metrics.Apps = append(metrics.Apps, a)
	}
	return &metrics, rows.Err()
}

func GetHistory(ctx context.Context, db *sql.DB, rangeStr string) (*HistoryResponse, error) {
	var since int64
	var granularity string
	var table string
	var timeCol string

	now := time.Now().UnixMilli()

	switch rangeStr {
	case "1h":
		since = now - 3600*1000
		granularity = "minute"
		table = "traffic_samples"
		timeCol = "timestamp"
	case "24h":
		since = now - 24*3600*1000
		granularity = "hour"
		table = "hourly_aggregates"
		timeCol = "hour_start"
	case "7d":
		since = now - 7*24*3600*1000
		granularity = "hour"
		table = "hourly_aggregates"
		timeCol = "hour_start"
	case "30d":
		since = now - 30*24*3600*1000
		granularity = "day"
		table = "daily_aggregates"
		timeCol = "day_start"
	default:
		since = now - 24*3600*1000
		granularity = "hour"
		table = "hourly_aggregates"
		timeCol = "hour_start"
	}

	seriesRows, err := db.QueryContext(ctx, `
		SELECT `+timeCol+`, SUM(bytes_in), SUM(bytes_out)
		FROM `+table+`
		WHERE `+timeCol+` >= ?
		GROUP BY `+timeCol+`
		ORDER BY `+timeCol+` ASC
	`, since)
	if err != nil {
		return nil, err
	}
	defer seriesRows.Close()

	var series []HistoryPoint
	for seriesRows.Next() {
		var p HistoryPoint
		if err := seriesRows.Scan(&p.Timestamp, &p.Download, &p.Upload); err != nil {
			return nil, err
		}
		series = append(series, p)
	}

	appsRows, err := db.QueryContext(ctx, `
		SELECT p.pid, p.exe_name, SUM(`+table+`.bytes_in), SUM(`+table+`.bytes_out), `+table+`.country_code
		FROM `+table+`
		JOIN processes p ON `+table+`.process_id = p.id
		WHERE `+table+`.`+timeCol+` >= ?
		GROUP BY p.pid, p.exe_name, `+table+`.country_code
	`, since)
	if err != nil {
		return nil, err
	}
	defer appsRows.Close()

	var apps []AppHistoryPoint
	for appsRows.Next() {
		var a AppHistoryPoint
		if err := appsRows.Scan(&a.PID, &a.Name, &a.Download, &a.Upload, &a.Country); err != nil {
			return nil, err
		}
		apps = append(apps, a)
	}

	return &HistoryResponse{
		Range:       rangeStr,
		Granularity: granularity,
		Series:      series,
		Apps:        apps,
	}, nil
}

func GetGeoIPStats(ctx context.Context, db *sql.DB, rangeStr string) (*GeoIPStats, error) {
	var since int64
	var table string
	var timeCol string

	now := time.Now().UnixMilli()

	switch rangeStr {
	case "1h":
		since = now - 3600*1000
		table = "traffic_samples"
		timeCol = "timestamp"
	case "24h":
		since = now - 24*3600*1000
		table = "hourly_aggregates"
		timeCol = "hour_start"
	case "7d":
		since = now - 7*24*3600*1000
		table = "hourly_aggregates"
		timeCol = "hour_start"
	case "30d":
		since = now - 30*24*3600*1000
		table = "daily_aggregates"
		timeCol = "day_start"
	default:
		since = now - 24*3600*1000
		table = "hourly_aggregates"
		timeCol = "hour_start"
	}

	rows, err := db.QueryContext(ctx, `
		SELECT country_code, SUM(bytes_in), SUM(bytes_out)
		FROM `+table+`
		WHERE `+timeCol+` >= ? AND country_code IS NOT NULL AND country_code != ''
		GROUP BY country_code
		ORDER BY (SUM(bytes_in) + SUM(bytes_out)) DESC
		LIMIT 20
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats GeoIPStats
	for rows.Next() {
		var c CountryStats
		if err := rows.Scan(&c.CountryCode, &c.Download, &c.Upload); err != nil {
			return nil, err
		}
		c.Total = c.Download + c.Upload
		stats.Countries = append(stats.Countries, c)
	}
	return &stats, rows.Err()
}