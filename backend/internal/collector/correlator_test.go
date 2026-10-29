package collector

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"bandwidth-tracker/internal/db"
	_ "modernc.org/sqlite"
)

func setupTestCorrelator(t *testing.T) (*Correlator, *sql.DB, func()) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "\\test.db"
	database, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if _, err := database.Exec(db.Schema); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	
	correlator := NewCorrelator(database)
	return correlator, database, func() { database.Close() }
}

func TestConnectionKey(t *testing.T) {
	conn := Connection{
		PID:        1234,
		LocalIP:    "192.168.1.100",
		LocalPort:  54321,
		RemoteIP:   "192.168.1.1",
		RemotePort: 443,
		Protocol:   "tcp",
	}
	
	key := connectionKey(conn)
	expected := "1234|192.168.1.100|54321|192.168.1.1|443|tcp"
	if key != expected {
		t.Errorf("key mismatch: got %s, want %s", key, expected)
	}
}

func TestProcessConnectionsNew(t *testing.T) {
	correlator, database, cleanup := setupTestCorrelator(t)
	defer cleanup()
	ctx := context.Background()
	
	conns := []Connection{
		{PID: 1234, LocalIP: "192.168.1.100", LocalPort: 54321, RemoteIP: "192.168.1.1", RemotePort: 443, Protocol: "tcp", State: MIB_TCP_STATE_ESTAB},
	}
	
	err := correlator.ProcessConnections(ctx, conns)
	if err != nil {
		t.Fatalf("process connections: %v", err)
	}
	
	samples, err := db.GetSamplesSince(ctx, database, time.Now().UnixMilli()-10000)
	if err != nil {
		t.Fatalf("get samples: %v", err)
	}
	
	if len(samples) != 1 {
		t.Errorf("expected 1 sample, got %d", len(samples))
	}
	if samples[0].BytesIn != 0 || samples[0].BytesOut != 0 {
		t.Errorf("new connection should have 0 bytes, got in=%d out=%d", samples[0].BytesIn, samples[0].BytesOut)
	}
	if !samples[0].Estimated {
		t.Errorf("should be estimated")
	}
}

func TestProcessConnectionsExisting(t *testing.T) {
	correlator, database, cleanup := setupTestCorrelator(t)
	defer cleanup()
	ctx := context.Background()
	
	conns := []Connection{
		{PID: 1234, LocalIP: "192.168.1.100", LocalPort: 54321, RemoteIP: "192.168.1.1", RemotePort: 443, Protocol: "tcp", State: MIB_TCP_STATE_ESTAB},
	}
	
	err := correlator.ProcessConnections(ctx, conns)
	if err != nil {
		t.Fatalf("first poll: %v", err)
	}
	
	time.Sleep(10 * time.Millisecond)
	
	conns2 := []Connection{
		{PID: 1234, LocalIP: "192.168.1.100", LocalPort: 54321, RemoteIP: "192.168.1.1", RemotePort: 443, Protocol: "tcp", State: MIB_TCP_STATE_ESTAB},
	}
	
	err = correlator.ProcessConnections(ctx, conns2)
	if err != nil {
		t.Fatalf("second poll: %v", err)
	}
	
	samples, err := db.GetSamplesSince(ctx, database, time.Now().UnixMilli()-10000)
	if err != nil {
		t.Fatalf("get samples: %v", err)
	}
	
	if len(samples) != 2 {
		t.Errorf("expected 2 samples, got %d", len(samples))
	}
	
	if samples[1].BytesIn == 0 && samples[1].BytesOut == 0 {
		t.Errorf("existing connection should have estimated bytes")
	}
}

func TestEstimateBytes(t *testing.T) {
	prev := Connection{State: MIB_TCP_STATE_ESTAB, Protocol: "tcp"}
	curr := Connection{State: MIB_TCP_STATE_ESTAB, Protocol: "tcp"}
	
	in := estimateBytes(prev, curr, true)
	out := estimateBytes(prev, curr, false)
	
	if in != out {
		t.Errorf("tcp in/out should be equal: %d vs %d", in, out)
	}
	
	prevUDP := Connection{State: 0, Protocol: "udp"}
	currUDP := Connection{State: 0, Protocol: "udp"}
	
	udpIn := estimateBytes(prevUDP, currUDP, true)
	if udpIn <= 0 {
		t.Errorf("udp should have estimated bytes: %d", udpIn)
	}
}

func TestProcessConnectionsNotEstablished(t *testing.T) {
	correlator, database, cleanup := setupTestCorrelator(t)
	defer cleanup()
	ctx := context.Background()
	
	conns := []Connection{
		{PID: 1234, LocalIP: "192.168.1.100", LocalPort: 54321, RemoteIP: "192.168.1.1", RemotePort: 443, Protocol: "tcp", State: MIB_TCP_STATE_LISTEN},
	}
	
	err := correlator.ProcessConnections(ctx, conns)
	if err != nil {
		t.Fatalf("process connections: %v", err)
	}
	
	time.Sleep(10 * time.Millisecond)
	
	conns2 := []Connection{
		{PID: 1234, LocalIP: "192.168.1.100", LocalPort: 54321, RemoteIP: "192.168.1.1", RemotePort: 443, Protocol: "tcp", State: MIB_TCP_STATE_LISTEN},
	}
	
	err = correlator.ProcessConnections(ctx, conns2)
	if err != nil {
		t.Fatalf("second poll: %v", err)
	}
	
	samples, err := db.GetSamplesSince(ctx, database, time.Now().UnixMilli()-10000)
	if err != nil {
		t.Fatalf("get samples: %v", err)
	}
	
	if len(samples) != 2 {
		t.Errorf("expected 2 samples, got %d", len(samples))
	}
	
	if samples[1].BytesIn != 0 || samples[1].BytesOut != 0 {
		t.Errorf("non-established connection should have 0 bytes, got in=%d out=%d", samples[1].BytesIn, samples[1].BytesOut)
	}
}