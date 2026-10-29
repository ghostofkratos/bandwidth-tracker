package collector

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
)

type Worker struct {
	correlator    *Correlator
	interval      time.Duration
	stopCh        chan struct{}
	wg            sync.WaitGroup
}

func NewWorker(correlator *Correlator, intervalSeconds int) *Worker {
	if intervalSeconds <= 0 {
		intervalSeconds = 5
	}
	return &Worker{
		correlator: correlator,
		interval:   time.Duration(intervalSeconds) * time.Second,
		stopCh:     make(chan struct{}),
	}
}

func (w *Worker) Start(ctx context.Context) {
	w.wg.Add(1)
	go w.run(ctx)
}

func (w *Worker) run(ctx context.Context) {
	defer w.wg.Done()

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-w.stopCh:
			return
		case <-sigCh:
			log.Printf("Worker received shutdown signal")
			return
		case <-ticker.C:
			if err := w.collectAndProcess(ctx); err != nil {
				log.Printf("Collection error: %v", err)
			}
		}
	}
}

func (w *Worker) collectAndProcess(ctx context.Context) error {
	tcpConns, err := GetTcpConnections()
	if err != nil {
		return fmt.Errorf("get TCP connections: %w", err)
	}

	udpConns, err := GetUdpConnections()
	if err != nil {
		return fmt.Errorf("get UDP connections: %w", err)
	}

	allConns := append(tcpConns, udpConns...)
	log.Printf("Polled %d connections (TCP: %d, UDP: %d)", len(allConns), len(tcpConns), len(udpConns))

	if err := w.correlator.ProcessConnections(ctx, allConns); err != nil {
		return fmt.Errorf("process connections: %w", err)
	}

	CleanExpiredCache()
	return nil
}

func (w *Worker) Stop() {
	close(w.stopCh)
	w.wg.Wait()
}

func getPollInterval() int {
	if v := os.Getenv("POLL_INTERVAL_SECONDS"); v != "" {
		if i, err := strconv.Atoi(v); err == nil && i > 0 {
			return i
		}
	}
	return 5
}