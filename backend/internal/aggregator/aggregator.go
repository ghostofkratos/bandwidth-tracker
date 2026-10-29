package aggregator

import (
	"context"
	"database/sql"
	"log"
	"os"
	"strconv"
	"time"
)

func getEnvInt(key string, defaultValue int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultValue
}

func AggregateHourly(ctx context.Context, db *sql.DB) error {
	now := time.Now()
	lastHour := now.Truncate(time.Hour).Add(-time.Hour)
	hourStart := lastHour.UnixMilli()
	hourEnd := lastHour.Add(time.Hour).UnixMilli()

	log.Printf("Aggregating hourly data for %s", lastHour.Format(time.RFC3339))

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO hourly_aggregates (hour_start, process_id, country_code, bytes_in, bytes_out, connection_count)
		SELECT 
			? as hour_start,
			process_id,
			country_code,
			SUM(bytes_in) as bytes_in,
			SUM(bytes_out) as bytes_out,
			COUNT(DISTINCT remote_ip || ':' || remote_port) as connection_count
		FROM traffic_samples
		WHERE timestamp >= ? AND timestamp < ?
		GROUP BY process_id, country_code
		ON CONFLICT(hour_start, process_id, country_code) DO UPDATE SET
			bytes_in = excluded.bytes_in,
			bytes_out = excluded.bytes_out,
			connection_count = excluded.connection_count
	`, hourStart, hourStart, hourEnd)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func AggregateDaily(ctx context.Context, db *sql.DB) error {
	now := time.Now()
	yesterday := now.Truncate(24 * time.Hour).Add(-24 * time.Hour)
	dayStart := yesterday.UnixMilli()
	dayEnd := yesterday.Add(24 * time.Hour).UnixMilli()

	log.Printf("Aggregating daily data for %s", yesterday.Format(time.RFC3339))

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO daily_aggregates (day_start, process_id, country_code, bytes_in, bytes_out, connection_count)
		SELECT 
			? as day_start,
			process_id,
			country_code,
			SUM(bytes_in) as bytes_in,
			SUM(bytes_out) as bytes_out,
			SUM(connection_count) as connection_count
		FROM hourly_aggregates
		WHERE hour_start >= ? AND hour_start < ?
		GROUP BY process_id, country_code
		ON CONFLICT(day_start, process_id, country_code) DO UPDATE SET
			bytes_in = excluded.bytes_in,
			bytes_out = excluded.bytes_out,
			connection_count = excluded.connection_count
	`, dayStart, dayStart, dayEnd)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func CleanupRaw(ctx context.Context, db *sql.DB) error {
	retentionDays := getEnvInt("RAW_RETENTION_DAYS", 7)
	cutoff := time.Now().AddDate(0, 0, -retentionDays).UnixMilli()

	log.Printf("Cleaning up raw samples older than %d days", retentionDays)

	result, err := db.ExecContext(ctx, `
		DELETE FROM traffic_samples WHERE timestamp < ?
	`, cutoff)
	if err != nil {
		return err
	}

	affected, _ := result.RowsAffected()
	log.Printf("Deleted %d raw samples", affected)
	return nil
}

func CleanupHourly(ctx context.Context, db *sql.DB) error {
	retentionDays := getEnvInt("HOURLY_RETENTION_DAYS", 365)
	cutoff := time.Now().AddDate(0, 0, -retentionDays).UnixMilli()

	log.Printf("Cleaning up hourly aggregates older than %d days", retentionDays)

	result, err := db.ExecContext(ctx, `
		DELETE FROM hourly_aggregates WHERE hour_start < ?
	`, cutoff)
	if err != nil {
		return err
	}

	affected, _ := result.RowsAffected()
	log.Printf("Deleted %d hourly aggregates", affected)
	return nil
}

func RunAll(ctx context.Context, db *sql.DB) error {
	if err := AggregateHourly(ctx, db); err != nil {
		log.Printf("Hourly aggregation error: %v", err)
		return err
	}

	if err := AggregateDaily(ctx, db); err != nil {
		log.Printf("Daily aggregation error: %v", err)
		return err
	}

	if err := CleanupRaw(ctx, db); err != nil {
		log.Printf("Raw cleanup error: %v", err)
		return err
	}

	if err := CleanupHourly(ctx, db); err != nil {
		log.Printf("Hourly cleanup error: %v", err)
		return err
	}

	log.Printf("Aggregation cycle completed successfully")
	return nil
}