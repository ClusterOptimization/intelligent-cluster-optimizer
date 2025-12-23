package sla

import (
	"time"
)

// Metric represents a time-series metric value
type Metric struct {
	Timestamp time.Time
	Value     float64
	Labels    map[string]string
}

// SLAType defines the type of SLA
type SLAType string

const (
	// SLATypeLatency monitors response time/latency
	SLATypeLatency SLAType = "latency"

	// SLATypeErrorRate monitors error rate percentage
	SLATypeErrorRate SLAType = "error_rate"

	// SLATypeAvailability monitors uptime/availability percentage
	SLATypeAvailability SLAType = "availability"

	// SLATypeThroughput monitors requests per second
	SLATypeThroughput SLAType = "throughput"

	// SLATypeCustom allows custom SLA definitions
	SLATypeCustom SLAType = "custom"
)

// SLADefinition defines an SLA with thresholds
type SLADefinition struct {
	// Name of the SLA
	Name string

	// Type of SLA
	Type SLAType

	// Target is the target value for the metric
	Target float64

	// Threshold is the acceptable deviation from target
	Threshold float64

	// Percentile for percentile-based SLAs (e.g., P95, P99)
	Percentile float64

	// Window is the time window for evaluation
	Window time.Duration

	// Description of what this SLA monitors
	Description string
}

// SLAViolation represents a detected SLA violation
type SLAViolation struct {
	// SLA that was violated
	SLA SLADefinition

	// Timestamp when violation occurred
	Timestamp time.Time

	// ActualValue that violated the SLA
	ActualValue float64

	// ExpectedValue based on SLA definition
	ExpectedValue float64

	// Severity of the violation (0-1, where 1 is most severe)
	Severity float64

	// Message describing the violation
	Message string
}

// ControlChartPoint represents a point on a control chart
type ControlChartPoint struct {
	Timestamp   time.Time
	Value       float64
	Mean        float64
	UCL         float64 // Upper Control Limit
	LCL         float64 // Lower Control Limit
	IsOutlier   bool
	OutlierType OutlierType
}

// OutlierType classifies the type of outlier
type OutlierType string

const (
	// OutlierTypeNone indicates no outlier
	OutlierTypeNone OutlierType = "none"

	// OutlierTypeAbove indicates value above UCL
	OutlierTypeAbove OutlierType = "above_ucl"

	// OutlierTypeBelow indicates value below LCL
	OutlierTypeBelow OutlierType = "below_lcl"

	// OutlierTypeTrend indicates a trending pattern
	OutlierTypeTrend OutlierType = "trend"
)

// ControlChartConfig configures control chart generation
type ControlChartConfig struct {
	// SigmaLevel for control limits (typically 3)
	SigmaLevel float64

	// MinSamples required before generating control limits
	MinSamples int

	// EnableTrendDetection detects trending patterns
	EnableTrendDetection bool

	// TrendWindowSize for trend detection (number of consecutive points)
	TrendWindowSize int
}

// HealthCheckResult represents the result of a health check
type HealthCheckResult struct {
	// Timestamp when check was performed
	Timestamp time.Time

	// IsHealthy indicates if the system is healthy
	IsHealthy bool

	// Score is the health score (0-100)
	Score float64

	// Violations detected during the check
	Violations []SLAViolation

	// Metrics collected during the check
	Metrics []Metric

	// Outliers detected in the metrics
	Outliers []ControlChartPoint

	// Message providing context
	Message string
}

// OptimizationImpact represents the impact of an optimization
type OptimizationImpact struct {
	// PreOptimization health check before optimization
	PreOptimization HealthCheckResult

	// PostOptimization health check after optimization
	PostOptimization HealthCheckResult

	// ImpactScore measures the impact (-1 to 1, negative is degradation)
	ImpactScore float64

	// ViolationsAdded are new violations introduced
	ViolationsAdded []SLAViolation

	// ViolationsResolved are violations that were resolved
	ViolationsResolved []SLAViolation

	// Recommendation based on the impact
	Recommendation string
}

// Monitor defines the interface for SLA monitoring
type Monitor interface {
	// AddSLA adds an SLA definition to monitor
	AddSLA(sla SLADefinition) error

	// RemoveSLA removes an SLA definition
	RemoveSLA(name string) error

	// CheckSLA checks if metrics violate an SLA
	CheckSLA(slaName string, metrics []Metric) ([]SLAViolation, error)

	// CheckAllSLAs checks all defined SLAs
	CheckAllSLAs(metrics []Metric) ([]SLAViolation, error)

	// GetSLA retrieves an SLA definition
	GetSLA(name string) (*SLADefinition, error)

	// ListSLAs returns all defined SLAs
	ListSLAs() []SLADefinition
}

// ControlChart defines the interface for control chart generation
type ControlChart interface {
	// GenerateChart generates control chart data from metrics
	GenerateChart(metrics []Metric, config ControlChartConfig) ([]ControlChartPoint, error)

	// DetectOutliers detects outliers using 3-sigma method
	DetectOutliers(metrics []Metric, sigmaLevel float64) ([]ControlChartPoint, error)

	// CalculateControlLimits calculates UCL and LCL
	CalculateControlLimits(metrics []Metric, sigmaLevel float64) (mean, ucl, lcl float64, err error)
}

// HealthChecker defines the interface for health checking
type HealthChecker interface {
	// CheckHealth performs a health check
	CheckHealth(metrics []Metric, slas []SLADefinition) (*HealthCheckResult, error)

	// PreOptimizationCheck performs health check before optimization
	PreOptimizationCheck(metrics []Metric) (*HealthCheckResult, error)

	// PostOptimizationCheck performs health check after optimization
	PostOptimizationCheck(metrics []Metric) (*HealthCheckResult, error)

	// CompareHealth compares pre and post optimization health
	CompareHealth(pre, post *HealthCheckResult) (*OptimizationImpact, error)
}

// ValidateSLADefinition validates an SLA definition
func ValidateSLADefinition(sla SLADefinition) error {
	if sla.Name == "" {
		return &ValidationError{Field: "Name", Message: "SLA name is required"}
	}

	if sla.Type == "" {
		return &ValidationError{Field: "Type", Message: "SLA type is required"}
	}

	validTypes := map[SLAType]bool{
		SLATypeLatency:      true,
		SLATypeErrorRate:    true,
		SLATypeAvailability: true,
		SLATypeThroughput:   true,
		SLATypeCustom:       true,
	}

	if !validTypes[sla.Type] {
		return &ValidationError{Field: "Type", Message: "invalid SLA type"}
	}

	if sla.Threshold < 0 {
		return &ValidationError{Field: "Threshold", Message: "threshold must be non-negative"}
	}

	if sla.Percentile < 0 || sla.Percentile > 100 {
		return &ValidationError{Field: "Percentile", Message: "percentile must be between 0 and 100"}
	}

	if sla.Window <= 0 {
		return &ValidationError{Field: "Window", Message: "window must be positive"}
	}

	return nil
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
