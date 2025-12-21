package anomaly

import (
	"fmt"
	"time"
)

// ConsensusDetector combines multiple anomaly detection methods and
// reports anomalies that are detected by a minimum number of methods.
// This reduces false positives by requiring agreement between methods.
//
// The consensus approach is valuable because:
// - Z-Score assumes normal distribution (may miss non-normal outliers)
// - IQR is robust but may be too conservative
// - Moving Average is good for trends but may miss global outliers
//
// When multiple methods agree, we have higher confidence in the anomaly.
type ConsensusDetector struct {
	// Detectors is the list of detection methods to use
	Detectors []Detector

	// MinAgreement is the minimum number of methods that must agree
	// for an anomaly to be reported (default: 2)
	MinAgreement int

	// Config for creating default detectors
	Config *Config
}

// NewConsensusDetector creates a new consensus detector with all methods
func NewConsensusDetector() *ConsensusDetector {
	config := DefaultConfig()
	return NewConsensusDetectorWithConfig(config)
}

// NewConsensusDetectorWithConfig creates a consensus detector with custom config
func NewConsensusDetectorWithConfig(config *Config) *ConsensusDetector {
	return &ConsensusDetector{
		Detectors: []Detector{
			NewZScoreDetectorWithConfig(config),
			NewIQRDetectorWithConfig(config),
			NewMovingAverageDetectorWithConfig(config),
		},
		MinAgreement: config.ConsensusThreshold,
		Config:       config,
	}
}

// Name returns the detector's method name
func (d *ConsensusDetector) Name() DetectionMethod {
	return MethodConsensus
}

// Detect analyzes the data and returns anomalies agreed upon by multiple methods
func (d *ConsensusDetector) Detect(data []float64) *DetectionResult {
	return d.DetectWithTimestamps(data, nil)
}

// DetectWithTimestamps analyzes data with associated timestamps
func (d *ConsensusDetector) DetectWithTimestamps(data []float64, timestamps []time.Time) *DetectionResult {
	result := &DetectionResult{
		Method:      MethodConsensus,
		Threshold:   float64(d.MinAgreement),
		SampleCount: len(data),
		Anomalies:   []Anomaly{},
	}

	if len(data) < d.Config.MinSamples {
		return result
	}

	// Run all detectors
	allResults := make([]*DetectionResult, len(d.Detectors))
	for i, detector := range d.Detectors {
		allResults[i] = detector.DetectWithTimestamps(data, timestamps)
	}

	// Aggregate statistics from the first result that has them
	for _, r := range allResults {
		if r.Mean != 0 || r.StdDev != 0 {
			result.Mean = r.Mean
			result.StdDev = r.StdDev
			result.MinValue = r.MinValue
			result.MaxValue = r.MaxValue
			result.Q1 = r.Q1
			result.Q3 = r.Q3
			result.IQR = r.IQR
			result.Median = r.Median
			break
		}
	}

	// Count how many methods flagged each index as anomalous
	indexVotes := make(map[int][]Anomaly)
	for _, r := range allResults {
		for _, anomaly := range r.Anomalies {
			indexVotes[anomaly.Index] = append(indexVotes[anomaly.Index], anomaly)
		}
	}

	// Only include anomalies that meet the minimum agreement threshold
	for index, anomalies := range indexVotes {
		if len(anomalies) >= d.MinAgreement {
			// Create a consensus anomaly combining information from all detectors
			consensusAnomaly := d.createConsensusAnomaly(index, anomalies, data, timestamps)
			result.Anomalies = append(result.Anomalies, consensusAnomaly)
		}
	}

	// Sort anomalies by index for consistent output
	sortAnomaliesByIndex(result.Anomalies)

	return result
}

// createConsensusAnomaly creates a single anomaly from multiple detector results
func (d *ConsensusDetector) createConsensusAnomaly(index int, anomalies []Anomaly, data []float64, timestamps []time.Time) Anomaly {
	// Use the highest severity among all detectors
	highestSeverity := SeverityLow
	for _, a := range anomalies {
		if severityRank(a.Severity) > severityRank(highestSeverity) {
			highestSeverity = a.Severity
		}
	}

	// Collect which methods detected this
	methods := make([]string, len(anomalies))
	for i, a := range anomalies {
		methods[i] = string(a.DetectedBy)
	}

	// Use average deviation
	var totalDeviation float64
	for _, a := range anomalies {
		totalDeviation += a.Deviation
	}
	avgDeviation := totalDeviation / float64(len(anomalies))

	// Get timestamp if available
	var ts time.Time
	if timestamps != nil && index < len(timestamps) {
		ts = timestamps[index]
	}

	// Determine anomaly type (majority vote)
	anomalyType := anomalies[0].Type

	// Calculate expected bounds (average of all detectors)
	var lowerSum, upperSum float64
	for _, a := range anomalies {
		lowerSum += a.ExpectedLower
		upperSum += a.ExpectedUpper
	}

	return Anomaly{
		Timestamp:     ts,
		Type:          anomalyType,
		Severity:      highestSeverity,
		DetectedBy:    MethodConsensus,
		Value:         data[index],
		ExpectedLower: lowerSum / float64(len(anomalies)),
		ExpectedUpper: upperSum / float64(len(anomalies)),
		Deviation:     avgDeviation,
		Index:         index,
		Message: fmt.Sprintf("Consensus anomaly: %d/%d methods agree (methods: %v, value=%.2f)",
			len(anomalies), len(d.Detectors), methods, data[index]),
	}
}

// severityRank returns a numeric rank for severity comparison
func severityRank(s Severity) int {
	switch s {
	case SeverityCritical:
		return 4
	case SeverityHigh:
		return 3
	case SeverityMedium:
		return 2
	case SeverityLow:
		return 1
	default:
		return 0
	}
}

// sortAnomaliesByIndex sorts anomalies by their index in ascending order
func sortAnomaliesByIndex(anomalies []Anomaly) {
	// Simple insertion sort for small lists
	for i := 1; i < len(anomalies); i++ {
		key := anomalies[i]
		j := i - 1
		for j >= 0 && anomalies[j].Index > key.Index {
			anomalies[j+1] = anomalies[j]
			j--
		}
		anomalies[j+1] = key
	}
}

// GetIndividualResults returns results from each individual detector
// Useful for debugging or detailed analysis
func (d *ConsensusDetector) GetIndividualResults(data []float64, timestamps []time.Time) map[DetectionMethod]*DetectionResult {
	results := make(map[DetectionMethod]*DetectionResult)
	for _, detector := range d.Detectors {
		results[detector.Name()] = detector.DetectWithTimestamps(data, timestamps)
	}
	return results
}

// AgreementLevel represents how many methods agreed on an anomaly
type AgreementLevel int

const (
	AgreementNone     AgreementLevel = 0
	AgreementSingle   AgreementLevel = 1
	AgreementDouble   AgreementLevel = 2
	AgreementTriple   AgreementLevel = 3
	AgreementUnanimous AgreementLevel = -1 // All methods agree
)

// GetAgreementLevel returns the agreement level for a specific index
func (d *ConsensusDetector) GetAgreementLevel(data []float64, index int) AgreementLevel {
	count := 0
	for _, detector := range d.Detectors {
		result := detector.Detect(data)
		for _, a := range result.Anomalies {
			if a.Index == index {
				count++
				break
			}
		}
	}

	if count == len(d.Detectors) {
		return AgreementUnanimous
	}
	return AgreementLevel(count)
}
