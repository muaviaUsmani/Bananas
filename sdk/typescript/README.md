# Bananas TypeScript Client

TypeScript/Node.js client library for the Bananas distributed task queue system.

## Installation

```bash
npm install @bananas/client
```

Or with Yarn:

```bash
yarn add @bananas/client
```

## Quick Start

```typescript
import { Client, JobPriority } from '@bananas/client';

// Create client
const client = new Client({ redisUrl: 'redis://localhost:6379' });

// Submit a job
const jobId = await client.submitJob({
  name: 'sendEmail',
  payload: { to: 'user@example.com', subject: 'Hello' },
  priority: JobPriority.HIGH,
  description: 'Welcome email to new user'
});

console.log(`Job submitted: ${jobId}`);

// Get job status
const job = await client.getJob(jobId);
console.log(`Job status: ${job?.status}`);

// Get result (when available)
const result = await client.getResult(jobId);
if (result && result.status === 'completed') {
  console.log(`Result: ${JSON.stringify(result.result)}`);
}

// Or use submitAndWait for RPC-style execution
const result2 = await client.submitAndWait({
  name: 'generateReport',
  payload: { reportType: 'sales' },
  priority: JobPriority.NORMAL,
  timeout: 5 * 60 * 1000 // 5 minutes
});

if (result2 && result2.status === 'completed') {
  console.log(`Report: ${JSON.stringify(result2.result)}`);
}

await client.close();
```

## Scheduled Jobs

```typescript
import { Client, JobPriority } from '@bananas/client';

const client = new Client({ redisUrl: 'redis://localhost:6379' });

// Schedule job for 1 hour from now
const scheduledTime = new Date(Date.now() + 60 * 60 * 1000);
const jobId = await client.submitJobScheduled({
  name: 'sendReminder',
  payload: { userId: 123 },
  priority: JobPriority.NORMAL,
  scheduledFor: scheduledTime
});

await client.close();
```

## Task Routing

```typescript
import { Client, JobPriority } from '@bananas/client';

const client = new Client({ redisUrl: 'redis://localhost:6379' });

// Route job to GPU workers
const jobId = await client.submitJobWithRoute(
  'processVideo',
  { videoUrl: 'https://...' },
  JobPriority.HIGH,
  'gpu' // routing key
);

await client.close();
```

## API Reference

### Client

**`new Client(options: ClientOptions)`**

Create a new client instance.

Options:
- `redisUrl` (required): Redis connection URL
- `successTTL` (optional): TTL for successful results in ms (default: 1 hour)
- `failureTTL` (optional): TTL for failed results in ms (default: 24 hours)
- `keyPrefix` (optional): Prefix for Redis keys (default: 'bananas')

**`submitJob(options: SubmitJobOptions): Promise<string>`**

Submit a new job to the queue.

Options:
- `name`: Job name (identifies the handler)
- `payload`: Job data as object
- `priority`: JobPriority enum value (HIGH, NORMAL, LOW)
- `description` (optional): Human-readable description
- `routingKey` (optional): Routing key for task routing

Returns: Job ID (UUID string)

**`submitJobScheduled(options: SubmitScheduledJobOptions): Promise<string>`**

Submit a job scheduled for future execution.

Options: Same as `submitJob` plus:
- `scheduledFor`: Date when the job should execute

Returns: Job ID (UUID string)

**`submitJobWithRoute(...): Promise<string>`**

Convenience method for submitting jobs with routing keys.

Parameters:
- `name`: Job name
- `payload`: Job data
- `priority`: Job priority
- `routingKey`: Routing key
- `description` (optional): Description

Returns: Job ID (UUID string)

**`getJob(jobId: string): Promise<Job | null>`**

Retrieve a job by its ID.

Returns: Job object or null if not found

**`getResult(jobId: string): Promise<JobResult | null>`**

Retrieve the result of a completed job.

Returns: JobResult object or null if not available

**`submitAndWait(options: SubmitAndWaitOptions): Promise<JobResult | null>`**

Submit a job and wait for its result (RPC-style).

Options: Same as `submitJob` plus:
- `timeout` (optional): Maximum time to wait in ms (default: 5 minutes)

Returns: JobResult or null if timeout

**`close(): Promise<void>`**

Close all Redis connections.

### Enums

**`JobPriority`**

- `JobPriority.HIGH`: High priority
- `JobPriority.NORMAL`: Normal priority
- `JobPriority.LOW`: Low priority

**`JobStatus`**

- `JobStatus.PENDING`: Waiting to be processed
- `JobStatus.PROCESSING`: Currently being processed
- `JobStatus.COMPLETED`: Successfully completed
- `JobStatus.FAILED`: Failed (no more retries)
- `JobStatus.SCHEDULED`: Scheduled for future execution

## Development

### Building

```bash
npm run build
```

### Running Tests

```bash
# Run tests
npm test

# Run with coverage
npm run test:coverage
```

### Code Quality

```bash
# Lint
npm run lint

# Format
npm run format
```

## Requirements

- Node.js 16+
- Redis 5.0+
- TypeScript 5.0+ (for development)

## License

MIT
