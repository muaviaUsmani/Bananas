/**
 * Model classes and utilities for Bananas.
 */

import { v4 as uuidv4 } from "uuid";
import { Job, JobPriority, JobResult, JobStatus } from "./types";

/**
 * Create a new job.
 *
 * @param name - Job name
 * @param payload - Job data
 * @param priority - Job priority
 * @param options - Optional job options
 * @returns New job object
 */
export function createJob(
  name: string,
  payload: Record<string, any>,
  priority: JobPriority,
  options: {
    description?: string;
    routingKey?: string;
    scheduledFor?: Date;
    id?: string;
  } = {},
): Job {
  const now = new Date();

  return {
    id: options.id || uuidv4(),
    name,
    description: options.description,
    payload,
    status: options.scheduledFor ? JobStatus.SCHEDULED : JobStatus.PENDING,
    priority,
    createdAt: now,
    updatedAt: now,
    scheduledFor: options.scheduledFor,
    attempts: 0,
    maxRetries: 3,
    error: "",
    routingKey: options.routingKey,
  };
}

/**
 * Serialize a job to JSON string.
 *
 * @param job - Job to serialize
 * @returns JSON string
 */
export function jobToJSON(job: Job): string {
  const data: any = {
    id: job.id,
    name: job.name,
    description: job.description || "",
    payload: job.payload,
    status: job.status,
    priority: job.priority,
    created_at: job.createdAt.toISOString(),
    updated_at: job.updatedAt.toISOString(),
    attempts: job.attempts,
    max_retries: job.maxRetries,
    error: job.error || "",
    routing_key: job.routingKey || "",
  };

  if (job.scheduledFor) {
    data.scheduled_for = job.scheduledFor.toISOString();
  }

  return JSON.stringify(data);
}

/**
 * Deserialize a job from JSON string.
 *
 * @param json - JSON string
 * @returns Job object
 */
export function jobFromJSON(json: string): Job {
  const data = JSON.parse(json);

  return {
    id: data.id,
    name: data.name,
    description: data.description,
    payload: data.payload,
    status: data.status as JobStatus,
    priority: data.priority as JobPriority,
    createdAt: new Date(data.created_at),
    updatedAt: new Date(data.updated_at),
    scheduledFor: data.scheduled_for ? new Date(data.scheduled_for) : undefined,
    attempts: data.attempts || 0,
    maxRetries: data.max_retries || 3,
    error: data.error || "",
    routingKey: data.routing_key,
  };
}

/**
 * Serialize a job result to Redis hash format.
 *
 * @param result - Job result
 * @returns Redis hash object
 */
export function resultToRedisHash(result: JobResult): Record<string, string> {
  const hash: Record<string, string> = {
    status: result.status,
    completed_at: result.completedAt.toISOString(),
    duration_ms: result.durationMs.toString(),
  };

  if (result.result) {
    hash.result = JSON.stringify(result.result);
  }

  if (result.error) {
    hash.error = result.error;
  }

  return hash;
}

/**
 * Deserialize a job result from Redis hash.
 *
 * @param jobId - Job identifier
 * @param hash - Redis hash data
 * @returns Job result
 */
export function resultFromRedisHash(
  jobId: string,
  hash: Record<string, string>,
): JobResult {
  let result: Record<string, any> | undefined;
  if (hash.result) {
    try {
      result = JSON.parse(hash.result);
    } catch {
      // If parsing fails, keep as string
      result = { value: hash.result };
    }
  }

  return {
    jobId,
    status: hash.status as JobStatus,
    result,
    error: hash.error,
    completedAt: new Date(hash.completed_at),
    durationMs: parseInt(hash.duration_ms || "0", 10),
  };
}

/**
 * Check if a job result indicates success.
 *
 * @param result - Job result
 * @returns True if successful
 */
export function isResultSuccess(result: JobResult): boolean {
  return result.status === JobStatus.COMPLETED;
}

/**
 * Check if a job result indicates failure.
 *
 * @param result - Job result
 * @returns True if failed
 */
export function isResultFailed(result: JobResult): boolean {
  return result.status === JobStatus.FAILED;
}
