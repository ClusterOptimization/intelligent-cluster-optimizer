package recommendation

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"intelligent-cluster-optimizer/pkg/leakdetector"
	"intelligent-cluster-optimizer/pkg/timepattern"
)

// CSVTestData holds parsed test data from CSV files
type CSVTestData struct {
	Name        string
	Description string
	Expected    string
	Samples     []TestSample
}

// TestSample represents a single sample from CSV
type TestSample struct {
	TimestampOffset time.Duration
	CPUMillicores   int64
	MemoryBytes     int64
	RequestCPU      int64
	RequestMemory   int64
	HourOfDay       int // Optional, for time pattern tests
}

// loadCSVTestData loads test data from a CSV file
func loadCSVTestData(filepath string) (*CSVTestData, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	data := &CSVTestData{
		Name:    strings.TrimSuffix(filepath, ".csv"),
		Samples: []TestSample{},
	}

	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines
		if line == "" {
			continue
		}

		// Parse comments for metadata
		if strings.HasPrefix(line, "#") {
			comment := strings.TrimPrefix(line, "#")
			comment = strings.TrimSpace(comment)

			if strings.Contains(comment, "Pattern") {
				data.Description = comment
			} else if strings.Contains(comment, "Expected:") {
				data.Expected = strings.TrimPrefix(comment, "Expected:")
				data.Expected = strings.TrimSpace(data.Expected)
			}
			continue
		}

		// Parse data line
		fields := strings.Split(line, ",")
		if len(fields) < 5 {
			continue // Skip invalid lines
		}

		sample := TestSample{}

		// Parse timestamp offset (minutes)
		if offset, err := strconv.Atoi(strings.TrimSpace(fields[0])); err == nil {
			sample.TimestampOffset = time.Duration(offset) * time.Minute
		}

		// Parse CPU millicores
		if cpu, err := strconv.ParseInt(strings.TrimSpace(fields[1]), 10, 64); err == nil {
			sample.CPUMillicores = cpu
		}

		// Parse memory bytes
		if mem, err := strconv.ParseInt(strings.TrimSpace(fields[2]), 10, 64); err == nil {
			sample.MemoryBytes = mem
		}

		// Parse request CPU
		if reqCPU, err := strconv.ParseInt(strings.TrimSpace(fields[3]), 10, 64); err == nil {
			sample.RequestCPU = reqCPU
		}

		// Parse request memory
		if reqMem, err := strconv.ParseInt(strings.TrimSpace(fields[4]), 10, 64); err == nil {
			sample.RequestMemory = reqMem
		}

		// Parse optional hour of day (for time pattern tests)
		if len(fields) > 5 {
			if hour, err := strconv.Atoi(strings.TrimSpace(fields[5])); err == nil {
				sample.HourOfDay = hour
			}
		}

		data.Samples = append(data.Samples, sample)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return data, nil
}

// TestIntegration_MemoryLeakDetection tests memory leak CSV data
func TestIntegration_MemoryLeakDetection(t *testing.T) {
	testdataDir := "testdata"
	csvPath := filepath.Join(testdataDir, "memory_leak.csv")

	data, err := loadCSVTestData(csvPath)
	if err != nil {
		t.Fatalf("Failed to load test data: %v", err)
	}

	t.Logf("Loaded %d samples from %s", len(data.Samples), csvPath)
	t.Logf("Description: %s", data.Description)
	t.Logf("Expected: %s", data.Expected)

	// Convert to leak detector samples
	baseTime := time.Now().Add(-3 * time.Hour) // Start 3 hours ago
	leakSamples := make([]leakdetector.MemorySample, len(data.Samples))

	for i, s := range data.Samples {
		leakSamples[i] = leakdetector.MemorySample{
			Timestamp: baseTime.Add(s.TimestampOffset),
			Bytes:     s.MemoryBytes,
		}
	}

	// Run leak detector
	detector := leakdetector.NewDetector()
	analysis := detector.Analyze(leakSamples)

	// Verify results
	t.Logf("\n=== Leak Analysis Results ===")
	t.Logf("Is Leak: %v", analysis.IsLeak)
	t.Logf("Severity: %s", analysis.Severity)
	t.Logf("Description: %s", analysis.Description)
	t.Logf("Confidence: %.1f%%", analysis.Confidence)
	t.Logf("Slope: %.2f MB/hour", analysis.Statistics.Slope/(1024*1024))
	t.Logf("R²: %.2f", analysis.Statistics.RSquared)
	t.Logf("Growth: %.1f%%", analysis.Statistics.GrowthPercent)
	t.Logf("Resets: %d", analysis.Statistics.ResetCount)

	// Expected: Memory leak should be detected
	if !analysis.IsLeak {
		t.Errorf("Expected memory leak to be detected, but IsLeak = false")
	}

	// Should prevent scaling
	shouldBlock, reason := analysis.ShouldPreventScaling()
	t.Logf("Should prevent scaling: %v - %s", shouldBlock, reason)

	if !shouldBlock {
		t.Errorf("Expected scaling to be blocked due to memory leak")
	}
}

// TestIntegration_StableUsage tests stable usage CSV data
func TestIntegration_StableUsage(t *testing.T) {
	testdataDir := "testdata"
	csvPath := filepath.Join(testdataDir, "stable_usage.csv")

	data, err := loadCSVTestData(csvPath)
	if err != nil {
		t.Fatalf("Failed to load test data: %v", err)
	}

	t.Logf("Loaded %d samples from %s", len(data.Samples), csvPath)
	t.Logf("Description: %s", data.Description)
	t.Logf("Expected: %s", data.Expected)

	// Convert to leak detector samples
	baseTime := time.Now().Add(-3 * time.Hour)
	leakSamples := make([]leakdetector.MemorySample, len(data.Samples))

	for i, s := range data.Samples {
		leakSamples[i] = leakdetector.MemorySample{
			Timestamp: baseTime.Add(s.TimestampOffset),
			Bytes:     s.MemoryBytes,
		}
	}

	// Run leak detector - should NOT detect a leak (normal GC pattern)
	detector := leakdetector.NewDetector()
	analysis := detector.Analyze(leakSamples)

	t.Logf("\n=== Leak Analysis Results ===")
	t.Logf("Is Leak: %v", analysis.IsLeak)
	t.Logf("Description: %s", analysis.Description)
	t.Logf("Slope: %.2f MB/hour", analysis.Statistics.Slope/(1024*1024))
	t.Logf("R²: %.2f", analysis.Statistics.RSquared)

	// Expected: No memory leak (normal GC sawtooth pattern)
	if analysis.IsLeak {
		t.Errorf("Expected no memory leak (stable usage), but IsLeak = true")
	}

	// Should NOT prevent scaling
	shouldBlock, _ := analysis.ShouldPreventScaling()
	if shouldBlock {
		t.Errorf("Expected scaling to be allowed for stable usage")
	}

	// Test recommendation engine - should recommend scale down
	engine := NewEngine()

	// Create container samples for recommendation
	baseTimeRec := time.Now().Add(-3 * time.Hour)
	containerSamples := make([]containerSample, len(data.Samples))
	for i, s := range data.Samples {
		containerSamples[i] = containerSample{
			timestamp:     baseTimeRec.Add(s.TimestampOffset),
			usageCPU:      s.CPUMillicores,
			usageMemory:   s.MemoryBytes,
			requestCPU:    s.RequestCPU,
			requestMemory: s.RequestMemory,
		}
	}

	// Generate recommendation
	rec := engine.generateContainerRecommendation(
		"test-container",
		containerSamples,
		95, // CPU percentile
		95, // Memory percentile
		1.2, // Safety margin
		10, // Min samples
		nil,
	)

	if rec != nil {
		t.Logf("\n=== Recommendation Results ===")
		t.Logf("Current CPU: %dm, Recommended CPU: %dm", rec.CurrentCPU, rec.RecommendedCPU)
		t.Logf("Current Memory: %dMi, Recommended Memory: %dMi",
			rec.CurrentMemory/(1024*1024), rec.RecommendedMemory/(1024*1024))
		t.Logf("Confidence: %.1f%%", rec.Confidence)

		// Expected: Recommend scale down (usage is much less than request)
		if rec.RecommendedCPU >= rec.CurrentCPU {
			t.Logf("Note: CPU recommendation suggests keeping or increasing (actual usage near request)")
		}
		if rec.RecommendedMemory >= rec.CurrentMemory {
			t.Logf("Note: Memory recommendation suggests keeping or increasing (actual usage near request)")
		}

		// The key check: actual usage should be significantly less than request
		cpuUsageRatio := float64(rec.RecommendedCPU) / float64(rec.CurrentCPU)
		memUsageRatio := float64(rec.RecommendedMemory) / float64(rec.CurrentMemory)
		t.Logf("CPU usage ratio: %.2f, Memory usage ratio: %.2f", cpuUsageRatio, memUsageRatio)

		// With stable usage pattern, recommended should be less than current
		if cpuUsageRatio >= 1.0 {
			t.Errorf("Expected CPU scale down recommendation (ratio < 1), got %.2f", cpuUsageRatio)
		}
		if memUsageRatio >= 1.0 {
			t.Errorf("Expected Memory scale down recommendation (ratio < 1), got %.2f", memUsageRatio)
		}
	}
}

// TestIntegration_BusinessHoursPattern tests business hours CSV data
func TestIntegration_BusinessHoursPattern(t *testing.T) {
	testdataDir := "testdata"
	csvPath := filepath.Join(testdataDir, "business_hours.csv")

	data, err := loadCSVTestData(csvPath)
	if err != nil {
		t.Fatalf("Failed to load test data: %v", err)
	}

	t.Logf("Loaded %d samples from %s", len(data.Samples), csvPath)
	t.Logf("Description: %s", data.Description)
	t.Logf("Expected: %s", data.Expected)

	// Convert to time pattern samples
	// Use a fixed base time so we can set specific hours
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	patternSamples := make([]timepattern.Sample, len(data.Samples))

	for i, s := range data.Samples {
		// For business hours pattern, use the hour from CSV if available
		sampleTime := baseTime.Add(s.TimestampOffset)
		patternSamples[i] = timepattern.Sample{
			Timestamp: sampleTime,
			CPU:       s.CPUMillicores,
			Memory:    s.MemoryBytes,
		}
	}

	// Run time pattern analyzer with adjusted settings for test data
	// (2 days = 2 samples per hour, lower MinDataPoints from default 3)
	analyzer := timepattern.NewAnalyzer()
	analyzer.MinDataPoints = 2
	pattern := analyzer.Analyze(patternSamples)

	t.Logf("\n=== Time Pattern Analysis Results ===")
	t.Logf("Has Pattern: %v", pattern.HasPattern)
	t.Logf("Pattern Type: %s", pattern.PatternType)
	t.Logf("Description: %s", pattern.Description)
	t.Logf("Peak Hours: %v", pattern.PeakHours)
	t.Logf("Off-Peak Hours: %v", pattern.OffPeakHours)
	t.Logf("CV: %.2f%%", pattern.OverallStats.CoefficientOfVar*100)

	// Expected: Should detect business hours pattern
	if !pattern.HasPattern {
		t.Errorf("Expected time pattern to be detected, but HasPattern = false")
	}

	// Should recommend schedule-based scaling
	if pattern.ScalingRecommendation != nil {
		t.Logf("\n=== Scaling Recommendation ===")
		t.Logf("Enabled: %v", pattern.ScalingRecommendation.Enabled)
		t.Logf("Reason: %s", pattern.ScalingRecommendation.Reason)
		t.Logf("Estimated Savings: %.1f%%", pattern.ScalingRecommendation.EstimatedSavingsPercent)

		if !pattern.ScalingRecommendation.Enabled {
			t.Errorf("Expected schedule-based scaling to be recommended")
		}

		for _, schedule := range pattern.ScalingRecommendation.Schedules {
			t.Logf("  Schedule: %s - %s (CPU: %.0f%%, Memory: %.0f%%)",
				schedule.Name, schedule.CronSchedule,
				schedule.CPUMultiplier*100, schedule.MemoryMultiplier*100)
		}
	}
}

// TestIntegration_HighUsage tests high usage CSV data
func TestIntegration_HighUsage(t *testing.T) {
	testdataDir := "testdata"
	csvPath := filepath.Join(testdataDir, "high_usage.csv")

	data, err := loadCSVTestData(csvPath)
	if err != nil {
		t.Fatalf("Failed to load test data: %v", err)
	}

	t.Logf("Loaded %d samples from %s", len(data.Samples), csvPath)
	t.Logf("Description: %s", data.Description)
	t.Logf("Expected: %s", data.Expected)

	// Test recommendation engine
	engine := NewEngine()

	baseTime := time.Now().Add(-3 * time.Hour)
	containerSamples := make([]containerSample, len(data.Samples))
	for i, s := range data.Samples {
		containerSamples[i] = containerSample{
			timestamp:     baseTime.Add(s.TimestampOffset),
			usageCPU:      s.CPUMillicores,
			usageMemory:   s.MemoryBytes,
			requestCPU:    s.RequestCPU,
			requestMemory: s.RequestMemory,
		}
	}

	rec := engine.generateContainerRecommendation(
		"test-container",
		containerSamples,
		99, // Use P99 for high usage pattern
		99,
		1.2,
		10,
		nil,
	)

	if rec != nil {
		t.Logf("\n=== Recommendation Results ===")
		t.Logf("Current CPU: %dm, Recommended CPU: %dm", rec.CurrentCPU, rec.RecommendedCPU)
		t.Logf("Current Memory: %dMi, Recommended Memory: %dMi",
			rec.CurrentMemory/(1024*1024), rec.RecommendedMemory/(1024*1024))
		t.Logf("Confidence: %.1f%%", rec.Confidence)

		cpuUsageRatio := float64(rec.RecommendedCPU) / float64(rec.CurrentCPU)
		memUsageRatio := float64(rec.RecommendedMemory) / float64(rec.CurrentMemory)
		t.Logf("CPU usage ratio: %.2f, Memory usage ratio: %.2f", cpuUsageRatio, memUsageRatio)

		// Expected: Scale UP - usage is very close to limits
		// The recommended values should be >= current values when using P99 + safety margin
		// on data that's already near the limit
		if cpuUsageRatio < 0.95 {
			t.Errorf("Expected CPU to stay same or scale up (high usage), got ratio %.2f", cpuUsageRatio)
		}
		if memUsageRatio < 0.9 {
			t.Errorf("Expected Memory to stay same or scale up (high usage), got ratio %.2f", memUsageRatio)
		}
	}

	// Also verify no memory leak
	leakSamples := make([]leakdetector.MemorySample, len(data.Samples))
	for i, s := range data.Samples {
		leakSamples[i] = leakdetector.MemorySample{
			Timestamp: baseTime.Add(s.TimestampOffset),
			Bytes:     s.MemoryBytes,
		}
	}

	detector := leakdetector.NewDetector()
	analysis := detector.Analyze(leakSamples)

	t.Logf("\n=== Leak Check ===")
	t.Logf("Is Leak: %v", analysis.IsLeak)
	t.Logf("Description: %s", analysis.Description)

	// High usage is NOT a memory leak - it's just high but stable
	if analysis.IsLeak {
		t.Logf("Warning: High usage pattern incorrectly flagged as leak")
	}
}

// TestIntegration_AllScenarios runs all CSV test scenarios
func TestIntegration_AllScenarios(t *testing.T) {
	testdataDir := "testdata"
	files, err := filepath.Glob(filepath.Join(testdataDir, "*.csv"))
	if err != nil {
		t.Fatalf("Failed to list test files: %v", err)
	}

	t.Logf("Found %d test data files", len(files))

	for _, file := range files {
		data, err := loadCSVTestData(file)
		if err != nil {
			t.Errorf("Failed to load %s: %v", file, err)
			continue
		}

		t.Logf("\n=== %s ===", filepath.Base(file))
		t.Logf("Samples: %d", len(data.Samples))
		t.Logf("Description: %s", data.Description)
		t.Logf("Expected: %s", data.Expected)
	}
}

// TestIntegration_RecommendationExpiry tests the recommendation expiry logic
func TestIntegration_RecommendationExpiry(t *testing.T) {
	// Create a recommendation
	now := time.Now()
	rec := &WorkloadRecommendation{
		Namespace:    "test",
		WorkloadKind: "Deployment",
		WorkloadName: "test-app",
		GeneratedAt:  now,
		ExpiresAt:    now.Add(24 * time.Hour),
		Containers: []ContainerRecommendation{
			{ContainerName: "app", Confidence: 85.0},
		},
	}

	// Test non-expired recommendation
	if rec.IsExpired() {
		t.Error("Fresh recommendation should not be expired")
	}

	shouldApply, reason := rec.ShouldApply(70.0)
	if !shouldApply {
		t.Errorf("Should apply fresh recommendation with high confidence: %s", reason)
	}

	// Test expiry status
	status := rec.ExpiryStatus()
	if !strings.Contains(status, "Valid") {
		t.Errorf("Expected 'Valid' in status, got: %s", status)
	}

	// Create expired recommendation
	expiredRec := &WorkloadRecommendation{
		Namespace:    "test",
		WorkloadKind: "Deployment",
		WorkloadName: "expired-app",
		GeneratedAt:  now.Add(-48 * time.Hour),
		ExpiresAt:    now.Add(-24 * time.Hour),
		Containers: []ContainerRecommendation{
			{ContainerName: "app", Confidence: 90.0},
		},
	}

	if !expiredRec.IsExpired() {
		t.Error("Old recommendation should be expired")
	}

	shouldApply, reason = expiredRec.ShouldApply(70.0)
	if shouldApply {
		t.Error("Should not apply expired recommendation")
	}
	if !strings.Contains(reason, "expired") {
		t.Errorf("Reason should mention expiry: %s", reason)
	}

	// Test FilterExpired
	recs := []WorkloadRecommendation{*rec, *expiredRec}
	valid := FilterExpired(recs)
	if len(valid) != 1 {
		t.Errorf("Expected 1 valid recommendation after filtering, got %d", len(valid))
	}
	if valid[0].WorkloadName != "test-app" {
		t.Error("Wrong recommendation kept after filtering")
	}

	// Test low confidence rejection
	lowConfidenceRec := &WorkloadRecommendation{
		Namespace:    "test",
		WorkloadKind: "Deployment",
		WorkloadName: "low-conf-app",
		GeneratedAt:  now,
		ExpiresAt:    now.Add(24 * time.Hour),
		Containers: []ContainerRecommendation{
			{ContainerName: "app", Confidence: 45.0},
		},
	}

	shouldApply, reason = lowConfidenceRec.ShouldApply(70.0)
	if shouldApply {
		t.Error("Should not apply low confidence recommendation")
	}
	if !strings.Contains(reason, "confidence") {
		t.Errorf("Reason should mention confidence: %s", reason)
	}
}

// TestIntegration_EndToEnd runs a complete end-to-end simulation
func TestIntegration_EndToEnd(t *testing.T) {
	t.Log("=== End-to-End Integration Test ===")
	t.Log("Testing complete pipeline: CSV -> Analyzers -> Recommendations")

	testdataDir := "testdata"
	scenarios := []struct {
		file               string
		expectLeak         bool
		expectScaleDown    bool
		expectTimePattern  bool
		expectScaleUp      bool
		description        string
	}{
		{
			file:            "memory_leak.csv",
			expectLeak:      true,
			expectScaleDown: false,
			description:     "Memory leak should be detected, scaling blocked",
		},
		{
			file:            "stable_usage.csv",
			expectLeak:      false,
			expectScaleDown: true,
			description:     "Stable low usage should recommend scale down",
		},
		{
			file:              "business_hours.csv",
			expectLeak:        false,
			expectTimePattern: true,
			description:       "Business hours pattern should be detected",
		},
		{
			file:          "high_usage.csv",
			expectLeak:    false,
			expectScaleUp: true,
			description:   "High usage should recommend scale up or maintain",
		},
	}

	leakDetector := leakdetector.NewDetector()
	timeAnalyzer := timepattern.NewAnalyzer()
	timeAnalyzer.MinDataPoints = 2 // Adjusted for test data (2 samples per hour)
	engine := NewEngine()

	for _, scenario := range scenarios {
		t.Logf("\n--- Testing: %s ---", scenario.file)
		t.Logf("Description: %s", scenario.description)

		csvPath := filepath.Join(testdataDir, scenario.file)
		data, err := loadCSVTestData(csvPath)
		if err != nil {
			t.Errorf("Failed to load %s: %v", scenario.file, err)
			continue
		}

		baseTime := time.Now().Add(-3 * time.Hour)

		// Run leak detection
		leakSamples := make([]leakdetector.MemorySample, len(data.Samples))
		for i, s := range data.Samples {
			leakSamples[i] = leakdetector.MemorySample{
				Timestamp: baseTime.Add(s.TimestampOffset),
				Bytes:     s.MemoryBytes,
			}
		}
		leakAnalysis := leakDetector.Analyze(leakSamples)

		// Run time pattern analysis
		patternSamples := make([]timepattern.Sample, len(data.Samples))
		for i, s := range data.Samples {
			patternSamples[i] = timepattern.Sample{
				Timestamp: baseTime.Add(s.TimestampOffset),
				CPU:       s.CPUMillicores,
				Memory:    s.MemoryBytes,
			}
		}
		timePattern := timeAnalyzer.Analyze(patternSamples)

		// Run recommendation engine
		containerSamples := make([]containerSample, len(data.Samples))
		for i, s := range data.Samples {
			containerSamples[i] = containerSample{
				timestamp:     baseTime.Add(s.TimestampOffset),
				usageCPU:      s.CPUMillicores,
				usageMemory:   s.MemoryBytes,
				requestCPU:    s.RequestCPU,
				requestMemory: s.RequestMemory,
			}
		}
		rec := engine.generateContainerRecommendation(
			"test-container",
			containerSamples,
			95, 95, 1.2, 10, nil,
		)

		// Validate expectations
		if scenario.expectLeak {
			if !leakAnalysis.IsLeak {
				t.Errorf("[%s] Expected memory leak, got none", scenario.file)
			} else {
				t.Logf("[%s] ✓ Memory leak detected (severity: %s)", scenario.file, leakAnalysis.Severity)
			}
		} else {
			if leakAnalysis.IsLeak {
				t.Errorf("[%s] Unexpected memory leak detected", scenario.file)
			} else {
				t.Logf("[%s] ✓ No memory leak (correct)", scenario.file)
			}
		}

		if scenario.expectTimePattern {
			if !timePattern.HasPattern {
				t.Errorf("[%s] Expected time pattern, got none", scenario.file)
			} else {
				t.Logf("[%s] ✓ Time pattern detected: %s", scenario.file, timePattern.PatternType)
			}
		}

		if rec != nil {
			cpuRatio := float64(rec.RecommendedCPU) / float64(rec.CurrentCPU)
			memRatio := float64(rec.RecommendedMemory) / float64(rec.CurrentMemory)

			if scenario.expectScaleDown {
				if cpuRatio >= 1.0 && memRatio >= 1.0 {
					t.Errorf("[%s] Expected scale down, but got scale up/maintain", scenario.file)
				} else {
					t.Logf("[%s] ✓ Scale down recommended (CPU: %.0f%%, Memory: %.0f%%)",
						scenario.file, cpuRatio*100, memRatio*100)
				}
			}

			if scenario.expectScaleUp {
				// For high usage, we expect the recommendation to be close to current (maintaining)
				// or slightly higher due to safety margin
				if cpuRatio < 0.9 {
					t.Errorf("[%s] Expected scale up/maintain, but got significant scale down", scenario.file)
				} else {
					t.Logf("[%s] ✓ Scale up/maintain recommended (CPU: %.0f%%, Memory: %.0f%%)",
						scenario.file, cpuRatio*100, memRatio*100)
				}
			}
		}
	}

	t.Log("\n=== End-to-End Test Complete ===")
}
