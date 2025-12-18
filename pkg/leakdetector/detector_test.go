package leakdetector

import (
	"testing"
	"time"
)

const (
	MB = 1024 * 1024
	GB = 1024 * MB
)

// Helper to generate samples with linear growth
func generateLeakingSamples(start time.Time, duration time.Duration, interval time.Duration, startMem int64, growthPerHour int64) []MemorySample {
	samples := []MemorySample{}
	current := start
	mem := startMem

	for current.Before(start.Add(duration)) {
		samples = append(samples, MemorySample{
			Timestamp: current,
			Bytes:     mem,
		})
		current = current.Add(interval)
		mem += int64(float64(growthPerHour) * interval.Hours())
	}

	return samples
}

// Helper to generate stable memory samples with GC-like drops
func generateStableSamples(start time.Time, duration time.Duration, interval time.Duration, baseMem int64, variance int64) []MemorySample {
	samples := []MemorySample{}
	current := start
	mem := baseMem
	growing := true
	peak := baseMem

	for current.Before(start.Add(duration)) {
		samples = append(samples, MemorySample{
			Timestamp: current,
			Bytes:     mem,
		})
		current = current.Add(interval)

		if growing {
			mem += variance / 10
			if mem > peak+variance {
				peak = mem
				growing = false // Start dropping (GC)
			}
		} else {
			mem -= variance / 5 // Drop faster than grow
			if mem < baseMem {
				mem = baseMem
				growing = true
			}
		}
	}

	return samples
}

func TestDetectorInsufficientSamples(t *testing.T) {
	detector := NewDetector()
	detector.MinSamples = 20

	samples := []MemorySample{
		{Timestamp: time.Now(), Bytes: 100 * MB},
		{Timestamp: time.Now().Add(time.Hour), Bytes: 110 * MB},
	}

	analysis := detector.Analyze(samples)

	if analysis.IsLeak {
		t.Error("should not detect leak with insufficient samples")
	}
	// With insufficient samples, analysis returns early without calculating stats
	if analysis.Description == "" {
		t.Error("should have description explaining insufficient samples")
	}
}

func TestDetectorInsufficientDuration(t *testing.T) {
	detector := NewDetector()
	detector.MinSamples = 5
	detector.MinDuration = 1 * time.Hour

	start := time.Now()
	samples := []MemorySample{}
	for i := 0; i < 10; i++ {
		samples = append(samples, MemorySample{
			Timestamp: start.Add(time.Duration(i) * time.Minute), // Only 10 minutes
			Bytes:     int64(100+i) * MB,
		})
	}

	analysis := detector.Analyze(samples)

	if analysis.IsLeak {
		t.Error("should not detect leak with insufficient duration")
	}
}

func TestDetectorStableMemory(t *testing.T) {
	detector := NewDetector()
	detector.MinSamples = 10

	start := time.Now()
	samples := generateStableSamples(start, 4*time.Hour, 10*time.Minute, 500*MB, 50*MB)

	analysis := detector.Analyze(samples)

	if analysis.IsLeak {
		t.Errorf("should not detect leak in stable memory pattern. Analysis: %s", analysis.Description)
	}
}

func TestDetectorClearLeak(t *testing.T) {
	detector := NewDetector()
	detector.MinSamples = 10
	detector.MinDuration = 1 * time.Hour

	start := time.Now()
	// 50 MB/hour growth - clear leak
	samples := generateLeakingSamples(start, 4*time.Hour, 5*time.Minute, 500*MB, 50*MB)

	analysis := detector.Analyze(samples)

	if !analysis.IsLeak {
		t.Errorf("should detect clear memory leak. Analysis: %s", analysis.Description)
	}
	if analysis.Severity == SeverityNone {
		t.Error("severity should not be none for a leak")
	}
	if analysis.Confidence <= 0 {
		t.Error("confidence should be positive for a leak")
	}

	// Verify statistics
	if analysis.Statistics.Slope <= 0 {
		t.Errorf("slope should be positive, got %.2f", analysis.Statistics.Slope)
	}
	if analysis.Statistics.RSquared < 0.9 {
		t.Errorf("R² should be high for linear growth, got %.2f", analysis.Statistics.RSquared)
	}

	t.Logf("Analysis: %s", analysis.Description)
	t.Logf("Severity: %s, Confidence: %.0f%%", analysis.Severity, analysis.Confidence)
}

func TestDetectorSeverityClassification(t *testing.T) {
	detector := NewDetector()
	detector.MinSamples = 10

	// Based on classifySeverity thresholds:
	// Critical: >= 100 MB/hour or >= 10% growth/hour
	// High: >= 50 MB/hour or >= 5% growth/hour
	// Medium: >= 10 MB/hour or >= 2% growth/hour
	// Low: everything else above threshold
	tests := []struct {
		name             string
		growthMBPerHour  int64
		expectedSeverity LeakSeverity
	}{
		{"moderate leak", 15 * MB, SeverityMedium},    // 15 MB/hour -> Medium
		{"fast leak", 50 * MB, SeverityHigh},          // 50 MB/hour -> High
		{"critical leak", 100 * MB, SeverityCritical}, // 100 MB/hour -> Critical
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()
			samples := generateLeakingSamples(start, 4*time.Hour, 5*time.Minute, 500*MB, tt.growthMBPerHour)

			analysis := detector.Analyze(samples)

			if !analysis.IsLeak {
				t.Fatalf("should detect leak for %s", tt.name)
			}
			if analysis.Severity != tt.expectedSeverity {
				t.Errorf("expected severity %s, got %s (growth: %.2f MB/h)",
					tt.expectedSeverity, analysis.Severity, analysis.Statistics.Slope/float64(MB))
			}
		})
	}
}

func TestDetectorSlowLeak(t *testing.T) {
	detector := NewDetector()
	detector.MinSamples = 10
	detector.SlopeThreshold = 1 * MB     // 1 MB/hour threshold
	detector.GrowthRateThreshold = 0.05  // 5% growth threshold (lowered)

	// Create a slow leak - 8 MB/hour over 6 hours from 500MB = ~9.6% growth
	start := time.Now()
	samples := generateLeakingSamples(start, 6*time.Hour, 5*time.Minute, 500*MB, 8*MB)

	analysis := detector.Analyze(samples)

	if !analysis.IsLeak {
		t.Errorf("should detect slow leak. Slope: %.2f MB/hour, Growth: %.1f%%, Description: %s",
			analysis.Statistics.Slope/float64(MB), analysis.Statistics.GrowthPercent, analysis.Description)
		return
	}
	if analysis.Severity != SeverityLow {
		t.Errorf("slow leak should be low severity, got %s", analysis.Severity)
	}
}

func TestDetectorResetDetection(t *testing.T) {
	detector := NewDetector()
	detector.MinSamples = 10
	detector.MaxResets = 2

	start := time.Now()
	samples := []MemorySample{}

	// Create pattern with multiple resets (like normal GC)
	mem := int64(500 * MB)
	for i := 0; i < 48; i++ { // 4 hours at 5 min intervals
		samples = append(samples, MemorySample{
			Timestamp: start.Add(time.Duration(i) * 5 * time.Minute),
			Bytes:     mem,
		})

		// Grow for a while, then reset
		if i%12 == 11 { // Reset every hour
			mem = 500 * MB // Drop back to baseline
		} else {
			mem += 10 * MB
		}
	}

	analysis := detector.Analyze(samples)

	if analysis.IsLeak {
		t.Errorf("should not detect leak with frequent resets. Resets: %d, Description: %s",
			analysis.Statistics.ResetCount, analysis.Description)
	}
}

func TestDetectorNoResets(t *testing.T) {
	detector := NewDetector()
	detector.MinSamples = 10

	start := time.Now()
	// Pure linear growth with no resets
	samples := generateLeakingSamples(start, 6*time.Hour, 5*time.Minute, 500*MB, 30*MB)

	analysis := detector.Analyze(samples)

	if !analysis.IsLeak {
		t.Errorf("should detect leak with no resets: %s", analysis.Description)
	}
	if analysis.Statistics.ResetCount != 0 {
		t.Errorf("expected 0 resets, got %d", analysis.Statistics.ResetCount)
	}
}

func TestDetectorRSquared(t *testing.T) {
	detector := NewDetector()
	detector.MinSamples = 10
	detector.ConsistencyThreshold = 0.7

	start := time.Now()

	t.Run("high consistency", func(t *testing.T) {
		// Pure linear growth should have R² close to 1
		samples := generateLeakingSamples(start, 4*time.Hour, 5*time.Minute, 500*MB, 30*MB)
		analysis := detector.Analyze(samples)

		if analysis.Statistics.RSquared < 0.95 {
			t.Errorf("expected high R² for linear growth, got %.4f", analysis.Statistics.RSquared)
		}
	})

	t.Run("low consistency", func(t *testing.T) {
		// Noisy data should have lower R²
		samples := []MemorySample{}
		mem := int64(500 * MB)
		for i := 0; i < 48; i++ {
			// Add random-like noise
			noise := int64((i % 7) * 20 * MB)
			if i%3 == 0 {
				noise = -noise
			}
			samples = append(samples, MemorySample{
				Timestamp: start.Add(time.Duration(i) * 5 * time.Minute),
				Bytes:     mem + noise,
			})
			mem += 5 * MB
		}

		analysis := detector.Analyze(samples)
		if analysis.Statistics.RSquared > 0.8 {
			t.Errorf("expected lower R² for noisy data, got %.4f", analysis.Statistics.RSquared)
		}
	})
}

func TestDetectorProjections(t *testing.T) {
	detector := NewDetector()
	detector.MinSamples = 10

	start := time.Now()
	growthPerHour := int64(20 * MB)
	startMem := int64(500 * MB)
	duration := 4 * time.Hour

	samples := generateLeakingSamples(start, duration, 5*time.Minute, startMem, growthPerHour)

	analysis := detector.Analyze(samples)

	// Check projections are reasonable
	expectedEndMem := startMem + int64(duration.Hours())*growthPerHour
	expected24h := expectedEndMem + 24*growthPerHour
	expected7d := expectedEndMem + 24*7*growthPerHour

	// Allow 10% tolerance
	tolerance := 0.1

	if !withinTolerance(analysis.Statistics.ProjectedMemory24h, expected24h, tolerance) {
		t.Errorf("24h projection off: expected ~%s, got %s",
			formatBytes(expected24h), formatBytes(analysis.Statistics.ProjectedMemory24h))
	}

	if !withinTolerance(analysis.Statistics.ProjectedMemory7d, expected7d, tolerance) {
		t.Errorf("7d projection off: expected ~%s, got %s",
			formatBytes(expected7d), formatBytes(analysis.Statistics.ProjectedMemory7d))
	}
}

func TestDetectorWithLimit(t *testing.T) {
	detector := NewDetector()
	detector.MinSamples = 10

	start := time.Now()
	startMem := int64(500 * MB)
	growthPerHour := int64(50 * MB)
	memoryLimit := int64(2 * GB)

	samples := generateLeakingSamples(start, 4*time.Hour, 5*time.Minute, startMem, growthPerHour)

	analysis := detector.AnalyzeWithLimit(samples, memoryLimit)

	if !analysis.IsLeak {
		t.Fatal("should detect leak")
	}

	// Should calculate time to OOM
	if analysis.Statistics.TimeToOOM <= 0 {
		t.Error("should calculate time to OOM with limit")
	}

	// Verify time to OOM is reasonable
	currentMem := analysis.Statistics.EndMemory
	remainingMem := memoryLimit - currentMem
	expectedHoursToOOM := float64(remainingMem) / float64(growthPerHour)

	actualHoursToOOM := analysis.Statistics.TimeToOOM.Hours()
	if !withinTolerance(int64(actualHoursToOOM), int64(expectedHoursToOOM), 0.2) {
		t.Errorf("time to OOM off: expected ~%.1f hours, got %.1f hours",
			expectedHoursToOOM, actualHoursToOOM)
	}

	t.Logf("Time to OOM: %v", analysis.Statistics.TimeToOOM.Round(time.Minute))
}

func TestDetectorAlert(t *testing.T) {
	detector := NewDetector()
	detector.MinSamples = 10

	start := time.Now()
	samples := generateLeakingSamples(start, 4*time.Hour, 5*time.Minute, 500*MB, 50*MB)

	analysis := detector.Analyze(samples)

	if !analysis.IsLeak {
		t.Fatal("should detect leak")
	}

	if analysis.Alert == nil {
		t.Fatal("should generate alert for leak")
	}

	alert := analysis.Alert
	if alert.Title == "" {
		t.Error("alert should have title")
	}
	if alert.Message == "" {
		t.Error("alert should have message")
	}
	if alert.GrowthRate == "" {
		t.Error("alert should have growth rate")
	}
	if len(alert.SuggestedActions) == 0 {
		t.Error("alert should have suggested actions")
	}

	t.Logf("Alert Title: %s", alert.Title)
	t.Logf("Suggested Actions: %v", alert.SuggestedActions)
}

func TestShouldPreventScaling(t *testing.T) {
	tests := []struct {
		name           string
		isLeak         bool
		severity       LeakSeverity
		shouldPrevent  bool
	}{
		{"no leak", false, SeverityNone, false},
		{"low severity leak", true, SeverityLow, false},
		{"medium severity leak", true, SeverityMedium, true},
		{"high severity leak", true, SeverityHigh, true},
		{"critical leak", true, SeverityCritical, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := &LeakAnalysis{
				IsLeak:   tt.isLeak,
				Severity: tt.severity,
			}

			prevent, reason := analysis.ShouldPreventScaling()

			if prevent != tt.shouldPrevent {
				t.Errorf("expected shouldPrevent=%v, got %v (reason: %s)", tt.shouldPrevent, prevent, reason)
			}
		})
	}
}

func TestFormatAnalysisSummary(t *testing.T) {
	detector := NewDetector()
	detector.MinSamples = 10

	start := time.Now()
	samples := generateLeakingSamples(start, 4*time.Hour, 5*time.Minute, 500*MB, 30*MB)

	analysis := detector.Analyze(samples)
	summary := analysis.FormatAnalysisSummary()

	if summary == "" {
		t.Error("summary should not be empty")
	}

	// Check for key sections
	requiredStrings := []string{
		"Memory Leak Analysis",
		"Leak Detected",
		"Severity",
		"Statistics",
		"Samples Analyzed",
		"Growth Rate",
	}

	for _, s := range requiredStrings {
		if !containsSubstring(summary, s) {
			t.Errorf("summary should contain '%s'", s)
		}
	}

	t.Log(summary)
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{500, "500 bytes"},
		{1024, "1.00 KB"},
		{1024 * 1024, "1.00 MB"},
		{1024 * 1024 * 1024, "1.00 GB"},
		{1536 * 1024 * 1024, "1.50 GB"},
	}

	for _, tt := range tests {
		result := formatBytes(tt.bytes)
		if result != tt.expected {
			t.Errorf("formatBytes(%d) = %s, expected %s", tt.bytes, result, tt.expected)
		}
	}
}

func TestLinearRegression(t *testing.T) {
	detector := NewDetector()

	// Perfect linear growth: 10 MB/hour
	start := time.Now()
	samples := []MemorySample{}
	for i := 0; i < 10; i++ {
		samples = append(samples, MemorySample{
			Timestamp: start.Add(time.Duration(i) * time.Hour),
			Bytes:     int64(100+i*10) * MB,
		})
	}

	slope, rSquared := detector.calculateLinearRegression(samples)

	// Slope should be close to 10 MB/hour
	expectedSlope := float64(10 * MB)
	if !withinTolerance(int64(slope), int64(expectedSlope), 0.01) {
		t.Errorf("slope should be ~10 MB/hour, got %.2f MB/hour", slope/float64(MB))
	}

	// R² should be very close to 1 for perfect linear data
	if rSquared < 0.99 {
		t.Errorf("R² should be ~1 for perfect linear data, got %.4f", rSquared)
	}
}

// Helper functions
func withinTolerance(actual, expected int64, tolerance float64) bool {
	if expected == 0 {
		return actual == 0
	}
	diff := float64(actual-expected) / float64(expected)
	return diff >= -tolerance && diff <= tolerance
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
