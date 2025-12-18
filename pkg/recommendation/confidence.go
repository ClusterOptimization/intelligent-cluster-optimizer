package recommendation

import (
	"fmt"
	"math"
	"time"
)

// ConfidenceScore represents a detailed confidence assessment for a recommendation
type ConfidenceScore struct {
	// Overall confidence score (0-100)
	Score float64

	// Individual factor scores (0-100 each)
	DataDurationScore   float64 // Based on hours of historical data
	SampleCountScore    float64 // Based on number of metric samples
	DataConsistencyScore float64 // Based on variance/stability of metrics
	RecencyScore        float64 // Based on how recent the data is
	CoverageScore       float64 // Based on time coverage (gaps in data)

	// Raw metrics used for scoring
	DataDurationHours float64
	SampleCount       int
	OldestSampleAge   time.Duration
	NewestSampleAge   time.Duration
	CoefficientOfVariation float64 // Standard deviation / mean

	// Human-readable assessment
	Level       ConfidenceLevel
	Description string
	Warnings    []string
}

// ConfidenceLevel represents categorical confidence levels
type ConfidenceLevel string

const (
	ConfidenceLevelVeryLow  ConfidenceLevel = "VeryLow"   // 0-20%
	ConfidenceLevelLow      ConfidenceLevel = "Low"       // 20-40%
	ConfidenceLevelMedium   ConfidenceLevel = "Medium"    // 40-60%
	ConfidenceLevelHigh     ConfidenceLevel = "High"      // 60-80%
	ConfidenceLevelVeryHigh ConfidenceLevel = "VeryHigh"  // 80-100%
)

// ConfidenceConfig contains thresholds for confidence calculation
type ConfidenceConfig struct {
	// Minimum samples for any confidence
	MinSamples int

	// Ideal number of samples for 100% sample score
	IdealSamples int

	// Minimum hours of data for any confidence
	MinDataHours float64

	// Ideal hours of data for 100% duration score
	IdealDataHours float64

	// Maximum acceptable coefficient of variation for high consistency score
	MaxAcceptableCV float64

	// Maximum age of newest sample for high recency score
	MaxAcceptableRecency time.Duration

	// Weights for combining scores (should sum to 1.0)
	WeightDataDuration   float64
	WeightSampleCount    float64
	WeightDataConsistency float64
	WeightRecency        float64
	WeightCoverage       float64
}

// DefaultConfidenceConfig returns sensible default thresholds
func DefaultConfidenceConfig() ConfidenceConfig {
	return ConfidenceConfig{
		MinSamples:           10,
		IdealSamples:         500,
		MinDataHours:         1,
		IdealDataHours:       168, // 1 week
		MaxAcceptableCV:      0.5, // 50% coefficient of variation
		MaxAcceptableRecency: 1 * time.Hour,

		// Weights (sum to 1.0)
		WeightDataDuration:   0.25,
		WeightSampleCount:    0.25,
		WeightDataConsistency: 0.20,
		WeightRecency:        0.15,
		WeightCoverage:       0.15,
	}
}

// ConfidenceCalculator calculates confidence scores for recommendations
type ConfidenceCalculator struct {
	config ConfidenceConfig
}

// NewConfidenceCalculator creates a new calculator with default config
func NewConfidenceCalculator() *ConfidenceCalculator {
	return &ConfidenceCalculator{
		config: DefaultConfidenceConfig(),
	}
}

// NewConfidenceCalculatorWithConfig creates a calculator with custom config
func NewConfidenceCalculatorWithConfig(config ConfidenceConfig) *ConfidenceCalculator {
	return &ConfidenceCalculator{
		config: config,
	}
}

// MetricsSummary contains summary statistics for confidence calculation
type MetricsSummary struct {
	SampleCount    int
	OldestSample   time.Time
	NewestSample   time.Time
	Mean           float64
	StdDev         float64
	Min            int64
	Max            int64
	TimeGaps       []time.Duration // Gaps larger than expected interval
	ExpectedInterval time.Duration  // Expected time between samples
}

// CalculateConfidence computes a detailed confidence score
func (c *ConfidenceCalculator) CalculateConfidence(summary MetricsSummary) ConfidenceScore {
	score := ConfidenceScore{
		SampleCount: summary.SampleCount,
		Warnings:    []string{},
	}

	// Calculate data duration
	if !summary.OldestSample.IsZero() && !summary.NewestSample.IsZero() {
		duration := summary.NewestSample.Sub(summary.OldestSample)
		score.DataDurationHours = duration.Hours()
		score.OldestSampleAge = time.Since(summary.OldestSample)
		score.NewestSampleAge = time.Since(summary.NewestSample)
	}

	// Calculate coefficient of variation
	if summary.Mean > 0 {
		score.CoefficientOfVariation = summary.StdDev / summary.Mean
	}

	// Calculate individual scores
	score.DataDurationScore = c.calculateDurationScore(score.DataDurationHours)
	score.SampleCountScore = c.calculateSampleScore(summary.SampleCount)
	score.DataConsistencyScore = c.calculateConsistencyScore(score.CoefficientOfVariation)
	score.RecencyScore = c.calculateRecencyScore(score.NewestSampleAge)
	score.CoverageScore = c.calculateCoverageScore(summary)

	// Calculate weighted overall score
	score.Score = c.config.WeightDataDuration*score.DataDurationScore +
		c.config.WeightSampleCount*score.SampleCountScore +
		c.config.WeightDataConsistency*score.DataConsistencyScore +
		c.config.WeightRecency*score.RecencyScore +
		c.config.WeightCoverage*score.CoverageScore

	// Ensure score is in valid range
	score.Score = clamp(score.Score, 0, 100)

	// Determine confidence level and description
	score.Level = c.scoreToLevel(score.Score)
	score.Description = c.generateDescription(score)
	score.Warnings = c.generateWarnings(score, summary)

	return score
}

// calculateDurationScore scores based on hours of data (0-100)
func (c *ConfidenceCalculator) calculateDurationScore(hours float64) float64 {
	if hours < c.config.MinDataHours {
		// Below minimum, very low score
		return (hours / c.config.MinDataHours) * 20
	}

	if hours >= c.config.IdealDataHours {
		return 100
	}

	// Logarithmic scaling between min and ideal
	// This gives diminishing returns as we get more data
	minLog := math.Log(c.config.MinDataHours)
	idealLog := math.Log(c.config.IdealDataHours)
	currentLog := math.Log(hours)

	progress := (currentLog - minLog) / (idealLog - minLog)
	return 20 + (progress * 80) // Scale from 20 to 100
}

// calculateSampleScore scores based on number of samples (0-100)
func (c *ConfidenceCalculator) calculateSampleScore(samples int) float64 {
	if samples < c.config.MinSamples {
		return float64(samples) / float64(c.config.MinSamples) * 20
	}

	if samples >= c.config.IdealSamples {
		return 100
	}

	// Logarithmic scaling
	minLog := math.Log(float64(c.config.MinSamples))
	idealLog := math.Log(float64(c.config.IdealSamples))
	currentLog := math.Log(float64(samples))

	progress := (currentLog - minLog) / (idealLog - minLog)
	return 20 + (progress * 80)
}

// calculateConsistencyScore scores based on data consistency (0-100)
// Lower coefficient of variation = more consistent = higher score
func (c *ConfidenceCalculator) calculateConsistencyScore(cv float64) float64 {
	if cv <= 0 {
		// No variation (or invalid) - could mean constant load or not enough data
		return 50 // Neutral score
	}

	if cv <= 0.1 {
		// Very consistent (CV < 10%)
		return 100
	}

	if cv >= c.config.MaxAcceptableCV {
		// Too much variation
		return 20
	}

	// Linear interpolation between 0.1 and MaxAcceptableCV
	progress := (cv - 0.1) / (c.config.MaxAcceptableCV - 0.1)
	return 100 - (progress * 80) // Scale from 100 down to 20
}

// calculateRecencyScore scores based on how recent the newest data is (0-100)
func (c *ConfidenceCalculator) calculateRecencyScore(age time.Duration) float64 {
	if age <= 0 {
		return 100 // Very recent
	}

	if age <= c.config.MaxAcceptableRecency {
		// Within acceptable recency
		progress := float64(age) / float64(c.config.MaxAcceptableRecency)
		return 100 - (progress * 20) // 80-100
	}

	// Older than acceptable - penalize
	hoursOld := age.Hours()
	if hoursOld >= 24 {
		return 20 // Data is a day or more old
	}

	// Scale between 1 hour and 24 hours
	progress := (hoursOld - 1) / 23
	return 80 - (progress * 60) // Scale from 80 down to 20
}

// calculateCoverageScore scores based on time coverage / gaps (0-100)
func (c *ConfidenceCalculator) calculateCoverageScore(summary MetricsSummary) float64 {
	if summary.SampleCount < 2 {
		return 50 // Not enough samples to assess coverage
	}

	if len(summary.TimeGaps) == 0 {
		return 100 // No significant gaps
	}

	// Calculate total gap time
	var totalGapTime time.Duration
	for _, gap := range summary.TimeGaps {
		totalGapTime += gap
	}

	// Calculate expected total time
	totalTime := summary.NewestSample.Sub(summary.OldestSample)
	if totalTime <= 0 {
		return 50
	}

	// Gap ratio - what percentage of time is gaps
	gapRatio := float64(totalGapTime) / float64(totalTime)

	if gapRatio <= 0.05 {
		return 100 // Less than 5% gaps
	}
	if gapRatio >= 0.5 {
		return 20 // More than 50% gaps
	}

	// Linear interpolation
	progress := (gapRatio - 0.05) / 0.45
	return 100 - (progress * 80)
}

// scoreToLevel converts a numeric score to a confidence level
func (c *ConfidenceCalculator) scoreToLevel(score float64) ConfidenceLevel {
	switch {
	case score >= 80:
		return ConfidenceLevelVeryHigh
	case score >= 60:
		return ConfidenceLevelHigh
	case score >= 40:
		return ConfidenceLevelMedium
	case score >= 20:
		return ConfidenceLevelLow
	default:
		return ConfidenceLevelVeryLow
	}
}

// generateDescription creates a human-readable description of the confidence score
func (c *ConfidenceCalculator) generateDescription(score ConfidenceScore) string {
	var durationDesc string
	switch {
	case score.DataDurationHours < 1:
		durationDesc = fmt.Sprintf("%.0f minutes", score.DataDurationHours*60)
	case score.DataDurationHours < 24:
		durationDesc = fmt.Sprintf("%.1f hours", score.DataDurationHours)
	case score.DataDurationHours < 168:
		durationDesc = fmt.Sprintf("%.1f days", score.DataDurationHours/24)
	default:
		durationDesc = fmt.Sprintf("%.1f weeks", score.DataDurationHours/168)
	}

	return fmt.Sprintf("%s confidence (%.0f%%) based on %d samples over %s",
		score.Level, score.Score, score.SampleCount, durationDesc)
}

// generateWarnings creates warnings for low-scoring factors
func (c *ConfidenceCalculator) generateWarnings(score ConfidenceScore, summary MetricsSummary) []string {
	var warnings []string

	if score.DataDurationScore < 40 {
		warnings = append(warnings, fmt.Sprintf("Limited data history: only %.1f hours of data (recommend at least %.0f hours)",
			score.DataDurationHours, c.config.MinDataHours*4))
	}

	if score.SampleCountScore < 40 {
		warnings = append(warnings, fmt.Sprintf("Low sample count: only %d samples (recommend at least %d)",
			score.SampleCount, c.config.MinSamples*5))
	}

	if score.DataConsistencyScore < 40 {
		warnings = append(warnings, fmt.Sprintf("High variability in metrics: CV=%.2f (resource usage is highly variable)",
			score.CoefficientOfVariation))
	}

	if score.RecencyScore < 40 {
		warnings = append(warnings, fmt.Sprintf("Stale data: newest sample is %.1f hours old",
			score.NewestSampleAge.Hours()))
	}

	if score.CoverageScore < 40 && len(summary.TimeGaps) > 0 {
		warnings = append(warnings, fmt.Sprintf("Data gaps detected: %d significant gaps in collection",
			len(summary.TimeGaps)))
	}

	return warnings
}

// CalculateFromSamples is a convenience method to calculate confidence from raw samples
func (c *ConfidenceCalculator) CalculateFromSamples(timestamps []time.Time, values []int64, expectedInterval time.Duration) ConfidenceScore {
	if len(timestamps) == 0 || len(values) == 0 {
		return ConfidenceScore{
			Score:       0,
			Level:       ConfidenceLevelVeryLow,
			Description: "No data available",
			Warnings:    []string{"No metric samples available for analysis"},
		}
	}

	summary := c.computeMetricsSummary(timestamps, values, expectedInterval)
	return c.CalculateConfidence(summary)
}

// computeMetricsSummary computes summary statistics from raw data
func (c *ConfidenceCalculator) computeMetricsSummary(timestamps []time.Time, values []int64, expectedInterval time.Duration) MetricsSummary {
	n := len(values)
	if n == 0 {
		return MetricsSummary{}
	}

	// Find min, max, oldest, newest
	var minVal, maxVal int64 = values[0], values[0]
	oldest, newest := timestamps[0], timestamps[0]

	var sum float64
	for i, v := range values {
		if v < minVal {
			minVal = v
		}
		if v > maxVal {
			maxVal = v
		}
		sum += float64(v)

		t := timestamps[i]
		if t.Before(oldest) {
			oldest = t
		}
		if t.After(newest) {
			newest = t
		}
	}

	mean := sum / float64(n)

	// Calculate standard deviation
	var sumSquaredDiff float64
	for _, v := range values {
		diff := float64(v) - mean
		sumSquaredDiff += diff * diff
	}
	stdDev := math.Sqrt(sumSquaredDiff / float64(n))

	// Detect time gaps (gaps larger than 2x expected interval)
	var gaps []time.Duration
	if expectedInterval > 0 && len(timestamps) > 1 {
		// Sort timestamps
		sortedTimes := make([]time.Time, len(timestamps))
		copy(sortedTimes, timestamps)
		sortTimeSlice(sortedTimes)

		gapThreshold := expectedInterval * 2
		for i := 1; i < len(sortedTimes); i++ {
			gap := sortedTimes[i].Sub(sortedTimes[i-1])
			if gap > gapThreshold {
				gaps = append(gaps, gap)
			}
		}
	}

	return MetricsSummary{
		SampleCount:      n,
		OldestSample:     oldest,
		NewestSample:     newest,
		Mean:             mean,
		StdDev:           stdDev,
		Min:              minVal,
		Max:              maxVal,
		TimeGaps:         gaps,
		ExpectedInterval: expectedInterval,
	}
}

// sortTimeSlice sorts a slice of times in ascending order
func sortTimeSlice(times []time.Time) {
	for i := 0; i < len(times); i++ {
		for j := i + 1; j < len(times); j++ {
			if times[j].Before(times[i]) {
				times[i], times[j] = times[j], times[i]
			}
		}
	}
}

// clamp ensures a value is within the specified range
func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// String returns a string representation of ConfidenceLevel
func (l ConfidenceLevel) String() string {
	return string(l)
}

// FormatScoreBreakdown returns a detailed breakdown of the confidence score
func (s *ConfidenceScore) FormatScoreBreakdown() string {
	return fmt.Sprintf(`Confidence Score: %.1f%% (%s)
  ├─ Data Duration:   %.1f%% (%.1f hours of data)
  ├─ Sample Count:    %.1f%% (%d samples)
  ├─ Data Consistency: %.1f%% (CV=%.2f)
  ├─ Recency:         %.1f%% (newest: %v ago)
  └─ Coverage:        %.1f%%`,
		s.Score, s.Level,
		s.DataDurationScore, s.DataDurationHours,
		s.SampleCountScore, s.SampleCount,
		s.DataConsistencyScore, s.CoefficientOfVariation,
		s.RecencyScore, s.NewestSampleAge.Round(time.Minute),
		s.CoverageScore)
}
