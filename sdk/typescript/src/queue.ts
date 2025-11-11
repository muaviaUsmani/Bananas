/**
 * Redis queue operations for Bananas.
 */

import Redis from "ioredis";
import { ConnectionError, JobNotFoundError } from "./errors";
import { createJob, jobFromJSON, jobToJSON } from "./models";
import { Job } from "./types";

/**
 * Redis-backed job queue.
 */
export class RedisQueue {
  private redis: Redis;
  private keyPrefix: string;

  /**
   * Create a new Redis queue.
   *
   * @param redisUrl - Redis connection URL
   * @param keyPrefix - Prefix for all Redis keys (default: 'bananas')
   */
  constructor(redisUrl: string, keyPrefix: string = "bananas") {
    this.keyPrefix = keyPrefix;

    try {
      this.redis = new Redis(redisUrl, {
        retryStrategy: (times) => {
          if (times > 3) {
            return null; // Stop retrying
          }
          return Math.min(times * 50, 2000);
        },
      });

      this.redis.on("error", (err) => {
        console.error("Redis connection error:", err);
      });
    } catch (error) {
      throw new ConnectionError(`Failed to connect to Redis: ${error}`);
    }
  }

  /**
   * Enqueue a job to the appropriate priority queue.
   *
   * @param job - Job to enqueue
   */
  async enqueue(job: Job): Promise<void> {
    try {
      const jobKey = `${this.keyPrefix}:job:${job.id}`;
      const queueKey = `${this.keyPrefix}:queue:${job.priority}`;

      // Store job data
      await this.redis.set(jobKey, jobToJSON(job));

      // Add to priority queue
      await this.redis.lpush(queueKey, job.id);
    } catch (error) {
      throw new ConnectionError(`Failed to enqueue job: ${error}`);
    }
  }

  /**
   * Enqueue a job to the scheduled queue.
   *
   * @param job - Job to enqueue (must have scheduledFor set)
   */
  async enqueueScheduled(job: Job): Promise<void> {
    if (!job.scheduledFor) {
      throw new Error("Job must have scheduledFor set");
    }

    try {
      const jobKey = `${this.keyPrefix}:job:${job.id}`;
      const scheduledKey = `${this.keyPrefix}:queue:scheduled`;

      // Store job data
      await this.redis.set(jobKey, jobToJSON(job));

      // Add to scheduled set with timestamp as score
      const timestamp = job.scheduledFor.getTime() / 1000; // Convert to seconds
      await this.redis.zadd(scheduledKey, timestamp, job.id);
    } catch (error) {
      throw new ConnectionError(`Failed to enqueue scheduled job: ${error}`);
    }
  }

  /**
   * Get a job by its ID.
   *
   * @param jobId - Job identifier
   * @returns Job if found, null otherwise
   */
  async getJob(jobId: string): Promise<Job | null> {
    try {
      const jobKey = `${this.keyPrefix}:job:${jobId}`;
      const jobJson = await this.redis.get(jobKey);

      if (!jobJson) {
        return null;
      }

      return jobFromJSON(jobJson);
    } catch (error) {
      if (error instanceof SyntaxError) {
        throw new JobNotFoundError(`Failed to deserialize job: ${error}`);
      }
      throw new ConnectionError(`Failed to get job: ${error}`);
    }
  }

  /**
   * Close the Redis connection.
   */
  async close(): Promise<void> {
    await this.redis.quit();
  }
}
