package tests

import (
	"testing"
	"time"

	"github.com/muaviaUsmani/bananas/internal/serialization"
	"github.com/muaviaUsmani/bananas/proto/gen"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// =============================================================================
// BENCHMARK: Protobuf vs JSON - Small Payload (1KB)
// =============================================================================

func BenchmarkProto_Marshal_SmallPayload(b *testing.B) {
	s := serialization.NewProtobufSerializer()

	task := &supplychain.PackageIngestionTask{
		PackageName:  "test-package",
		Version:      "1.0.0",
		Registry:     "npm",
		DownloadStats: 1000,
		Maintainers:  []string{"alice", "bob"},
		Licenses:     []string{"MIT"},
		HomepageUrl:  "https://example.com",
		Description:  "A test package for benchmarking",
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
		"package_name":  "test-package",
		"version":       "1.0.0",
		"registry":      "npm",
		"download_stats": 1000,
		"maintainers":   []string{"alice", "bob"},
		"licenses":      []string{"MIT"},
		"homepage_url":  "https://example.com",
		"description":   "A test package for benchmarking",
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

	task := &supplychain.PackageIngestionTask{
		PackageName:  "test-package",
		Version:      "1.0.0",
		Registry:     "npm",
		DownloadStats: 1000,
		Maintainers:  []string{"alice", "bob"},
		Licenses:     []string{"MIT"},
	}

	bytes, _ := s.Marshal(task)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result := &supplychain.PackageIngestionTask{}
		err := s.Unmarshal(bytes, result)
		if err != nil {
			b.Fatalf("Unmarshal failed: %v", err)
		}
	}
}

func BenchmarkJSON_Unmarshal_SmallPayload(b *testing.B) {
	s := serialization.NewJSONSerializer()

	data := map[string]interface{}{
		"package_name":  "test-package",
		"version":       "1.0.0",
		"registry":      "npm",
		"download_stats": 1000,
		"maintainers":   []string{"alice", "bob"},
		"licenses":      []string{"MIT"},
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

func createMediumProtoPayload() *supplychain.DependencyResolutionTask {
	// Create a dependency tree with ~100 nodes
	deps := make([]*supplychain.DependencyNode, 0, 20)
	for i := 0; i < 20; i++ {
		node := &supplychain.DependencyNode{
			PackageName:  "package-" + string(rune(i)),
			Version:      "1.0.0",
			VersionRange: "^1.0.0",
			Dependencies: make([]*supplychain.DependencyNode, 0, 5),
		}
		// Add sub-dependencies
		for j := 0; j < 5; j++ {
			node.Dependencies = append(node.Dependencies, &supplychain.DependencyNode{
				PackageName:  "sub-package-" + string(rune(j)),
				Version:      "2.0.0",
				VersionRange: ">=2.0.0",
			})
		}
		deps = append(deps, node)
	}

	return &supplychain.DependencyResolutionTask{
		PackageIdentifier:      "root-package",
		VersionRange:           "^3.0.0",
		TransitiveDependencies: deps,
		Metadata: &supplychain.ResolutionMetadata{
			TotalDependencies: 120,
			UniquePackages:    95,
			Depth:             3,
			ResolvedAt:        timestamppb.Now(),
			ResolverVersion:   "1.0.0",
			Conflicts:         []string{"conflict1", "conflict2"},
		},
	}
}

func createMediumJSONPayload() map[string]interface{} {
	deps := make([]map[string]interface{}, 0, 20)
	for i := 0; i < 20; i++ {
		subDeps := make([]map[string]interface{}, 0, 5)
		for j := 0; j < 5; j++ {
			subDeps = append(subDeps, map[string]interface{}{
				"package_name":  "sub-package-" + string(rune(j)),
				"version":       "2.0.0",
				"version_range": ">=2.0.0",
			})
		}
		deps = append(deps, map[string]interface{}{
			"package_name":  "package-" + string(rune(i)),
			"version":       "1.0.0",
			"version_range": "^1.0.0",
			"dependencies":  subDeps,
		})
	}

	return map[string]interface{}{
		"package_identifier":      "root-package",
		"version_range":           "^3.0.0",
		"transitive_dependencies": deps,
		"metadata": map[string]interface{}{
			"total_dependencies": 120,
			"unique_packages":    95,
			"depth":              3,
			"resolved_at":        time.Now().Format(time.RFC3339),
			"resolver_version":   "1.0.0",
			"conflicts":          []string{"conflict1", "conflict2"},
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
		result := &supplychain.DependencyResolutionTask{}
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

func createLargeProtoPayload() *supplychain.HealthMetricsTask {
	return &supplychain.HealthMetricsTask{
		PackageIdentifier: "large-package",
		MaintenanceVelocity: &supplychain.MaintenanceVelocity{
			CommitsLastMonth: 150,
			CommitsLastYear:  1800,
			ReleasesLastYear: 24,
			LastCommitDate:   timestamppb.Now(),
			LastReleaseDate:  timestamppb.Now(),
		},
		ContributorMetrics: &supplychain.ContributorMetrics{
			TotalContributors:             500,
			ActiveContributorsLastMonth:   75,
			ActiveContributorsLastYear:    200,
			TopContributors:               generateStringArray(100),
			BusFactor:                     12.5,
		},
		SecurityPosture: &supplychain.SecurityPosture{
			HasSecurityPolicy:          true,
			HasVulnerabilityDisclosure: true,
			OpenSecurityIssues:         5,
			ResolvedSecurityIssues:     250,
			SecurityContacts:           []string{"security@example.com", "admin@example.com"},
			SecurityScore:              92.5,
		},
		AdoptionMetrics: &supplychain.AdoptionMetrics{
			TotalDownloads:      50000000,
			DownloadsLastMonth:  2500000,
			DownloadsLastWeek:   600000,
			DependentPackages:   5000,
			GithubStars:         25000,
			GithubForks:         3000,
			GithubWatchers:      1500,
			AdoptionGrowthRate:  25.5,
		},
		OverallHealthScore: 91.5,
		HealthGrade:        "A+",
		CalculatedAt:       timestamppb.Now(),
		ComponentScores:    generateScoresMap(50),
	}
}

func createLargeJSONPayload() map[string]interface{} {
	return map[string]interface{}{
		"package_identifier": "large-package",
		"maintenance_velocity": map[string]interface{}{
			"commits_last_month": 150,
			"commits_last_year":  1800,
			"releases_last_year": 24,
			"last_commit_date":   time.Now().Format(time.RFC3339),
			"last_release_date":  time.Now().Format(time.RFC3339),
		},
		"contributor_metrics": map[string]interface{}{
			"total_contributors":               500,
			"active_contributors_last_month":   75,
			"active_contributors_last_year":    200,
			"top_contributors":                 generateStringArray(100),
			"bus_factor":                       12.5,
		},
		"security_posture": map[string]interface{}{
			"has_security_policy":           true,
			"has_vulnerability_disclosure":  true,
			"open_security_issues":          5,
			"resolved_security_issues":      250,
			"security_contacts":             []string{"security@example.com", "admin@example.com"},
			"security_score":                92.5,
		},
		"adoption_metrics": map[string]interface{}{
			"total_downloads":       50000000,
			"downloads_last_month":  2500000,
			"downloads_last_week":   600000,
			"dependent_packages":    5000,
			"github_stars":          25000,
			"github_forks":          3000,
			"github_watchers":       1500,
			"adoption_growth_rate":  25.5,
		},
		"overall_health_score": 91.5,
		"health_grade":         "A+",
		"calculated_at":        time.Now().Format(time.RFC3339),
		"component_scores":     generateScoresMapJSON(50),
	}
}

func generateStringArray(n int) []string {
	result := make([]string, n)
	for i := 0; i < n; i++ {
		result[i] = "contributor-" + string(rune(i%26+'a'))
	}
	return result
}

func generateScoresMap(n int) map[string]float32 {
	result := make(map[string]float32)
	for i := 0; i < n; i++ {
		result["metric-"+string(rune(i%26+'a'))] = float32(i%100) + 0.5
	}
	return result
}

func generateScoresMapJSON(n int) map[string]interface{} {
	result := make(map[string]interface{})
	for i := 0; i < n; i++ {
		result["metric-"+string(rune(i%26+'a'))] = float64(i%100) + 0.5
	}
	return result
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
		result := &supplychain.HealthMetricsTask{}
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

	task := &supplychain.PackageIngestionTask{
		PackageName:  "test-package",
		Version:      "1.0.0",
		Registry:     "npm",
		DownloadStats: 1000,
		Maintainers:  []string{"alice", "bob"},
		Licenses:     []string{"MIT"},
	}

	jsonData := map[string]interface{}{
		"package_name":  "test-package",
		"version":       "1.0.0",
		"registry":      "npm",
		"download_stats": 1000,
		"maintainers":   []string{"alice", "bob"},
		"licenses":      []string{"MIT"},
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
	task := &supplychain.PackageIngestionTask{
		PackageName:  "test-package",
		Version:      "1.0.0",
		Registry:     "npm",
		DownloadStats: 1000,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bytes, _ := s.Marshal(task)
		result := &supplychain.PackageIngestionTask{}
		_ = s.Unmarshal(bytes, result)
	}
}

func BenchmarkRoundTrip_JSON_Small(b *testing.B) {
	s := serialization.NewJSONSerializer()
	data := map[string]interface{}{
		"package_name":  "test-package",
		"version":       "1.0.0",
		"registry":      "npm",
		"download_stats": 1000,
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
		result := &supplychain.HealthMetricsTask{}
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
