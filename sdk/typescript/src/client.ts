/**
 * Bananas client for job submission and management.
 */

import { ConnectionError } from "./errors";
import { createJob } from "./models";
import { RedisQueue } from "./queue";
import { RedisResultBackend } from "./resultBackend";
import {
  ClientOptions,
  Job,
  JobPriority,
  JobResult,
  SubmitAndWaitOptions,
  SubmitJobOptions,
  SubmitScheduledJobOptions,
} from "./types";

/**
 * Bananas job queue client.
 *
 * Main entry point for submitting and managing jobs.
 *
 * @example
 * ```typescript
 * const client = new Client({ redisUrl: 'redis://localhost:6379' });
 * const jobId = await client.submitJob({
 *   name: 'sendEmail',
 *   payload: { to: 'user@example.com' },
 *   priority: JobPriority.HIGH
 * });
 * await client.close();
 * ```
 */
export class Client {
  private queue: RedisQueue;
  private resultBackend: RedisResultBackend;

  /**
   * Create a new Bananas client.
   *
   * @param options - Client configuration options
   */
  constructor(options: ClientOptions) {
    const {
      redisUrl,
      successTTL = 60 * 60 * 1000, // 1 hour
      failureTTL = 24 * 60 * 60 * 1000, // 24 hours
      keyPrefix = "bananas",
    } = options;

    try {
      this.queue = new RedisQueue(redisUrl, keyPrefix);
      this.resultBackend = new RedisResultBackend(
        redisUrl,
        successTTL,
        failureTTL,
        keyPrefix,
      );
    } catch (error) {
      throw new ConnectionError(`Failed to initialize client: ${error}`);
    }
  }

  /**
   * Submit a new job to the queue.
   *
   * @param options - Job submission options
   * @returns Job ID (UUID)
   *
   * @example
   * ```typescript
   * const jobId = await client.submitJob({
   *   name: 'processData',
   *   payload: { data: [1, 2, 3] },
   *   priority: JobPriority.NORMAL,
   *   description: 'Process user data'
   * });
   * ```
   */
  async submitJob(options: SubmitJobOptions): Promise<string> {
    const job = createJob(options.name, options.payload, options.priority, {
      description: options.description,
      routingKey: options.routingKey,
    });

    await this.queue.enqueue(job);
    return job.id;
  }

  /**
   * Submit a job with a routing key.
   *
   * Alias for submitJob with routingKey parameter.
   *
   * @param name - Job name
   * @param payload - Job data
   * @param priority - Job priority
   * @param routingKey - Routing key for directed task routing
   * @param description - Optional description
   * @returns Job ID (UUID)
   *
   * @example
   * ```typescript
   * const jobId = await client.submitJobWithRoute(
   *   'processVideo',
   *   { videoUrl: 'https://...' },
   *   JobPriority.HIGH,
   *   'gpu' // Route to GPU workers
   * );
   * ```
   */
  async submitJobWithRoute(
    name: string,
    payload: Record<string, any>,
    priority: JobPriority,
    routingKey: string,
    description?: string,
  ): Promise<string> {
    return this.submitJob({
      name,
      payload,
      priority,
      routingKey,
      description,
    });
  }

  /**
   * Submit a job scheduled for future execution.
   *
   * @param options - Scheduled job submission options
   * @returns Job ID (UUID)
   *
   * @example
   * ```typescript
   * const scheduledTime = new Date(Date.now() + 60 * 60 * 1000); // 1 hour from now
   * const jobId = await client.submitJobScheduled({
   *   name: 'sendReminder',
   *   payload: { userId: 123 },
   *   priority: JobPriority.NORMAL,
   *   scheduledFor: scheduledTime
   * });
   * ```
   */
  async submitJobScheduled(
    options: SubmitScheduledJobOptions,
  ): Promise<string> {
    const job = createJob(options.name, options.payload, options.priority, {
      description: options.description,
      routingKey: options.routingKey,
      scheduledFor: options.scheduledFor,
    });

    await this.queue.enqueueScheduled(job);
    return job.id;
  }

  /**
   * Get a job by its ID.
   *
   * @param jobId - Job identifier
   * @returns Job if found, null otherwise
   *
   * @example
   * ```typescript
   * const job = await client.getJob(jobId);
   * if (job) {
   *   console.log(`Job status: ${job.status}`);
   * }
   * ```
   */
  async getJob(jobId: string): Promise<Job | null> {
    return this.queue.getJob(jobId);
  }

  /**
   * Get the result of a completed job.
   *
   * @param jobId - Job identifier
   * @returns Job result if found, null otherwise
   *
   * @example
   * ```typescript
   * const result = await client.getResult(jobId);
   * if (result && result.status === 'completed') {
   *   console.log(`Result: ${JSON.stringify(result.result)}`);
   * }
   * ```
   */
  async getResult(jobId: string): Promise<JobResult | null> {
    return this.resultBackend.getResult(jobId);
  }

  /**
   * Submit a job and wait for its result (RPC-style execution).
   *
   * This is a convenience method for synchronous job execution.
   * It submits the job and waits until it completes or times out.
   *
   * @param options - Submit and wait options
   * @returns Job result if job completes within timeout, null otherwise
   *
   * @example
   * ```typescript
   * const result = await client.submitAndWait({
   *   name: 'generateReport',
   *   payload: { reportType: 'sales' },
   *   priority: JobPriority.HIGH,
   *   timeout: 5 * 60 * 1000 // 5 minutes
   * });
   *
   * if (result && result.status === 'completed') {
   *   console.log(`Report: ${JSON.stringify(result.result)}`);
   * }
   * ```
   */
  async submitAndWait(
    options: SubmitAndWaitOptions,
  ): Promise<JobResult | null> {
    const timeout = options.timeout || 5 * 60 * 1000; // Default: 5 minutes

    const job = createJob(options.name, options.payload, options.priority, {
      description: options.description,
      routingKey: options.routingKey,
    });

    await this.queue.enqueue(job);
    return this.resultBackend.waitForResult(job.id, timeout);
  }

  /**
   * Close all Redis connections.
   *
   * Should be called when done using the client.
   */
  async close(): Promise<void> {
    await this.queue.close();
    await this.resultBackend.close();
  }
}
