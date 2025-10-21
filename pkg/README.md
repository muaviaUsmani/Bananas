# Public Packages (`pkg/`)

This directory contains public-facing packages that can be imported and used by external applications. These packages provide the client SDK for interacting with the Bananas task queue system.

## Packages

### 📚 Client SDK (`client/`)

**Purpose**: Provides a simple, clean API for submitting and managing jobs. Currently implements an in-memory store for development/testing, with production Redis integration planned.

**Installation**:
```go
import "github.com/muaviaUsmani/bananas/pkg/client"
```

**Usage Example**:
```go
package main

import (
    "fmt"
    "log"
    
    "github.com/muaviaUsmani/bananas/pkg/client"
    "github.com/muaviaUsmani/bananas/internal/job"
)

func main() {
    // Create a new client
    c := client.NewClient()
    
    // Submit a high-priority job
    payload := map[string]interface{}{
        "to":      "user@example.com",
        "subject": "Welcome!",
        "body":    "Thanks for signing up.",
    }
    
    jobID, err := c.SubmitJob(
        "send_email",           // job name
        payload,                // payload (will be JSON marshaled)
        job.PriorityHigh,       // priority
        "Welcome email",        // optional description
    )
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Job submitted: %s\n", jobID)
    
    // Retrieve job status
    j, err := c.GetJob(jobID)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Job status: %s\n", j.Status)
    
    // List all jobs
    allJobs := c.ListJobs()
    fmt.Printf("Total jobs: %d\n", len(allJobs))
}
```

---

## Client API Reference

### Types

**Client**: Main client struct
```go
type Client struct {
    jobs map[string]*job.Job  // In-memory job storage
    mu   sync.RWMutex          // Thread-safe access
}
```

### Functions

#### `NewClient() *Client`

Creates and initializes a new client instance.

**Returns**: Initialized client ready for use

**Example**:
```go
c := client.NewClient()
```

---

#### `SubmitJob(name string, payload interface{}, priority job.JobPriority, description ...string) (string, error)`

Submits a new job to the queue.

**Parameters**:
- `name` (string): Job type name (must match registered handler)
- `payload` (interface{}): Any JSON-serializable data
- `priority` (job.JobPriority): One of `PriorityHigh`, `PriorityNormal`, `PriorityLow`
- `description` (...string): Optional human-readable description

**Returns**:
- `string`: Job ID (UUID)
- `error`: Error if payload marshaling fails

**Examples**:
```go
// Simple job without description
jobID, err := c.SubmitJob(
    "process_data",
    map[string]int{"count": 100},
    job.PriorityNormal,
)

// Job with description
jobID, err := c.SubmitJob(
    "send_email",
    EmailPayload{To: "user@example.com"},
    job.PriorityHigh,
    "Welcome email for new user",
)
```

---

#### `GetJob(jobID string) (*job.Job, error)`

Retrieves a job by its ID.

**Parameters**:
- `jobID` (string): The UUID of the job to retrieve

**Returns**:
- `*job.Job`: The job object with full details
- `error`: Error if job not found

**Example**:
```go
j, err := c.GetJob("550e8400-e29b-41d4-a716-446655440000")
if err != nil {
    log.Printf("Job not found: %v", err)
    return
}

fmt.Printf("Status: %s, Attempts: %d\n", j.Status, j.Attempts)
```

---

#### `ListJobs() []*job.Job`

Returns all jobs currently stored in the client.

**Returns**:
- `[]*job.Job`: Slice of all jobs (order not guaranteed)

**Example**:
```go
allJobs := c.ListJobs()
for _, j := range allJobs {
    fmt.Printf("Job %s: %s (priority: %s)\n", 
        j.ID, j.Name, j.Priority)
}
```

---

## Tests (`client_test.go`)

**8 tests covering all client functionality**:

1. ✅ **TestNewClient**
   - Tests client initialization
   - Verifies jobs map is created
   - Checks client is not nil

2. ✅ **TestSubmitJob_CreatesJobCorrectly**
   - Submits a job with all parameters
   - Verifies job has correct name, priority, and description
   - Checks job status is `pending`
   - Validates job is stored in client

3. ✅ **TestSubmitJob_ReturnsValidUUID**
   - Submits a job
   - Verifies returned ID is a valid UUID format
   - Checks ID is not empty

4. ✅ **TestSubmitJob_MarshalsPayloadCorrectly**
   - Submits job with complex payload
   - Retrieves the job
   - Unmarshals payload and verifies all fields
   - Checks JSON marshaling worked correctly

5. ✅ **TestGetJob_RetrievesSubmittedJob**
   - Submits a job
   - Retrieves it by ID
   - Verifies all fields match
   - Checks timestamps are preserved

6. ✅ **TestGetJob_ReturnsErrorForNonExistent**
   - Attempts to get job with fake UUID
   - Verifies returns appropriate error
   - Checks error message contains job ID

7. ✅ **TestListJobs_ReturnsAllSubmittedJobs**
   - Submits 3 jobs with different priorities
   - Calls `ListJobs()`
   - Verifies all 3 jobs are returned
   - Checks each job has correct properties

8. ✅ **TestSubmitJob_ThreadSafety**
   - Spawns 100 goroutines
   - Each submits a job concurrently
   - Waits for all to complete
   - Verifies exactly 100 jobs stored
   - Checks no race conditions or lost updates
   - Tests `sync.RWMutex` implementation

**Run tests**:
```bash
make test
# or specifically
go test ./pkg/client/
```

---

## Current Implementation vs. Roadmap

### Current Implementation ✅
- In-memory storage (suitable for development/testing)
- Thread-safe operations with `sync.RWMutex`
- Full job lifecycle support
- Clean, intuitive API

### Planned Features 🚧
1. **Redis Integration**
   - Replace in-memory store with direct Redis calls
   - Use `internal/queue` package for job submission
   - Real-time job status updates

2. **HTTP Client**
   - REST API client for remote job submission
   - Communicate with API server instead of direct Redis
   - Authentication token support

3. **Batch Operations**
   - `SubmitBulk()` - Submit multiple jobs in one call
   - `GetBulk()` - Retrieve multiple jobs by IDs
   - Efficient bulk status queries

4. **Job Filtering**
   - Filter jobs by status
   - Filter by priority
   - Time-range queries

5. **Job Cancellation**
   - `CancelJob(jobID)` - Cancel pending jobs
   - `CancelAll(name)` - Cancel all jobs of a type

6. **Webhooks/Callbacks**
   - Register callback URLs for job completion
   - Streaming job status updates

---

## Design Philosophy

The client SDK follows these principles:

1. **Simple by Default**: Common operations should be one-liners
2. **Type-Safe**: Use strong typing where possible (enums for priority/status)
3. **Flexible Payload**: Accept any JSON-serializable data
4. **Thread-Safe**: Safe for concurrent use without additional locking
5. **Error-Friendly**: Clear error messages with actionable information
6. **Minimal Dependencies**: Only depends on internal job types

---

## Examples

### Basic Job Submission
```go
c := client.NewClient()
jobID, _ := c.SubmitJob("send_email", emailData, job.PriorityHigh)
```

### With Description
```go
jobID, _ := c.SubmitJob(
    "process_video",
    VideoData{URL: "...", Quality: "1080p"},
    job.PriorityNormal,
    "Transcode uploaded video to multiple resolutions",
)
```

### Checking Job Status
```go
j, err := c.GetJob(jobID)
if err != nil {
    // Handle not found
}

switch j.Status {
case job.StatusPending:
    fmt.Println("Waiting in queue...")
case job.StatusProcessing:
    fmt.Println("Currently running...")
case job.StatusCompleted:
    fmt.Println("Done!")
case job.StatusFailed:
    fmt.Printf("Failed: %s\n", j.Error)
}
```

### Bulk Status Check
```go
allJobs := c.ListJobs()

pending := 0
completed := 0
failed := 0

for _, j := range allJobs {
    switch j.Status {
    case job.StatusPending:
        pending++
    case job.StatusCompleted:
        completed++
    case job.StatusFailed:
        failed++
    }
}

fmt.Printf("Pending: %d, Completed: %d, Failed: %d\n", 
    pending, completed, failed)
```

---

## Performance Considerations

**Current In-Memory Implementation**:
- ✅ Very fast (microsecond operations)
- ✅ No network overhead
- ❌ Not persistent (lost on restart)
- ❌ Not shared across processes
- ❌ Memory grows with number of jobs

**Future Redis Implementation**:
- ✅ Persistent storage
- ✅ Shared across all services
- ✅ Bounded memory (can set TTL on completed jobs)
- ⚠️ Network latency (milliseconds)
- ✅ Can scale independently

---

## Testing

The client package has **8 comprehensive tests** covering:
- Initialization
- Job submission and retrieval
- Error handling
- Thread-safety
- Payload marshaling

All tests pass with 100% coverage of critical paths.

```bash
# Run client tests
make test

# Run with coverage
go test -cover ./pkg/client/
```

---

## Contributing

When adding new client features:

1. **Update the interface** - Add method signatures
2. **Implement for in-memory** - Update current implementation
3. **Add tests** - Ensure behavior is tested
4. **Update docs** - Document new methods here
5. **Plan for Redis** - Consider how it will work with real backend

Keep the API simple and consistent with existing patterns.

