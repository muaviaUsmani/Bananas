package main

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/muaviaUsmani/bananas/internal/config"
)

func main() {
	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Print configuration
	fmt.Println("API server starting...")
	fmt.Printf("Configuration loaded:\n")
	fmt.Printf("  - Redis URL: %s\n", cfg.RedisURL)
	fmt.Printf("  - API Port: %s\n", cfg.APIPort)
	fmt.Printf("  - Job Timeout: %s\n", cfg.JobTimeout)
	fmt.Printf("  - Max Retries: %d\n", cfg.MaxRetries)

	// Start pprof server on separate port for profiling
	pprofPort := os.Getenv("PPROF_PORT")
	if pprofPort == "" {
		pprofPort = "6060"
	}
	go func() {
		fmt.Printf("  - pprof server: http://localhost:%s/debug/pprof/\n", pprofPort)
		if err := http.ListenAndServe(":"+pprofPort, nil); err != nil {
			log.Printf("pprof server failed: %v", err)
		}
	}()

	// Setup main API routes
	mainMux := http.NewServeMux()
	mainMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Bananas API Server")
	})

	addr := ":" + cfg.APIPort
	fmt.Printf("Listening on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, mainMux))
}

