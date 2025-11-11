package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/muaviaUsmani/bananas/internal/job"
	"github.com/muaviaUsmani/bananas/internal/queue"
	"github.com/muaviaUsmani/bananas/internal/worker"
	"github.com/muaviaUsmani/bananas/pkg/client"
)

// BenchmarkResults stores comprehensive benchmark data
type BenchmarkResults struct {
	TestName      string
	TotalOps      int64
	Duration      time.Duration
	OpsPerSecond  float64
	AvgLatency    time.Duration
	P50Latency    time.Duration
	P95Latency    time.Duration
	P99Latency    time.Duration
	MinLatency    time.Duration
	MaxLatency    time.Duration
	Configuration map[string]interface{}
	Timestamp     time.Time
	SystemInfo    SystemInfo
}

// SystemInfo captures system details for benchmarks
type SystemInfo struct {
	GoVersion    string
	NumCPU       int
	GOMAXPROCS   int
	OS           string
	Arch         string
	RedisVersion string
}

// getSystemInfo captures current system information
func getSystemInfo() SystemInfo {
	return SystemInfo{
		GoVersion:    runtime.Version(),
		NumCPU:       runtime.NumCPU(),
		GOMAXPROCS:   runtime.GOMAXPROCS(0),
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		RedisVersion: "miniredis-mock",
	}
}

// calculatePercentiles computes latency percentiles from sorted durations
func calculatePercentiles(latencies []time.Duration) (p50, p95, p99 time.Duration) {
	if len(latencies) == 0 {
		return 0, 0, 0
	}

	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})

	p50Index := int(math.Ceil(float64(len(latencies)) * 0.50))
	p95Index := int(math.Ceil(float64(len(latencies)) * 0.95))
	p99Index := int(math.Ceil(float64(len(latencies)) * 0.99))

	if p50Index >= len(latencies) {
		p50Index = len(latencies) - 1
	}
	if p95Index >= len(latencies) {
		p95Index = len(latencies) - 1
	}
	if p99Index >= len(latencies) {
		p99Index = len(latencies) - 1
	}

	return latencies[p50Index], latencies[p95Index], latencies[p99Index]
}

// generatePayload creates a JSON payload of approximately the specified size
func generatePayload(sizeKB int) map[string]interface{} {
	// Create a payload that's approximately sizeKB kilobytes
	// Each character is ~1 byte in JSON
	targetBytes := sizeKB * 1024

	// Account for JSON overhead (quotes, braces, etc.) - roughly 20%
	dataSize := int(float64(targetBytes) * 0.8)

	// Create a string of the desired length
	data := make([]byte, dataSize)
	for i := range data {
		// Use printable ASCII characters
		data[i] = byte('a' + (i % 26))
	}

	return map[string]interface{}{
		"data":      string(data),
		"timestamp": time.Now().Unix(),
		"size_kb":   sizeKB,
	}
}

// setupBenchmarkRedis creates a miniredis instance for benchmarking
func setupBenchmarkRedis(t testing.TB) (*miniredis.Miniredis, *queue.RedisQueue) {
	s := miniredis.RunT(t)

	q, err := queue.NewRedisQueue("redis://" + s.Addr())
	if err != nil {
		t.Fatalf("Failed to create queue: %v", err)
	}

	return s, q
}

// setupBenchmarkClient creates a client for benchmarking
func setupBenchmarkClient(t testing.TB, redisAddr string) *client.Client {
	c, err := client.NewClient("redis://" + redisAddr)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	return c
}

// =============================================================================
// BENCHMARK: Job Submission Rate
// =============================================================================

// BenchmarkJobSubmission_1KB tests job submission with 1KB payloads
func BenchmarkJobSubmission_1KB(b *testing.B) {
	benchmarkJobSubmissionWithPayloadSize(b, 1)
}

// BenchmarkJobSubmission_10KB tests job submission with 10KB payloads
func BenchmarkJobSubmission_10KB(b *testing.B) {
	benchmarkJobSubmissionWithPayloadSize(b, 10)
}

// BenchmarkJobSubmission_100KB tests job submission with 100KB payloads
func BenchmarkJobSubmission_100KB(b *testing.B) {
	benchmarkJobSubmissionWithPayloadSize(b, 100)
}

// benchmarkJobSubmissionWithPayloadSize is the core submission benchmark
func benchmarkJobSubmissionWithPayloadSize(b *testing.B, sizeKB int) {
	s, _ := setupBenchmarkRedis(b)
	defer s.Close()

	c := setupBenchmarkClient(b, s.Addr())
	defer c.Close()

	payload := generatePayload(sizeKB)

	// Track latencies
	latencies := make([]time.Duration, 0, b.N)
	var mu sync.Mutex

	b.ResetTimer()
	start := time.Now()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			startOp := time.Now()
			_, err := c.SubmitJob(
				"benchmark_job",
				payload,
				job.PriorityNormal,
				fmt.Sprintf("Benchmark job %dKB", sizeKB),
			)
			latency := time.Since(startOp)

			if err != nil {
				b.Errorf("Failed to submit job: %v", err)
			}

			mu.Lock()
			latencies = append(latencies, latency)
			mu.Unlock()
		}
	})

	duration := time.Since(start)
	b.StopTimer()

	// Calculate metrics
	p50, p95, p99 := calculatePercentiles(latencies)
	opsPerSec := float64(b.N) / duration.Seconds()

	// Report results
	b.ReportMetric(opsPerSec, "ops/sec")
	b.ReportMetric(float64(p50.Microseconds()), "p50-μs")
	b.ReportMetric(float64(p95.Microseconds()), "p95-μs")
	b.ReportMetric(float64(p99.Microseconds()), "p99-μs")
}

// =============================================================================
// BENCHMARK: Job Processing Rate (End-to-End)
// =============================================================================

// BenchmarkJobProcessing_1Worker tests end-to-end processing with 1 worker
func BenchmarkJobProcessing_1Worker(b *testing.B) {
	benchmarkJobProcessingWithWorkers(b, 1)
}

// BenchmarkJobProcessing_5Workers tests end-to-end processing with 5 workers
func BenchmarkJobProcessing_5Workers(b *testing.B) {
	benchmarkJobProcessingWithWorkers(b, 5)
}

// BenchmarkJobProcessing_10Workers tests end-to-end processing with 10 workers
func BenchmarkJobProcessing_10Workers(b *testing.B) {
	benchmarkJobProcessingWithWorkers(b, 10)
}

// BenchmarkJobProcessing_20Workers tests end-to-end processing with 20 workers
func BenchmarkJobProcessing_20Workers(b *testing.B) {
	benchmarkJobProcessingWithWorkers(b, 20)
}

// benchmarkJobProcessingWithWorkers measures end-to-end job processing
func benchmarkJobProcessingWithWorkers(b *testing.B, numWorkers int) {
	s, q := setupBenchmarkRedis(b)
	defer s.Close()
	defer q.Close()

	c := setupBenchmarkClient(b, s.Addr())
	defer c.Close()

	// Create worker registry with a simple handler
	registry := worker.NewRegistry()
	var processedCount atomic.Int64

	registry.Register("benchmark_job", func(ctx context.Context, j *job.Job) error {
		// Simulate minimal work
		processedCount.Add(1)
		return nil
	})

	// Create and start worker pool
	executor := worker.NewExecutor(registry, q, numWorkers)
	pool := worker.NewPool(executor, q, numWorkers, 30*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go pool.Start(ctx)
	defer pool.Stop()

	// Track latencies (submission to completion)
	latencies := make([]time.Duration, 0, b.N)
	var mu sync.Mutex

	b.ResetTimer()
	start := time.Now()

	// Submit jobs
	jobIDs := make([]string, b.N)
	for i := 0; i < b.N; i++ {
		startOp := time.Now()
		jobID, err := c.SubmitJob(
			"benchmark_job",
			map[string]string{"index": fmt.Sprintf("%d", i)},
			job.PriorityNormal,
		)
		if err != nil {
			b.Fatalf("Failed to submit job: %v", err)
		}
		jobIDs[i] = jobID

		mu.Lock()
		latencies = append(latencies, time.Since(startOp))
		mu.Unlock()
	}

	// Wait for all jobs to complete
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			b.Fatalf("Timeout waiting for jobs to complete. Processed: %d/%d", processedCount.Load(), b.N)
		case <-ticker.C:
			if int(processedCount.Load()) >= b.N {
				goto done
			}
		}
	}

done:
	duration := time.Since(start)
	b.StopTimer()

	// Calculate metrics
	p50, p95, p99 := calculatePercentiles(latencies)
	opsPerSec := float64(b.N) / duration.Seconds()

	// Report results
	b.ReportMetric(opsPerSec, "jobs/sec")
	b.ReportMetric(float64(p50.Microseconds()), "p50-μs")
	b.ReportMetric(float64(p95.Microseconds()), "p95-μs")
	b.ReportMetric(float64(p99.Microseconds()), "p99-μs")
	b.ReportMetric(float64(numWorkers), "workers")
}

// =============================================================================
// BENCHMARK: Queue Operations
// =============================================================================

// BenchmarkQueueEnqueue tests enqueue operation performance
func BenchmarkQueueEnqueue(b *testing.B) {
	s, q := setupBenchmarkRedis(b)
	defer s.Close()
	defer q.Close()

	ctx := context.Background()
	payload, _ := json.Marshal(map[string]string{"test": "data"})

	latencies := make([]time.Duration, 0, b.N)
	var mu sync.Mutex

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		j := job.NewJob("test_job", payload, job.PriorityNormal)

		start := time.Now()
		err := q.Enqueue(ctx, j)
		latency := time.Since(start)

		if err != nil {
			b.Fatalf("Failed to enqueue: %v", err)
		}

		mu.Lock()
		latencies = append(latencies, latency)
		mu.Unlock()
	}

	b.StopTimer()

	// Calculate and report metrics
	p50, p95, p99 := calculatePercentiles(latencies)
	b.ReportMetric(float64(p50.Microseconds()), "p50-μs")
	b.ReportMetric(float64(p95.Microseconds()), "p95-μs")
	b.ReportMetric(float64(p99.Microseconds()), "p99-μs")
}

// BenchmarkQueueDequeue tests dequeue operation performance
func BenchmarkQueueDequeue(b *testing.B) {
	s, q := setupBenchmarkRedis(b)
	defer s.Close()
	defer q.Close()

	ctx := context.Background()
	payload, _ := json.Marshal(map[string]string{"test": "data"})

	// Pre-populate queue with jobs
	for i := 0; i < b.N; i++ {
		j := job.NewJob("test_job", payload, job.PriorityNormal)
		if err := q.Enqueue(ctx, j); err != nil {
			b.Fatalf("Failed to enqueue: %v", err)
		}
	}

	latencies := make([]time.Duration, 0, b.N)
	var mu sync.Mutex

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		start := time.Now()
		_, err := q.Dequeue(ctx, []job.JobPriority{
			job.PriorityHigh,
			job.PriorityNormal,
			job.PriorityLow,
		})
		latency := time.Since(start)

		if err != nil {
			b.Fatalf("Failed to dequeue: %v", err)
		}

		mu.Lock()
		latencies = append(latencies, latency)
		mu.Unlock()
	}

	b.StopTimer()

	// Calculate and report metrics
	p50, p95, p99 := calculatePercentiles(latencies)
	b.ReportMetric(float64(p50.Microseconds()), "p50-μs")
	b.ReportMetric(float64(p95.Microseconds()), "p95-μs")
	b.ReportMetric(float64(p99.Microseconds()), "p99-μs")
}

// BenchmarkQueueComplete tests complete operation performance
func BenchmarkQueueComplete(b *testing.B) {
	s, q := setupBenchmarkRedis(b)
	defer s.Close()
	defer q.Close()

	ctx := context.Background()
	payload, _ := json.Marshal(map[string]string{"test": "data"})

	// Pre-populate queue and dequeue jobs to get them in processing state
	jobIDs := make([]string, b.N)
	for i := 0; i < b.N; i++ {
		j := job.NewJob("test_job", payload, job.PriorityNormal)
		if err := q.Enqueue(ctx, j); err != nil {
			b.Fatalf("Failed to enqueue: %v", err)
		}

		dequeuedJob, err := q.Dequeue(ctx, []job.JobPriority{
			job.PriorityHigh,
			job.PriorityNormal,
			job.PriorityLow,
		})
		if err != nil {
			b.Fatalf("Failed to dequeue: %v", err)
		}
		jobIDs[i] = dequeuedJob.ID
	}

	latencies := make([]time.Duration, 0, b.N)
	var mu sync.Mutex

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		start := time.Now()
		err := q.Complete(ctx, jobIDs[i])
		latency := time.Since(start)

		if err != nil {
			b.Fatalf("Failed to complete: %v", err)
		}

		mu.Lock()
		latencies = append(latencies, latency)
		mu.Unlock()
	}

	b.StopTimer()

	// Calculate and report metrics
	p50, p95, p99 := calculatePercentiles(latencies)
	b.ReportMetric(float64(p50.Microseconds()), "p50-μs")
	b.ReportMetric(float64(p95.Microseconds()), "p95-μs")
	b.ReportMetric(float64(p99.Microseconds()), "p99-μs")
}

// BenchmarkQueueFail tests fail operation performance
func BenchmarkQueueFail(b *testing.B) {
	s, q := setupBenchmarkRedis(b)
	defer s.Close()
	defer q.Close()

	ctx := context.Background()
	payload, _ := json.Marshal(map[string]string{"test": "data"})

	// Pre-populate queue and dequeue jobs
	jobs := make([]*job.Job, b.N)
	for i := 0; i < b.N; i++ {
		j := job.NewJob("test_job", payload, job.PriorityNormal)
		if err := q.Enqueue(ctx, j); err != nil {
			b.Fatalf("Failed to enqueue: %v", err)
		}

		dequeuedJob, err := q.Dequeue(ctx, []job.JobPriority{
			job.PriorityHigh,
			job.PriorityNormal,
			job.PriorityLow,
		})
		if err != nil {
			b.Fatalf("Failed to dequeue: %v", err)
		}
		jobs[i] = dequeuedJob
	}

	latencies := make([]time.Duration, 0, b.N)
	var mu sync.Mutex

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		start := time.Now()
		err := q.Fail(ctx, jobs[i], "benchmark test error")
		latency := time.Since(start)

		if err != nil {
			b.Fatalf("Failed to fail job: %v", err)
		}

		mu.Lock()
		latencies = append(latencies, latency)
		mu.Unlock()
	}

	b.StopTimer()

	// Calculate and report metrics
	p50, p95, p99 := calculatePercentiles(latencies)
	b.ReportMetric(float64(p50.Microseconds()), "p50-μs")
	b.ReportMetric(float64(p95.Microseconds()), "p95-μs")
	b.ReportMetric(float64(p99.Microseconds()), "p99-μs")
}

// =============================================================================
// BENCHMARK: Queue Depth Impact
// =============================================================================

// BenchmarkQueueDepth_100Jobs tests performance with 100 jobs in queue
func BenchmarkQueueDepth_100Jobs(b *testing.B) {
	benchmarkQueueDepth(b, 100)
}

// BenchmarkQueueDepth_1000Jobs tests performance with 1000 jobs in queue
func BenchmarkQueueDepth_1000Jobs(b *testing.B) {
	benchmarkQueueDepth(b, 1000)
}

// BenchmarkQueueDepth_10000Jobs tests performance with 10000 jobs in queue
func BenchmarkQueueDepth_10000Jobs(b *testing.B) {
	benchmarkQueueDepth(b, 10000)
}

// benchmarkQueueDepth tests dequeue performance at different queue depths
func benchmarkQueueDepth(b *testing.B, queueDepth int) {
	s, q := setupBenchmarkRedis(b)
	defer s.Close()
	defer q.Close()

	ctx := context.Background()
	payload, _ := json.Marshal(map[string]string{"test": "data"})

	// Pre-populate queue with specified depth
	for i := 0; i < queueDepth; i++ {
		j := job.NewJob("test_job", payload, job.PriorityNormal)
		if err := q.Enqueue(ctx, j); err != nil {
			b.Fatalf("Failed to enqueue: %v", err)
		}
	}

	latencies := make([]time.Duration, 0, b.N)
	var mu sync.Mutex

	b.ResetTimer()

	for i := 0; i < b.N && i < queueDepth; i++ {
		start := time.Now()
		_, err := q.Dequeue(ctx, []job.JobPriority{
			job.PriorityHigh,
			job.PriorityNormal,
			job.PriorityLow,
		})
		latency := time.Since(start)

		if err != nil {
			b.Fatalf("Failed to dequeue: %v", err)
		}

		mu.Lock()
		latencies = append(latencies, latency)
		mu.Unlock()
	}

	b.StopTimer()

	// Calculate and report metrics
	p50, p95, p99 := calculatePercentiles(latencies)
	b.ReportMetric(float64(p50.Microseconds()), "p50-μs")
	b.ReportMetric(float64(p95.Microseconds()), "p95-μs")
	b.ReportMetric(float64(p99.Microseconds()), "p99-μs")
	b.ReportMetric(float64(queueDepth), "queue-depth")
}

// =============================================================================
// BENCHMARK: Concurrent Load
// =============================================================================

// BenchmarkConcurrentLoad_10Clients tests concurrent load with 10 clients
func BenchmarkConcurrentLoad_10Clients(b *testing.B) {
	benchmarkConcurrentLoad(b, 10)
}

// BenchmarkConcurrentLoad_50Clients tests concurrent load with 50 clients
func BenchmarkConcurrentLoad_50Clients(b *testing.B) {
	benchmarkConcurrentLoad(b, 50)
}

// BenchmarkConcurrentLoad_100Clients tests concurrent load with 100 clients
func BenchmarkConcurrentLoad_100Clients(b *testing.B) {
	benchmarkConcurrentLoad(b, 100)
}

// benchmarkConcurrentLoad tests system performance under concurrent client load
func benchmarkConcurrentLoad(b *testing.B, numClients int) {
	s, _ := setupBenchmarkRedis(b)
	defer s.Close()

	// Create multiple clients
	clients := make([]*client.Client, numClients)
	for i := 0; i < numClients; i++ {
		clients[i] = setupBenchmarkClient(b, s.Addr())
		defer clients[i].Close()
	}

	payload := generatePayload(1) // 1KB payload

	var totalOps atomic.Int64
	var wg sync.WaitGroup

	latencies := make([]time.Duration, 0, b.N)
	var mu sync.Mutex

	b.ResetTimer()
	start := time.Now()

	// Each client submits jobs concurrently
	jobsPerClient := b.N / numClients
	if jobsPerClient == 0 {
		jobsPerClient = 1
	}

	for clientIdx := 0; clientIdx < numClients; clientIdx++ {
		wg.Add(1)
		go func(c *client.Client) {
			defer wg.Done()

			for i := 0; i < jobsPerClient; i++ {
				startOp := time.Now()
				_, err := c.SubmitJob(
					"benchmark_job",
					payload,
					job.PriorityNormal,
				)
				latency := time.Since(startOp)

				if err != nil {
					b.Errorf("Failed to submit job: %v", err)
					continue
				}

				totalOps.Add(1)

				mu.Lock()
				latencies = append(latencies, latency)
				mu.Unlock()
			}
		}(clients[clientIdx])
	}

	wg.Wait()
	duration := time.Since(start)
	b.StopTimer()

	// Calculate metrics
	p50, p95, p99 := calculatePercentiles(latencies)
	opsPerSec := float64(totalOps.Load()) / duration.Seconds()

	// Report results
	b.ReportMetric(opsPerSec, "ops/sec")
	b.ReportMetric(float64(p50.Microseconds()), "p50-μs")
	b.ReportMetric(float64(p95.Microseconds()), "p95-μs")
	b.ReportMetric(float64(p99.Microseconds()), "p99-μs")
	b.ReportMetric(float64(numClients), "clients")
}

// =============================================================================
// HELPER: Generate Benchmark Report
// =============================================================================

// GenerateBenchmarkReport creates a markdown report from benchmark results
// This can be called manually after running benchmarks
func GenerateBenchmarkReport(results []BenchmarkResults, outputPath string) error {
	f, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create report file: %w", err)
	}
	defer f.Close()

	fmt.Fprintf(f, "# Bananas Performance Benchmark Report\n\n")
	fmt.Fprintf(f, "**Generated:** %s\n\n", time.Now().Format(time.RFC3339))

	// System information
	if len(results) > 0 {
		sysInfo := results[0].SystemInfo
		fmt.Fprintf(f, "## System Information\n\n")
		fmt.Fprintf(f, "- **Go Version:** %s\n", sysInfo.GoVersion)
		fmt.Fprintf(f, "- **OS/Arch:** %s/%s\n", sysInfo.OS, sysInfo.Arch)
		fmt.Fprintf(f, "- **CPUs:** %d\n", sysInfo.NumCPU)
		fmt.Fprintf(f, "- **GOMAXPROCS:** %d\n", sysInfo.GOMAXPROCS)
		fmt.Fprintf(f, "- **Redis:** %s\n\n", sysInfo.RedisVersion)
	}

	// Results table
	fmt.Fprintf(f, "## Benchmark Results\n\n")
	fmt.Fprintf(f, "| Test | Ops/Sec | Avg Latency | p50 | p95 | p99 | Config |\n")
	fmt.Fprintf(f, "|------|---------|-------------|-----|-----|-----|--------|\n")

	for _, r := range results {
		configStr := ""
		for k, v := range r.Configuration {
			configStr += fmt.Sprintf("%s=%v ", k, v)
		}

		fmt.Fprintf(f, "| %s | %.0f | %v | %v | %v | %v | %s |\n",
			r.TestName,
			r.OpsPerSecond,
			r.AvgLatency,
			r.P50Latency,
			r.P95Latency,
			r.P99Latency,
			configStr,
		)
	}

	fmt.Fprintf(f, "\n")
	return nil
}
