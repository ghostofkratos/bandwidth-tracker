package geoip

import (
	"context"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/oschwald/maxminddb-golang"
)

var (
	reader *maxminddb.Reader
	once   sync.Once
	initErr error
)

type cachedEntry struct {
	countryCode string
	expiresAt   time.Time
}

var (
	cache     = make(map[string]cachedEntry)
	cacheMu   sync.RWMutex
	cacheTTL  = 24 * time.Hour
)

func getReader() (*maxminddb.Reader, error) {
	once.Do(func() {
		dbPath := os.Getenv("GEOIP_DB_PATH")
		if dbPath == "" {
			appData := os.Getenv("APPDATA")
			if appData == "" {
				home, _ := os.UserHomeDir()
				appData = home + "\\AppData\\Roaming"
			}
			dbPath = appData + "\\bandwidth-tracker\\GeoLite2-Country.mmdb"
		}

		r, err := maxminddb.Open(dbPath)
		if err != nil {
			initErr = err
			log.Printf("GeoIP database not found at %s: %v (falling back to XX)", dbPath, err)
			return
		}
		reader = r
		log.Printf("GeoIP database loaded from %s", dbPath)
	})
	return reader, initErr
}

func Lookup(ctx context.Context, ip string) (string, error) {
	cacheMu.RLock()
	if entry, ok := cache[ip]; ok && time.Now().Before(entry.expiresAt) {
		cacheMu.RUnlock()
		return entry.countryCode, nil
	}
	cacheMu.RUnlock()

	r, err := getReader()
	if err != nil || r == nil {
		return "XX", nil
	}

	var record struct {
		Country struct {
			IsoCode string `maxminddb:"iso_code"`
		} `maxminddb:"country"`
	}

	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return "XX", nil
	}

	err = r.Lookup(parsedIP, &record)
	if err != nil {
		return "XX", nil
	}

	countryCode := record.Country.IsoCode
	if countryCode == "" {
		countryCode = "XX"
	}

	cacheMu.Lock()
	cache[ip] = cachedEntry{
		countryCode: countryCode,
		expiresAt:   time.Now().Add(cacheTTL),
	}
	cacheMu.Unlock()

	return countryCode, nil
}

func CleanupCache() {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	now := time.Now()
	for ip, entry := range cache {
		if now.After(entry.expiresAt) {
			delete(cache, ip)
		}
	}
}

func Close() error {
	if reader != nil {
		return reader.Close()
	}
	return nil
}