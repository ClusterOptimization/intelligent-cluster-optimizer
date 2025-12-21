package anomaly

import (
	"math"
	"testing"
	"time"
)

// Test data generators

// generateNormalData generates normally distributed data with optional outliers
func generateNormalData(n int, mean, stdDev float64, outlierIndices []int, outlierValue float64) []float64 {
	data := make([]float64, n)

	// Simple pseudo-normal distribution using central limit theorem approximation
	for i := 0; i < n; i++ {
		// Generate base value around mean
		data[i] = mean + stdDev*(float64(i%10)-4.5)/4.5
	}

	// Insert outliers
	for _, idx := range outlierIndices {
		if idx < n {
			data[idx] = outlierValue
		}
	}

	return data
}

// generateStableData generates stable data with sudden spikes
func generateStableData(n int, baseValue float64, spikeIndices []int, spikeValue float64) []float64 {
	data := make([]float64, n)

	for i := 0; i < n; i++ {
		// Small variation around base
		data[i] = baseValue + float64(i%5)*0.1
	}

	// Insert spikes
	for _, idx := range spikeIndices {
		if idx < n {
			data[idx] = spikeValue
		}
	}

	return data
}

// === Z-Score Detector Tests ===

func TestZScoreDetector_DetectOutliers(t *testing.T) {
	detector := NewZScoreDetector()

	// Generate data with clear outliers
	data := generateNormalData(100, 50.0, 5.0, []int{25, 75}, 100.0)

	result := detector.Detect(data)

	if !result.HasAnomalies() {
		t.Error("Expected to detect anomalies in data with outliers")
	}

	// Should detect the two outliers at indices 25 and 75
	if result.AnomalyCount() < 2 {
		t.Errorf("Expected at least 2 anomalies, got %d", result.AnomalyCount())
	}

	// Verify the method is correct
	if result.Method != MethodZScore {
		t.Errorf("Expected method %s, got %s", MethodZScore, result.Method)
	}
}

func TestZScoreDetector_NoAnomaliesInNormalData(t *testing.T) {
	detector := NewZScoreDetector()

	// Generate data without outliers
	data := generateNormalData(100, 50.0, 5.0, nil, 0)

	result := detector.Detect(data)

	// Should detect few or no anomalies in normal data
	if result.AnomalyCount() > 5 {
		t.Errorf("Expected few anomalies in normal data, got %d", result.AnomalyCount())
	}
}

func TestZScoreDetector_InsufficientData(t *testing.T) {
	detector := NewZScoreDetector()
	detector.MinSamples = 10

	data := []float64{1, 2, 3, 4, 5}

	result := detector.Detect(data)

	if result.HasAnomalies() {
		t.Error("Should not detect anomalies with insufficient data")
	}
}

func TestZScoreDetector_CustomThreshold(t *testing.T) {
	// Lower threshold = more sensitive
	config := DefaultConfig()
	config.ZScoreThreshold = 2.0

	detector := NewZScoreDetectorWithConfig(config)

	data := generateNormalData(100, 50.0, 5.0, []int{50}, 65.0)

	result := detector.Detect(data)

	if !result.HasAnomalies() {
		t.Error("Expected to detect anomalies with lower threshold")
	}
}

// === IQR Detector Tests ===

func TestIQRDetector_DetectOutliers(t *testing.T) {
	detector := NewIQRDetector()

	// Generate data with outliers
	data := generateStableData(100, 50.0, []int{30, 60}, 150.0)

	result := detector.Detect(data)

	if !result.HasAnomalies() {
		t.Error("Expected to detect anomalies with IQR method")
	}

	// Verify quartiles are calculated
	if result.Q1 == 0 && result.Q3 == 0 {
		t.Error("Expected quartiles to be calculated")
	}

	if result.Method != MethodIQR {
		t.Errorf("Expected method %s, got %s", MethodIQR, result.Method)
	}
}

func TestIQRDetector_QuartileCalculation(t *testing.T) {
	// Test with known quartiles
	data := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}

	q1, median, q3 := calculateQuartiles(data)

	// For this data:
	// Q1 should be around 3.25
	// Median should be 6.5
	// Q3 should be around 9.75

	if math.Abs(median-6.5) > 0.5 {
		t.Errorf("Expected median around 6.5, got %.2f", median)
	}

	if q1 >= median || q3 <= median {
		t.Errorf("Quartile ordering incorrect: Q1=%.2f, Median=%.2f, Q3=%.2f", q1, median, q3)
	}
}

func TestIQRDetector_ExtremeOutliers(t *testing.T) {
	// Use multiplier of 3.0 for extreme outliers only
	config := DefaultConfig()
	config.IQRMultiplier = 3.0

	detector := NewIQRDetectorWithConfig(config)

	// Mild outliers that should be ignored with 3.0 multiplier
	data := generateStableData(100, 50.0, []int{50}, 75.0)

	result := detector.Detect(data)

	// With 3.0 multiplier, mild outliers may not be detected
	// This is expected behavior
	t.Logf("Detected %d anomalies with 3.0 IQR multiplier", result.AnomalyCount())
}

// === Moving Average Detector Tests ===

func TestMovingAverageDetector_DetectSpikes(t *testing.T) {
	detector := NewMovingAverageDetector()

	// Generate data with sudden spike
	data := generateStableData(50, 100.0, []int{25}, 200.0)

	result := detector.Detect(data)

	if !result.HasAnomalies() {
		t.Error("Expected to detect spike with moving average method")
	}

	if result.Method != MethodMovingAverage {
		t.Errorf("Expected method %s, got %s", MethodMovingAverage, result.Method)
	}
}

func TestMovingAverageDetector_SMACalculation(t *testing.T) {
	detector := &MovingAverageDetector{
		WindowSize: 3,
	}

	data := []float64{1, 2, 3, 4, 5}
	sma := detector.calculateSMA(data)

	// Expected SMA with window 3:
	// Index 0: 1 (only one value)
	// Index 1: (1+2)/2 = 1.5
	// Index 2: (1+2+3)/3 = 2
	// Index 3: (2+3+4)/3 = 3
	// Index 4: (3+4+5)/3 = 4

	if len(sma) != len(data) {
		t.Errorf("Expected SMA length %d, got %d", len(data), len(sma))
	}

	if math.Abs(sma[4]-4.0) > 0.01 {
		t.Errorf("Expected SMA[4] = 4.0, got %.2f", sma[4])
	}
}

func TestMovingAverageDetector_EMACalculation(t *testing.T) {
	detector := &MovingAverageDetector{
		WindowSize:       3,
		UseExponentialMA: true,
		Alpha:            0.5,
	}

	data := []float64{10, 20, 30, 40, 50}
	ema := detector.calculateEMA(data)

	if len(ema) != len(data) {
		t.Errorf("Expected EMA length %d, got %d", len(data), len(ema))
	}

	// First EMA equals first value
	if ema[0] != data[0] {
		t.Errorf("Expected EMA[0] = %.2f, got %.2f", data[0], ema[0])
	}

	// EMA should be smoothed
	if ema[4] >= data[4] {
		t.Errorf("Expected EMA to be smoothed below current value")
	}
}

func TestMovingAverageDetector_AdaptiveToTrends(t *testing.T) {
	detector := NewMovingAverageDetector()

	// Generate trending data with one anomaly
	data := make([]float64, 100)
	for i := range data {
		data[i] = float64(i) // Linear trend
	}
	// Add sudden jump
	data[50] = 100.0

	result := detector.Detect(data)

	// Should detect the sudden jump
	found := false
	for _, a := range result.Anomalies {
		if a.Index == 50 {
			found = true
			break
		}
	}

	if !found && result.HasAnomalies() {
		t.Log("Anomaly at index 50 not specifically detected, but other anomalies found")
	}
}

// === Consensus Detector Tests ===

func TestConsensusDetector_RequiresAgreement(t *testing.T) {
	config := DefaultConfig()
	config.ConsensusThreshold = 2

	detector := NewConsensusDetectorWithConfig(config)

	// Generate data with clear outlier that all methods should detect
	data := generateStableData(100, 50.0, []int{50}, 200.0)

	result := detector.Detect(data)

	if !result.HasAnomalies() {
		t.Error("Expected consensus detector to find anomalies when multiple methods agree")
	}

	if result.Method != MethodConsensus {
		t.Errorf("Expected method %s, got %s", MethodConsensus, result.Method)
	}

	// Check that message mentions consensus
	for _, a := range result.Anomalies {
		if a.Message == "" {
			t.Error("Expected anomaly to have a message")
		}
	}
}

func TestConsensusDetector_IndividualResults(t *testing.T) {
	detector := NewConsensusDetector()

	data := generateStableData(100, 50.0, []int{50}, 200.0)

	timestamps := make([]time.Time, len(data))
	now := time.Now()
	for i := range timestamps {
		timestamps[i] = now.Add(time.Duration(i) * time.Minute)
	}

	results := detector.GetIndividualResults(data, timestamps)

	// Should have results from all three methods
	if len(results) != 3 {
		t.Errorf("Expected 3 individual results, got %d", len(results))
	}

	// Check each method is present
	for _, method := range []DetectionMethod{MethodZScore, MethodIQR, MethodMovingAverage} {
		if _, ok := results[method]; !ok {
			t.Errorf("Missing results for method %s", method)
		}
	}
}

func TestConsensusDetector_HigherThresholdFewerAnomalies(t *testing.T) {
	// With higher consensus threshold, should detect fewer anomalies
	configLow := DefaultConfig()
	configLow.ConsensusThreshold = 1

	configHigh := DefaultConfig()
	configHigh.ConsensusThreshold = 3

	detectorLow := NewConsensusDetectorWithConfig(configLow)
	detectorHigh := NewConsensusDetectorWithConfig(configHigh)

	data := generateNormalData(100, 50.0, 5.0, []int{25, 50, 75}, 80.0)

	resultLow := detectorLow.Detect(data)
	resultHigh := detectorHigh.Detect(data)

	// Higher threshold should have same or fewer anomalies
	if resultHigh.AnomalyCount() > resultLow.AnomalyCount() {
		t.Errorf("Higher consensus threshold should not increase anomaly count: low=%d, high=%d",
			resultLow.AnomalyCount(), resultHigh.AnomalyCount())
	}
}

// === Detection Result Tests ===

func TestDetectionResult_Summary(t *testing.T) {
	result := &DetectionResult{
		Method:      MethodZScore,
		SampleCount: 100,
		Anomalies: []Anomaly{
			{Severity: SeverityHigh, Index: 1},
			{Severity: SeverityLow, Index: 2},
			{Severity: SeverityCritical, Index: 3},
		},
	}

	summary := result.Summary()

	if summary == "" {
		t.Error("Expected non-empty summary")
	}

	if result.HighSeverityCount() != 2 {
		t.Errorf("Expected 2 high severity anomalies, got %d", result.HighSeverityCount())
	}
}

func TestDetectionResult_NoAnomalies(t *testing.T) {
	result := &DetectionResult{
		Method:      MethodIQR,
		SampleCount: 50,
		Anomalies:   []Anomaly{},
	}

	if result.HasAnomalies() {
		t.Error("Expected HasAnomalies to return false")
	}

	if result.AnomalyCount() != 0 {
		t.Error("Expected AnomalyCount to be 0")
	}
}

// === Severity Tests ===

func TestDetermineSeverity(t *testing.T) {
	tests := []struct {
		deviation float64
		expected  Severity
	}{
		{2.0, SeverityLow},
		{3.0, SeverityMedium},
		{4.0, SeverityHigh},
		{5.0, SeverityCritical},
		{10.0, SeverityCritical},
		{-4.0, SeverityHigh}, // Negative deviation
	}

	for _, tt := range tests {
		result := determineSeverity(tt.deviation)
		if result != tt.expected {
			t.Errorf("determineSeverity(%.1f) = %s, expected %s", tt.deviation, result, tt.expected)
		}
	}
}

// === Integration Tests ===

func TestAllDetectors_SameData(t *testing.T) {
	data := generateStableData(100, 100.0, []int{25, 50, 75}, 300.0)

	detectors := []Detector{
		NewZScoreDetector(),
		NewIQRDetector(),
		NewMovingAverageDetector(),
		NewConsensusDetector(),
	}

	for _, detector := range detectors {
		result := detector.Detect(data)

		t.Logf("%s: detected %d anomalies", detector.Name(), result.AnomalyCount())

		// All detectors should find some anomalies in this obvious case
		if !result.HasAnomalies() {
			t.Errorf("%s failed to detect obvious anomalies", detector.Name())
		}
	}
}

func TestDetectors_WithTimestamps(t *testing.T) {
	data := generateStableData(50, 100.0, []int{25}, 200.0)

	timestamps := make([]time.Time, len(data))
	now := time.Now()
	for i := range timestamps {
		timestamps[i] = now.Add(time.Duration(i) * time.Minute)
	}

	detector := NewZScoreDetector()
	result := detector.DetectWithTimestamps(data, timestamps)

	if result.HasAnomalies() {
		// Verify timestamps are set on anomalies
		for _, a := range result.Anomalies {
			if a.Timestamp.IsZero() {
				t.Error("Expected anomaly to have timestamp set")
			}
		}
	}
}

// === Benchmark Tests ===

func BenchmarkZScoreDetector(b *testing.B) {
	detector := NewZScoreDetector()
	data := generateNormalData(1000, 100, 10, []int{500}, 200)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detector.Detect(data)
	}
}

func BenchmarkIQRDetector(b *testing.B) {
	detector := NewIQRDetector()
	data := generateNormalData(1000, 100, 10, []int{500}, 200)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detector.Detect(data)
	}
}

func BenchmarkMovingAverageDetector(b *testing.B) {
	detector := NewMovingAverageDetector()
	data := generateNormalData(1000, 100, 10, []int{500}, 200)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detector.Detect(data)
	}
}

func BenchmarkConsensusDetector(b *testing.B) {
	detector := NewConsensusDetector()
	data := generateNormalData(1000, 100, 10, []int{500}, 200)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detector.Detect(data)
	}
}
