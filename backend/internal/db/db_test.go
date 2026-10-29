package db

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) (*sql.DB, func()) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "\\test.db"
	
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	
	if _, err := db.Exec(Schema); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	
	return db, func() { db.Close() }
}

func TestMigrations(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	
	var tables []string
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table'")
	if err != nil {
		t.Fatalf("query tables: %v", err)
	}
	defer rows.Close()
	
	for rows.Next() {
		var name string
		rows.Scan(&name)
		tables = append(tables, name)
	}
	
	expected := map[string]bool{"processes": true, "traffic_samples": true}
	for _, t := range tables {
		if expected[t] {
			delete(expected, t)
		}
	}
	
	if len(expected) > 0 {
		t.Errorf("missing tables: %v", expected)
	}
}

func TestUpsertProcess(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()
	
	p1, err := UpsertProcess(ctx, db, 1234, "chrome.exe", "C:\\Program Files\\Chrome\\chrome.exe")
	if err != nil {
		t.Fatalf("upsert 1: %v", err)
	}
	
	if p1.PID != 1234 || p1.ExeName != "chrome.exe" {
		t.Errorf("unexpected process: %+v", p1)
	}
	
	p2, err := UpsertProcess(ctx, db, 1234, "chrome.exe", "C:\\Program Files\\Chrome\\chrome.exe")
	if err != nil {
		t.Fatalf("upsert 2: %v", err)
	}
	
	if p2.ID != p1.ID {
		t.Errorf("expected same ID on upsert, got %d vs %d", p2.ID, p1.ID)
	}
	
	if p2.LastSeen <= p1.LastSeen {
		t.Errorf("last_seen should update: %d <= %d", p2.LastSeen, p1.LastSeen)
	}
	
	p3, err := UpsertProcess(ctx, db, 1234, "chrome.exe", "C:\\Other\\chrome.exe")
	if err != nil {
		t.Fatalf("upsert 3: %v", err)
	}
	
	if p3.ID == p1.ID {
		t.Errorf("different exe_path should create new row")
	}
}

func TestGetProcessByPID(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()
	
	_, err := UpsertProcess(ctx, db, 1234, "chrome.exe", "C:\\Chrome\\chrome.exe")
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	
	p, err := GetProcessByPID(ctx, db, 1234)
	if err != nil {
		t.Fatalf("get by pid: %v", err)
	}
	if p == nil || p.PID != 1234 {
		t.Errorf("expected process with PID 1234, got %+v", p)
	}
	
	p, err = GetProcessByPID(ctx, db, 9999)
	if err != nil {
		t.Fatalf("get missing: %v", err)
	}
	if p != nil {
		t.Errorf("expected nil for missing PID, got %+v", p)
	}
}

func TestInsertSample(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()
	
	p, err := UpsertProcess(ctx, db, 1234, "chrome.exe", "C:\\Chrome\\chrome.exe")
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	
	sample := &TrafficSample{
		Timestamp:  time.Now().UnixMilli(),
		ProcessID:  p.ID,
		BytesIn:    1000,
		BytesOut:   500,
		RemoteIP:   "192.168.1.1",
		RemotePort: 443,
		LocalIP:    "192.168.1.100",
		LocalPort:  54321,
		Protocol:   "tcp",
		Estimated:  true,
	}
	
	if err := InsertSample(ctx, db, sample); err != nil {
		t.Fatalf("insert sample: %v", err)
	}
}

func TestGetSamplesSince(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()
	
	p, _ := UpsertProcess(ctx, db, 1234, "chrome.exe", "C:\\Chrome\\chrome.exe")
	
	now := time.Now().UnixMilli()
	samples := []TrafficSample{
		{Timestamp: now - 10000, ProcessID: p.ID, BytesIn: 100, BytesOut: 50, Protocol: "tcp"},
		{Timestamp: now - 5000, ProcessID: p.ID, BytesIn: 200, BytesOut: 100, Protocol: "tcp"},
		{Timestamp: now - 1000, ProcessID: p.ID, BytesIn: 300, BytesOut: 150, Protocol: "tcp"},
	}
	
	for _, s := range samples {
		if err := InsertSample(ctx, db, &s); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	
	results, err := GetSamplesSince(ctx, db, now-7000)
	if err != nil {
		t.Fatalf("get since: %v", err)
	}
	
	if len(results) != 2 {
		t.Errorf("expected 2 samples, got %d", len(results))
	}
}

func TestGetAggregatedMetrics(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()
	ctx := context.Background()
	
	p1, _ := UpsertProcess(ctx, db, 1234, "chrome.exe", "C:\\Chrome\\chrome.exe")
	p2, _ := UpsertProcess(ctx, db, 5678, "firefox.exe", "C:\\Firefox\\firefox.exe")
	
	now := time.Now().UnixMilli()
	samples := []TrafficSample{
		{Timestamp: now - 1000, ProcessID: p1.ID, BytesIn: 1000, BytesOut: 500, RemoteIP: "1.1.1.1", RemotePort: 443, LocalIP: "192.168.1.1", LocalPort: 50000, Protocol: "tcp"},
		{Timestamp: now - 2000, ProcessID: p1.ID, BytesIn: 2000, BytesOut: 1000, RemoteIP: "2.2.2.2", RemotePort: 80, LocalIP: "192.168.1.1", LocalPort: 50001, Protocol: "tcp"},
		{Timestamp: now - 3000, ProcessID: p2.ID, BytesIn: 500, BytesOut: 250, RemoteIP: "3.3.3.3", RemotePort: 443, LocalIP: "192.168.1.1", LocalPort: 50002, Protocol: "tcp"},
	}
	
	for _, s := range samples {
		InsertSample(ctx, db, &s)
	}
	
	metrics, err := GetAggregatedMetrics(ctx, db, 10)
	if err != nil {
		t.Fatalf("get metrics: %v", err)
	}
	
	if metrics.Download != 3500 {
		t.Errorf("expected download 3500, got %d", metrics.Download)
	}
	if metrics.Upload != 1750 {
		t.Errorf("expected upload 1750, got %d", metrics.Upload)
	}
	if len(metrics.Apps) != 2 {
		t.Errorf("expected 2 apps, got %d", len(metrics.Apps))
	}
}