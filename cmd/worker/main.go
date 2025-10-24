package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"

	"github.com/muaviaUsmani/bananas/internal/config"
	"github.com/muaviaUsmani/bananas/internal/queue"
	"github.com/muaviaUsmani/bananas/internal/worker"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	fmt.Println("Worker starting...")
	fmt.Printf("Worker concurrency: %d\n", cfg.WorkerConcurrency)
	fmt.Printf("Job timeout: %s\n", cfg.JobTimeout)
	fmt.Printf("Connecting to Redis: %s\n", cfg.RedisURL)

	// Start pprof server on separate port for profiling
	pprofPort := os.Getenv("PPROF_PORT")
	if pprofPort == "" {
		pprofPort = "6061"
	}
	go func() {
		fmt.Printf("pprof server: http://localhost:%s/debug/pprof/\n", pprofPort)
		if err := http.ListenAndServe(":"+pprofPort, nil); err != nil {
			log.Printf("pprof server failed: %v", err)
		}
	}()

	// Connect to Redis queue
	redisQueue, err := queue.NewRedisQueue(cfg.RedisURL)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisQueue.Close()

	// Create handler registry
	registry := worker.NewRegistry()

	// TODO: Replace example handlers with your actual job handlers
	// Register example handlers for demonstration
	registry.Register("count_items", worker.HandleCountItems)
	registry.Register("send_email", worker.HandleSendEmail)
	registry.Register("process_data", worker.HandleProcessData)

	fmt.Printf("Registered %d job handlers\n", registry.Count())

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
	log.Printf("Received signal %v, initiating graceful shutdown...", sig)

	// Cancel context to stop workers
	cancel()

	// Stop the pool (waits for all workers to finish)
	pool.Stop()

	log.Println("Worker shut down successfully")
}

