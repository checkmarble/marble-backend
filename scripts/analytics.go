package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	// Request body
	requestBody := []byte(`{
    "scenario_id": "718c1e31-b858-452d-83d0-8784d54db2f0",
    "start": "2025-09-24T00:00:00Z",
    "end": "2025-11-15T00:00:00Z"
}`)

	// Read configuration from environment variables
	url := os.Getenv("URL")
	if url == "" {
		log.Fatal("URL environment variable is required")
	}

	authHeader := os.Getenv("AUTH_HEADER")
	if authHeader == "" {
		log.Fatal("AUTH_HEADER environment variable is required")
	}

	// Get concurrency (default: 10)
	concurrency := 10
	if concStr := os.Getenv("CONCURRENCY"); concStr != "" {
		if c, err := strconv.Atoi(concStr); err == nil {
			concurrency = c
		}
	}

	// Get duration in seconds (default: 30)
	durationSeconds := 30
	if durStr := os.Getenv("DURATION_SECONDS"); durStr != "" {
		if d, err := strconv.Atoi(durStr); err == nil {
			durationSeconds = d
		}
	}

	duration := time.Duration(durationSeconds) * time.Second

	fmt.Printf("Starting load test:\n")
	fmt.Printf("  URL: %s\n", url)
	fmt.Printf("  Concurrency: %d\n", concurrency)
	fmt.Printf("  Duration: %v\n", duration)
	fmt.Println()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	// Counters for statistics
	var successCount, errorCount atomic.Int64

	// WaitGroup to track all workers
	var wg sync.WaitGroup

	startTime := time.Now()

	// Start concurrent workers
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			client := &http.Client{
				Timeout: 30 * time.Second,
			}

			for {
				select {
				case <-ctx.Done():
					return
				default:
					req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(requestBody))
					if err != nil {
						errorCount.Add(1)
						log.Printf("Worker %d: Failed to create request: %v\n", workerID, err)
						continue
					}

					req.Header.Set("Authorization", "Bearer "+authHeader)
					req.Header.Set("Content-Type", "application/json")

					resp, err := client.Do(req)
					if err != nil {
						errorCount.Add(1)
						log.Printf("Worker %d: Request failed: %v\n", workerID, err)
						continue
					}

					// Read response body
					body, err := io.ReadAll(resp.Body)
					resp.Body.Close()

					if err != nil {
						errorCount.Add(1)
						log.Printf("Worker %d: Failed to read response body: %v\n", workerID, err)
						continue
					}

					if resp.StatusCode >= 200 && resp.StatusCode < 300 {
						successCount.Add(1)
					} else {
						errorCount.Add(1)
						log.Printf("Worker %d: Unexpected status code: %d, Response: %s\n",
							workerID, resp.StatusCode, string(body))
					}
				}
			}
		}(i)
	}

	// Wait for all workers to finish
	wg.Wait()

	elapsed := time.Since(startTime)

	// Print statistics
	fmt.Println()
	fmt.Println("Load test completed!")
	fmt.Printf("  Duration: %v\n", elapsed)
	fmt.Printf("  Success: %d\n", successCount.Load())
	fmt.Printf("  Errors: %d\n", errorCount.Load())
	fmt.Printf("  Total requests: %d\n", successCount.Load()+errorCount.Load())
	fmt.Printf("  Requests/sec: %.2f\n", float64(successCount.Load()+errorCount.Load())/elapsed.Seconds())
}
