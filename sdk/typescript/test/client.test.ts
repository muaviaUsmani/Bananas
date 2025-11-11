import { Client } from '../src/client';
import { JobPriority } from '../src/types';

// Mock ioredis
jest.mock('ioredis', () => {
  return jest.fn().mockImplementation(() => {
    return {
      set: jest.fn().mockResolvedValue('OK'),
      get: jest.fn().mockResolvedValue(null),
      lpush: jest.fn().mockResolvedValue(1),
      brpoplpush: jest.fn().mockResolvedValue(null),
      lrem: jest.fn().mockResolvedValue(1),
      zadd: jest.fn().mockResolvedValue(1),
      hset: jest.fn().mockResolvedValue(1),
      pipeline: jest.fn(() => ({
        set: jest.fn().mockReturnThis(),
        lpush: jest.fn().mockReturnThis(),
        exec: jest.fn().mockResolvedValue([]),
      })),
      quit: jest.fn().mockResolvedValue('OK'),
    };
  });
});

describe('Client', () => {
  let client: Client;

  beforeEach(() => {
    client = new Client({ redisUrl: 'redis://localhost:6379' });
  });

  afterEach(async () => {
    await client.close();
  });

  describe('constructor', () => {
    it('should create a client with Redis URL', () => {
      expect(client).toBeInstanceOf(Client);
    });
  });

  describe('submitJob', () => {
    it('should return a job ID', async () => {
      const jobId = await client.submitJob({
        name: 'send_email',
        payload: { to: 'user@example.com' },
        priority: JobPriority.NORMAL,
      });

      expect(jobId).toBeDefined();
      expect(typeof jobId).toBe('string');
      expect(jobId.length).toBeGreaterThan(0);
    });

    it('should submit job with description', async () => {
      const jobId = await client.submitJob({
        name: 'send_email',
        payload: { to: 'user@example.com' },
        priority: JobPriority.NORMAL,
        description: 'Send welcome email',
      });

      expect(jobId).toBeDefined();
    });

    it('should submit high priority job', async () => {
      const jobId = await client.submitJob({
        name: 'urgent_task',
        payload: { data: 'test' },
        priority: JobPriority.HIGH,
      });

      expect(jobId).toBeDefined();
    });

    it('should submit low priority job', async () => {
      const jobId = await client.submitJob({
        name: 'batch_task',
        payload: { data: 'test' },
        priority: JobPriority.LOW,
      });

      expect(jobId).toBeDefined();
    });
  });

  describe('submitJobScheduled', () => {
    it('should return a job ID for scheduled job', async () => {
      const scheduledFor = new Date(Date.now() + 3600000); // 1 hour from now
      const jobId = await client.submitJobScheduled({
        name: 'send_reminder',
        payload: { to: 'user@example.com' },
        priority: JobPriority.NORMAL,
        scheduledFor,
      });

      expect(jobId).toBeDefined();
      expect(typeof jobId).toBe('string');
    });

    it('should submit scheduled job with description', async () => {
      const scheduledFor = new Date(Date.now() + 3600000);
      const jobId = await client.submitJobScheduled({
        name: 'send_reminder',
        payload: { to: 'user@example.com' },
        priority: JobPriority.NORMAL,
        scheduledFor,
        description: 'Send reminder email',
      });

      expect(jobId).toBeDefined();
    });
  });
});
