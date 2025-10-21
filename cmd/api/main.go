package main

import (
	"fmt"
	"log"
	"net/http"

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

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Bananas API Server")
	})

	addr := ":" + cfg.APIPort
	fmt.Printf("Listening on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

