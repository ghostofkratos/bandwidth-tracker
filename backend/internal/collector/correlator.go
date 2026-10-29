package collector

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"bandwidth-tracker/internal/db"
	"bandwidth-tracker/internal/geoip"
)

const (
	mtuEstimate       = 1500
	packetsPerPollEst = 10
)

type Correlator struct {
	db        *sql.DB
	prevConns map[string]Connection
	mu        sync.Mutex
}

func NewCorrelator(database *sql.DB) *Correlator {
	return &Correlator{
		db:        database,
		prevConns: make(map[string]Connection),
	}
}

func (c *Correlator) ProcessConnections(ctx context.Context, conns []Connection) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UnixMilli()
	currMap := make(map[string]Connection)

	for _, conn := range conns {
		key := connectionKey(conn)
		currMap[key] = conn

		prev, existed := c.prevConns[key]
		var bytesIn, bytesOut int64

		if !existed {
			bytesIn = 0
			bytesOut = 0
		} else {
			bytesIn = estimateBytes(prev, conn, true)
			bytesOut = estimateBytes(prev, conn, false)
		}

		procInfo, err := GetProcessInfo(int32(conn.PID))
		if err != nil {
			procInfo = ProcessInfo{ExeName: fmt.Sprintf("pid-%d", conn.PID), ExePath: ""}
		}

		proc, err := db.UpsertProcess(ctx, c.db, int32(conn.PID), procInfo.ExeName, procInfo.ExePath)
		if err != nil {
			return fmt.Errorf("upsert process: %w", err)
		}

		countryCode := "XX"
		if conn.RemoteIP != "" {
			countryCode, _ = geoip.Lookup(ctx, conn.RemoteIP)
		}

		sample := &db.TrafficSample{
			Timestamp:   now,
			ProcessID:   proc.ID,
			BytesIn:     bytesIn,
			BytesOut:    bytesOut,
			RemoteIP:    conn.RemoteIP,
			RemotePort:  int32(conn.RemotePort),
			LocalIP:     conn.LocalIP,
			LocalPort:   int32(conn.LocalPort),
			Protocol:    conn.Protocol,
			Estimated:   true,
			CountryCode: countryCode,
		}

		if err := db.InsertSample(ctx, c.db, sample); err != nil {
			return fmt.Errorf("insert sample: %w", err)
		}
	}

	c.prevConns = currMap
	return nil
}

func connectionKey(conn Connection) string {
	return fmt.Sprintf("%d|%s|%d|%s|%d|%s", conn.PID, conn.LocalIP, conn.LocalPort, conn.RemoteIP, conn.RemotePort, conn.Protocol)
}

func estimateBytes(prev, curr Connection, inbound bool) int64 {
	if curr.Protocol == "tcp" {
		if prev.State != MIB_TCP_STATE_ESTAB && curr.State != MIB_TCP_STATE_ESTAB {
			return 0
		}
		return int64(mtuEstimate * packetsPerPollEst)
	}

	if curr.Protocol == "udp" {
		return int64(mtuEstimate * packetsPerPollEst / 2)
	}

	return 0
}