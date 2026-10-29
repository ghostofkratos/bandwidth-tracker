package aggregator

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

type Worker struct {
	db        *sql.DB
	interval  time.Duration
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

func NewWorker(db *sql.DB, intervalMinutes int) *Worker {
	if intervalMinutes <= 0 {
		intervalMinutes = 60
	}
	return &Worker{
		db:       db,
		interval: time.Duration(intervalMinutes) * time.Minute,
		stopCh:   make(chan struct{}),
	}
}

func (w *Worker) Start(ctx context.Context) {
	w.wg.Add(1)
	go w.run(ctx)
}

func (w *Worker) run(ctx context.Context) {
	defer w.wg.Done()

	// Run immediately on start
	if err := RunAll(ctx, w.db); err != nil {
		log.Printf("Initial aggregation error: %v", err)
	}

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-w.stopCh:
			return
		case <-sigCh:
			log.Printf("Aggregation worker received shutdown signal")
			return
		case <-ticker.C:
			if err := RunAll(ctx, w.db); err != nil {
				log.Printf("Aggregation cycle error: %v", err)
			}
		}
	}
}

func (w *Worker) Stop() {
	close(w.stopCh)
	w.wg.Wait()
}

func getAggregationInterval() int {
	if v := os.Getenv("AGGREGATION_INTERVAL_MINUTES"); v != "" {
		if i, err := strconv.Atoi(v); err == nil && i > 0 {
			return i
		}
	}
	return 60
}