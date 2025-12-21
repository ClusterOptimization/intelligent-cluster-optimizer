package prediction

import (
	"fmt"
	"time"

	"intelligent-cluster-optimizer/pkg/models"
)

// WorkloadPrediction contains resource usage predictions for a workload
type WorkloadPrediction struct {
	Namespace    string
	WorkloadName string

	// CPU predictions
	CPUForecast *ForecastResult

	// Memory predictions
	MemoryForecast *ForecastResult

	// Peak predictions
	PeakCPU        float64
	PeakMemory     float64
	PeakCPUTime    time.Time
	PeakMemoryTime time.Time

	// Trend analysis
	CPUTrend    TrendDirection
	MemoryTrend TrendDirection

	// Recommendations based on predictions
	RecommendedCPU    int64 // millicores
	RecommendedMemory int64 // bytes

	// Confidence in the prediction
	Confidence float64

	// Generated at
	GeneratedAt time.Time
}

// TrendDirection indicates the direction of the trend
type TrendDirection string

const (
	TrendUp     TrendDirection = "increasing"
	TrendDown   TrendDirection = "decreasing"
	TrendStable TrendDirection = "stable"
)

// WorkloadPredictor predicts future resource usage for workloads
type WorkloadPredictor struct {
	// Config for prediction
	Config *Config

	// Predictor to use
	predictor *HoltWinters

	// ForecastHorizon is how many periods ahead to predict
	ForecastHorizon int

	// SafetyMargin is multiplied to peak predictions for recommendations
	SafetyMargin float64

	// MinDataPoints required for prediction
	MinDataPoints int
}

// NewWorkloadPredictor creates a new workload predictor with default settings
func NewWorkloadPredictor() *WorkloadPredictor {
	config := DefaultConfig()
	config.SeasonalPeriod = 24 // Hourly data, daily seasonality

	return &WorkloadPredictor{
		Config:          config,
		predictor:       NewHoltWintersWithConfig(config),
		ForecastHorizon: 24, // Predict 24 periods ahead (e.g., next 24 hours)
		SafetyMargin:    1.2,
		MinDataPoints:   48, // 2 days minimum for hourly data
	}
}

// NewWorkloadPredictorWithConfig creates a workload predictor with custom settings
func NewWorkloadPredictorWithConfig(config *Config, horizon int, safetyMargin float64) *WorkloadPredictor {
	return &WorkloadPredictor{
		Config:          config,
		predictor:       NewHoltWintersWithConfig(config),
		ForecastHorizon: horizon,
		SafetyMargin:    safetyMargin,
		MinDataPoints:   config.SeasonalPeriod * config.MinDataPoints,
	}
}

// PredictWorkload predicts future resource usage for a workload
func (wp *WorkloadPredictor) PredictWorkload(namespace, workloadName string, metrics []models.PodMetric) (*WorkloadPrediction, error) {
	if len(metrics) < wp.MinDataPoints {
		return nil, fmt.Errorf("insufficient data: need at least %d points, got %d",
			wp.MinDataPoints, len(metrics))
	}

	// Extract CPU and memory values
	cpuValues, memoryValues, timestamps := extractValues(metrics)

	prediction := &WorkloadPrediction{
		Namespace:    namespace,
		WorkloadName: workloadName,
		GeneratedAt:  time.Now(),
	}

	// Predict CPU
	cpuPredictor := NewHoltWintersWithConfig(wp.Config)
	if err := cpuPredictor.FitWithTimestamps(cpuValues, timestamps); err == nil {
		cpuForecast, err := cpuPredictor.Predict(wp.ForecastHorizon)
		if err == nil {
			prediction.CPUForecast = cpuForecast
			peak := cpuForecast.PeakForecast()
			if peak != nil {
				prediction.PeakCPU = peak.Value
				prediction.PeakCPUTime = peak.Timestamp
			}
			prediction.CPUTrend = wp.determineTrend(cpuPredictor.GetTrend(), cpuPredictor.GetLevel())
		}
	}

	// Predict Memory
	memPredictor := NewHoltWintersWithConfig(wp.Config)
	if err := memPredictor.FitWithTimestamps(memoryValues, timestamps); err == nil {
		memForecast, err := memPredictor.Predict(wp.ForecastHorizon)
		if err == nil {
			prediction.MemoryForecast = memForecast
			peak := memForecast.PeakForecast()
			if peak != nil {
				prediction.PeakMemory = peak.Value
				prediction.PeakMemoryTime = peak.Timestamp
			}
			prediction.MemoryTrend = wp.determineTrend(memPredictor.GetTrend(), memPredictor.GetLevel())
		}
	}

	// Generate recommendations based on predicted peaks
	prediction.RecommendedCPU = int64(prediction.PeakCPU * wp.SafetyMargin)
	prediction.RecommendedMemory = int64(prediction.PeakMemory * wp.SafetyMargin)

	// Calculate overall confidence
	prediction.Confidence = wp.calculateConfidence(prediction)

	return prediction, nil
}

// PredictFromValues predicts from raw CPU and memory value slices
func (wp *WorkloadPredictor) PredictFromValues(cpuValues, memoryValues []float64, timestamps []time.Time) (*WorkloadPrediction, error) {
	if len(cpuValues) < wp.MinDataPoints {
		return nil, fmt.Errorf("insufficient data: need at least %d points, got %d",
			wp.MinDataPoints, len(cpuValues))
	}

	prediction := &WorkloadPrediction{
		GeneratedAt: time.Now(),
	}

	// Predict CPU
	cpuPredictor := NewHoltWintersWithConfig(wp.Config)
	if err := cpuPredictor.FitWithTimestamps(cpuValues, timestamps); err == nil {
		cpuForecast, err := cpuPredictor.Predict(wp.ForecastHorizon)
		if err == nil {
			prediction.CPUForecast = cpuForecast
			peak := cpuForecast.PeakForecast()
			if peak != nil {
				prediction.PeakCPU = peak.Value
				prediction.PeakCPUTime = peak.Timestamp
			}
			prediction.CPUTrend = wp.determineTrend(cpuPredictor.GetTrend(), cpuPredictor.GetLevel())
		}
	}

	// Predict Memory
	memPredictor := NewHoltWintersWithConfig(wp.Config)
	if err := memPredictor.FitWithTimestamps(memoryValues, timestamps); err == nil {
		memForecast, err := memPredictor.Predict(wp.ForecastHorizon)
		if err == nil {
			prediction.MemoryForecast = memForecast
			peak := memForecast.PeakForecast()
			if peak != nil {
				prediction.PeakMemory = peak.Value
				prediction.PeakMemoryTime = peak.Timestamp
			}
			prediction.MemoryTrend = wp.determineTrend(memPredictor.GetTrend(), memPredictor.GetLevel())
		}
	}

	// Generate recommendations
	prediction.RecommendedCPU = int64(prediction.PeakCPU * wp.SafetyMargin)
	prediction.RecommendedMemory = int64(prediction.PeakMemory * wp.SafetyMargin)

	// Calculate confidence
	prediction.Confidence = wp.calculateConfidence(prediction)

	return prediction, nil
}

// extractValues extracts CPU and memory values from pod metrics
func extractValues(metrics []models.PodMetric) (cpuValues, memoryValues []float64, timestamps []time.Time) {
	cpuValues = make([]float64, 0, len(metrics))
	memoryValues = make([]float64, 0, len(metrics))
	timestamps = make([]time.Time, 0, len(metrics))

	for _, pm := range metrics {
		var totalCPU, totalMemory int64
		for _, cm := range pm.Containers {
			totalCPU += cm.UsageCPU
			totalMemory += cm.UsageMemory
		}

		cpuValues = append(cpuValues, float64(totalCPU))
		memoryValues = append(memoryValues, float64(totalMemory))
		timestamps = append(timestamps, pm.Timestamp)
	}

	return cpuValues, memoryValues, timestamps
}

// determineTrend determines the trend direction based on trend component
func (wp *WorkloadPredictor) determineTrend(trend, level float64) TrendDirection {
	if level == 0 {
		return TrendStable
	}

	// Calculate trend as percentage of level
	trendPercent := (trend / level) * 100

	switch {
	case trendPercent > 5:
		return TrendUp
	case trendPercent < -5:
		return TrendDown
	default:
		return TrendStable
	}
}

// calculateConfidence calculates overall prediction confidence
func (wp *WorkloadPredictor) calculateConfidence(pred *WorkloadPrediction) float64 {
	var totalRMSE float64
	var count int

	if pred.CPUForecast != nil && pred.CPUForecast.Metrics != nil {
		// Normalize RMSE by converting to percentage accuracy
		if pred.PeakCPU > 0 {
			accuracy := 1 - (pred.CPUForecast.Metrics.RMSE / pred.PeakCPU)
			if accuracy < 0 {
				accuracy = 0
			}
			totalRMSE += accuracy
			count++
		}
	}

	if pred.MemoryForecast != nil && pred.MemoryForecast.Metrics != nil {
		if pred.PeakMemory > 0 {
			accuracy := 1 - (pred.MemoryForecast.Metrics.RMSE / pred.PeakMemory)
			if accuracy < 0 {
				accuracy = 0
			}
			totalRMSE += accuracy
			count++
		}
	}

	if count == 0 {
		return 0
	}

	return (totalRMSE / float64(count)) * 100
}

// Summary returns a human-readable summary of the prediction
func (p *WorkloadPrediction) Summary() string {
	return fmt.Sprintf(
		"Prediction for %s/%s: CPU trend=%s (peak=%.0f), Memory trend=%s (peak=%.0f), confidence=%.1f%%",
		p.Namespace, p.WorkloadName,
		p.CPUTrend, p.PeakCPU,
		p.MemoryTrend, p.PeakMemory,
		p.Confidence,
	)
}

// ShouldScaleUp returns true if predictions indicate scaling up is needed
func (p *WorkloadPrediction) ShouldScaleUp(currentCPU, currentMemory int64) bool {
	// Scale up if predicted peak exceeds current resources
	cpuNeedsScale := p.RecommendedCPU > currentCPU
	memoryNeedsScale := p.RecommendedMemory > currentMemory

	return cpuNeedsScale || memoryNeedsScale
}

// ShouldScaleDown returns true if predictions indicate scaling down is safe
func (p *WorkloadPrediction) ShouldScaleDown(currentCPU, currentMemory int64, threshold float64) bool {
	// Scale down if current resources exceed predicted needs by threshold
	cpuOverProvisioned := float64(currentCPU) > p.PeakCPU*(1+threshold)
	memoryOverProvisioned := float64(currentMemory) > p.PeakMemory*(1+threshold)

	return cpuOverProvisioned && memoryOverProvisioned
}

// GetScalingRecommendation returns scaling advice based on predictions
func (p *WorkloadPrediction) GetScalingRecommendation(currentCPU, currentMemory int64) string {
	if p.ShouldScaleUp(currentCPU, currentMemory) {
		return fmt.Sprintf("SCALE UP: Predicted peak CPU=%.0f (current=%d), Memory=%.0f (current=%d)",
			p.PeakCPU, currentCPU, p.PeakMemory, currentMemory)
	}

	if p.ShouldScaleDown(currentCPU, currentMemory, 0.3) {
		return fmt.Sprintf("SCALE DOWN: Current resources exceed predicted peak by >30%% (CPU: %d vs %.0f, Memory: %d vs %.0f)",
			currentCPU, p.PeakCPU, currentMemory, p.PeakMemory)
	}

	return "MAINTAIN: Current resources are appropriate for predicted usage"
}

// TimeUntilPeak returns the duration until the predicted peak
func (p *WorkloadPrediction) TimeUntilPeak() (cpuDuration, memoryDuration time.Duration) {
	now := time.Now()

	if !p.PeakCPUTime.IsZero() {
		cpuDuration = p.PeakCPUTime.Sub(now)
	}

	if !p.PeakMemoryTime.IsZero() {
		memoryDuration = p.PeakMemoryTime.Sub(now)
	}

	return cpuDuration, memoryDuration
}
