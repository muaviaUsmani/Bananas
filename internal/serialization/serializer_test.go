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

	task := &supplychain.PackageIngestionTask{
		PackageName:  "test-package",
		Version:      "1.0.0",
		Registry:     "npm",
		DownloadStats: 1000,
		Maintainers:  []string{"alice", "bob"},
		Licenses:     []string{"MIT"},
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
	if strings.Contains(payload, `"package_name"`) || strings.Contains(payload, `{`) {
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

	original := &supplychain.PackageIngestionTask{
		PackageName:  "test-package",
		Version:      "1.0.0",
		Registry:     "npm",
		DownloadStats: 1000,
		Maintainers:  []string{"alice", "bob"},
		Licenses:     []string{"MIT", "Apache-2.0"},
	}

	bytes, err := s.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	result := &supplychain.PackageIngestionTask{}
	if err := s.Unmarshal(bytes, result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if result.PackageName != original.PackageName {
		t.Errorf("PackageName mismatch: got %s, want %s", result.PackageName, original.PackageName)
	}
	if result.Version != original.Version {
		t.Errorf("Version mismatch: got %s, want %s", result.Version, original.Version)
	}
	if result.DownloadStats != original.DownloadStats {
		t.Errorf("DownloadStats mismatch: got %d, want %d", result.DownloadStats, original.DownloadStats)
	}
	if len(result.Maintainers) != len(original.Maintainers) {
		t.Errorf("Maintainers length mismatch: got %d, want %d", len(result.Maintainers), len(original.Maintainers))
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
		result := &supplychain.PackageIngestionTask{}
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

	original := &supplychain.HealthMetricsTask{
		PackageIdentifier: "example/package",
		MaintenanceVelocity: &supplychain.MaintenanceVelocity{
			CommitsLastMonth: 50,
			CommitsLastYear:  500,
			ReleasesLastYear: 12,
			LastCommitDate:   timestamppb.Now(),
			LastReleaseDate:  timestamppb.Now(),
		},
		ContributorMetrics: &supplychain.ContributorMetrics{
			TotalContributors:             100,
			ActiveContributorsLastMonth:   20,
			ActiveContributorsLastYear:    50,
			TopContributors:               []string{"alice", "bob", "carol"},
			BusFactor:                     3.5,
		},
		SecurityPosture: &supplychain.SecurityPosture{
			HasSecurityPolicy:          true,
			HasVulnerabilityDisclosure: true,
			OpenSecurityIssues:         2,
			ResolvedSecurityIssues:     15,
			SecurityContacts:           []string{"security@example.com"},
			SecurityScore:              85.5,
		},
		AdoptionMetrics: &supplychain.AdoptionMetrics{
			TotalDownloads:      1000000,
			DownloadsLastMonth:  50000,
			DownloadsLastWeek:   12000,
			DependentPackages:   250,
			GithubStars:         1500,
			GithubForks:         200,
			GithubWatchers:      100,
			AdoptionGrowthRate:  15.5,
		},
		OverallHealthScore: 88.5,
		HealthGrade:        "A",
		CalculatedAt:       timestamppb.Now(),
		ComponentScores: map[string]float32{
			"maintenance": 90.0,
			"security":    85.5,
			"adoption":    92.0,
		},
	}

	bytes, err := s.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	result := &supplychain.HealthMetricsTask{}
	if err := s.Unmarshal(bytes, result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify key fields
	if result.PackageIdentifier != original.PackageIdentifier {
		t.Errorf("PackageIdentifier mismatch")
	}
	if result.OverallHealthScore != original.OverallHealthScore {
		t.Errorf("OverallHealthScore mismatch")
	}
	if result.HealthGrade != original.HealthGrade {
		t.Errorf("HealthGrade mismatch")
	}

	// Verify nested messages
	if result.MaintenanceVelocity.CommitsLastMonth != original.MaintenanceVelocity.CommitsLastMonth {
		t.Errorf("Nested field mismatch: CommitsLastMonth")
	}
	if len(result.ContributorMetrics.TopContributors) != len(original.ContributorMetrics.TopContributors) {
		t.Errorf("Nested array length mismatch")
	}
	if len(result.ComponentScores) != len(original.ComponentScores) {
		t.Errorf("Map length mismatch")
	}
}
