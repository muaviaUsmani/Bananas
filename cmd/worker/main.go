// Package main provides the Bananas worker service for processing background jobs.
package main

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof" // #nosec G108 - pprof is intentionally exposed for debugging, isolated to separate port
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/muaviaUsmani/bananas/internal/config"
	"github.com/muaviaUsmani/bananas/internal/logger"
	"github.com/muaviaUsmani/bananas/internal/metrics"
	"github.com/muaviaUsmani/bananas/internal/queue"
	"github.com/muaviaUsmani/bananas/internal/result"
	"github.com/muaviaUsmani/bananas/internal/worker"
	"github.com/redis/go-redis/v9"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Load worker-specific configuration
	workerCfg, err := config.LoadWorkerConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load worker config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log, err := logger.NewLogger(cfg.Logging)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if err := log.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to close logger: %v\n", err)
		}
	}()

	// Set as default logger
	logger.SetDefault(log)

	// Create component-specific logger
	workerLog := log.WithComponent(logger.ComponentWorker).WithSource(logger.LogSourceInternal)

	workerLog.Info("Worker starting",
		"mode", workerCfg.Mode,
		"concurrency", workerCfg.Concurrency,
		"priorities", len(workerCfg.Priorities),
		"job_types", len(workerCfg.JobTypes),
		"job_timeout", cfg.JobTimeout,
		"redis_url", cfg.RedisURL)

	// Log detailed worker configuration
	workerLog.Info("Worker configuration details", "config", workerCfg.String())

	// Start pprof server on separate port for profiling
	pprofPort := os.Getenv("PPROF_PORT")
	if pprofPort == "" {
		pprofPort = "6061"
	}
	go func() {
		workerLog.Info("Starting pprof server", "port", pprofPort, "url", fmt.Sprintf("http://localhost:%s/debug/pprof/", pprofPort))
		// Create server with timeouts for security
		server := &http.Server{
			Addr:              ":" + pprofPort,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       60 * time.Second,
		}
		if err := server.ListenAndServe(); err != nil {
			workerLog.Error("pprof server failed", "error", err)
		}
	}()

	// Connect to Redis queue
	redisQueue, err := queue.NewRedisQueue(cfg.RedisURL)
	if err != nil {
		workerLog.Error("Failed to connect to Redis", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := redisQueue.Close(); err != nil {
			workerLog.Error("Failed to close Redis queue", "error", err)
		}
	}()

	// Create result backend if enabled
	var resultBackend result.Backend
	if cfg.ResultBackendEnabled {
		opts, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			workerLog.Error("Failed to parse Redis URL for result backend", "error", err)
			os.Exit(1)
		}
		redisClient := redis.NewClient(opts)
		resultBackend = result.NewRedisBackend(redisClient, cfg.ResultBackendTTLSuccess, cfg.ResultBackendTTLFailure)
		workerLog.Info("Result backend enabled",
			"success_ttl", cfg.ResultBackendTTLSuccess,
			"failure_ttl", cfg.ResultBackendTTLFailure)
	}

	// Create handler registry
	registry := worker.NewRegistry()

	// TODO: Replace example handlers with your actual job handlers
	// Register example handlers for demonstration
	registry.Register("count_items", worker.HandleCountItems)
	registry.Register("send_email", worker.HandleSendEmail)
	registry.Register("process_data", worker.HandleProcessData)

	workerLog.Info("Registered job handlers", "count", registry.Count())

	// Create executor with queue integration
	executor := worker.NewExecutor(registry, redisQueue, workerCfg.Concurrency)

	// Set result backend if enabled
	if resultBackend != nil {
		executor.SetResultBackend(resultBackend)
	}

	// Create worker pool with new configuration system
	pool := worker.NewPoolWithConfig(executor, redisQueue, workerCfg, cfg.JobTimeout)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start worker pool
	pool.Start(ctx)

	// Start periodic metrics logging
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m := metrics.GetMetrics()
				workerLog.Info("System metrics",
					"jobs_processed", m.TotalJobsProcessed,
					"jobs_completed", m.TotalJobsCompleted,
					"jobs_failed", m.TotalJobsFailed,
					"avg_duration_ms", m.AvgJobDuration.Milliseconds(),
					"worker_utilization", fmt.Sprintf("%.1f%%", m.WorkerUtilization),
					"error_rate", fmt.Sprintf("%.2f%%", m.ErrorRate),
					"uptime", m.Uptime.String(),
				)
			}
		}
	}()

	// Wait for shutdown signal
	sig := <-sigChan
	workerLog.Info("Received shutdown signal, initiating graceful shutdown", "signal", sig)

	// Cancel context to stop workers
	cancel()

	// Stop the pool (waits for all workers to finish)
	pool.Stop()

	workerLog.Info("Worker shut down successfully")
}
