package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"bandwidth-tracker/internal/aggregator"
	"bandwidth-tracker/internal/collector"
	"bandwidth-tracker/internal/db"
	"bandwidth-tracker/internal/geoip"

	_ "modernc.org/sqlite"
)

func main() {
	port := getEnv("PORT", "8080")
	pollInterval := getEnvInt("POLL_INTERVAL_SECONDS", 5)
	dbPath := getEnv("DB_PATH", defaultDBPath())
	aggWindow := getEnvInt("AGGREGATION_WINDOW_SECONDS", 60)
	aggInterval := getEnvInt("AGGREGATION_INTERVAL_MINUTES", 60)

	log.Printf("Starting bandwidth-tracker backend on port %s", port)
	log.Printf("Poll interval: %ds, DB: %s, Aggregation window: %ds, Aggregation interval: %dm", pollInterval, dbPath, aggWindow, aggInterval)

	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	correlator := collector.NewCorrelator(database)
	collectorWorker := collector.NewWorker(correlator, pollInterval)
	aggregatorWorker := aggregator.NewWorker(database, aggInterval)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	collectorWorker.Start(ctx)
	log.Printf("Collector worker started")

	aggregatorWorker.Start(ctx)
	log.Printf("Aggregation worker started")

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	http.HandleFunc("/api/metrics", func(w http.ResponseWriter, r *http.Request) {
		metrics, err := db.GetAggregatedMetrics(r.Context(), database, aggWindow)
		if err != nil {
			log.Printf("Metrics error: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"download":  metrics.Download,
			"upload":    metrics.Upload,
			"apps":      metrics.Apps,
			"estimated": metrics.Estimated,
		})
	})

	http.HandleFunc("/api/history", func(w http.ResponseWriter, r *http.Request) {
		rangeStr := r.URL.Query().Get("range")
		if rangeStr == "" {
			rangeStr = "24h"
		}

		history, err := db.GetHistory(r.Context(), database, rangeStr)
		if err != nil {
			log.Printf("History error: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(history)
	})

	http.HandleFunc("/api/geoip/stats", func(w http.ResponseWriter, r *http.Request) {
		rangeStr := r.URL.Query().Get("range")
		if rangeStr == "" {
			rangeStr = "24h"
		}

		stats, err := db.GetGeoIPStats(r.Context(), database, rangeStr)
		if err != nil {
			log.Printf("GeoIP stats error: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	})

	server := &http.Server{Addr: "127.0.0.1:" + port}
	serverErr := make(chan error, 1)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-sigCh:
		log.Printf("Shutting down...")
	case err := <-serverErr:
		log.Printf("HTTP server stopped unexpectedly: %v", err)
	}

	cancel()
	collectorWorker.Stop()
	aggregatorWorker.Stop()
	geoip.Close()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	server.Shutdown(shutdownCtx)
	log.Printf("Server stopped")
}

func getEnv(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultValue
}

func defaultDBPath() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, _ := os.UserHomeDir()
		appData = home + "\\AppData\\Roaming"
	}
	return appData + "\\bandwidth-tracker"
}
