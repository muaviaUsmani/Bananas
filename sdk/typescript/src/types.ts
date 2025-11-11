/**
 * Type definitions for Bananas task queue.
 */

/**
 * Job status enumeration.
 */
export enum JobStatus {
  PENDING = 'pending',
  PROCESSING = 'processing',
  COMPLETED = 'completed',
  FAILED = 'failed',
  SCHEDULED = 'scheduled',
}

/**
 * Job priority enumeration.
 */
export enum JobPriority {
  HIGH = 'high',
  NORMAL = 'normal',
  LOW = 'low',
}

/**
 * Job interface representing a unit of work.
 */
export interface Job {
  /** Unique job identifier */
  id: string;
  /** Job name (identifies the handler) */
  name: string;
  /** Optional human-readable description */
  description?: string;
  /** Job-specific data */
  payload: Record<string, any>;
  /** Current job status */
  status: JobStatus;
  /** Job priority */
  priority: JobPriority;
  /** When the job was created */
  createdAt: Date;
  /** When the job was last updated */
  updatedAt: Date;
  /** When a scheduled job should execute (optional) */
  scheduledFor?: Date;
  /** Number of attempts */
  attempts: number;
  /** Maximum retry attempts */
  maxRetries: number;
  /** Error message (if failed) */
  error: string;
  /** Optional routing key for task routing */
  routingKey?: string;
}

/**
 * Job result interface.
 */
export interface JobResult {
  /** Job identifier */
  jobId: string;
  /** Final job status */
  status: JobStatus;
  /** Result data (for successful jobs) */
  result?: Record<string, any>;
  /** Error message (for failed jobs) */
  error?: string;
  /** When the job completed */
  completedAt: Date;
  /** Execution duration in milliseconds */
  durationMs: number;
}

/**
 * Client configuration options.
 */
export interface ClientOptions {
  /** Redis connection URL */
  redisUrl: string;
  /** TTL for successful results (milliseconds, default: 1 hour) */
  successTTL?: number;
  /** TTL for failed results (milliseconds, default: 24 hours) */
  failureTTL?: number;
  /** Key prefix for Redis keys (default: 'bananas') */
  keyPrefix?: string;
}

/**
 * Job submission options.
 */
export interface SubmitJobOptions {
  /** Job name */
  name: string;
  /** Job payload */
  payload: Record<string, any>;
  /** Job priority */
  priority: JobPriority;
  /** Optional description */
  description?: string;
  /** Optional routing key */
  routingKey?: string;
}

/**
 * Scheduled job submission options.
 */
export interface SubmitScheduledJobOptions extends SubmitJobOptions {
  /** When the job should execute */
  scheduledFor: Date;
}

/**
 * Submit and wait options.
 */
export interface SubmitAndWaitOptions extends SubmitJobOptions {
  /** Maximum time to wait in milliseconds (default: 5 minutes) */
  timeout?: number;
}
