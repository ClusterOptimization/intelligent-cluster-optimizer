package anomaly

import (
	"fmt"
	"math"
	"time"
)

// ZScoreDetector detects anomalies using the Z-Score (Standard Score) method.
// A Z-Score measures how many standard deviations a value is from the mean.
// Values with |Z| > threshold are considered anomalies.
//
// The Z-Score is calculated as: Z = (x - μ) / σ
// where x is the value, μ is the mean, and σ is the standard deviation.
//
// This method assumes the data is approximately normally distributed.
// It works well for detecting outliers in stable, normally distributed data.
type ZScoreDetector struct {
	// Threshold for considering a value an anomaly (default: 3.0)
	// A threshold of 3.0 means values more than 3 standard deviations
	// from the mean are flagged (covers 99.7% of normal distribution)
	Threshold float64

	// MinSamples is the minimum number of data points required
	MinSamples int
}

// NewZScoreDetector creates a new Z-Score detector with default settings
func NewZScoreDetector() *ZScoreDetector {
	return &ZScoreDetector{
		Threshold:  3.0,
		MinSamples: 10,
	}
}

// NewZScoreDetectorWithConfig creates a Z-Score detector with custom config
func NewZScoreDetectorWithConfig(config *Config) *ZScoreDetector {
	return &ZScoreDetector{
		Threshold:  config.ZScoreThreshold,
		MinSamples: config.MinSamples,
	}
}

// Name returns the detector's method name
func (d *ZScoreDetector) Name() DetectionMethod {
	return MethodZScore
}

// Detect analyzes the data and returns anomalies using Z-Score method
func (d *ZScoreDetector) Detect(data []float64) *DetectionResult {
	return d.DetectWithTimestamps(data, nil)
}

// DetectWithTimestamps analyzes data with associated timestamps
func (d *ZScoreDetector) DetectWithTimestamps(data []float64, timestamps []time.Time) *DetectionResult {
	result := &DetectionResult{
		Method:      MethodZScore,
		Threshold:   d.Threshold,
		SampleCount: len(data),
		Anomalies:   []Anomaly{},
	}

	if len(data) < d.MinSamples {
		return result
	}

	// Calculate statistics
	mean := calculateMean(data)
	stdDev := calculateStdDev(data, mean)
	result.Mean = mean
	result.StdDev = stdDev
	result.MinValue, result.MaxValue = findMinMax(data)

	// Avoid division by zero
	if stdDev == 0 {
		return result
	}

	// Calculate expected bounds
	lowerBound := mean - d.Threshold*stdDev
	upperBound := mean + d.Threshold*stdDev

	// Detect anomalies
	for i, value := range data {
		zScore := (value - mean) / stdDev

		if math.Abs(zScore) > d.Threshold {
			var ts time.Time
			if timestamps != nil && i < len(timestamps) {
				ts = timestamps[i]
			}

			anomalyType := AnomalyTypeCPUSpike
			if value < mean {
				anomalyType = AnomalyTypeCPUDrop
			}

			anomaly := Anomaly{
				Timestamp:     ts,
				Type:          anomalyType,
				Severity:      determineSeverity(zScore),
				DetectedBy:    MethodZScore,
				Value:         value,
				ExpectedLower: lowerBound,
				ExpectedUpper: upperBound,
				Deviation:     zScore,
				Index:         i,
				Message: fmt.Sprintf("Z-Score %.2f exceeds threshold %.2f (value=%.2f, mean=%.2f, stdDev=%.2f)",
					zScore, d.Threshold, value, mean, stdDev),
			}

			result.Anomalies = append(result.Anomalies, anomaly)
		}
	}

	return result
}

// calculateMean calculates the arithmetic mean of a slice
func calculateMean(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}

	var sum float64
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
}

// calculateStdDev calculates the standard deviation given the mean
func calculateStdDev(data []float64, mean float64) float64 {
	if len(data) < 2 {
		return 0
	}

	var sumSquares float64
	for _, v := range data {
		diff := v - mean
		sumSquares += diff * diff
	}

	// Using sample standard deviation (n-1)
	variance := sumSquares / float64(len(data)-1)
	return math.Sqrt(variance)
}

// findMinMax finds the minimum and maximum values in a slice
func findMinMax(data []float64) (min, max float64) {
	if len(data) == 0 {
		return 0, 0
	}

	min, max = data[0], data[0]
	for _, v := range data[1:] {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	return min, max
}
