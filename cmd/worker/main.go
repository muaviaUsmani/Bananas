package main

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	"github.com/muaviaUsmani/bananas/internal/config"
	"github.com/muaviaUsmani/bananas/internal/logger"
	"github.com/muaviaUsmani/bananas/internal/queue"
	"github.com/muaviaUsmani/bananas/internal/worker"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	log, err := logger.NewLogger(cfg.Logging)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer log.Close()

	// Set as default logger
	logger.SetDefault(log)

	// Create component-specific logger
	workerLog := log.WithComponent(logger.ComponentWorker).WithSource(logger.LogSourceInternal)

	workerLog.Info("Worker starting",
		"concurrency", cfg.WorkerConcurrency,
		"job_timeout", cfg.JobTimeout,
		"redis_url", cfg.RedisURL)

	// Start pprof server on separate port for profiling
	pprofPort := os.Getenv("PPROF_PORT")
	if pprofPort == "" {
		pprofPort = "6061"
	}
	go func() {
		workerLog.Info("Starting pprof server", "port", pprofPort, "url", fmt.Sprintf("http://localhost:%s/debug/pprof/", pprofPort))
		if err := http.ListenAndServe(":"+pprofPort, nil); err != nil {
			workerLog.Error("pprof server failed", "error", err)
		}
	}()

	// Connect to Redis queue
	redisQueue, err := queue.NewRedisQueue(cfg.RedisURL)
	if err != nil {
		workerLog.Error("Failed to connect to Redis", "error", err)
		os.Exit(1)
	}
	defer redisQueue.Close()

	// Create handler registry
	registry := worker.NewRegistry()

	// TODO: Replace example handlers with your actual job handlers
	// Register example handlers for demonstration
	registry.Register("count_items", worker.HandleCountItems)
	registry.Register("send_email", worker.HandleSendEmail)
	registry.Register("process_data", worker.HandleProcessData)

	workerLog.Info("Registered job handlers", "count", registry.Count())

	// Create executor with queue integration
	executor := worker.NewExecutor(registry, redisQueue, cfg.WorkerConcurrency)

	// Create worker pool
	pool := worker.NewPool(executor, redisQueue, cfg.WorkerConcurrency, cfg.JobTimeout)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start worker pool
	pool.Start(ctx)

	// Wait for shutdown signal
	sig := <-sigChan
	workerLog.Info("Received shutdown signal, initiating graceful shutdown", "signal", sig)

	// Cancel context to stop workers
	cancel()

	// Stop the pool (waits for all workers to finish)
	pool.Stop()

	workerLog.Info("Worker shut down successfully")
}

