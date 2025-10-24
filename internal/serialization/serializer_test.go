package serialization

import (
	"strings"
	"testing"

	"github.com/muaviaUsmani/bananas/proto/gen"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestSerializer_Marshal_JSON(t *testing.T) {
	s := NewJSONSerializer()

	type testData struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	data := testData{Name: "test", Value: 42}
	bytes, err := s.Marshal(data)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Check format prefix
	if bytes[0] != byte(FormatJSON) {
		t.Errorf("Expected JSON format prefix, got %d", bytes[0])
	}

	// Verify JSON content
	if !strings.Contains(string(bytes[1:]), "test") {
		t.Errorf("JSON content not found in serialized data")
	}
}

func TestSerializer_Marshal_Protobuf(t *testing.T) {
	s := NewProtobufSerializer()

	task := &tasks.EmailTask{
		To:      "user@example.com",
		From:    "noreply@example.com",
		Subject: "Test Email",
		BodyText: "This is a test email",
		Headers: map[string]string{
			"X-Priority": "1",
		},
	}

	bytes, err := s.Marshal(task)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Check format prefix
	if bytes[0] != byte(FormatProtobuf) {
		t.Errorf("Expected Protobuf format prefix, got %d", bytes[0])
	}

	// Protobuf encodes strings as length-delimited fields, so text may be visible
	// The important thing is that it's not JSON format (no quotes, braces, etc.)
	payload := string(bytes[1:])
	if strings.Contains(payload, `"to"`) || strings.Contains(payload, `{`) {
		t.Errorf("Protobuf should not be in JSON format")
	}
}

func TestSerializer_Unmarshal_JSON(t *testing.T) {
	s := NewJSONSerializer()

	type testData struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	original := testData{Name: "test", Value: 42}
	bytes, err := s.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var result testData
	if err := s.Unmarshal(bytes, &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if result.Name != original.Name || result.Value != original.Value {
		t.Errorf("Unmarshal produced incorrect result: got %+v, want %+v", result, original)
	}
}

func TestSerializer_Unmarshal_Protobuf(t *testing.T) {
	s := NewProtobufSerializer()

	original := &tasks.EmailTask{
		To:       "user@example.com",
		From:     "noreply@example.com",
		Subject:  "Test Email",
		BodyText: "This is a test email",
		Cc:       []string{"cc1@example.com", "cc2@example.com"},
		Headers: map[string]string{
			"X-Priority": "1",
			"X-Custom":   "value",
		},
	}

	bytes, err := s.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	result := &tasks.EmailTask{}
	if err := s.Unmarshal(bytes, result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if result.To != original.To {
		t.Errorf("To mismatch: got %s, want %s", result.To, original.To)
	}
	if result.Subject != original.Subject {
		t.Errorf("Subject mismatch: got %s, want %s", result.Subject, original.Subject)
	}
	if len(result.Cc) != len(original.Cc) {
		t.Errorf("Cc length mismatch: got %d, want %d", len(result.Cc), len(original.Cc))
	}
	if len(result.Headers) != len(original.Headers) {
		t.Errorf("Headers length mismatch: got %d, want %d", len(result.Headers), len(original.Headers))
	}
}

func TestSerializer_DetectFormat_WithPrefix(t *testing.T) {
	s := NewSerializer(FormatJSON)

	tests := []struct {
		name           string
		data           []byte
		expectedFormat PayloadFormat
		expectError    bool
	}{
		{
			name:           "JSON with prefix",
			data:           []byte{byte(FormatJSON), '{', '}'},
			expectedFormat: FormatJSON,
			expectError:    false,
		},
		{
			name:           "Protobuf with prefix",
			data:           []byte{byte(FormatProtobuf), 0x0a, 0x05},
			expectedFormat: FormatProtobuf,
			expectError:    false,
		},
		{
			name:           "Legacy JSON without prefix",
			data:           []byte("{\"key\":\"value\"}"),
			expectedFormat: FormatJSON,
			expectError:    false,
		},
		{
			name:           "Legacy JSON array without prefix",
			data:           []byte("[1,2,3]"),
			expectedFormat: FormatJSON,
			expectError:    false,
		},
		{
			name:        "Empty data",
			data:        []byte{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			format, payload, err := s.DetectFormat(tt.data)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if format != tt.expectedFormat {
				t.Errorf("Format mismatch: got %d, want %d", format, tt.expectedFormat)
			}

			// Verify payload is correct (without prefix for prefixed data)
			if tt.data[0] == byte(FormatJSON) || tt.data[0] == byte(FormatProtobuf) {
				if len(payload) != len(tt.data)-1 {
					t.Errorf("Payload length mismatch: got %d, want %d", len(payload), len(tt.data)-1)
				}
			}
		})
	}
}

func TestSerializer_BackwardCompatibility_JSON(t *testing.T) {
	s := NewProtobufSerializer() // Default to protobuf

	// Simulate legacy JSON payload without format prefix
	legacyJSON := []byte("{\"name\":\"test\",\"value\":123}")

	type testData struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	var result testData
	if err := s.Unmarshal(legacyJSON, &result); err != nil {
		t.Fatalf("Failed to unmarshal legacy JSON: %v", err)
	}

	if result.Name != "test" || result.Value != 123 {
		t.Errorf("Legacy JSON deserialization failed: got %+v", result)
	}
}

func TestSerializer_IsProtobuf(t *testing.T) {
	s := NewSerializer(FormatJSON)

	tests := []struct {
		name     string
		data     []byte
		expected bool
	}{
		{
			name:     "Protobuf with prefix",
			data:     []byte{byte(FormatProtobuf), 0x0a, 0x05},
			expected: true,
		},
		{
			name:     "JSON with prefix",
			data:     []byte{byte(FormatJSON), '{', '}'},
			expected: false,
		},
		{
			name:     "Legacy JSON",
			data:     []byte("{\"key\":\"value\"}"),
			expected: false,
		},
		{
			name:     "Empty",
			data:     []byte{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.IsProtobuf(tt.data)
			if result != tt.expected {
				t.Errorf("IsProtobuf() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSerializer_IsJSON(t *testing.T) {
	s := NewSerializer(FormatJSON)

	tests := []struct {
		name     string
		data     []byte
		expected bool
	}{
		{
			name:     "JSON with prefix",
			data:     []byte{byte(FormatJSON), '{', '}'},
			expected: true,
		},
		{
			name:     "Legacy JSON object",
			data:     []byte("{\"key\":\"value\"}"),
			expected: true,
		},
		{
			name:     "Legacy JSON array",
			data:     []byte("[1,2,3]"),
			expected: true,
		},
		{
			name:     "Protobuf with prefix",
			data:     []byte{byte(FormatProtobuf), 0x0a, 0x05},
			expected: false,
		},
		{
			name:     "Empty",
			data:     []byte{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := s.IsJSON(tt.data)
			if result != tt.expected {
				t.Errorf("IsJSON() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSerializer_MarshalWithFormat(t *testing.T) {
	s := NewSerializer(FormatJSON)

	type testData struct {
		Name string `json:"name"`
	}

	data := testData{Name: "test"}

	// Test explicit JSON format
	jsonBytes, err := s.MarshalWithFormat(data, FormatJSON)
	if err != nil {
		t.Fatalf("MarshalWithFormat(JSON) failed: %v", err)
	}
	if jsonBytes[0] != byte(FormatJSON) {
		t.Errorf("Expected JSON prefix")
	}

	// Test protobuf format with non-proto message (should fail)
	_, err = s.MarshalWithFormat(data, FormatProtobuf)
	if err == nil {
		t.Errorf("Expected error when marshaling non-proto message as protobuf")
	}
}

func TestSerializer_UnmarshalWithFormat(t *testing.T) {
	s := NewSerializer(FormatJSON)

	type testData struct {
		Name string `json:"name"`
	}

	original := testData{Name: "test"}

	// Marshal with JSON
	bytes, err := s.MarshalWithFormat(original, FormatJSON)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Get payload without prefix
	_, payload, err := s.DetectFormat(bytes)
	if err != nil {
		t.Fatalf("DetectFormat failed: %v", err)
	}

	// Unmarshal with explicit format
	var result testData
	if err := s.UnmarshalWithFormat(payload, &result, FormatJSON); err != nil {
		t.Fatalf("UnmarshalWithFormat failed: %v", err)
	}

	if result.Name != original.Name {
		t.Errorf("Data mismatch after unmarshal")
	}
}

func TestSerializer_ErrorCases(t *testing.T) {
	s := NewSerializer(FormatJSON)

	t.Run("Empty payload unmarshal", func(t *testing.T) {
		var result map[string]string
		err := s.Unmarshal([]byte{}, &result)
		if err == nil {
			t.Errorf("Expected error for empty payload")
		}
	})

	t.Run("Malformed JSON", func(t *testing.T) {
		data := []byte{byte(FormatJSON), '{', '{', '{'}
		var result map[string]string
		err := s.Unmarshal(data, &result)
		if err == nil {
			t.Errorf("Expected error for malformed JSON")
		}
	})

	t.Run("Malformed protobuf", func(t *testing.T) {
		data := []byte{byte(FormatProtobuf), 0xFF, 0xFF, 0xFF}
		result := &tasks.EmailTask{}
		err := s.Unmarshal(data, result)
		if err == nil {
			t.Errorf("Expected error for malformed protobuf")
		}
	})

	t.Run("Unknown format", func(t *testing.T) {
		data := []byte{0xFF, 0x00, 0x00}
		var result map[string]string
		err := s.Unmarshal(data, &result)
		if err == nil {
			t.Errorf("Expected error for unknown format")
		}
	})
}

func TestSerializer_RoundTrip_ComplexProto(t *testing.T) {
	s := NewProtobufSerializer()

	original := &tasks.NotificationTask{
		RecipientId: "user-123",
		Channel:     "email",
		Title:       "Important Notification",
		Message:     "This is an important message that needs your attention.",
		Metadata: map[string]string{
			"category": "alert",
			"source":   "system",
			"urgency":  "high",
		},
		Priority:  tasks.NotificationPriority_NOTIFICATION_PRIORITY_HIGH,
		CreatedAt: timestamppb.Now(),
	}

	bytes, err := s.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	result := &tasks.NotificationTask{}
	if err := s.Unmarshal(bytes, result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify key fields
	if result.RecipientId != original.RecipientId {
		t.Errorf("RecipientId mismatch")
	}
	if result.Channel != original.Channel {
		t.Errorf("Channel mismatch")
	}
	if result.Title != original.Title {
		t.Errorf("Title mismatch")
	}
	if result.Priority != original.Priority {
		t.Errorf("Priority mismatch")
	}

	// Verify map
	if len(result.Metadata) != len(original.Metadata) {
		t.Errorf("Metadata map length mismatch")
	}
	for k, v := range original.Metadata {
		if result.Metadata[k] != v {
			t.Errorf("Metadata mismatch for key %s", k)
		}
	}
}

func TestSerializer_BatchTask(t *testing.T) {
	s := NewProtobufSerializer()

	original := &tasks.BatchTask{
		BatchId:   "batch-123",
		Operation: "process_orders",
		Items: []*tasks.BatchItem{
			{
				ItemId: "item-1",
				Data:   []byte("data-1"),
				Metadata: map[string]string{
					"type": "order",
				},
			},
			{
				ItemId: "item-2",
				Data:   []byte("data-2"),
				Metadata: map[string]string{
					"type": "order",
				},
			},
		},
		Concurrency: 10,
		CreatedAt:   timestamppb.Now(),
		Options: &tasks.BatchOptions{
			StopOnError:   true,
			ReturnResults: true,
			TimeoutSeconds: 300,
			ResultFormat:  "json",
		},
	}

	bytes, err := s.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	result := &tasks.BatchTask{}
	if err := s.Unmarshal(bytes, result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if result.BatchId != original.BatchId {
		t.Errorf("BatchId mismatch")
	}
	if len(result.Items) != len(original.Items) {
		t.Errorf("Items length mismatch")
	}
	if result.Options.StopOnError != original.Options.StopOnError {
		t.Errorf("Options.StopOnError mismatch")
	}
}

func TestSerializer_WebhookTask(t *testing.T) {
	s := NewProtobufSerializer()

	original := &tasks.WebhookTask{
		Url:    "https://api.example.com/webhook",
		Method: "POST",
		Headers: map[string]string{
			"Content-Type":  "application/json",
			"Authorization": "Bearer token123",
		},
		Payload:        []byte(`{"event": "user_created", "data": {"id": 123}}`),
		TimeoutSeconds: 30,
		MaxRetries:     3,
		VerifySsl:      true,
		CreatedAt:      timestamppb.Now(),
	}

	bytes, err := s.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	result := &tasks.WebhookTask{}
	if err := s.Unmarshal(bytes, result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if result.Url != original.Url {
		t.Errorf("URL mismatch")
	}
	if result.Method != original.Method {
		t.Errorf("Method mismatch")
	}
	if string(result.Payload) != string(original.Payload) {
		t.Errorf("Payload mismatch")
	}
	if len(result.Headers) != len(original.Headers) {
		t.Errorf("Headers length mismatch")
	}
}
