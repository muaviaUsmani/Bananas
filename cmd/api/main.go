// Package main provides the Bananas API server.
package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof" // #nosec G108 - pprof is intentionally exposed for debugging, isolated to separate port
	"os"
	"time"

	"github.com/muaviaUsmani/bananas/internal/config"
	"github.com/muaviaUsmani/bananas/internal/logger"
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
	defer func() {
		if err := log.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to close logger: %v\n", err)
		}
	}()

	// Set as default logger
	logger.SetDefault(log)

	// Create component-specific logger
	apiLog := log.WithComponent(logger.ComponentAPI).WithSource(logger.LogSourceInternal)

	apiLog.Info("API server starting",
		"redis_url", cfg.RedisURL,
		"api_port", cfg.APIPort,
		"job_timeout", cfg.JobTimeout,
		"max_retries", cfg.MaxRetries)

	// Start pprof server on separate port for profiling
	pprofPort := os.Getenv("PPROF_PORT")
	if pprofPort == "" {
		pprofPort = "6060"
	}
	go func() {
		apiLog.Info("Starting pprof server", "port", pprofPort, "url", fmt.Sprintf("http://localhost:%s/debug/pprof/", pprofPort))
		pprofServer := &http.Server{
			Addr:              ":" + pprofPort,
			ReadHeaderTimeout: 5 * time.Second,
			ReadTimeout:       10 * time.Second,
			WriteTimeout:      10 * time.Second,
			IdleTimeout:       60 * time.Second,
		}
		if err := pprofServer.ListenAndServe(); err != nil {
			apiLog.Error("pprof server failed", "error", err)
		}
	}()

	// Setup main API routes
	mainMux := http.NewServeMux()
	mainMux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		// Ignore write error - nothing we can do if client disconnected
		_, _ = fmt.Fprintf(w, "Bananas API Server")
	})

	addr := ":" + cfg.APIPort
	apiLog.Info("API server listening", "address", addr)

	server := &http.Server{
		Addr:              addr,
		Handler:           mainMux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		apiLog.Error("API server failed", "error", err)
		os.Exit(1)
	}
}
