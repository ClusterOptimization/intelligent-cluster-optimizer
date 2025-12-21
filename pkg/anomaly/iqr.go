package anomaly

import (
	"fmt"
	"sort"
	"time"
)

// IQRDetector detects anomalies using the Interquartile Range (IQR) method.
// IQR is a measure of statistical dispersion and is robust to outliers.
//
// The method works by:
// 1. Calculating Q1 (25th percentile) and Q3 (75th percentile)
// 2. Computing IQR = Q3 - Q1
// 3. Defining bounds as: [Q1 - k*IQR, Q3 + k*IQR] where k is typically 1.5
// 4. Flagging values outside these bounds as anomalies
//
// Unlike Z-Score, IQR doesn't assume normal distribution and is more robust
// to extreme outliers in the data (since median-based measures are used).
type IQRDetector struct {
	// Multiplier for IQR bounds (default: 1.5)
	// 1.5 = outliers, 3.0 = extreme outliers only
	Multiplier float64

	// MinSamples is the minimum number of data points required
	MinSamples int
}

// NewIQRDetector creates a new IQR detector with default settings
func NewIQRDetector() *IQRDetector {
	return &IQRDetector{
		Multiplier: 1.5,
		MinSamples: 10,
	}
}

// NewIQRDetectorWithConfig creates an IQR detector with custom config
func NewIQRDetectorWithConfig(config *Config) *IQRDetector {
	return &IQRDetector{
		Multiplier: config.IQRMultiplier,
		MinSamples: config.MinSamples,
	}
}

// Name returns the detector's method name
func (d *IQRDetector) Name() DetectionMethod {
	return MethodIQR
}

// Detect analyzes the data and returns anomalies using IQR method
func (d *IQRDetector) Detect(data []float64) *DetectionResult {
	return d.DetectWithTimestamps(data, nil)
}

// DetectWithTimestamps analyzes data with associated timestamps
func (d *IQRDetector) DetectWithTimestamps(data []float64, timestamps []time.Time) *DetectionResult {
	result := &DetectionResult{
		Method:      MethodIQR,
		Threshold:   d.Multiplier,
		SampleCount: len(data),
		Anomalies:   []Anomaly{},
	}

	if len(data) < d.MinSamples {
		return result
	}

	// Calculate quartiles
	q1, median, q3 := calculateQuartiles(data)
	iqr := q3 - q1

	result.Q1 = q1
	result.Q3 = q3
	result.IQR = iqr
	result.Median = median
	result.Mean = calculateMean(data)
	result.StdDev = calculateStdDev(data, result.Mean)
	result.MinValue, result.MaxValue = findMinMax(data)

	// Calculate bounds
	lowerBound := q1 - d.Multiplier*iqr
	upperBound := q3 + d.Multiplier*iqr

	// Avoid issues with zero IQR (all values the same)
	if iqr == 0 {
		return result
	}

	// Detect anomalies
	for i, value := range data {
		if value < lowerBound || value > upperBound {
			var ts time.Time
			if timestamps != nil && i < len(timestamps) {
				ts = timestamps[i]
			}

			// Calculate deviation as multiple of IQR from nearest quartile
			var deviation float64
			if value < lowerBound {
				deviation = (q1 - value) / iqr
			} else {
				deviation = (value - q3) / iqr
			}

			anomalyType := AnomalyTypeCPUSpike
			if value < median {
				anomalyType = AnomalyTypeCPUDrop
			}

			anomaly := Anomaly{
				Timestamp:     ts,
				Type:          anomalyType,
				Severity:      determineSeverityFromIQR(deviation),
				DetectedBy:    MethodIQR,
				Value:         value,
				ExpectedLower: lowerBound,
				ExpectedUpper: upperBound,
				Deviation:     deviation,
				Index:         i,
				Message: fmt.Sprintf("Value %.2f outside IQR bounds [%.2f, %.2f] (Q1=%.2f, Q3=%.2f, IQR=%.2f)",
					value, lowerBound, upperBound, q1, q3, iqr),
			}

			result.Anomalies = append(result.Anomalies, anomaly)
		}
	}

	return result
}

// calculateQuartiles calculates Q1, median (Q2), and Q3
func calculateQuartiles(data []float64) (q1, median, q3 float64) {
	if len(data) == 0 {
		return 0, 0, 0
	}

	// Sort a copy of the data
	sorted := make([]float64, len(data))
	copy(sorted, data)
	sort.Float64s(sorted)

	n := len(sorted)

	// Calculate median (Q2)
	median = calculatePercentileFromSorted(sorted, 50)

	// Calculate Q1 (25th percentile)
	q1 = calculatePercentileFromSorted(sorted, 25)

	// Calculate Q3 (75th percentile)
	q3 = calculatePercentileFromSorted(sorted, 75)

	// Fallback for very small datasets
	if n < 4 {
		q1 = sorted[0]
		q3 = sorted[n-1]
		if n >= 2 {
			median = (sorted[0] + sorted[n-1]) / 2
		} else {
			median = sorted[0]
		}
	}

	return q1, median, q3
}

// calculatePercentileFromSorted calculates a percentile from sorted data
// Uses linear interpolation between points
func calculatePercentileFromSorted(sorted []float64, percentile float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}

	// Calculate the rank
	n := float64(len(sorted))
	rank := (percentile / 100.0) * (n - 1)

	// Get the lower and upper indices
	lower := int(rank)
	upper := lower + 1

	if upper >= len(sorted) {
		return sorted[len(sorted)-1]
	}

	// Linear interpolation
	fraction := rank - float64(lower)
	return sorted[lower] + fraction*(sorted[upper]-sorted[lower])
}

// determineSeverityFromIQR determines severity based on IQR deviation
func determineSeverityFromIQR(deviation float64) Severity {
	absDeviation := deviation
	if absDeviation < 0 {
		absDeviation = -absDeviation
	}

	switch {
	case absDeviation >= 3.0: // More than 3x IQR from quartile
		return SeverityCritical
	case absDeviation >= 2.0: // More than 2x IQR from quartile
		return SeverityHigh
	case absDeviation >= 1.5: // More than 1.5x IQR from quartile
		return SeverityMedium
	default:
		return SeverityLow
	}
}
