package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"

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
	defer log.Close()

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
		if err := http.ListenAndServe(":"+pprofPort, nil); err != nil {
			apiLog.Error("pprof server failed", "error", err)
		}
	}()

	// Setup main API routes
	mainMux := http.NewServeMux()
	mainMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Bananas API Server")
	})

	addr := ":" + cfg.APIPort
	apiLog.Info("API server listening", "address", addr)

	if err := http.ListenAndServe(addr, mainMux); err != nil {
		apiLog.Error("API server failed", "error", err)
		os.Exit(1)
	}
}
