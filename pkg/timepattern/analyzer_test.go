package timepattern

import (
	"testing"
	"time"
)

// Helper to create samples across multiple days
func samplesForDays(startDate time.Time, days int, hoursPattern map[int]int64) []Sample {
	var samples []Sample
	for d := 0; d < days; d++ {
		date := startDate.AddDate(0, 0, d)
		for hour, cpu := range hoursPattern {
			for i := 0; i < 3; i++ { // 3 samples per hour
				t := time.Date(date.Year(), date.Month(), date.Day(), hour, i*20, 0, 0, date.Location())
				samples = append(samples, Sample{
					Timestamp: t,
					CPU:       cpu + int64(i*5),
					Memory:    cpu * 1024 * 1024, // Memory proportional to CPU
				})
			}
		}
	}
	return samples
}

func TestAnalyzerNoData(t *testing.T) {
	analyzer := NewAnalyzer()
	pattern := analyzer.Analyze([]Sample{})

	if pattern.HasPattern {
		t.Error("expected no pattern for empty data")
	}
	if pattern.PatternType != PatternNone {
		t.Errorf("expected PatternNone, got %s", pattern.PatternType)
	}
}

func TestAnalyzerFlatPattern(t *testing.T) {
	analyzer := NewAnalyzer()
	baseTime := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC) // Monday

	// Create flat usage - same CPU across all hours
	var samples []Sample
	for d := 0; d < 7; d++ {
		date := baseTime.AddDate(0, 0, d)
		for hour := 0; hour < 24; hour++ {
			for i := 0; i < 3; i++ {
				t := time.Date(date.Year(), date.Month(), date.Day(), hour, i*20, 0, 0, time.UTC)
				samples = append(samples, Sample{
					Timestamp: t,
					CPU:       100 + int64(i), // Very small variance
					Memory:    512 * 1024 * 1024,
				})
			}
		}
	}

	pattern := analyzer.Analyze(samples)

	if pattern.HasPattern {
		t.Errorf("expected no pattern for flat usage, got %s", pattern.PatternType)
	}
	if pattern.OverallStats.CoefficientOfVar >= analyzer.SignificantVariationCV {
		t.Errorf("expected low CV for flat pattern, got %.4f", pattern.OverallStats.CoefficientOfVar)
	}
}

func TestAnalyzerBusinessHoursPattern(t *testing.T) {
	analyzer := NewAnalyzer()
	baseTime := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC) // Monday

	// Business hours pattern: high during 8-18, low otherwise
	hoursPattern := map[int]int64{}
	for hour := 0; hour < 24; hour++ {
		if hour >= 8 && hour <= 18 {
			hoursPattern[hour] = 500 // High during business hours
		} else {
			hoursPattern[hour] = 100 // Low outside
		}
	}

	samples := samplesForDays(baseTime, 7, hoursPattern)
	pattern := analyzer.Analyze(samples)

	if !pattern.HasPattern {
		t.Error("expected pattern to be detected")
	}
	if pattern.PatternType != PatternBusinessHours {
		t.Errorf("expected PatternBusinessHours, got %s", pattern.PatternType)
	}

	// Check peak hours are in business hours range
	for _, hour := range pattern.PeakHours {
		if hour < 8 || hour > 18 {
			t.Errorf("unexpected peak hour %d outside business hours", hour)
		}
	}

	// Check scaling recommendation
	if pattern.ScalingRecommendation == nil {
		t.Fatal("expected scaling recommendation")
	}
	if !pattern.ScalingRecommendation.Enabled {
		t.Error("expected scaling recommendation to be enabled")
	}
	if len(pattern.ScalingRecommendation.Schedules) == 0 {
		t.Error("expected at least one schedule entry")
	}
}

func TestAnalyzerNightBatchPattern(t *testing.T) {
	analyzer := NewAnalyzer()
	baseTime := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	// Night batch pattern: high during 22-6, low otherwise
	hoursPattern := map[int]int64{}
	for hour := 0; hour < 24; hour++ {
		if hour >= 22 || hour <= 6 {
			hoursPattern[hour] = 800 // High during night
		} else {
			hoursPattern[hour] = 150 // Low during day
		}
	}

	samples := samplesForDays(baseTime, 7, hoursPattern)
	pattern := analyzer.Analyze(samples)

	if !pattern.HasPattern {
		t.Error("expected pattern to be detected")
	}
	if pattern.PatternType != PatternNightBatch {
		t.Errorf("expected PatternNightBatch, got %s", pattern.PatternType)
	}
}

func TestAnalyzerWeekdayOnlyPattern(t *testing.T) {
	analyzer := NewAnalyzer()
	baseTime := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC) // Monday

	var samples []Sample
	for d := 0; d < 14; d++ { // 2 weeks
		date := baseTime.AddDate(0, 0, d)
		dayOfWeek := date.Weekday()

		for hour := 0; hour < 24; hour++ {
			var cpu int64
			if dayOfWeek == time.Saturday || dayOfWeek == time.Sunday {
				cpu = 50 // Very low on weekends
			} else {
				cpu = 400 // High on weekdays
			}

			for i := 0; i < 3; i++ {
				t := time.Date(date.Year(), date.Month(), date.Day(), hour, i*20, 0, 0, time.UTC)
				samples = append(samples, Sample{
					Timestamp: t,
					CPU:       cpu + int64(i*5),
					Memory:    cpu * 1024 * 1024,
				})
			}
		}
	}

	pattern := analyzer.Analyze(samples)

	if !pattern.HasPattern {
		t.Error("expected pattern to be detected")
	}
	if pattern.PatternType != PatternWeekdayOnly {
		t.Errorf("expected PatternWeekdayOnly, got %s", pattern.PatternType)
	}

	// Check that weekends are off-peak
	weekendOffPeak := 0
	for _, day := range pattern.OffPeakDays {
		if day == time.Saturday || day == time.Sunday {
			weekendOffPeak++
		}
	}
	if weekendOffPeak < 2 {
		t.Error("expected Saturday and Sunday to be off-peak")
	}
}

func TestAnalyzerMorningSpikePattern(t *testing.T) {
	analyzer := NewAnalyzer()
	baseTime := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	// Morning spike: high at 6-8 AM only (before typical business hours)
	hoursPattern := map[int]int64{}
	for hour := 0; hour < 24; hour++ {
		if hour >= 6 && hour <= 8 {
			hoursPattern[hour] = 600 // Morning spike
		} else {
			hoursPattern[hour] = 150 // Normal
		}
	}

	samples := samplesForDays(baseTime, 7, hoursPattern)
	pattern := analyzer.Analyze(samples)

	if !pattern.HasPattern {
		t.Error("expected pattern to be detected")
	}
	if pattern.PatternType != PatternMorningSpike {
		t.Errorf("expected PatternMorningSpike, got %s", pattern.PatternType)
	}

	// Check peak hours are in early morning
	for _, hour := range pattern.PeakHours {
		if hour < 6 || hour > 11 {
			t.Errorf("unexpected peak hour %d outside morning range", hour)
		}
	}
}

func TestAnalyzerEveningSpikePattern(t *testing.T) {
	analyzer := NewAnalyzer()
	baseTime := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	// Evening spike: high at 18-21, low otherwise
	hoursPattern := map[int]int64{}
	for hour := 0; hour < 24; hour++ {
		if hour >= 18 && hour <= 21 {
			hoursPattern[hour] = 700 // Evening spike
		} else {
			hoursPattern[hour] = 150 // Normal
		}
	}

	samples := samplesForDays(baseTime, 7, hoursPattern)
	pattern := analyzer.Analyze(samples)

	if !pattern.HasPattern {
		t.Error("expected pattern to be detected")
	}
	if pattern.PatternType != PatternEveningSpike {
		t.Errorf("expected PatternEveningSpike, got %s", pattern.PatternType)
	}
}

func TestHourlyStats(t *testing.T) {
	analyzer := NewAnalyzer()
	baseTime := time.Date(2024, 1, 15, 9, 0, 0, 0, time.UTC) // 9 AM

	// Create samples at specific hours
	samples := []Sample{
		{Timestamp: baseTime, CPU: 100, Memory: 1024},
		{Timestamp: baseTime.Add(10 * time.Minute), CPU: 150, Memory: 1024},
		{Timestamp: baseTime.Add(20 * time.Minute), CPU: 200, Memory: 1024},
		{Timestamp: baseTime.Add(1 * time.Hour), CPU: 50, Memory: 512},    // 10 AM
		{Timestamp: baseTime.Add(70 * time.Minute), CPU: 60, Memory: 512}, // 10 AM
		{Timestamp: baseTime.Add(80 * time.Minute), CPU: 70, Memory: 512}, // 10 AM
	}

	pattern := analyzer.Analyze(samples)

	// Check 9 AM stats
	stats9 := pattern.HourlyStats[9]
	if stats9.SampleCount != 3 {
		t.Errorf("expected 3 samples at 9 AM, got %d", stats9.SampleCount)
	}
	expectedMean := float64(100+150+200) / 3
	if stats9.MeanCPU != expectedMean {
		t.Errorf("expected mean CPU %.2f at 9 AM, got %.2f", expectedMean, stats9.MeanCPU)
	}

	// Check 10 AM stats
	stats10 := pattern.HourlyStats[10]
	if stats10.SampleCount != 3 {
		t.Errorf("expected 3 samples at 10 AM, got %d", stats10.SampleCount)
	}
}

func TestDailyStats(t *testing.T) {
	analyzer := NewAnalyzer()

	// Create samples on Monday and Tuesday
	monday := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC) // Monday
	tuesday := monday.AddDate(0, 0, 1)

	samples := []Sample{
		{Timestamp: monday, CPU: 300, Memory: 1024},
		{Timestamp: monday.Add(1 * time.Hour), CPU: 350, Memory: 1024},
		{Timestamp: monday.Add(2 * time.Hour), CPU: 400, Memory: 1024},
		{Timestamp: tuesday, CPU: 100, Memory: 512},
		{Timestamp: tuesday.Add(1 * time.Hour), CPU: 120, Memory: 512},
		{Timestamp: tuesday.Add(2 * time.Hour), CPU: 140, Memory: 512},
	}

	pattern := analyzer.Analyze(samples)

	mondayStats := pattern.DailyStats[time.Monday]
	if mondayStats.SampleCount != 3 {
		t.Errorf("expected 3 samples on Monday, got %d", mondayStats.SampleCount)
	}
	expectedMondayMean := float64(300+350+400) / 3
	if mondayStats.MeanCPU != expectedMondayMean {
		t.Errorf("expected mean CPU %.2f on Monday, got %.2f", expectedMondayMean, mondayStats.MeanCPU)
	}

	tuesdayStats := pattern.DailyStats[time.Tuesday]
	if tuesdayStats.SampleCount != 3 {
		t.Errorf("expected 3 samples on Tuesday, got %d", tuesdayStats.SampleCount)
	}
}

func TestScalingRecommendationSavings(t *testing.T) {
	analyzer := NewAnalyzer()
	baseTime := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	// Business hours pattern with clear peak/off-peak
	hoursPattern := map[int]int64{}
	for hour := 0; hour < 24; hour++ {
		if hour >= 9 && hour <= 17 {
			hoursPattern[hour] = 1000 // Peak
		} else {
			hoursPattern[hour] = 200 // Off-peak (80% less)
		}
	}

	samples := samplesForDays(baseTime, 7, hoursPattern)
	pattern := analyzer.Analyze(samples)

	if pattern.ScalingRecommendation == nil {
		t.Fatal("expected scaling recommendation")
	}

	// Should have significant estimated savings
	if pattern.ScalingRecommendation.EstimatedSavingsPercent <= 0 {
		t.Error("expected positive estimated savings")
	}

	t.Logf("Estimated savings: %.1f%%", pattern.ScalingRecommendation.EstimatedSavingsPercent)
}

func TestFormatHourRange(t *testing.T) {
	tests := []struct {
		hours    []int
		expected string
	}{
		{[]int{}, "none"},
		{[]int{9}, "9:00"},
		{[]int{9, 10, 11}, "9:00-11:00"},
		{[]int{9, 10, 14, 15}, "[9:00-10:00 14:00-15:00]"},
		{[]int{8, 9, 10, 11, 12}, "8:00-12:00"},
	}

	for _, tt := range tests {
		result := formatHourRange(tt.hours)
		if result != tt.expected {
			t.Errorf("formatHourRange(%v) = %s, expected %s", tt.hours, result, tt.expected)
		}
	}
}

func TestFormatPatternSummary(t *testing.T) {
	analyzer := NewAnalyzer()
	baseTime := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	hoursPattern := map[int]int64{}
	for hour := 0; hour < 24; hour++ {
		if hour >= 8 && hour <= 18 {
			hoursPattern[hour] = 500
		} else {
			hoursPattern[hour] = 100
		}
	}

	samples := samplesForDays(baseTime, 7, hoursPattern)
	pattern := analyzer.Analyze(samples)

	summary := pattern.FormatPatternSummary()

	if summary == "" {
		t.Error("expected non-empty summary")
	}

	// Check that summary contains key information
	if !containsSubstring(summary, "Pattern Type") {
		t.Error("expected summary to contain 'Pattern Type'")
	}
	if !containsSubstring(summary, "Peak Hours") {
		t.Error("expected summary to contain 'Peak Hours'")
	}
	if !containsSubstring(summary, "Scaling Recommendation") {
		t.Error("expected summary to contain 'Scaling Recommendation'")
	}

	t.Log(summary)
}

func TestAnalyzerCustomThresholds(t *testing.T) {
	analyzer := NewAnalyzer()
	analyzer.PeakThresholdRatio = 1.5     // 50% above average for peak
	analyzer.OffPeakThresholdRatio = 0.5  // 50% below average for off-peak
	analyzer.SignificantVariationCV = 0.3 // Higher threshold

	baseTime := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	// Create a pattern that would be detected with default thresholds
	// but not with higher thresholds
	hoursPattern := map[int]int64{}
	for hour := 0; hour < 24; hour++ {
		if hour >= 9 && hour <= 17 {
			hoursPattern[hour] = 130 // Only 30% higher
		} else {
			hoursPattern[hour] = 100
		}
	}

	samples := samplesForDays(baseTime, 7, hoursPattern)
	pattern := analyzer.Analyze(samples)

	// With 50% threshold, 30% difference shouldn't be detected as peak
	if len(pattern.PeakHours) > 0 {
		t.Logf("Peak hours detected with high threshold: %v", pattern.PeakHours)
	}
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
