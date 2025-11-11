"""
Scheduled jobs example for Bananas Python client.

This example demonstrates:
- Scheduling jobs for future execution
- Working with different time zones
- Scheduling recurring tasks
"""

from datetime import datetime, timedelta, timezone

from bananas import Client, JobPriority


def main():
    print("Bananas Scheduled Jobs Example\n")

    # Create client
    client = Client("redis://localhost:6379/0")

    try:
        # Example 1: Schedule job for 1 hour from now
        print("1. Scheduling job for 1 hour from now...")
        scheduled_time = datetime.now(timezone.utc) + timedelta(hours=1)

        job_id_1 = client.submit_job_scheduled(
            name="send_reminder",
            payload={"user_id": 123, "message": "Don't forget your appointment!"},
            priority=JobPriority.NORMAL,
            scheduled_for=scheduled_time,
            description="Appointment reminder"
        )
        print(f"   Job scheduled: {job_id_1}")
        print(f"   Will execute at: {scheduled_time.isoformat()}")

        # Example 2: Schedule job for specific time tomorrow
        print("\n2. Scheduling job for tomorrow at 9 AM UTC...")
        tomorrow_9am = datetime.now(timezone.utc).replace(
            hour=9, minute=0, second=0, microsecond=0
        ) + timedelta(days=1)

        job_id_2 = client.submit_job_scheduled(
            name="generate_daily_report",
            payload={"report_type": "sales", "recipients": ["admin@example.com"]},
            priority=JobPriority.HIGH,
            scheduled_for=tomorrow_9am,
            description="Daily sales report"
        )
        print(f"   Job scheduled: {job_id_2}")
        print(f"   Will execute at: {tomorrow_9am.isoformat()}")

        # Example 3: Schedule multiple jobs at different times
        print("\n3. Scheduling batch of reminder jobs...")
        intervals = [15, 30, 45, 60]  # minutes

        for interval in intervals:
            scheduled_time = datetime.now(timezone.utc) + timedelta(minutes=interval)
            job_id = client.submit_job_scheduled(
                name="send_notification",
                payload={"type": "reminder", "minutes": interval},
                priority=JobPriority.LOW,
                scheduled_for=scheduled_time
            )
            print(f"   Scheduled job {job_id} for {interval} minutes from now")

        # Example 4: Schedule job with routing
        print("\n4. Scheduling GPU job for later...")
        scheduled_time = datetime.now(timezone.utc) + timedelta(minutes=30)

        job_id_4 = client.submit_job_scheduled(
            name="train_model",
            payload={"model_id": "resnet50", "dataset": "imagenet"},
            priority=JobPriority.HIGH,
            scheduled_for=scheduled_time,
            routing_key="gpu",
            description="Model training on GPU"
        )
        print(f"   GPU job scheduled: {job_id_4}")

        # Check job status
        print("\n5. Checking scheduled job status...")
        job = client.get_job(job_id_1)
        if job:
            print(f"   Job: {job.name}")
            print(f"   Status: {job.status.value}")
            print(f"   Priority: {job.priority.value}")
            print(f"   Scheduled for: {job.scheduled_for}")
            print(f"   Created at: {job.created_at}")

        print("\n✓ All jobs scheduled successfully!")
        print("\nNote: These jobs will be executed by the scheduler service")
        print("      when their scheduled time arrives.")

    finally:
        client.close()
        print("\nClient closed.")


if __name__ == "__main__":
    main()
