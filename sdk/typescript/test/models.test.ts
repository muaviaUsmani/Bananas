import { Job, JobStatus, JobPriority } from "../src/types";
import { createJob, jobToJSON, jobFromJSON } from "../src/models";

describe("createJob", () => {
  it("should create a job with required fields", () => {
    const job = createJob(
      "send_email",
      { to: "user@example.com" },
      JobPriority.NORMAL,
    );

    expect(job.id).toBeDefined();
    expect(job.name).toBe("send_email");
    expect(job.payload).toEqual({ to: "user@example.com" });
    expect(job.status).toBe(JobStatus.PENDING);
    expect(job.priority).toBe(JobPriority.NORMAL);
    expect(job.attempts).toBe(0);
    expect(job.maxRetries).toBe(3);
  });

  it("should create a job with description", () => {
    const job = createJob(
      "send_email",
      { to: "user@example.com" },
      JobPriority.HIGH,
      { description: "Send welcome email" },
    );

    expect(job.description).toBe("Send welcome email");
    expect(job.priority).toBe(JobPriority.HIGH);
  });

  it("should create a scheduled job", () => {
    const scheduledFor = new Date("2025-12-01T00:00:00Z");
    const job = createJob("send_reminder", {}, JobPriority.NORMAL, {
      scheduledFor,
    });

    expect(job.scheduledFor).toEqual(scheduledFor);
    expect(job.status).toBe(JobStatus.SCHEDULED);
  });
});

describe("jobToJSON and jobFromJSON", () => {
  it("should serialize and deserialize a job", () => {
    const original = createJob(
      "send_email",
      { to: "user@example.com" },
      JobPriority.NORMAL,
      { description: "Test job" },
    );

    const json = jobToJSON(original);
    const restored = jobFromJSON(json);

    expect(restored.id).toBe(original.id);
    expect(restored.name).toBe(original.name);
    expect(restored.payload).toEqual(original.payload);
    expect(restored.status).toBe(original.status);
    expect(restored.priority).toBe(original.priority);
    expect(restored.description).toBe(original.description);
  });

  it("should handle scheduled jobs", () => {
    const scheduledFor = new Date("2025-12-01T00:00:00Z");
    const original = createJob("send_reminder", {}, JobPriority.NORMAL, {
      scheduledFor,
    });

    const json = jobToJSON(original);
    const restored = jobFromJSON(json);

    expect(restored.scheduledFor).toEqual(scheduledFor);
  });
});

describe("JobStatus enum", () => {
  it("should have all expected status values", () => {
    expect(JobStatus.PENDING).toBe("pending");
    expect(JobStatus.PROCESSING).toBe("processing");
    expect(JobStatus.COMPLETED).toBe("completed");
    expect(JobStatus.FAILED).toBe("failed");
    expect(JobStatus.SCHEDULED).toBe("scheduled");
  });
});

describe("JobPriority enum", () => {
  it("should have all expected priority values", () => {
    expect(JobPriority.HIGH).toBe("high");
    expect(JobPriority.NORMAL).toBe("normal");
    expect(JobPriority.LOW).toBe("low");
  });
});
