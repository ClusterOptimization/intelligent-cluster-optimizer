package sla

import (
	"fmt"
	"sync"
	"time"
)

// DefaultMonitor is the default implementation of Monitor
type DefaultMonitor struct {
	mu   sync.RWMutex
	slas map[string]SLADefinition
}

// NewMonitor creates a new SLA monitor
func NewMonitor() Monitor {
	return &DefaultMonitor{
		slas: make(map[string]SLADefinition),
	}
}

// AddSLA adds an SLA definition to monitor
func (m *DefaultMonitor) AddSLA(sla SLADefinition) error {
	if err := ValidateSLADefinition(sla); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.slas[sla.Name] = sla
	return nil
}

// RemoveSLA removes an SLA definition
func (m *DefaultMonitor) RemoveSLA(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.slas[name]; !exists {
		return fmt.Errorf("SLA %s not found", name)
	}

	delete(m.slas, name)
	return nil
}

// CheckSLA checks if metrics violate an SLA
func (m *DefaultMonitor) CheckSLA(slaName string, metrics []Metric) ([]SLAViolation, error) {
	m.mu.RLock()
	sla, exists := m.slas[slaName]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("SLA %s not found", slaName)
	}

	return m.checkSLAViolations(sla, metrics)
}

// CheckAllSLAs checks all defined SLAs
func (m *DefaultMonitor) CheckAllSLAs(metrics []Metric) ([]SLAViolation, error) {
	m.mu.RLock()
	slasCopy := make([]SLADefinition, 0, len(m.slas))
	for _, sla := range m.slas {
		slasCopy = append(slasCopy, sla)
	}
	m.mu.RUnlock()

	var allViolations []SLAViolation
	for _, sla := range slasCopy {
		violations, err := m.checkSLAViolations(sla, metrics)
		if err != nil {
			return nil, fmt.Errorf("failed to check SLA %s: %w", sla.Name, err)
		}
		allViolations = append(allViolations, violations...)
	}

	return allViolations, nil
}

// GetSLA retrieves an SLA definition
func (m *DefaultMonitor) GetSLA(name string) (*SLADefinition, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sla, exists := m.slas[name]
	if !exists {
		return nil, fmt.Errorf("SLA %s not found", name)
	}

	return &sla, nil
}

// ListSLAs returns all defined SLAs
func (m *DefaultMonitor) ListSLAs() []SLADefinition {
	m.mu.RLock()
	defer m.mu.RUnlock()

	slas := make([]SLADefinition, 0, len(m.slas))
	for _, sla := range m.slas {
		slas = append(slas, sla)
	}

	return slas
}

// checkSLAViolations checks if metrics violate the given SLA
func (m *DefaultMonitor) checkSLAViolations(sla SLADefinition, metrics []Metric) ([]SLAViolation, error) {
	if len(metrics) == 0 {
		return nil, nil
	}

	// Filter metrics within the SLA window
	now := time.Now()
	windowStart := now.Add(-sla.Window)

	var windowMetrics []Metric
	for _, metric := range metrics {
		if metric.Timestamp.After(windowStart) {
			windowMetrics = append(windowMetrics, metric)
		}
	}

	if len(windowMetrics) == 0 {
		return nil, nil
	}

	var violations []SLAViolation

	// Check based on SLA type
	switch sla.Type {
	case SLATypeLatency:
		violations = m.checkLatencySLA(sla, windowMetrics)
	case SLATypeErrorRate:
		violations = m.checkErrorRateSLA(sla, windowMetrics)
	case SLATypeAvailability:
		violations = m.checkAvailabilitySLA(sla, windowMetrics)
	case SLATypeThroughput:
		violations = m.checkThroughputSLA(sla, windowMetrics)
	case SLATypeCustom:
		violations = m.checkCustomSLA(sla, windowMetrics)
	default:
		return nil, fmt.Errorf("unsupported SLA type: %s", sla.Type)
	}

	return violations, nil
}

// checkLatencySLA checks latency SLA violations
func (m *DefaultMonitor) checkLatencySLA(sla SLADefinition, metrics []Metric) []SLAViolation {
	var violations []SLAViolation

	// If percentile is specified, calculate percentile
	if sla.Percentile > 0 {
		values := make([]float64, len(metrics))
		for i, metric := range metrics {
			values[i] = metric.Value
		}

		percentileValue := calculatePercentile(values, sla.Percentile)
		threshold := sla.Target + sla.Threshold

		if percentileValue > threshold {
			severity := (percentileValue - threshold) / threshold
			if severity > 1.0 {
				severity = 1.0
			}

			violations = append(violations, SLAViolation{
				SLA:           sla,
				Timestamp:     time.Now(),
				ActualValue:   percentileValue,
				ExpectedValue: threshold,
				Severity:      severity,
				Message:       fmt.Sprintf("P%.0f latency %.2fms exceeds threshold %.2fms", sla.Percentile, percentileValue, threshold),
			})
		}
	} else {
		// Check each metric
		for _, metric := range metrics {
			threshold := sla.Target + sla.Threshold
			if metric.Value > threshold {
				severity := (metric.Value - threshold) / threshold
				if severity > 1.0 {
					severity = 1.0
				}

				violations = append(violations, SLAViolation{
					SLA:           sla,
					Timestamp:     metric.Timestamp,
					ActualValue:   metric.Value,
					ExpectedValue: threshold,
					Severity:      severity,
					Message:       fmt.Sprintf("Latency %.2fms exceeds threshold %.2fms", metric.Value, threshold),
				})
			}
		}
	}

	return violations
}

// checkErrorRateSLA checks error rate SLA violations
func (m *DefaultMonitor) checkErrorRateSLA(sla SLADefinition, metrics []Metric) []SLAViolation {
	var violations []SLAViolation

	// Calculate average error rate
	var sum float64
	for _, metric := range metrics {
		sum += metric.Value
	}
	avgErrorRate := sum / float64(len(metrics))

	threshold := sla.Target + sla.Threshold
	if avgErrorRate > threshold {
		severity := (avgErrorRate - threshold) / threshold
		if severity > 1.0 {
			severity = 1.0
		}

		violations = append(violations, SLAViolation{
			SLA:           sla,
			Timestamp:     time.Now(),
			ActualValue:   avgErrorRate,
			ExpectedValue: threshold,
			Severity:      severity,
			Message:       fmt.Sprintf("Error rate %.2f%% exceeds threshold %.2f%%", avgErrorRate, threshold),
		})
	} else {
		// Also check individual metrics for spikes
		for _, metric := range metrics {
			if metric.Value > threshold {
				severity := (metric.Value - threshold) / threshold
				if severity > 1.0 {
					severity = 1.0
				}

				violations = append(violations, SLAViolation{
					SLA:           sla,
					Timestamp:     metric.Timestamp,
					ActualValue:   metric.Value,
					ExpectedValue: threshold,
					Severity:      severity,
					Message:       fmt.Sprintf("Error rate %.2f%% exceeds threshold %.2f%%", metric.Value, threshold),
				})
			}
		}
	}

	return violations
}

// checkAvailabilitySLA checks availability SLA violations
func (m *DefaultMonitor) checkAvailabilitySLA(sla SLADefinition, metrics []Metric) []SLAViolation {
	var violations []SLAViolation

	// Calculate availability percentage
	var upCount float64
	for _, metric := range metrics {
		if metric.Value > 0 {
			upCount++
		}
	}
	availability := (upCount / float64(len(metrics))) * 100

	threshold := sla.Target - sla.Threshold
	if availability < threshold {
		severity := (threshold - availability) / threshold
		if severity > 1.0 {
			severity = 1.0
		}

		violations = append(violations, SLAViolation{
			SLA:           sla,
			Timestamp:     time.Now(),
			ActualValue:   availability,
			ExpectedValue: threshold,
			Severity:      severity,
			Message:       fmt.Sprintf("Availability %.2f%% below threshold %.2f%%", availability, threshold),
		})
	}

	return violations
}

// checkThroughputSLA checks throughput SLA violations
func (m *DefaultMonitor) checkThroughputSLA(sla SLADefinition, metrics []Metric) []SLAViolation {
	var violations []SLAViolation

	// Calculate average throughput
	var sum float64
	for _, metric := range metrics {
		sum += metric.Value
	}
	avgThroughput := sum / float64(len(metrics))

	threshold := sla.Target - sla.Threshold
	if avgThroughput < threshold {
		severity := (threshold - avgThroughput) / threshold
		if severity > 1.0 {
			severity = 1.0
		}

		violations = append(violations, SLAViolation{
			SLA:           sla,
			Timestamp:     time.Now(),
			ActualValue:   avgThroughput,
			ExpectedValue: threshold,
			Severity:      severity,
			Message:       fmt.Sprintf("Throughput %.2f req/s below threshold %.2f req/s", avgThroughput, threshold),
		})
	}

	return violations
}

// checkCustomSLA checks custom SLA violations
func (m *DefaultMonitor) checkCustomSLA(sla SLADefinition, metrics []Metric) []SLAViolation {
	// For custom SLAs, use a simple threshold check
	return m.checkLatencySLA(sla, metrics)
}

// calculatePercentile calculates the percentile of a slice of values
func calculatePercentile(values []float64, percentile float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// Sort values
	sorted := make([]float64, len(values))
	copy(sorted, values)

	// Simple bubble sort for small arrays
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// Calculate index
	index := (percentile / 100.0) * float64(len(sorted)-1)
	lower := int(index)
	upper := lower + 1

	if upper >= len(sorted) {
		return sorted[len(sorted)-1]
	}

	// Linear interpolation
	weight := index - float64(lower)
	return sorted[lower]*(1-weight) + sorted[upper]*weight
}
