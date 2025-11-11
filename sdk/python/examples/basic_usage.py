"""
Basic usage example for Bananas Python client.

This example demonstrates:
- Creating a client
- Submitting jobs with different priorities
- Retrieving job status
- Getting results
"""

from datetime import timedelta

from bananas import Client, JobPriority


def main():
    # Create client
    print("Creating Bananas client...")
    client = Client("redis://localhost:6379/0")

    try:
        # Submit a high-priority job
        print("\n1. Submitting high-priority job...")
        job_id_1 = client.submit_job(
            name="send_email",
            payload={
                "to": "user@example.com",
                "subject": "Welcome!",
                "body": "Thanks for joining!"
            },
            priority=JobPriority.HIGH,
            description="Welcome email to new user"
        )
        print(f"   Job submitted: {job_id_1}")

        # Submit a normal-priority job with routing
        print("\n2. Submitting GPU job with routing...")
        job_id_2 = client.submit_job_with_route(
            name="process_video",
            payload={
                "video_url": "https://example.com/video.mp4",
                "resolution": "1080p"
            },
            priority=JobPriority.NORMAL,
            routing_key="gpu"
        )
        print(f"   Job submitted: {job_id_2}")

        # Check job status
        print("\n3. Checking job status...")
        job = client.get_job(job_id_1)
        if job:
            print(f"   Job {job_id_1}:")
            print(f"   - Name: {job.name}")
            print(f"   - Status: {job.status.value}")
            print(f"   - Priority: {job.priority.value}")
            print(f"   - Created: {job.created_at}")

        # Submit and wait for result (RPC-style)
        print("\n4. Submitting job and waiting for result...")
        print("   (This will timeout in development without workers)")
        result = client.submit_and_wait(
            name="generate_report",
            payload={"report_type": "sales", "month": "January"},
            priority=JobPriority.HIGH,
            timeout=timedelta(seconds=5)
        )

        if result:
            if result.is_success():
                print(f"   Success! Result: {result.result}")
            else:
                print(f"   Failed: {result.error}")
        else:
            print("   Timeout waiting for result (expected without workers)")

        # Try to get result for first job
        print("\n5. Trying to get result for first job...")
        result = client.get_result(job_id_1)
        if result:
            print(f"   Status: {result.status.value}")
            if result.is_success():
                print(f"   Result: {result.result}")
            else:
                print(f"   Error: {result.error}")
        else:
            print("   Result not available yet (expected without workers)")

    finally:
        # Always close the client
        print("\n6. Closing client...")
        client.close()
        print("   Done!")


if __name__ == "__main__":
    main()
