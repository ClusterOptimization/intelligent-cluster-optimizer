package recommendation

import (
	"testing"
	"time"
)

func TestConfidenceCalculator_DurationScore(t *testing.T) {
	calc := NewConfidenceCalculator()

	tests := []struct {
		name          string
		hours         float64
		expectedMin   float64
		expectedMax   float64
		expectedLevel ConfidenceLevel
	}{
		{
			name:          "less than 1 hour (very low)",
			hours:         0.5,
			expectedMin:   0,
			expectedMax:   20,
			expectedLevel: ConfidenceLevelVeryLow,
		},
		{
			name:          "exactly 1 hour (minimum threshold)",
			hours:         1.0,
			expectedMin:   20,
			expectedMax:   30,
			expectedLevel: ConfidenceLevelLow,
		},
		{
			name:          "6 hours of data",
			hours:         6.0,
			expectedMin:   40,
			expectedMax:   60,
			expectedLevel: ConfidenceLevelMedium,
		},
		{
			name:          "24 hours (1 day)",
			hours:         24.0,
			expectedMin:   50,
			expectedMax:   70,
			expectedLevel: ConfidenceLevelMedium,
		},
		{
			name:          "168 hours (1 week - ideal)",
			hours:         168.0,
			expectedMin:   100,
			expectedMax:   100,
			expectedLevel: ConfidenceLevelVeryHigh,
		},
		{
			name:          "more than 1 week",
			hours:         336.0,
			expectedMin:   100,
			expectedMax:   100,
			expectedLevel: ConfidenceLevelVeryHigh,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := calc.calculateDurationScore(tt.hours)
			if score < tt.expectedMin || score > tt.expectedMax {
				t.Errorf("calculateDurationScore(%v) = %v, expected between %v and %v",
					tt.hours, score, tt.expectedMin, tt.expectedMax)
			}
		})
	}
}

func TestConfidenceCalculator_SampleScore(t *testing.T) {
	calc := NewConfidenceCalculator()

	tests := []struct {
		name        string
		samples     int
		expectedMin float64
		expectedMax float64
	}{
		{
			name:        "5 samples (below minimum)",
			samples:     5,
			expectedMin: 0,
			expectedMax: 15,
		},
		{
			name:        "10 samples (minimum threshold)",
			samples:     10,
			expectedMin: 20,
			expectedMax: 30,
		},
		{
			name:        "50 samples",
			samples:     50,
			expectedMin: 40,
			expectedMax: 60,
		},
		{
			name:        "100 samples",
			samples:     100,
			expectedMin: 50,
			expectedMax: 70,
		},
		{
			name:        "500 samples (ideal)",
			samples:     500,
			expectedMin: 100,
			expectedMax: 100,
		},
		{
			name:        "1000 samples (above ideal)",
			samples:     1000,
			expectedMin: 100,
			expectedMax: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := calc.calculateSampleScore(tt.samples)
			if score < tt.expectedMin || score > tt.expectedMax {
				t.Errorf("calculateSampleScore(%v) = %v, expected between %v and %v",
					tt.samples, score, tt.expectedMin, tt.expectedMax)
			}
		})
	}
}

func TestConfidenceCalculator_ConsistencyScore(t *testing.T) {
	calc := NewConfidenceCalculator()

	tests := []struct {
		name        string
		cv          float64
		expectedMin float64
		expectedMax float64
	}{
		{
			name:        "zero variation",
			cv:          0,
			expectedMin: 50,
			expectedMax: 50,
		},
		{
			name:        "very consistent (5% CV)",
			cv:          0.05,
			expectedMin: 100,
			expectedMax: 100,
		},
		{
			name:        "consistent (10% CV)",
			cv:          0.10,
			expectedMin: 100,
			expectedMax: 100,
		},
		{
			name:        "moderate variation (25% CV)",
			cv:          0.25,
			expectedMin: 50,
			expectedMax: 80,
		},
		{
			name:        "high variation (50% CV - max acceptable)",
			cv:          0.50,
			expectedMin: 20,
			expectedMax: 25,
		},
		{
			name:        "very high variation (100% CV)",
			cv:          1.0,
			expectedMin: 20,
			expectedMax: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := calc.calculateConsistencyScore(tt.cv)
			if score < tt.expectedMin || score > tt.expectedMax {
				t.Errorf("calculateConsistencyScore(%v) = %v, expected between %v and %v",
					tt.cv, score, tt.expectedMin, tt.expectedMax)
			}
		})
	}
}

func TestConfidenceCalculator_RecencyScore(t *testing.T) {
	calc := NewConfidenceCalculator()

	tests := []struct {
		name        string
		age         time.Duration
		expectedMin float64
		expectedMax float64
	}{
		{
			name:        "just now",
			age:         0,
			expectedMin: 100,
			expectedMax: 100,
		},
		{
			name:        "5 minutes ago",
			age:         5 * time.Minute,
			expectedMin: 90,
			expectedMax: 100,
		},
		{
			name:        "1 hour ago (max acceptable)",
			age:         1 * time.Hour,
			expectedMin: 80,
			expectedMax: 85,
		},
		{
			name:        "6 hours ago",
			age:         6 * time.Hour,
			expectedMin: 50,
			expectedMax: 70,
		},
		{
			name:        "24 hours ago",
			age:         24 * time.Hour,
			expectedMin: 20,
			expectedMax: 25,
		},
		{
			name:        "48 hours ago",
			age:         48 * time.Hour,
			expectedMin: 20,
			expectedMax: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := calc.calculateRecencyScore(tt.age)
			if score < tt.expectedMin || score > tt.expectedMax {
				t.Errorf("calculateRecencyScore(%v) = %v, expected between %v and %v",
					tt.age, score, tt.expectedMin, tt.expectedMax)
			}
		})
	}
}

func TestConfidenceCalculator_CoverageScore(t *testing.T) {
	calc := NewConfidenceCalculator()
	now := time.Now()

	tests := []struct {
		name        string
		summary     MetricsSummary
		expectedMin float64
		expectedMax float64
	}{
		{
			name: "no gaps",
			summary: MetricsSummary{
				SampleCount:  100,
				OldestSample: now.Add(-24 * time.Hour),
				NewestSample: now,
				TimeGaps:     nil,
			},
			expectedMin: 100,
			expectedMax: 100,
		},
		{
			name: "small gap (5% of total time)",
			summary: MetricsSummary{
				SampleCount:  100,
				OldestSample: now.Add(-24 * time.Hour),
				NewestSample: now,
				TimeGaps:     []time.Duration{time.Hour}, // ~4% of 24h
			},
			expectedMin: 95,
			expectedMax: 100,
		},
		{
			name: "significant gaps (25% of total time)",
			summary: MetricsSummary{
				SampleCount:  100,
				OldestSample: now.Add(-24 * time.Hour),
				NewestSample: now,
				TimeGaps:     []time.Duration{6 * time.Hour}, // 25% of 24h
			},
			expectedMin: 50,
			expectedMax: 70,
		},
		{
			name: "large gaps (50% of total time)",
			summary: MetricsSummary{
				SampleCount:  100,
				OldestSample: now.Add(-24 * time.Hour),
				NewestSample: now,
				TimeGaps:     []time.Duration{12 * time.Hour}, // 50% of 24h
			},
			expectedMin: 20,
			expectedMax: 25,
		},
		{
			name: "insufficient samples for coverage",
			summary: MetricsSummary{
				SampleCount: 1,
			},
			expectedMin: 50,
			expectedMax: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := calc.calculateCoverageScore(tt.summary)
			if score < tt.expectedMin || score > tt.expectedMax {
				t.Errorf("calculateCoverageScore() = %v, expected between %v and %v",
					score, tt.expectedMin, tt.expectedMax)
			}
		})
	}
}

func TestConfidenceCalculator_ScoreToLevel(t *testing.T) {
	calc := NewConfidenceCalculator()

	tests := []struct {
		score    float64
		expected ConfidenceLevel
	}{
		{0, ConfidenceLevelVeryLow},
		{10, ConfidenceLevelVeryLow},
		{19.9, ConfidenceLevelVeryLow},
		{20, ConfidenceLevelLow},
		{30, ConfidenceLevelLow},
		{39.9, ConfidenceLevelLow},
		{40, ConfidenceLevelMedium},
		{50, ConfidenceLevelMedium},
		{59.9, ConfidenceLevelMedium},
		{60, ConfidenceLevelHigh},
		{70, ConfidenceLevelHigh},
		{79.9, ConfidenceLevelHigh},
		{80, ConfidenceLevelVeryHigh},
		{90, ConfidenceLevelVeryHigh},
		{100, ConfidenceLevelVeryHigh},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			level := calc.scoreToLevel(tt.score)
			if level != tt.expected {
				t.Errorf("scoreToLevel(%v) = %v, expected %v", tt.score, level, tt.expected)
			}
		})
	}
}

func TestConfidenceCalculator_CalculateFromSamples(t *testing.T) {
	calc := NewConfidenceCalculator()

	t.Run("no data", func(t *testing.T) {
		score := calc.CalculateFromSamples(nil, nil, 30*time.Second)
		if score.Score != 0 {
			t.Errorf("expected score 0 for no data, got %v", score.Score)
		}
		if score.Level != ConfidenceLevelVeryLow {
			t.Errorf("expected VeryLow level for no data, got %v", score.Level)
		}
		if len(score.Warnings) == 0 {
			t.Error("expected warnings for no data")
		}
	})

	t.Run("minimal data - 1 hour", func(t *testing.T) {
		now := time.Now()
		timestamps := make([]time.Time, 120) // 2 samples per minute for 1 hour
		values := make([]int64, 120)
		for i := 0; i < 120; i++ {
			timestamps[i] = now.Add(-time.Duration(60-i/2) * time.Minute)
			values[i] = 100 + int64(i%10) // Some variation
		}

		score := calc.CalculateFromSamples(timestamps, values, 30*time.Second)

		// With only 1 hour of data but 120 samples (well above minimum),
		// score is moderate due to good sample count, recency, and coverage
		// Duration score is low, but other factors bring it up
		if score.Score < 50 || score.Score > 85 {
			t.Errorf("expected moderate confidence for 1h of data with good samples, got %v", score.Score)
		}
		// Duration score specifically should reflect the limited data
		if score.DataDurationScore > 30 {
			t.Errorf("expected low duration score for 1h of data, got %v", score.DataDurationScore)
		}
	})

	t.Run("good data - 1 week", func(t *testing.T) {
		now := time.Now()
		// 1 sample every 30 seconds for a week = 20160 samples
		// Let's use 500 samples spread over a week for testing
		timestamps := make([]time.Time, 500)
		values := make([]int64, 500)
		for i := 0; i < 500; i++ {
			// Spread samples over 1 week
			offset := time.Duration(float64(i) / 500 * float64(168*time.Hour))
			timestamps[i] = now.Add(-168*time.Hour + offset)
			values[i] = 100 + int64(i%20) // Some variation
		}

		score := calc.CalculateFromSamples(timestamps, values, 30*time.Second)

		// With 1 week of data and 500 samples, confidence should be high
		if score.Score < 60 {
			t.Errorf("expected high confidence for 1 week of data, got %v", score.Score)
		}
	})
}

func TestConfidenceCalculator_GenerateWarnings(t *testing.T) {
	calc := NewConfidenceCalculator()

	t.Run("low duration warning", func(t *testing.T) {
		now := time.Now()
		score := ConfidenceScore{
			DataDurationScore: 30,
			DataDurationHours: 0.5,
		}
		warnings := calc.generateWarnings(score, MetricsSummary{
			OldestSample: now.Add(-30 * time.Minute),
			NewestSample: now,
		})

		found := false
		for _, w := range warnings {
			if len(w) > 0 && w[0:8] == "Limited " {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected warning about limited data history")
		}
	})

	t.Run("low sample count warning", func(t *testing.T) {
		score := ConfidenceScore{
			SampleCountScore: 30,
			SampleCount:      15,
		}
		warnings := calc.generateWarnings(score, MetricsSummary{SampleCount: 15})

		found := false
		for _, w := range warnings {
			if len(w) > 0 && w[0:3] == "Low" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected warning about low sample count")
		}
	})

	t.Run("high variability warning", func(t *testing.T) {
		score := ConfidenceScore{
			DataConsistencyScore:   30,
			CoefficientOfVariation: 0.75,
		}
		warnings := calc.generateWarnings(score, MetricsSummary{})

		found := false
		for _, w := range warnings {
			if len(w) > 0 && w[0:4] == "High" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected warning about high variability")
		}
	})

	t.Run("stale data warning", func(t *testing.T) {
		score := ConfidenceScore{
			RecencyScore:    30,
			NewestSampleAge: 12 * time.Hour,
		}
		warnings := calc.generateWarnings(score, MetricsSummary{})

		found := false
		for _, w := range warnings {
			if len(w) > 0 && w[0:5] == "Stale" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected warning about stale data")
		}
	})
}

func TestConfidenceScore_FormatScoreBreakdown(t *testing.T) {
	score := &ConfidenceScore{
		Score:                  75.5,
		Level:                  ConfidenceLevelHigh,
		DataDurationScore:      80.0,
		DataDurationHours:      48.0,
		SampleCountScore:       70.0,
		SampleCount:            200,
		DataConsistencyScore:   85.0,
		CoefficientOfVariation: 0.15,
		RecencyScore:           90.0,
		NewestSampleAge:        5 * time.Minute,
		CoverageScore:          65.0,
	}

	breakdown := score.FormatScoreBreakdown()

	if breakdown == "" {
		t.Error("expected non-empty breakdown")
	}

	// Verify it contains key elements
	if !containsSubstring(breakdown, "75.5%") {
		t.Error("expected breakdown to contain overall score")
	}
	if !containsSubstring(breakdown, "High") {
		t.Error("expected breakdown to contain confidence level")
	}
	if !containsSubstring(breakdown, "48.0 hours") {
		t.Error("expected breakdown to contain duration")
	}
	if !containsSubstring(breakdown, "200 samples") {
		t.Error("expected breakdown to contain sample count")
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestConfidenceLevel_String(t *testing.T) {
	tests := []struct {
		level    ConfidenceLevel
		expected string
	}{
		{ConfidenceLevelVeryLow, "VeryLow"},
		{ConfidenceLevelLow, "Low"},
		{ConfidenceLevelMedium, "Medium"},
		{ConfidenceLevelHigh, "High"},
		{ConfidenceLevelVeryHigh, "VeryHigh"},
	}

	for _, tt := range tests {
		if tt.level.String() != tt.expected {
			t.Errorf("ConfidenceLevel.String() = %v, expected %v", tt.level.String(), tt.expected)
		}
	}
}
