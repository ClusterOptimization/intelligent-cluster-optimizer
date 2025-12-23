package sla

import (
	"testing"
	"time"
)

func TestControlChart_CalculateControlLimits(t *testing.T) {
	chart := NewControlChart()

	now := time.Now()
	metrics := []Metric{
		{Timestamp: now, Value: 10.0},
		{Timestamp: now, Value: 12.0},
		{Timestamp: now, Value: 11.0},
		{Timestamp: now, Value: 13.0},
		{Timestamp: now, Value: 9.0},
	}

	mean, ucl, lcl, err := chart.CalculateControlLimits(metrics, 3.0)
	if err != nil {
		t.Fatalf("Failed to calculate control limits: %v", err)
	}

	// Mean should be 11.0
	expectedMean := 11.0
	if mean < expectedMean-0.1 || mean > expectedMean+0.1 {
		t.Errorf("Expected mean %v, got %v", expectedMean, mean)
	}

	// UCL should be higher than mean
	if ucl <= mean {
		t.Errorf("UCL (%v) should be greater than mean (%v)", ucl, mean)
	}

	// LCL should be lower than mean
	if lcl >= mean {
		t.Errorf("LCL (%v) should be less than mean (%v)", lcl, mean)
	}

	t.Logf("Control limits: Mean=%.2f, UCL=%.2f, LCL=%.2f", mean, ucl, lcl)
}

func TestControlChart_DetectOutliers(t *testing.T) {
	chart := NewControlChart()

	now := time.Now()
	metrics := []Metric{
		{Timestamp: now.Add(-5 * time.Minute), Value: 10.0},
		{Timestamp: now.Add(-4 * time.Minute), Value: 11.0},
		{Timestamp: now.Add(-3 * time.Minute), Value: 12.0},
		{Timestamp: now.Add(-2 * time.Minute), Value: 11.5},
		{Timestamp: now.Add(-1 * time.Minute), Value: 1000.0}, // Extreme outlier
		{Timestamp: now, Value: 10.5},
	}

	// First check control limits
	mean, ucl, lcl, err := chart.CalculateControlLimits(metrics, 3.0)
	if err != nil {
		t.Fatalf("Failed to calculate control limits: %v", err)
	}
	t.Logf("Control limits: Mean=%.2f, UCL=%.2f, LCL=%.2f", mean, ucl, lcl)

	outliers, err := chart.DetectOutliers(metrics, 3.0)
	if err != nil {
		t.Fatalf("Failed to detect outliers: %v", err)
	}

	t.Logf("Found %d outliers", len(outliers))
	if len(outliers) == 0 {
		t.Error("Expected to detect outliers")
	}

	// Verify outlier detection
	for _, outlier := range outliers {
		if !outlier.IsOutlier {
			t.Error("Expected IsOutlier to be true")
		}
		t.Logf("Detected outlier: Value=%.2f at %v, Type=%s", outlier.Value, outlier.Timestamp, outlier.OutlierType)
	}
}

func TestControlChart_GenerateChart(t *testing.T) {
	chart := NewControlChart()

	now := time.Now()
	metrics := []Metric{
		{Timestamp: now.Add(-5 * time.Minute), Value: 10.0},
		{Timestamp: now.Add(-4 * time.Minute), Value: 11.0},
		{Timestamp: now.Add(-3 * time.Minute), Value: 12.0},
		{Timestamp: now.Add(-2 * time.Minute), Value: 11.5},
		{Timestamp: now.Add(-1 * time.Minute), Value: 10.5},
		{Timestamp: now, Value: 11.0},
	}

	config := ControlChartConfig{
		SigmaLevel:           3.0,
		MinSamples:           5,
		EnableTrendDetection: false,
		TrendWindowSize:      3,
	}

	points, err := chart.GenerateChart(metrics, config)
	if err != nil {
		t.Fatalf("Failed to generate chart: %v", err)
	}

	if len(points) != len(metrics) {
		t.Errorf("Expected %d points, got %d", len(metrics), len(points))
	}

	// Verify each point has control limits
	for i, point := range points {
		if point.Mean == 0 {
			t.Errorf("Point %d has zero mean", i)
		}
		if point.UCL == 0 {
			t.Errorf("Point %d has zero UCL", i)
		}
		t.Logf("Point %d: Value=%.2f, Mean=%.2f, UCL=%.2f, LCL=%.2f, IsOutlier=%v",
			i, point.Value, point.Mean, point.UCL, point.LCL, point.IsOutlier)
	}
}

func TestControlChart_TrendDetection(t *testing.T) {
	chart := NewControlChart()

	now := time.Now()
	// Create metrics with an increasing trend
	metrics := []Metric{
		{Timestamp: now.Add(-6 * time.Minute), Value: 10.0},
		{Timestamp: now.Add(-5 * time.Minute), Value: 11.0},
		{Timestamp: now.Add(-4 * time.Minute), Value: 12.0},
		{Timestamp: now.Add(-3 * time.Minute), Value: 13.0},
		{Timestamp: now.Add(-2 * time.Minute), Value: 14.0},
		{Timestamp: now.Add(-1 * time.Minute), Value: 15.0},
		{Timestamp: now, Value: 16.0},
	}

	config := ControlChartConfig{
		SigmaLevel:           3.0,
		MinSamples:           5,
		EnableTrendDetection: true,
		TrendWindowSize:      4,
	}

	points, err := chart.GenerateChart(metrics, config)
	if err != nil {
		t.Fatalf("Failed to generate chart: %v", err)
	}

	// Count points marked as trend
	trendCount := 0
	for _, point := range points {
		if point.OutlierType == OutlierTypeTrend {
			trendCount++
		}
	}

	if trendCount == 0 {
		t.Error("Expected to detect trend but found none")
	}

	t.Logf("Detected %d trend points", trendCount)
}

func TestControlChart_InsufficientSamples(t *testing.T) {
	chart := NewControlChart()

	now := time.Now()
	metrics := []Metric{
		{Timestamp: now, Value: 10.0},
		{Timestamp: now, Value: 11.0},
	}

	config := ControlChartConfig{
		SigmaLevel: 3.0,
		MinSamples: 5,
	}

	_, err := chart.GenerateChart(metrics, config)
	if err == nil {
		t.Error("Expected error for insufficient samples")
	}
}

func TestCalculateMovingAverage(t *testing.T) {
	now := time.Now()
	metrics := []Metric{
		{Timestamp: now, Value: 10.0},
		{Timestamp: now, Value: 20.0},
		{Timestamp: now, Value: 30.0},
		{Timestamp: now, Value: 40.0},
		{Timestamp: now, Value: 50.0},
	}

	result := CalculateMovingAverage(metrics, 3)

	if len(result) != len(metrics) {
		t.Errorf("Expected %d values, got %d", len(metrics), len(result))
	}

	// Check that moving average smooths the data
	for i, val := range result {
		t.Logf("Moving average[%d] = %.2f", i, val)
	}
}

func TestCalculateStandardDeviation(t *testing.T) {
	now := time.Now()
	metrics := []Metric{
		{Timestamp: now, Value: 10.0},
		{Timestamp: now, Value: 12.0},
		{Timestamp: now, Value: 23.0},
		{Timestamp: now, Value: 23.0},
		{Timestamp: now, Value: 16.0},
		{Timestamp: now, Value: 23.0},
		{Timestamp: now, Value: 21.0},
		{Timestamp: now, Value: 16.0},
	}

	stdDev := CalculateStandardDeviation(metrics)

	if stdDev == 0 {
		t.Error("Expected non-zero standard deviation")
	}

	t.Logf("Standard deviation: %.2f", stdDev)
}

func TestControlChart_NoMetrics(t *testing.T) {
	chart := NewControlChart()

	metrics := []Metric{}

	_, _, _, err := chart.CalculateControlLimits(metrics, 3.0)
	if err == nil {
		t.Error("Expected error for empty metrics")
	}
}

func TestControlChart_SingleMetric(t *testing.T) {
	chart := NewControlChart()

	now := time.Now()
	metrics := []Metric{
		{Timestamp: now, Value: 10.0},
	}

	mean, ucl, lcl, err := chart.CalculateControlLimits(metrics, 3.0)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// With single metric, mean should equal the value
	if mean != 10.0 {
		t.Errorf("Expected mean 10.0, got %.2f", mean)
	}

	// UCL and LCL should equal mean (no variance)
	if ucl != mean || lcl != mean {
		t.Errorf("Expected UCL and LCL to equal mean for single metric")
	}
}

func TestControlChart_HighVariance(t *testing.T) {
	chart := NewControlChart()

	now := time.Now()
	// Metrics with high variance
	metrics := []Metric{
		{Timestamp: now, Value: 1.0},
		{Timestamp: now, Value: 100.0},
		{Timestamp: now, Value: 2.0},
		{Timestamp: now, Value: 99.0},
		{Timestamp: now, Value: 3.0},
	}

	mean, ucl, lcl, err := chart.CalculateControlLimits(metrics, 3.0)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// UCL should be significantly higher than mean
	if ucl-mean < 50.0 {
		t.Errorf("Expected large UCL-mean difference for high variance data, got %.2f", ucl-mean)
	}

	t.Logf("High variance: Mean=%.2f, UCL=%.2f, LCL=%.2f", mean, ucl, lcl)
}
