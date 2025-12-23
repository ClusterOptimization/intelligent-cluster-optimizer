package sla

import (
	"testing"
	"time"
)

func TestMonitor_AddSLA(t *testing.T) {
	monitor := NewMonitor()

	sla := SLADefinition{
		Name:        "test-latency",
		Type:        SLATypeLatency,
		Target:      100.0,
		Threshold:   50.0,
		Window:      5 * time.Minute,
		Description: "Test SLA",
	}

	err := monitor.AddSLA(sla)
	if err != nil {
		t.Fatalf("Failed to add SLA: %v", err)
	}

	// Verify SLA was added
	retrieved, err := monitor.GetSLA("test-latency")
	if err != nil {
		t.Fatalf("Failed to retrieve SLA: %v", err)
	}

	if retrieved.Name != sla.Name {
		t.Errorf("Expected SLA name %s, got %s", sla.Name, retrieved.Name)
	}
}

func TestMonitor_AddInvalidSLA(t *testing.T) {
	monitor := NewMonitor()

	// Missing name
	sla := SLADefinition{
		Type:      SLATypeLatency,
		Target:    100.0,
		Threshold: 50.0,
		Window:    5 * time.Minute,
	}

	err := monitor.AddSLA(sla)
	if err == nil {
		t.Error("Expected error for SLA without name")
	}
}

func TestMonitor_RemoveSLA(t *testing.T) {
	monitor := NewMonitor()

	sla := SLADefinition{
		Name:      "test-sla",
		Type:      SLATypeLatency,
		Target:    100.0,
		Threshold: 50.0,
		Window:    5 * time.Minute,
	}

	monitor.AddSLA(sla)

	err := monitor.RemoveSLA("test-sla")
	if err != nil {
		t.Fatalf("Failed to remove SLA: %v", err)
	}

	// Verify SLA was removed
	_, err = monitor.GetSLA("test-sla")
	if err == nil {
		t.Error("Expected error when getting removed SLA")
	}
}

func TestMonitor_CheckLatencySLA(t *testing.T) {
	monitor := NewMonitor()

	sla := SLADefinition{
		Name:        "latency-sla",
		Type:        SLATypeLatency,
		Target:      100.0,
		Threshold:   50.0,
		Percentile:  95.0,
		Window:      5 * time.Minute,
		Description: "P95 latency SLA",
	}

	err := monitor.AddSLA(sla)
	if err != nil {
		t.Fatalf("Failed to add SLA: %v", err)
	}

	now := time.Now()

	// Create metrics with some violations
	metrics := []Metric{
		{Timestamp: now.Add(-4 * time.Minute), Value: 80.0},
		{Timestamp: now.Add(-3 * time.Minute), Value: 90.0},
		{Timestamp: now.Add(-2 * time.Minute), Value: 120.0},
		{Timestamp: now.Add(-1 * time.Minute), Value: 160.0}, // Violation (> 150)
		{Timestamp: now, Value: 170.0},                        // Violation (> 150)
	}

	violations, err := monitor.CheckSLA("latency-sla", metrics)
	if err != nil {
		t.Fatalf("Failed to check SLA: %v", err)
	}

	if len(violations) == 0 {
		t.Error("Expected violations but got none")
	}

	t.Logf("Found %d violations", len(violations))
	for _, v := range violations {
		t.Logf("Violation: %s (severity: %.2f)", v.Message, v.Severity)
	}
}

func TestMonitor_CheckErrorRateSLA(t *testing.T) {
	monitor := NewMonitor()

	sla := SLADefinition{
		Name:      "error-rate-sla",
		Type:      SLATypeErrorRate,
		Target:    0.1,
		Threshold: 0.4,
		Window:    5 * time.Minute,
	}

	monitor.AddSLA(sla)

	now := time.Now()
	metrics := []Metric{
		{Timestamp: now.Add(-4 * time.Minute), Value: 0.1},
		{Timestamp: now.Add(-3 * time.Minute), Value: 0.2},
		{Timestamp: now.Add(-2 * time.Minute), Value: 0.3},
		{Timestamp: now.Add(-1 * time.Minute), Value: 0.6}, // Above threshold
		{Timestamp: now, Value: 0.7},                        // Above threshold
	}

	violations, err := monitor.CheckSLA("error-rate-sla", metrics)
	if err != nil {
		t.Fatalf("Failed to check SLA: %v", err)
	}

	if len(violations) == 0 {
		t.Error("Expected violations for high error rate")
	}
}

func TestMonitor_CheckAvailabilitySLA(t *testing.T) {
	monitor := NewMonitor()

	sla := SLADefinition{
		Name:      "availability-sla",
		Type:      SLATypeAvailability,
		Target:    99.9,
		Threshold: 0.5,
		Window:    5 * time.Minute,
	}

	monitor.AddSLA(sla)

	now := time.Now()
	// 20% downtime (2 out of 10 down)
	metrics := []Metric{
		{Timestamp: now.Add(-9 * time.Minute), Value: 1.0}, // Up
		{Timestamp: now.Add(-8 * time.Minute), Value: 1.0},
		{Timestamp: now.Add(-7 * time.Minute), Value: 0.0}, // Down
		{Timestamp: now.Add(-6 * time.Minute), Value: 1.0},
		{Timestamp: now.Add(-5 * time.Minute), Value: 1.0},
		{Timestamp: now.Add(-4 * time.Minute), Value: 0.0}, // Down
		{Timestamp: now.Add(-3 * time.Minute), Value: 1.0},
		{Timestamp: now.Add(-2 * time.Minute), Value: 1.0},
		{Timestamp: now.Add(-1 * time.Minute), Value: 1.0},
		{Timestamp: now, Value: 1.0},
	}

	violations, err := monitor.CheckSLA("availability-sla", metrics)
	if err != nil {
		t.Fatalf("Failed to check SLA: %v", err)
	}

	if len(violations) == 0 {
		t.Error("Expected violations for low availability")
	}
}

func TestMonitor_CheckAllSLAs(t *testing.T) {
	monitor := NewMonitor()

	// Add multiple SLAs
	monitor.AddSLA(SLADefinition{
		Name:      "latency",
		Type:      SLATypeLatency,
		Target:    100.0,
		Threshold: 50.0,
		Window:    5 * time.Minute,
	})

	monitor.AddSLA(SLADefinition{
		Name:      "error-rate",
		Type:      SLATypeErrorRate,
		Target:    0.1,
		Threshold: 0.4,
		Window:    5 * time.Minute,
	})

	now := time.Now()
	metrics := []Metric{
		{Timestamp: now.Add(-1 * time.Minute), Value: 200.0}, // High latency
	}

	violations, err := monitor.CheckAllSLAs(metrics)
	if err != nil {
		t.Fatalf("Failed to check all SLAs: %v", err)
	}

	if len(violations) == 0 {
		t.Error("Expected at least one violation")
	}

	t.Logf("Total violations across all SLAs: %d", len(violations))
}

func TestMonitor_ListSLAs(t *testing.T) {
	monitor := NewMonitor()

	slas := []SLADefinition{
		{
			Name:      "sla-1",
			Type:      SLATypeLatency,
			Target:    100.0,
			Threshold: 50.0,
			Window:    5 * time.Minute,
		},
		{
			Name:      "sla-2",
			Type:      SLATypeErrorRate,
			Target:    0.1,
			Threshold: 0.4,
			Window:    5 * time.Minute,
		},
	}

	for _, sla := range slas {
		monitor.AddSLA(sla)
	}

	listed := monitor.ListSLAs()
	if len(listed) != 2 {
		t.Errorf("Expected 2 SLAs, got %d", len(listed))
	}
}

func TestMonitor_WindowFiltering(t *testing.T) {
	monitor := NewMonitor()

	sla := SLADefinition{
		Name:      "test-window",
		Type:      SLATypeLatency,
		Target:    100.0,
		Threshold: 50.0,
		Window:    2 * time.Minute,
	}

	monitor.AddSLA(sla)

	now := time.Now()
	metrics := []Metric{
		{Timestamp: now.Add(-10 * time.Minute), Value: 200.0}, // Outside window
		{Timestamp: now.Add(-5 * time.Minute), Value: 200.0},  // Outside window
		{Timestamp: now.Add(-1 * time.Minute), Value: 200.0},  // Inside window
		{Timestamp: now, Value: 200.0},                         // Inside window
	}

	violations, err := monitor.CheckSLA("test-window", metrics)
	if err != nil {
		t.Fatalf("Failed to check SLA: %v", err)
	}

	// Should only consider metrics within the window
	if len(violations) == 0 {
		t.Error("Expected violations from metrics within window")
	}
}

func TestCalculatePercentile(t *testing.T) {
	tests := []struct {
		name       string
		values     []float64
		percentile float64
		want       float64
	}{
		{
			name:       "P50 of [1,2,3,4,5]",
			values:     []float64{1, 2, 3, 4, 5},
			percentile: 50.0,
			want:       3.0,
		},
		{
			name:       "P95 of [1,2,3,4,5,6,7,8,9,10]",
			values:     []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			percentile: 95.0,
			want:       9.5,
		},
		{
			name:       "P99 of [1-100]",
			values:     generateSequence(1, 100),
			percentile: 99.0,
			want:       99.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculatePercentile(tt.values, tt.percentile)
			if got < tt.want-0.5 || got > tt.want+0.5 {
				t.Errorf("calculatePercentile() = %v, want approximately %v", got, tt.want)
			}
		})
	}
}

func generateSequence(start, end int) []float64 {
	result := make([]float64, end-start+1)
	for i := range result {
		result[i] = float64(start + i)
	}
	return result
}
