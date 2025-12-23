package sla

import (
	"fmt"
	"time"
)

// DefaultHealthChecker is the default implementation of HealthChecker
type DefaultHealthChecker struct {
	monitor      Monitor
	controlChart ControlChart
}

// NewHealthChecker creates a new health checker
func NewHealthChecker() HealthChecker {
	return &DefaultHealthChecker{
		monitor:      NewMonitor(),
		controlChart: NewControlChart(),
	}
}

// NewHealthCheckerWithMonitor creates a health checker with a custom monitor
func NewHealthCheckerWithMonitor(monitor Monitor) HealthChecker {
	return &DefaultHealthChecker{
		monitor:      monitor,
		controlChart: NewControlChart(),
	}
}

// CheckHealth performs a health check
func (h *DefaultHealthChecker) CheckHealth(metrics []Metric, slas []SLADefinition) (*HealthCheckResult, error) {
	result := &HealthCheckResult{
		Timestamp:  time.Now(),
		IsHealthy:  true,
		Score:      100.0,
		Violations: []SLAViolation{},
		Metrics:    metrics,
		Outliers:   []ControlChartPoint{},
		Message:    "System is healthy",
	}

	// Add all SLAs to monitor
	for _, sla := range slas {
		if err := h.monitor.AddSLA(sla); err != nil {
			return nil, fmt.Errorf("failed to add SLA %s: %w", sla.Name, err)
		}
	}

	// Check all SLA violations
	violations, err := h.monitor.CheckAllSLAs(metrics)
	if err != nil {
		return nil, fmt.Errorf("failed to check SLAs: %w", err)
	}

	result.Violations = violations

	// Detect outliers using control chart (3-sigma)
	if len(metrics) > 0 {
		outliers, err := h.controlChart.DetectOutliers(metrics, 3.0)
		if err == nil {
			result.Outliers = outliers
		}
	}

	// Calculate health score
	result.Score = h.calculateHealthScore(violations, result.Outliers)
	result.IsHealthy = result.Score >= 70.0 // Healthy threshold

	// Update message based on health status
	if !result.IsHealthy {
		if len(violations) > 0 {
			result.Message = fmt.Sprintf("System unhealthy: %d SLA violations detected", len(violations))
		} else {
			result.Message = fmt.Sprintf("System unhealthy: %d outliers detected", len(result.Outliers))
		}
	} else if len(violations) > 0 || len(result.Outliers) > 0 {
		result.Message = fmt.Sprintf("System healthy with warnings: %d violations, %d outliers", len(violations), len(result.Outliers))
	}

	return result, nil
}

// PreOptimizationCheck performs health check before optimization
func (h *DefaultHealthChecker) PreOptimizationCheck(metrics []Metric) (*HealthCheckResult, error) {
	// Define default SLAs for pre-optimization check
	defaultSLAs := []SLADefinition{
		{
			Name:        "response-time",
			Type:        SLATypeLatency,
			Target:      100.0, // 100ms target
			Threshold:   50.0,  // 150ms max acceptable
			Percentile:  95.0,  // P95
			Window:      5 * time.Minute,
			Description: "P95 response time should be under 150ms",
		},
		{
			Name:        "error-rate",
			Type:        SLATypeErrorRate,
			Target:      0.1,   // 0.1% target
			Threshold:   0.4,   // 0.5% max acceptable
			Window:      5 * time.Minute,
			Description: "Error rate should be under 0.5%",
		},
	}

	return h.CheckHealth(metrics, defaultSLAs)
}

// PostOptimizationCheck performs health check after optimization
func (h *DefaultHealthChecker) PostOptimizationCheck(metrics []Metric) (*HealthCheckResult, error) {
	// Use same SLAs as pre-optimization
	return h.PreOptimizationCheck(metrics)
}

// CompareHealth compares pre and post optimization health
func (h *DefaultHealthChecker) CompareHealth(pre, post *HealthCheckResult) (*OptimizationImpact, error) {
	if pre == nil || post == nil {
		return nil, fmt.Errorf("pre and post health results are required")
	}

	impact := &OptimizationImpact{
		PreOptimization:    *pre,
		PostOptimization:   *post,
		ViolationsAdded:    []SLAViolation{},
		ViolationsResolved: []SLAViolation{},
	}

	// Calculate impact score (-1 to 1, where 1 is improvement)
	scoreDiff := post.Score - pre.Score
	impact.ImpactScore = scoreDiff / 100.0

	// Find violations added and resolved
	preViolationMap := make(map[string]SLAViolation)
	for _, v := range pre.Violations {
		preViolationMap[v.SLA.Name] = v
	}

	postViolationMap := make(map[string]SLAViolation)
	for _, v := range post.Violations {
		postViolationMap[v.SLA.Name] = v
	}

	// Check for new violations (added)
	for name, v := range postViolationMap {
		if _, existedBefore := preViolationMap[name]; !existedBefore {
			impact.ViolationsAdded = append(impact.ViolationsAdded, v)
		}
	}

	// Check for resolved violations
	for name, v := range preViolationMap {
		if _, stillExists := postViolationMap[name]; !stillExists {
			impact.ViolationsResolved = append(impact.ViolationsResolved, v)
		}
	}

	// Generate recommendation
	impact.Recommendation = h.generateRecommendation(impact)

	return impact, nil
}

// calculateHealthScore calculates a health score (0-100) based on violations and outliers
func (h *DefaultHealthChecker) calculateHealthScore(violations []SLAViolation, outliers []ControlChartPoint) float64 {
	score := 100.0

	// Penalize for SLA violations (more severe)
	for _, violation := range violations {
		penalty := violation.Severity * 35.0 // Max 35 points per violation
		if penalty < 15.0 {
			penalty = 15.0 // Minimum 15 points per violation
		}
		score -= penalty
	}

	// Penalize for outliers (less severe than violations)
	outlierPenalty := float64(len(outliers)) * 2.0 // 2 points per outlier
	score -= outlierPenalty

	// Ensure score is within bounds
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}

// generateRecommendation generates a recommendation based on optimization impact
func (h *DefaultHealthChecker) generateRecommendation(impact *OptimizationImpact) string {
	if impact.ImpactScore > 0.1 {
		// Significant improvement
		if len(impact.ViolationsResolved) > 0 {
			return fmt.Sprintf("Optimization successful: health improved by %.1f%%, resolved %d SLA violations. Recommend: Keep changes.",
				impact.ImpactScore*100, len(impact.ViolationsResolved))
		}
		return fmt.Sprintf("Optimization successful: health improved by %.1f%%. Recommend: Keep changes.",
			impact.ImpactScore*100)
	} else if impact.ImpactScore < -0.1 {
		// Significant degradation
		if len(impact.ViolationsAdded) > 0 {
			return fmt.Sprintf("Optimization caused degradation: health decreased by %.1f%%, introduced %d SLA violations. Recommend: Rollback changes.",
				-impact.ImpactScore*100, len(impact.ViolationsAdded))
		}
		return fmt.Sprintf("Optimization caused degradation: health decreased by %.1f%%. Recommend: Rollback changes.",
			-impact.ImpactScore*100)
	} else {
		// Minimal impact
		if len(impact.ViolationsAdded) > 0 {
			return fmt.Sprintf("Optimization had minimal impact but introduced %d SLA violations. Recommend: Monitor closely or consider rollback.",
				len(impact.ViolationsAdded))
		}
		return "Optimization had minimal impact on health. Recommend: Monitor and keep changes if cost savings are significant."
	}
}

// IsSystemHealthy checks if the system is in a healthy state for optimization
func IsSystemHealthy(result *HealthCheckResult, minScore float64) bool {
	if result == nil {
		return false
	}

	return result.IsHealthy && result.Score >= minScore
}

// ShouldBlockOptimization determines if optimization should be blocked based on health
func ShouldBlockOptimization(result *HealthCheckResult) (bool, string) {
	if result == nil {
		return true, "No health check result available"
	}

	if !result.IsHealthy {
		return true, fmt.Sprintf("System is unhealthy (score: %.1f). %s", result.Score, result.Message)
	}

	// Check for critical violations
	for _, violation := range result.Violations {
		if violation.Severity > 0.8 {
			return true, fmt.Sprintf("Critical SLA violation detected: %s", violation.Message)
		}
	}

	return false, ""
}

// ShouldRollback determines if optimization should be rolled back based on health comparison
func ShouldRollback(impact *OptimizationImpact) (bool, string) {
	if impact == nil {
		return false, "No impact data available"
	}

	// Rollback if health degraded significantly
	if impact.ImpactScore < -0.15 {
		return true, fmt.Sprintf("Significant health degradation detected (score change: %.1f%%)", impact.ImpactScore*100)
	}

	// Rollback if new critical violations introduced
	for _, violation := range impact.ViolationsAdded {
		if violation.Severity > 0.8 {
			return true, fmt.Sprintf("Critical SLA violation introduced: %s", violation.Message)
		}
	}

	// Rollback if multiple violations added
	if len(impact.ViolationsAdded) >= 3 {
		return true, fmt.Sprintf("Multiple SLA violations introduced: %d violations", len(impact.ViolationsAdded))
	}

	return false, ""
}
