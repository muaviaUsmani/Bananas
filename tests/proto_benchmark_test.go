package tests

import (
	"testing"
	"time"

	"github.com/muaviaUsmani/bananas/internal/serialization"
	tasks "github.com/muaviaUsmani/bananas/proto/gen"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// =============================================================================
// BENCHMARK: Protobuf vs JSON - Small Payload (1KB)
// =============================================================================

func BenchmarkProto_Marshal_SmallPayload(b *testing.B) {
	s := serialization.NewProtobufSerializer()

	task := &tasks.EmailTask{
		To:       "user@example.com",
		From:     "noreply@example.com",
		Subject:  "Test Email for Benchmarking",
		BodyText: "This is a test email body for performance benchmarking purposes.",
		Cc:       []string{"alice@example.com", "bob@example.com"},
		Headers: map[string]string{
			"X-Priority": "1",
			"X-Category": "benchmark",
		},
		ScheduledFor: timestamppb.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := s.Marshal(task)
		if err != nil {
			b.Fatalf("Marshal failed: %v", err)
		}
	}
}

func BenchmarkJSON_Marshal_SmallPayload(b *testing.B) {
	s := serialization.NewJSONSerializer()

	data := map[string]interface{}{
		"to":        "user@example.com",
		"from":      "noreply@example.com",
		"subject":   "Test Email for Benchmarking",
		"body_text": "This is a test email body for performance benchmarking purposes.",
		"cc":        []string{"alice@example.com", "bob@example.com"},
		"headers": map[string]string{
			"X-Priority": "1",
			"X-Category": "benchmark",
		},
		"scheduled_for": time.Now().Format(time.RFC3339),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := s.Marshal(data)
		if err != nil {
			b.Fatalf("Marshal failed: %v", err)
		}
	}
}

func BenchmarkProto_Unmarshal_SmallPayload(b *testing.B) {
	s := serialization.NewProtobufSerializer()

	task := &tasks.EmailTask{
		To:       "user@example.com",
		From:     "noreply@example.com",
		Subject:  "Test Email",
		BodyText: "This is a test email body.",
		Cc:       []string{"alice@example.com", "bob@example.com"},
		Headers: map[string]string{
			"X-Priority": "1",
		},
	}

	bytes, _ := s.Marshal(task)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := &tasks.EmailTask{}
		err := s.Unmarshal(bytes, result)
		if err != nil {
			b.Fatalf("Unmarshal failed: %v", err)
		}
	}
}

func BenchmarkJSON_Unmarshal_SmallPayload(b *testing.B) {
	s := serialization.NewJSONSerializer()

	data := map[string]interface{}{
		"to":        "user@example.com",
		"from":      "noreply@example.com",
		"subject":   "Test Email",
		"body_text": "This is a test email body.",
		"cc":        []string{"alice@example.com", "bob@example.com"},
		"headers": map[string]string{
			"X-Priority": "1",
		},
	}

	bytes, _ := s.Marshal(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result map[string]interface{}
		err := s.Unmarshal(bytes, &result)
		if err != nil {
			b.Fatalf("Unmarshal failed: %v", err)
		}
	}
}

// =============================================================================
// BENCHMARK: Protobuf vs JSON - Medium Payload (10KB)
// =============================================================================

func createMediumProtoPayload() *tasks.BatchTask {
	// Create a batch task with ~100 items to simulate medium payload size
	items := make([]*tasks.BatchItem, 0, 100)
	for i := 0; i < 100; i++ {
		item := &tasks.BatchItem{
			ItemId: "item-" + string(rune(i)),
			Data:   []byte("Sample data content for item " + string(rune(i))),
			Metadata: map[string]string{
				"type":     "batch-item",
				"priority": "normal",
				"index":    string(rune(i)),
			},
		}
		items = append(items, item)
	}

	return &tasks.BatchTask{
		BatchId:     "batch-benchmark-medium",
		Operation:   "process_items",
		Items:       items,
		Concurrency: 10,
		CreatedAt:   timestamppb.Now(),
		Options: &tasks.BatchOptions{
			StopOnError:    true,
			ReturnResults:  true,
			TimeoutSeconds: 300,
			ResultFormat:   "json",
		},
	}
}

func createMediumJSONPayload() map[string]interface{} {
	items := make([]map[string]interface{}, 0, 100)
	for i := 0; i < 100; i++ {
		items = append(items, map[string]interface{}{
			"item_id": "item-" + string(rune(i)),
			"data":    "Sample data content for item " + string(rune(i)),
			"metadata": map[string]string{
				"type":     "batch-item",
				"priority": "normal",
				"index":    string(rune(i)),
			},
		})
	}

	return map[string]interface{}{
		"batch_id":    "batch-benchmark-medium",
		"operation":   "process_items",
		"items":       items,
		"concurrency": 10,
		"created_at":  time.Now().Format(time.RFC3339),
		"options": map[string]interface{}{
			"stop_on_error":   true,
			"return_results":  true,
			"timeout_seconds": 300,
			"result_format":   "json",
		},
	}
}

func BenchmarkProto_Marshal_MediumPayload(b *testing.B) {
	s := serialization.NewProtobufSerializer()
	task := createMediumProtoPayload()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := s.Marshal(task)
		if err != nil {
			b.Fatalf("Marshal failed: %v", err)
		}
	}
}

func BenchmarkJSON_Marshal_MediumPayload(b *testing.B) {
	s := serialization.NewJSONSerializer()
	data := createMediumJSONPayload()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := s.Marshal(data)
		if err != nil {
			b.Fatalf("Marshal failed: %v", err)
		}
	}
}

func BenchmarkProto_Unmarshal_MediumPayload(b *testing.B) {
	s := serialization.NewProtobufSerializer()
	task := createMediumProtoPayload()
	bytes, _ := s.Marshal(task)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := &tasks.BatchTask{}
		err := s.Unmarshal(bytes, result)
		if err != nil {
			b.Fatalf("Unmarshal failed: %v", err)
		}
	}
}

func BenchmarkJSON_Unmarshal_MediumPayload(b *testing.B) {
	s := serialization.NewJSONSerializer()
	data := createMediumJSONPayload()
	bytes, _ := s.Marshal(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result map[string]interface{}
		err := s.Unmarshal(bytes, &result)
		if err != nil {
			b.Fatalf("Unmarshal failed: %v", err)
		}
	}
}

// =============================================================================
// BENCHMARK: Protobuf vs JSON - Large Payload (100KB+)
// =============================================================================

func createLargeProtoPayload() *tasks.BatchTask {
	// Create a large batch task with 500 items, each with substantial metadata
	items := make([]*tasks.BatchItem, 0, 500)
	for i := 0; i < 500; i++ {
		// Generate large data payload for each item
		data := make([]byte, 200) // ~200 bytes per item
		for j := 0; j < 200; j++ {
			data[j] = byte((i + j) % 256)
		}

		item := &tasks.BatchItem{
			ItemId: "large-item-" + string(rune(i)),
			Data:   data,
			Metadata: map[string]string{
				"type":        "large-batch-item",
				"priority":    "high",
				"index":       string(rune(i)),
				"category":    "benchmark",
				"status":      "pending",
				"created_by":  "system",
				"description": "Large benchmark item with substantial metadata",
			},
		}
		items = append(items, item)
	}

	return &tasks.BatchTask{
		BatchId:     "batch-benchmark-large-" + string(rune(time.Now().Unix())),
		Operation:   "process_large_batch",
		Items:       items,
		Concurrency: 50,
		CreatedAt:   timestamppb.Now(),
		Options: &tasks.BatchOptions{
			StopOnError:    false,
			ReturnResults:  true,
			TimeoutSeconds: 600,
			ResultFormat:   "protobuf",
		},
	}
}

func createLargeJSONPayload() map[string]interface{} {
	items := make([]map[string]interface{}, 0, 500)
	for i := 0; i < 500; i++ {
		// Generate large data payload for each item
		data := make([]byte, 200)
		for j := 0; j < 200; j++ {
			data[j] = byte((i + j) % 256)
		}

		items = append(items, map[string]interface{}{
			"item_id": "large-item-" + string(rune(i)),
			"data":    string(data),
			"metadata": map[string]string{
				"type":        "large-batch-item",
				"priority":    "high",
				"index":       string(rune(i)),
				"category":    "benchmark",
				"status":      "pending",
				"created_by":  "system",
				"description": "Large benchmark item with substantial metadata",
			},
		})
	}

	return map[string]interface{}{
		"batch_id":    "batch-benchmark-large-" + string(rune(time.Now().Unix())),
		"operation":   "process_large_batch",
		"items":       items,
		"concurrency": 50,
		"created_at":  time.Now().Format(time.RFC3339),
		"options": map[string]interface{}{
			"stop_on_error":   false,
			"return_results":  true,
			"timeout_seconds": 600,
			"result_format":   "protobuf",
		},
	}
}

func BenchmarkProto_Marshal_LargePayload(b *testing.B) {
	s := serialization.NewProtobufSerializer()
	task := createLargeProtoPayload()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := s.Marshal(task)
		if err != nil {
			b.Fatalf("Marshal failed: %v", err)
		}
	}
}

func BenchmarkJSON_Marshal_LargePayload(b *testing.B) {
	s := serialization.NewJSONSerializer()
	data := createLargeJSONPayload()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := s.Marshal(data)
		if err != nil {
			b.Fatalf("Marshal failed: %v", err)
		}
	}
}

func BenchmarkProto_Unmarshal_LargePayload(b *testing.B) {
	s := serialization.NewProtobufSerializer()
	task := createLargeProtoPayload()
	bytes, _ := s.Marshal(task)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := &tasks.BatchTask{}
		err := s.Unmarshal(bytes, result)
		if err != nil {
			b.Fatalf("Unmarshal failed: %v", err)
		}
	}
}

func BenchmarkJSON_Unmarshal_LargePayload(b *testing.B) {
	s := serialization.NewJSONSerializer()
	data := createLargeJSONPayload()
	bytes, _ := s.Marshal(data)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var result map[string]interface{}
		err := s.Unmarshal(bytes, &result)
		if err != nil {
			b.Fatalf("Unmarshal failed: %v", err)
		}
	}
}

// =============================================================================
// BENCHMARK: Payload Size Comparison
// =============================================================================

func BenchmarkPayloadSize_Small(b *testing.B) {
	protoSerializer := serialization.NewProtobufSerializer()
	jsonSerializer := serialization.NewJSONSerializer()

	task := &tasks.EmailTask{
		To:       "user@example.com",
		From:     "noreply@example.com",
		Subject:  "Test Email",
		BodyText: "This is a test email body.",
		Cc:       []string{"alice@example.com", "bob@example.com"},
		Headers: map[string]string{
			"X-Priority": "1",
		},
	}

	jsonData := map[string]interface{}{
		"to":        "user@example.com",
		"from":      "noreply@example.com",
		"subject":   "Test Email",
		"body_text": "This is a test email body.",
		"cc":        []string{"alice@example.com", "bob@example.com"},
		"headers": map[string]string{
			"X-Priority": "1",
		},
	}

	protoBytes, _ := protoSerializer.Marshal(task)
	jsonBytes, _ := jsonSerializer.Marshal(jsonData)

	b.Logf("Small payload - Protobuf size: %d bytes", len(protoBytes))
	b.Logf("Small payload - JSON size: %d bytes", len(jsonBytes))
	b.Logf("Small payload - Protobuf is %.1f%% smaller", float64(len(jsonBytes)-len(protoBytes))/float64(len(jsonBytes))*100)
}

func BenchmarkPayloadSize_Medium(b *testing.B) {
	protoSerializer := serialization.NewProtobufSerializer()
	jsonSerializer := serialization.NewJSONSerializer()

	protoTask := createMediumProtoPayload()
	jsonData := createMediumJSONPayload()

	protoBytes, _ := protoSerializer.Marshal(protoTask)
	jsonBytes, _ := jsonSerializer.Marshal(jsonData)

	b.Logf("Medium payload - Protobuf size: %d bytes", len(protoBytes))
	b.Logf("Medium payload - JSON size: %d bytes", len(jsonBytes))
	b.Logf("Medium payload - Protobuf is %.1f%% smaller", float64(len(jsonBytes)-len(protoBytes))/float64(len(jsonBytes))*100)
}

func BenchmarkPayloadSize_Large(b *testing.B) {
	protoSerializer := serialization.NewProtobufSerializer()
	jsonSerializer := serialization.NewJSONSerializer()

	protoTask := createLargeProtoPayload()
	jsonData := createLargeJSONPayload()

	protoBytes, _ := protoSerializer.Marshal(protoTask)
	jsonBytes, _ := jsonSerializer.Marshal(jsonData)

	b.Logf("Large payload - Protobuf size: %d bytes", len(protoBytes))
	b.Logf("Large payload - JSON size: %d bytes", len(jsonBytes))
	b.Logf("Large payload - Protobuf is %.1f%% smaller", float64(len(jsonBytes)-len(protoBytes))/float64(len(jsonBytes))*100)
}

// =============================================================================
// BENCHMARK: End-to-End Comparison (Marshal + Unmarshal)
// =============================================================================

func BenchmarkRoundTrip_Proto_Small(b *testing.B) {
	s := serialization.NewProtobufSerializer()
	task := &tasks.EmailTask{
		To:       "user@example.com",
		From:     "noreply@example.com",
		Subject:  "Test Email",
		BodyText: "Test body",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bytes, _ := s.Marshal(task)
		result := &tasks.EmailTask{}
		_ = s.Unmarshal(bytes, result)
	}
}

func BenchmarkRoundTrip_JSON_Small(b *testing.B) {
	s := serialization.NewJSONSerializer()
	data := map[string]interface{}{
		"to":        "user@example.com",
		"from":      "noreply@example.com",
		"subject":   "Test Email",
		"body_text": "Test body",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bytes, _ := s.Marshal(data)
		var result map[string]interface{}
		_ = s.Unmarshal(bytes, &result)
	}
}

func BenchmarkRoundTrip_Proto_Large(b *testing.B) {
	s := serialization.NewProtobufSerializer()
	task := createLargeProtoPayload()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bytes, _ := s.Marshal(task)
		result := &tasks.BatchTask{}
		_ = s.Unmarshal(bytes, result)
	}
}

func BenchmarkRoundTrip_JSON_Large(b *testing.B) {
	s := serialization.NewJSONSerializer()
	data := createLargeJSONPayload()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bytes, _ := s.Marshal(data)
		var result map[string]interface{}
		_ = s.Unmarshal(bytes, &result)
	}
}
