/**
 * Custom error classes for Bananas client.
 */

/**
 * Base error class for all Bananas errors.
 */
export class BananasError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "BananasError";
    Object.setPrototypeOf(this, BananasError.prototype);
  }
}

/**
 * Error thrown when there's a connection problem with Redis.
 */
export class ConnectionError extends BananasError {
  constructor(message: string) {
    super(message);
    this.name = "ConnectionError";
    Object.setPrototypeOf(this, ConnectionError.prototype);
  }
}

/**
 * Error thrown when a job cannot be found.
 */
export class JobNotFoundError extends BananasError {
  constructor(message: string) {
    super(message);
    this.name = "JobNotFoundError";
    Object.setPrototypeOf(this, JobNotFoundError.prototype);
  }
}

/**
 * Error thrown when a result cannot be found or has expired.
 */
export class ResultNotFoundError extends BananasError {
  constructor(message: string) {
    super(message);
    this.name = "ResultNotFoundError";
    Object.setPrototypeOf(this, ResultNotFoundError.prototype);
  }
}

/**
 * Error thrown when an operation times out.
 */
export class TimeoutError extends BananasError {
  constructor(message: string) {
    super(message);
    this.name = "TimeoutError";
    Object.setPrototypeOf(this, TimeoutError.prototype);
  }
}

/**
 * Error thrown when there's a serialization/deserialization problem.
 */
export class SerializationError extends BananasError {
  constructor(message: string) {
    super(message);
    this.name = "SerializationError";
    Object.setPrototypeOf(this, SerializationError.prototype);
  }
}

/**
 * Error thrown when job data is invalid.
 */
export class InvalidJobError extends BananasError {
  constructor(message: string) {
    super(message);
    this.name = "InvalidJobError";
    Object.setPrototypeOf(this, InvalidJobError.prototype);
  }
}
