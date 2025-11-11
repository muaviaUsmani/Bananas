/**
 * Bananas - Distributed Task Queue Client Library for TypeScript/Node.js
 *
 * @packageDocumentation
 *
 * @example
 * ```typescript
 * import { Client, JobPriority } from '@bananas/client';
 *
 * const client = new Client({ redisUrl: 'redis://localhost:6379' });
 *
 * // Submit a job
 * const jobId = await client.submitJob({
 *   name: 'sendEmail',
 *   payload: { to: 'user@example.com', subject: 'Hello' },
 *   priority: JobPriority.HIGH
 * });
 *
 * // Get result
 * const result = await client.getResult(jobId);
 * if (result && result.status === 'completed') {
 *   console.log(result.result);
 * }
 *
 * // Or use submit and wait for RPC-style execution
 * const result = await client.submitAndWait({
 *   name: 'generateReport',
 *   payload: { reportType: 'sales' },
 *   priority: JobPriority.NORMAL
 * });
 *
 * await client.close();
 * ```
 */

// Export main client
export { Client } from "./client";

// Export types and enums
export { JobStatus, JobPriority } from "./types";
export type {
  Job,
  JobResult,
  ClientOptions,
  SubmitJobOptions,
  SubmitScheduledJobOptions,
  SubmitAndWaitOptions,
} from "./types";

// Export errors
export {
  BananasError,
  ConnectionError,
  JobNotFoundError,
  ResultNotFoundError,
  TimeoutError,
  SerializationError,
  InvalidJobError,
} from "./errors";

// Export utility functions
export {
  createJob,
  jobToJSON,
  jobFromJSON,
  isResultSuccess,
  isResultFailed,
} from "./models";
