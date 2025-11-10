package metrics

import (
	"testing"
	"time"

	"github.com/muaviaUsmani/bananas/internal/job"
)

func TestNewCollector(t *testing.T) {
	c := NewCollector()
	if c == nil {
		t.Fatal("NewCollector returned nil")
	}

	metrics := c.GetMetrics()
	if metrics.TotalJobsProcessed != 0 {
		t.Errorf("Expected TotalJobsProcessed = 0, got %d", metrics.TotalJobsProcessed)
	}
	if metrics.TotalJobsCompleted != 0 {
		t.Errorf("Expected TotalJobsCompleted = 0, got %d", metrics.TotalJobsCompleted)
	}
	if metrics.TotalJobsFailed != 0 {
		t.Errorf("Expected TotalJobsFailed = 0, got %d", metrics.TotalJobsFailed)
	}
}

func TestRecordJobStarted(t *testing.T) {
	c := NewCollector()

	c.RecordJobStarted(job.JobPriorityHigh)
	c.RecordJobStarted(job.JobPriorityNormal)
	c.RecordJobStarted(job.JobPriorityHigh)

	metrics := c.GetMetrics()
	if metrics.TotalJobsProcessed != 3 {
		t.Errorf("Expected TotalJobsProcessed = 3, got %d", metrics.TotalJobsProcessed)
	}
	if metrics.JobsByPriority[job.JobPriorityHigh] != 2 {
		t.Errorf("Expected High priority count = 2, got %d", metrics.JobsByPriority[job.JobPriorityHigh])
	}
	if metrics.JobsByPriority[job.JobPriorityNormal] != 1 {
		t.Errorf("Expected Normal priority count = 1, got %d", metrics.JobsByPriority[job.JobPriorityNormal])
	}
	if metrics.JobsByStatus[job.JobStatusProcessing] != 3 {
		t.Errorf("Expected Processing status count = 3, got %d", metrics.JobsByStatus[job.JobStatusProcessing])
	}
}

func TestRecordJobCompleted(t *testing.T) {
	c := NewCollector()

	c.RecordJobStarted(job.JobPriorityHigh)
	c.RecordJobCompleted(job.JobPriorityHigh, 100*time.Millisecond)

	c.RecordJobStarted(job.JobPriorityNormal)
	c.RecordJobCompleted(job.JobPriorityNormal, 200*time.Millisecond)

	metrics := c.GetMetrics()
	if metrics.TotalJobsCompleted != 2 {
		t.Errorf("Expected TotalJobsCompleted = 2, got %d", metrics.TotalJobsCompleted)
	}
	if metrics.JobsByStatus[job.JobStatusCompleted] != 2 {
		t.Errorf("Expected Completed status count = 2, got %d", metrics.JobsByStatus[job.JobStatusCompleted])
	}
	if metrics.JobsByStatus[job.JobStatusProcessing] != 0 {
		t.Errorf("Expected Processing status count = 0, got %d", metrics.JobsByStatus[job.JobStatusProcessing])
	}

	// Average duration should be 150ms
	expectedAvg := 150 * time.Millisecond
	if metrics.AvgJobDuration != expectedAvg {
		t.Errorf("Expected AvgJobDuration = %v, got %v", expectedAvg, metrics.AvgJobDuration)
	}
}

func TestRecordJobFailed(t *testing.T) {
	c := NewCollector()

	c.RecordJobStarted(job.JobPriorityHigh)
	c.RecordJobFailed(job.JobPriorityHigh, 50*time.Millisecond)

	metrics := c.GetMetrics()
	if metrics.TotalJobsFailed != 1 {
		t.Errorf("Expected TotalJobsFailed = 1, got %d", metrics.TotalJobsFailed)
	}
	if metrics.JobsByStatus[job.JobStatusFailed] != 1 {
		t.Errorf("Expected Failed status count = 1, got %d", metrics.JobsByStatus[job.JobStatusFailed])
	}
	if metrics.JobsByStatus[job.JobStatusProcessing] != 0 {
		t.Errorf("Expected Processing status count = 0, got %d", metrics.JobsByStatus[job.JobStatusProcessing])
	}

	// Error rate should be 100% (1 failure out of 1 operation)
	if metrics.ErrorRate != 100.0 {
		t.Errorf("Expected ErrorRate = 100.0, got %f", metrics.ErrorRate)
	}
}

func TestMixedJobOutcomes(t *testing.T) {
	c := NewCollector()

	// 3 completed, 1 failed
	c.RecordJobStarted(job.JobPriorityHigh)
	c.RecordJobCompleted(job.JobPriorityHigh, 100*time.Millisecond)

	c.RecordJobStarted(job.JobPriorityNormal)
	c.RecordJobCompleted(job.JobPriorityNormal, 200*time.Millisecond)

	c.RecordJobStarted(job.JobPriorityLow)
	c.RecordJobCompleted(job.JobPriorityLow, 150*time.Millisecond)

	c.RecordJobStarted(job.JobPriorityHigh)
	c.RecordJobFailed(job.JobPriorityHigh, 50*time.Millisecond)

	metrics := c.GetMetrics()
	if metrics.TotalJobsProcessed != 4 {
		t.Errorf("Expected TotalJobsProcessed = 4, got %d", metrics.TotalJobsProcessed)
	}
	if metrics.TotalJobsCompleted != 3 {
		t.Errorf("Expected TotalJobsCompleted = 3, got %d", metrics.TotalJobsCompleted)
	}
	if metrics.TotalJobsFailed != 1 {
		t.Errorf("Expected TotalJobsFailed = 1, got %d", metrics.TotalJobsFailed)
	}

	// Error rate should be 25% (1 failure out of 4 operations)
	if metrics.ErrorRate != 25.0 {
		t.Errorf("Expected ErrorRate = 25.0, got %f", metrics.ErrorRate)
	}

	// Average duration should be 125ms (500ms total / 4 operations)
	expectedAvg := 125 * time.Millisecond
	if metrics.AvgJobDuration != expectedAvg {
		t.Errorf("Expected AvgJobDuration = %v, got %v", expectedAvg, metrics.AvgJobDuration)
	}
}

func TestRecordQueueDepth(t *testing.T) {
	c := NewCollector()

	c.RecordQueueDepth(job.JobPriorityHigh, 10)
	c.RecordQueueDepth(job.JobPriorityNormal, 25)
	c.RecordQueueDepth(job.JobPriorityLow, 5)

	metrics := c.GetMetrics()
	if metrics.QueueDepths[job.JobPriorityHigh] != 10 {
		t.Errorf("Expected High priority depth = 10, got %d", metrics.QueueDepths[job.JobPriorityHigh])
	}
	if metrics.QueueDepths[job.JobPriorityNormal] != 25 {
		t.Errorf("Expected Normal priority depth = 25, got %d", metrics.QueueDepths[job.JobPriorityNormal])
	}
	if metrics.QueueDepths[job.JobPriorityLow] != 5 {
		t.Errorf("Expected Low priority depth = 5, got %d", metrics.QueueDepths[job.JobPriorityLow])
	}
}

func TestRecordWorkerActivity(t *testing.T) {
	c := NewCollector()

	c.RecordWorkerActivity(5, 10)

	metrics := c.GetMetrics()
	if metrics.WorkerUtilization != 50.0 {
		t.Errorf("Expected WorkerUtilization = 50.0, got %f", metrics.WorkerUtilization)
	}

	c.RecordWorkerActivity(10, 10)
	metrics = c.GetMetrics()
	if metrics.WorkerUtilization != 100.0 {
		t.Errorf("Expected WorkerUtilization = 100.0, got %f", metrics.WorkerUtilization)
	}

	c.RecordWorkerActivity(0, 10)
	metrics = c.GetMetrics()
	if metrics.WorkerUtilization != 0.0 {
		t.Errorf("Expected WorkerUtilization = 0.0, got %f", metrics.WorkerUtilization)
	}
}

func TestReset(t *testing.T) {
	c := NewCollector()

	// Add some data
	c.RecordJobStarted(job.JobPriorityHigh)
	c.RecordJobCompleted(job.JobPriorityHigh, 100*time.Millisecond)
	c.RecordQueueDepth(job.JobPriorityHigh, 10)
	c.RecordWorkerActivity(5, 10)

	// Verify data exists
	metrics := c.GetMetrics()
	if metrics.TotalJobsProcessed == 0 {
		t.Error("Expected non-zero metrics before reset")
	}

	// Reset
	c.Reset()

	// Verify all metrics are cleared
	metrics = c.GetMetrics()
	if metrics.TotalJobsProcessed != 0 {
		t.Errorf("Expected TotalJobsProcessed = 0 after reset, got %d", metrics.TotalJobsProcessed)
	}
	if metrics.TotalJobsCompleted != 0 {
		t.Errorf("Expected TotalJobsCompleted = 0 after reset, got %d", metrics.TotalJobsCompleted)
	}
	if metrics.TotalJobsFailed != 0 {
		t.Errorf("Expected TotalJobsFailed = 0 after reset, got %d", metrics.TotalJobsFailed)
	}
	if len(metrics.JobsByStatus) != 0 {
		t.Errorf("Expected empty JobsByStatus after reset, got %d entries", len(metrics.JobsByStatus))
	}
	if len(metrics.JobsByPriority) != 0 {
		t.Errorf("Expected empty JobsByPriority after reset, got %d entries", len(metrics.JobsByPriority))
	}
	if len(metrics.QueueDepths) != 0 {
		t.Errorf("Expected empty QueueDepths after reset, got %d entries", len(metrics.QueueDepths))
	}
	if metrics.AvgJobDuration != 0 {
		t.Errorf("Expected AvgJobDuration = 0 after reset, got %v", metrics.AvgJobDuration)
	}
	if metrics.WorkerUtilization != 0 {
		t.Errorf("Expected WorkerUtilization = 0 after reset, got %f", metrics.WorkerUtilization)
	}
	if metrics.ErrorRate != 0 {
		t.Errorf("Expected ErrorRate = 0 after reset, got %f", metrics.ErrorRate)
	}
}

func TestUptime(t *testing.T) {
	c := NewCollector()

	// Sleep briefly
	time.Sleep(10 * time.Millisecond)

	metrics := c.GetMetrics()
	if metrics.Uptime < 10*time.Millisecond {
		t.Errorf("Expected Uptime >= 10ms, got %v", metrics.Uptime)
	}
	if metrics.Uptime > 1*time.Second {
		t.Errorf("Expected Uptime < 1s, got %v", metrics.Uptime)
	}
}

func TestGlobalCollector(t *testing.T) {
	// Reset global collector
	ResetMetrics()

	// Record some metrics using global functions
	Default().RecordJobStarted(job.JobPriorityHigh)
	Default().RecordJobCompleted(job.JobPriorityHigh, 100*time.Millisecond)

	metrics := GetMetrics()
	if metrics.TotalJobsProcessed != 1 {
		t.Errorf("Expected TotalJobsProcessed = 1, got %d", metrics.TotalJobsProcessed)
	}
	if metrics.TotalJobsCompleted != 1 {
		t.Errorf("Expected TotalJobsCompleted = 1, got %d", metrics.TotalJobsCompleted)
	}

	// Reset and verify
	ResetMetrics()
	metrics = GetMetrics()
	if metrics.TotalJobsProcessed != 0 {
		t.Errorf("Expected TotalJobsProcessed = 0 after reset, got %d", metrics.TotalJobsProcessed)
	}
}

func TestConcurrentAccess(t *testing.T) {
	c := NewCollector()
	done := make(chan bool)

	// Simulate concurrent job processing
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				c.RecordJobStarted(job.JobPriorityNormal)
				c.RecordJobCompleted(job.JobPriorityNormal, 1*time.Millisecond)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	metrics := c.GetMetrics()
	expected := int64(1000) // 10 goroutines * 100 jobs each
	if metrics.TotalJobsProcessed != expected {
		t.Errorf("Expected TotalJobsProcessed = %d, got %d", expected, metrics.TotalJobsProcessed)
	}
	if metrics.TotalJobsCompleted != expected {
		t.Errorf("Expected TotalJobsCompleted = %d, got %d", expected, metrics.TotalJobsCompleted)
	}
}

// Benchmarks

func BenchmarkRecordJobStarted(b *testing.B) {
	c := NewCollector()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.RecordJobStarted(job.JobPriorityHigh)
	}
}

func BenchmarkRecordJobCompleted(b *testing.B) {
	c := NewCollector()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.RecordJobCompleted(job.JobPriorityHigh, 1*time.Millisecond)
	}
}

func BenchmarkGetMetrics(b *testing.B) {
	c := NewCollector()
	// Add some data
	for i := 0; i < 1000; i++ {
		c.RecordJobStarted(job.JobPriorityHigh)
		c.RecordJobCompleted(job.JobPriorityHigh, 1*time.Millisecond)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.GetMetrics()
	}
}

func BenchmarkConcurrentRecording(b *testing.B) {
	c := NewCollector()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.RecordJobStarted(job.JobPriorityNormal)
			c.RecordJobCompleted(job.JobPriorityNormal, 1*time.Millisecond)
		}
	})
}
