package anomaly

import (
	"fmt"
	"math"
	"time"
)

// MovingAverageDetector detects anomalies based on deviation from a moving average.
// This method is particularly good at detecting sudden changes or spikes in
// time-series data while being adaptive to gradual trends.
//
// The method works by:
// 1. Computing a moving average over a sliding window
// 2. Calculating the standard deviation of the moving average
// 3. Flagging points that deviate more than threshold * stdDev from the moving average
//
// This approach is better for time-series data where the baseline may shift over time,
// as it adapts to the local context rather than the global mean.
type MovingAverageDetector struct {
	// WindowSize is the number of points in the moving average window
	WindowSize int

	// Threshold is the number of standard deviations for anomaly detection
	Threshold float64

	// MinSamples is the minimum number of data points required
	MinSamples int

	// UseExponentialMA uses exponential moving average if true
	UseExponentialMA bool

	// Alpha is the smoothing factor for EMA (0 < alpha <= 1)
	// Higher alpha = more weight on recent values
	Alpha float64
}

// NewMovingAverageDetector creates a new moving average detector with default settings
func NewMovingAverageDetector() *MovingAverageDetector {
	return &MovingAverageDetector{
		WindowSize:       10,
		Threshold:        2.0,
		MinSamples:       10,
		UseExponentialMA: false,
		Alpha:            0.3,
	}
}

// NewMovingAverageDetectorWithConfig creates a moving average detector with custom config
func NewMovingAverageDetectorWithConfig(config *Config) *MovingAverageDetector {
	return &MovingAverageDetector{
		WindowSize:       config.MovingAverageWindow,
		Threshold:        config.MovingAverageThreshold,
		MinSamples:       config.MinSamples,
		UseExponentialMA: false,
		Alpha:            0.3,
	}
}

// Name returns the detector's method name
func (d *MovingAverageDetector) Name() DetectionMethod {
	return MethodMovingAverage
}

// Detect analyzes the data and returns anomalies using moving average method
func (d *MovingAverageDetector) Detect(data []float64) *DetectionResult {
	return d.DetectWithTimestamps(data, nil)
}

// DetectWithTimestamps analyzes data with associated timestamps
func (d *MovingAverageDetector) DetectWithTimestamps(data []float64, timestamps []time.Time) *DetectionResult {
	result := &DetectionResult{
		Method:      MethodMovingAverage,
		Threshold:   d.Threshold,
		SampleCount: len(data),
		WindowSize:  d.WindowSize,
		Anomalies:   []Anomaly{},
	}

	if len(data) < d.MinSamples {
		return result
	}

	// Calculate moving average
	var movingAvg []float64
	if d.UseExponentialMA {
		movingAvg = d.calculateEMA(data)
	} else {
		movingAvg = d.calculateSMA(data)
	}

	// Calculate statistics
	result.Mean = calculateMean(data)
	result.StdDev = calculateStdDev(data, result.Mean)
	result.MinValue, result.MaxValue = findMinMax(data)

	// Calculate deviations from moving average
	deviations := make([]float64, len(data))
	for i := range data {
		if i < len(movingAvg) {
			deviations[i] = data[i] - movingAvg[i]
		}
	}

	// Calculate standard deviation of deviations
	devMean := calculateMean(deviations)
	devStdDev := calculateStdDev(deviations, devMean)

	if devStdDev == 0 {
		return result
	}

	// Detect anomalies
	for i, value := range data {
		if i >= len(movingAvg) {
			continue
		}

		deviation := math.Abs(value - movingAvg[i])
		normalizedDeviation := deviation / devStdDev

		if normalizedDeviation > d.Threshold {
			var ts time.Time
			if timestamps != nil && i < len(timestamps) {
				ts = timestamps[i]
			}

			anomalyType := AnomalyTypeCPUSpike
			if value < movingAvg[i] {
				anomalyType = AnomalyTypeCPUDrop
			}

			// Calculate bounds based on moving average
			lowerBound := movingAvg[i] - d.Threshold*devStdDev
			upperBound := movingAvg[i] + d.Threshold*devStdDev

			anomaly := Anomaly{
				Timestamp:     ts,
				Type:          anomalyType,
				Severity:      determineSeverity(normalizedDeviation),
				DetectedBy:    MethodMovingAverage,
				Value:         value,
				ExpectedLower: lowerBound,
				ExpectedUpper: upperBound,
				Deviation:     normalizedDeviation,
				Index:         i,
				Message: fmt.Sprintf("Value %.2f deviates %.2f std devs from moving average %.2f (threshold=%.2f)",
					value, normalizedDeviation, movingAvg[i], d.Threshold),
			}

			result.Anomalies = append(result.Anomalies, anomaly)
		}
	}

	return result
}

// calculateSMA calculates the Simple Moving Average
func (d *MovingAverageDetector) calculateSMA(data []float64) []float64 {
	if len(data) == 0 {
		return nil
	}

	result := make([]float64, len(data))
	windowSize := d.WindowSize

	for i := range data {
		// Determine the actual window (handle edges)
		start := i - windowSize + 1
		if start < 0 {
			start = 0
		}

		// Calculate average for the window
		var sum float64
		count := 0
		for j := start; j <= i; j++ {
			sum += data[j]
			count++
		}

		result[i] = sum / float64(count)
	}

	return result
}

// calculateEMA calculates the Exponential Moving Average
func (d *MovingAverageDetector) calculateEMA(data []float64) []float64 {
	if len(data) == 0 {
		return nil
	}

	result := make([]float64, len(data))
	result[0] = data[0]

	for i := 1; i < len(data); i++ {
		// EMA = alpha * current + (1 - alpha) * previous_EMA
		result[i] = d.Alpha*data[i] + (1-d.Alpha)*result[i-1]
	}

	return result
}

// GetMovingAverage returns the moving average for the given data
// This can be useful for visualization or debugging
func (d *MovingAverageDetector) GetMovingAverage(data []float64) []float64 {
	if d.UseExponentialMA {
		return d.calculateEMA(data)
	}
	return d.calculateSMA(data)
}
