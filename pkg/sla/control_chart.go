package sla

import (
	"fmt"
	"math"
)

// DefaultControlChart is the default implementation of ControlChart
type DefaultControlChart struct{}

// NewControlChart creates a new control chart generator
func NewControlChart() ControlChart {
	return &DefaultControlChart{}
}

// GenerateChart generates control chart data from metrics
func (c *DefaultControlChart) GenerateChart(metrics []Metric, config ControlChartConfig) ([]ControlChartPoint, error) {
	if len(metrics) < config.MinSamples {
		return nil, fmt.Errorf("insufficient samples: %d < %d", len(metrics), config.MinSamples)
	}

	// Calculate control limits
	mean, ucl, lcl, err := c.CalculateControlLimits(metrics, config.SigmaLevel)
	if err != nil {
		return nil, err
	}

	// Generate control chart points
	points := make([]ControlChartPoint, len(metrics))
	for i, metric := range metrics {
		point := ControlChartPoint{
			Timestamp:   metric.Timestamp,
			Value:       metric.Value,
			Mean:        mean,
			UCL:         ucl,
			LCL:         lcl,
			IsOutlier:   false,
			OutlierType: OutlierTypeNone,
		}

		// Check for outliers
		if metric.Value > ucl {
			point.IsOutlier = true
			point.OutlierType = OutlierTypeAbove
		} else if metric.Value < lcl {
			point.IsOutlier = true
			point.OutlierType = OutlierTypeBelow
		}

		points[i] = point
	}

	// Detect trends if enabled
	if config.EnableTrendDetection {
		c.detectTrends(points, config.TrendWindowSize)
	}

	return points, nil
}

// DetectOutliers detects outliers using robust iterative method
func (c *DefaultControlChart) DetectOutliers(metrics []Metric, sigmaLevel float64) ([]ControlChartPoint, error) {
	if len(metrics) == 0 {
		return nil, nil
	}

	// Use robust initial screening with median-based method
	// This prevents extreme outliers from skewing initial detection
	outlierIndices := c.detectOutliersRobust(metrics)

	// If we found potential outliers, refine with iterative 3-sigma method
	if len(outlierIndices) > 0 {
		// Remove initial outliers
		var filtered []Metric
		for i, metric := range metrics {
			if !outlierIndices[i] {
				filtered = append(filtered, metric)
			}
		}

		// Refine with 3-sigma method
		if len(filtered) >= 2 {
			_, ucl, lcl, err := c.CalculateControlLimits(filtered, sigmaLevel)
			if err == nil {
				// Re-check all metrics with refined limits
				refinedOutliers := make(map[int]bool)
				for i, metric := range metrics {
					if metric.Value > ucl || metric.Value < lcl {
						refinedOutliers[i] = true
					}
				}
				outlierIndices = refinedOutliers
			}
		}
	}

	// Calculate final control limits without outliers for reference
	var currentMetrics []Metric
	for i, metric := range metrics {
		if !outlierIndices[i] {
			currentMetrics = append(currentMetrics, metric)
		}
	}

	mean, ucl, lcl, err := c.CalculateControlLimits(currentMetrics, sigmaLevel)
	if err != nil {
		// Fallback to original metrics if filtered set is invalid
		mean, ucl, lcl, err = c.CalculateControlLimits(metrics, sigmaLevel)
		if err != nil {
			return nil, err
		}
	}

	// Build outlier results
	var outliers []ControlChartPoint
	for i, metric := range metrics {
		if outlierIndices[i] {
			outlierType := OutlierTypeAbove
			if metric.Value < lcl {
				outlierType = OutlierTypeBelow
			}

			outliers = append(outliers, ControlChartPoint{
				Timestamp:   metric.Timestamp,
				Value:       metric.Value,
				Mean:        mean,
				UCL:         ucl,
				LCL:         lcl,
				IsOutlier:   true,
				OutlierType: outlierType,
			})
		}
	}

	return outliers, nil
}

// detectOutliersRobust uses median and MAD for robust outlier detection
func (c *DefaultControlChart) detectOutliersRobust(metrics []Metric) map[int]bool {
	outliers := make(map[int]bool)

	if len(metrics) < 3 {
		return outliers
	}

	// Extract values and sort for median calculation
	values := make([]float64, len(metrics))
	for i, m := range metrics {
		values[i] = m.Value
	}

	// Calculate median
	sorted := make([]float64, len(values))
	copy(sorted, values)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	median := sorted[len(sorted)/2]
	if len(sorted)%2 == 0 {
		median = (sorted[len(sorted)/2-1] + sorted[len(sorted)/2]) / 2.0
	}

	// Calculate MAD (Median Absolute Deviation)
	absDeviations := make([]float64, len(values))
	for i, v := range values {
		absDeviations[i] = math.Abs(v - median)
	}

	for i := 0; i < len(absDeviations); i++ {
		for j := i + 1; j < len(absDeviations); j++ {
			if absDeviations[i] > absDeviations[j] {
				absDeviations[i], absDeviations[j] = absDeviations[j], absDeviations[i]
			}
		}
	}

	mad := absDeviations[len(absDeviations)/2]
	if len(absDeviations)%2 == 0 {
		mad = (absDeviations[len(absDeviations)/2-1] + absDeviations[len(absDeviations)/2]) / 2.0
	}

	// Avoid division by zero
	if mad == 0 {
		// If MAD is 0, use a simple threshold based on median
		mad = 0.6745 // Use a small constant
	}

	// Modified Z-Score: 0.6745 * (value - median) / MAD
	// Threshold: typically 3.5 for outliers
	threshold := 3.5
	for i, v := range values {
		modifiedZScore := 0.6745 * math.Abs(v-median) / mad
		if modifiedZScore > threshold {
			outliers[i] = true
		}
	}

	return outliers
}

// CalculateControlLimits calculates UCL and LCL using the 3-sigma method
func (c *DefaultControlChart) CalculateControlLimits(metrics []Metric, sigmaLevel float64) (mean, ucl, lcl float64, err error) {
	if len(metrics) == 0 {
		return 0, 0, 0, fmt.Errorf("no metrics provided")
	}

	// Calculate mean
	var sum float64
	for _, metric := range metrics {
		sum += metric.Value
	}
	mean = sum / float64(len(metrics))

	// Calculate standard deviation
	var variance float64
	for _, metric := range metrics {
		diff := metric.Value - mean
		variance += diff * diff
	}
	variance /= float64(len(metrics))
	stdDev := math.Sqrt(variance)

	// Calculate control limits
	ucl = mean + (sigmaLevel * stdDev)
	lcl = mean - (sigmaLevel * stdDev)

	// Ensure LCL is not negative for metrics that can't be negative
	if lcl < 0 {
		lcl = 0
	}

	return mean, ucl, lcl, nil
}

// detectTrends detects trending patterns in control chart points
func (c *DefaultControlChart) detectTrends(points []ControlChartPoint, windowSize int) {
	if windowSize <= 0 || windowSize > len(points) {
		return
	}

	// Look for consecutive increasing or decreasing trends
	for i := 0; i <= len(points)-windowSize; i++ {
		isIncreasing := true
		isDecreasing := true

		for j := i; j < i+windowSize-1; j++ {
			if points[j].Value >= points[j+1].Value {
				isIncreasing = false
			}
			if points[j].Value <= points[j+1].Value {
				isDecreasing = false
			}
		}

		// Mark trend points
		if isIncreasing || isDecreasing {
			for j := i; j < i+windowSize; j++ {
				if !points[j].IsOutlier {
					points[j].IsOutlier = true
					points[j].OutlierType = OutlierTypeTrend
				}
			}
		}
	}
}

// CalculateMovingAverage calculates moving average for smoothing
func CalculateMovingAverage(metrics []Metric, windowSize int) []float64 {
	if windowSize <= 0 || windowSize > len(metrics) {
		windowSize = len(metrics)
	}

	result := make([]float64, len(metrics))
	for i := 0; i < len(metrics); i++ {
		start := i - windowSize/2
		if start < 0 {
			start = 0
		}
		end := start + windowSize
		if end > len(metrics) {
			end = len(metrics)
			start = end - windowSize
			if start < 0 {
				start = 0
			}
		}

		var sum float64
		count := 0
		for j := start; j < end; j++ {
			sum += metrics[j].Value
			count++
		}
		result[i] = sum / float64(count)
	}

	return result
}

// CalculateStandardDeviation calculates standard deviation of metric values
func CalculateStandardDeviation(metrics []Metric) float64 {
	if len(metrics) == 0 {
		return 0
	}

	// Calculate mean
	var sum float64
	for _, metric := range metrics {
		sum += metric.Value
	}
	mean := sum / float64(len(metrics))

	// Calculate variance
	var variance float64
	for _, metric := range metrics {
		diff := metric.Value - mean
		variance += diff * diff
	}
	variance /= float64(len(metrics))

	return math.Sqrt(variance)
}
