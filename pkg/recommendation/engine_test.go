package recommendation

import (
	"math"
	"testing"
)

func TestCalculateChangePercent(t *testing.T) {
	tests := []struct {
		name        string
		current     int64
		recommended int64
		expected    float64
	}{
		{
			name:        "no change",
			current:     100,
			recommended: 100,
			expected:    0,
		},
		{
			name:        "50% increase",
			current:     100,
			recommended: 150,
			expected:    50,
		},
		{
			name:        "50% decrease",
			current:     100,
			recommended: 50,
			expected:    -50,
		},
		{
			name:        "100% increase (double)",
			current:     100,
			recommended: 200,
			expected:    100,
		},
		{
			name:        "zero current, zero recommended",
			current:     0,
			recommended: 0,
			expected:    0,
		},
		{
			name:        "zero current, non-zero recommended",
			current:     0,
			recommended: 100,
			expected:    100,
		},
		{
			name:        "large values",
			current:     1000000,
			recommended: 1200000,
			expected:    20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateChangePercent(tt.current, tt.recommended)
			if math.Abs(result-tt.expected) > 0.01 {
				t.Errorf("calculateChangePercent(%d, %d) = %.2f, expected %.2f",
					tt.current, tt.recommended, result, tt.expected)
			}
		})
	}
}

func TestContainerRecommendation_CalculateCPUChangePercent(t *testing.T) {
	rec := &ContainerRecommendation{
		CurrentCPU:     100,
		RecommendedCPU: 150,
	}

	result := rec.CalculateCPUChangePercent()
	expected := 50.0

	if math.Abs(result-expected) > 0.01 {
		t.Errorf("CalculateCPUChangePercent() = %.2f, expected %.2f", result, expected)
	}
}

func TestContainerRecommendation_CalculateMemoryChangePercent(t *testing.T) {
	rec := &ContainerRecommendation{
		CurrentMemory:     1024 * 1024 * 128, // 128Mi
		RecommendedMemory: 1024 * 1024 * 64,  // 64Mi
	}

	result := rec.CalculateMemoryChangePercent()
	expected := -50.0

	if math.Abs(result-expected) > 0.01 {
		t.Errorf("CalculateMemoryChangePercent() = %.2f, expected %.2f", result, expected)
	}
}

func TestContainerRecommendation_MaxChangePercent(t *testing.T) {
	tests := []struct {
		name              string
		currentCPU        int64
		recommendedCPU    int64
		currentMemory     int64
		recommendedMemory int64
		expected          float64
	}{
		{
			name:              "CPU change is larger",
			currentCPU:        100,
			recommendedCPU:    200, // 100% change
			currentMemory:     100,
			recommendedMemory: 120, // 20% change
			expected:          100,
		},
		{
			name:              "Memory change is larger",
			currentCPU:        100,
			recommendedCPU:    110, // 10% change
			currentMemory:     100,
			recommendedMemory: 150, // 50% change
			expected:          50,
		},
		{
			name:              "Negative changes - CPU decrease larger",
			currentCPU:        200,
			recommendedCPU:    100, // -50% change
			currentMemory:     100,
			recommendedMemory: 80, // -20% change
			expected:          50,
		},
		{
			name:              "No changes",
			currentCPU:        100,
			recommendedCPU:    100,
			currentMemory:     100,
			recommendedMemory: 100,
			expected:          0,
		},
		{
			name:              "Equal changes",
			currentCPU:        100,
			recommendedCPU:    130, // 30% change
			currentMemory:     100,
			recommendedMemory: 130, // 30% change
			expected:          30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &ContainerRecommendation{
				CurrentCPU:        tt.currentCPU,
				RecommendedCPU:    tt.recommendedCPU,
				CurrentMemory:     tt.currentMemory,
				RecommendedMemory: tt.recommendedMemory,
			}

			result := rec.MaxChangePercent()
			if math.Abs(result-tt.expected) > 0.01 {
				t.Errorf("MaxChangePercent() = %.2f, expected %.2f", result, tt.expected)
			}
		})
	}
}
