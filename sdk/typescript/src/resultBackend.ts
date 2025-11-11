/**
 * Result backend for storing and retrieving job results.
 */

import Redis from "ioredis";
import { ConnectionError, ResultNotFoundError } from "./errors";
import { resultFromRedisHash, resultToRedisHash } from "./models";
import { JobResult } from "./types";

/**
 * Redis-backed result storage.
 */
export class RedisResultBackend {
  private redis: Redis;
  private pubsub: Redis;
  private keyPrefix: string;
  private successTTL: number;
  private failureTTL: number;

  /**
   * Create a new Redis result backend.
   *
   * @param redisUrl - Redis connection URL
   * @param successTTL - TTL for successful results in milliseconds (default: 1 hour)
   * @param failureTTL - TTL for failed results in milliseconds (default: 24 hours)
   * @param keyPrefix - Prefix for all Redis keys (default: 'bananas')
   */
  constructor(
    redisUrl: string,
    successTTL: number = 60 * 60 * 1000, // 1 hour
    failureTTL: number = 24 * 60 * 60 * 1000, // 24 hours
    keyPrefix: string = "bananas",
  ) {
    this.successTTL = successTTL;
    this.failureTTL = failureTTL;
    this.keyPrefix = keyPrefix;

    try {
      this.redis = new Redis(redisUrl);
      this.pubsub = new Redis(redisUrl); // Separate connection for pub/sub

      this.redis.on("error", (err) => {
        console.error("Redis connection error:", err);
      });

      this.pubsub.on("error", (err) => {
        console.error("Redis pubsub error:", err);
      });
    } catch (error) {
      throw new ConnectionError(`Failed to connect to Redis: ${error}`);
    }
  }

  /**
   * Store a job result in Redis.
   *
   * @param result - Job result to store
   */
  async setResult(result: JobResult): Promise<void> {
    try {
      const resultKey = `${this.keyPrefix}:result:${result.jobId}`;
      const hash = resultToRedisHash(result);

      // Determine TTL
      const ttlSeconds =
        result.status === "completed"
          ? Math.floor(this.successTTL / 1000)
          : Math.floor(this.failureTTL / 1000);

      // Store result with TTL
      const pipeline = this.redis.pipeline();
      pipeline.hset(resultKey, hash);
      pipeline.expire(resultKey, ttlSeconds);
      await pipeline.exec();

      // Publish notification
      const notifyChannel = `${this.keyPrefix}:result:notify:${result.jobId}`;
      await this.redis.publish(notifyChannel, result.status);
    } catch (error) {
      throw new ConnectionError(`Failed to set result: ${error}`);
    }
  }

  /**
   * Retrieve a job result by job ID.
   *
   * @param jobId - Job identifier
   * @returns Job result if found, null otherwise
   */
  async getResult(jobId: string): Promise<JobResult | null> {
    try {
      const resultKey = `${this.keyPrefix}:result:${jobId}`;
      const hash = await this.redis.hgetall(resultKey);

      if (!hash || Object.keys(hash).length === 0) {
        return null;
      }

      return resultFromRedisHash(jobId, hash);
    } catch (error) {
      if (error instanceof SyntaxError) {
        throw new ResultNotFoundError(`Failed to deserialize result: ${error}`);
      }
      throw new ConnectionError(`Failed to get result: ${error}`);
    }
  }

  /**
   * Wait for a job result with timeout.
   *
   * This method uses Redis pub/sub to wait for result notifications.
   *
   * @param jobId - Job identifier
   * @param timeoutMs - Maximum time to wait in milliseconds
   * @returns Job result if available within timeout, null otherwise
   */
  async waitForResult(
    jobId: string,
    timeoutMs: number,
  ): Promise<JobResult | null> {
    // First check if result already exists
    const existingResult = await this.getResult(jobId);
    if (existingResult) {
      return existingResult;
    }

    return new Promise((resolve) => {
      const notifyChannel = `${this.keyPrefix}:result:notify:${jobId}`;
      let resolved = false;

      // Set timeout
      const timeoutHandle = setTimeout(async () => {
        if (!resolved) {
          resolved = true;
          await this.pubsub.unsubscribe(notifyChannel);
          resolve(null);
        }
      }, timeoutMs);

      // Subscribe to notifications
      this.pubsub.subscribe(notifyChannel, async (err) => {
        if (err) {
          console.error("Subscription error:", err);
          if (!resolved) {
            resolved = true;
            clearTimeout(timeoutHandle);
            resolve(null);
          }
          return;
        }
      });

      // Handle messages
      this.pubsub.on("message", async (channel, message) => {
        if (channel === notifyChannel && !resolved) {
          resolved = true;
          clearTimeout(timeoutHandle);
          await this.pubsub.unsubscribe(notifyChannel);

          // Fetch the result
          const result = await this.getResult(jobId);
          resolve(result);
        }
      });

      // Periodically check for result (in case notification was missed)
      const pollInterval = setInterval(async () => {
        if (!resolved) {
          const result = await this.getResult(jobId);
          if (result) {
            resolved = true;
            clearTimeout(timeoutHandle);
            clearInterval(pollInterval);
            await this.pubsub.unsubscribe(notifyChannel);
            resolve(result);
          }
        } else {
          clearInterval(pollInterval);
        }
      }, 500); // Poll every 500ms
    });
  }

  /**
   * Close Redis connections.
   */
  async close(): Promise<void> {
    await this.redis.quit();
    await this.pubsub.quit();
  }
}
