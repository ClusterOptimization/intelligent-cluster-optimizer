package leakdetector

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// Detector analyzes memory usage patterns to detect potential memory leaks
type Detector struct {
	// MinSamples is the minimum number of samples needed for analysis
	MinSamples int

	// MinDuration is the minimum time span needed for reliable analysis
	MinDuration time.Duration

	// SlopeThreshold is the minimum positive slope (bytes/hour) to consider a leak
	// Default: 1MB/hour
	SlopeThreshold float64

	// ResetThreshold is the percentage drop that indicates a memory reset
	// e.g., 0.2 means a 20% drop from peak is considered a reset
	ResetThreshold float64

	// MaxResets is the maximum number of resets allowed to still consider it a leak
	// If memory drops and recovers multiple times, it's normal GC behavior
	MaxResets int

	// ConsistencyThreshold is the R² value needed to consider the trend consistent
	// Higher = more consistent upward trend required
	ConsistencyThreshold float64

	// GrowthRateThreshold is the minimum percentage growth over the analysis period
	// to consider it significant
	GrowthRateThreshold float64
}

// MemorySample represents a single memory usage sample
type MemorySample struct {
	Timestamp time.Time
	Bytes     int64
}

// LeakAnalysis contains the results of memory leak analysis
type LeakAnalysis struct {
	// IsLeak indicates whether a memory leak is detected
	IsLeak bool

	// Severity indicates the severity of the detected leak
	Severity LeakSeverity

	// Confidence is the confidence level (0-100) in the leak detection
	Confidence float64

	// Description provides a human-readable description
	Description string

	// Recommendation provides action recommendations
	Recommendation string

	// Statistics contains detailed analysis statistics
	Statistics LeakStatistics

	// Alert contains alert information if leak is detected
	Alert *LeakAlert
}

// LeakSeverity indicates the severity of a memory leak
type LeakSeverity string

const (
	SeverityNone     LeakSeverity = "none"
	SeverityLow      LeakSeverity = "low"      // Slow leak, not urgent
	SeverityMedium   LeakSeverity = "medium"   // Moderate leak, investigate soon
	SeverityHigh     LeakSeverity = "high"     // Fast leak, investigate immediately
	SeverityCritical LeakSeverity = "critical" // Very fast leak, action required now
)

// LeakStatistics contains detailed statistics from the analysis
type LeakStatistics struct {
	// SampleCount is the number of samples analyzed
	SampleCount int

	// Duration is the time span of the analysis
	Duration time.Duration

	// StartMemory is the memory at the start of the period
	StartMemory int64

	// EndMemory is the memory at the end of the period
	EndMemory int64

	// MinMemory is the minimum memory observed
	MinMemory int64

	// MaxMemory is the maximum memory observed
	MaxMemory int64

	// MeanMemory is the average memory
	MeanMemory float64

	// Slope is the memory growth rate in bytes per hour
	Slope float64

	// SlopePerDay is the memory growth rate in bytes per day
	SlopePerDay float64

	// RSquared is the coefficient of determination (0-1)
	// Higher values indicate more consistent upward trend
	RSquared float64

	// GrowthPercent is the percentage growth from start to end
	GrowthPercent float64

	// ResetCount is the number of times memory dropped significantly
	ResetCount int

	// TimeToOOM is the estimated time until memory exhaustion (if limit known)
	TimeToOOM time.Duration

	// ProjectedMemory24h is the projected memory in 24 hours at current rate
	ProjectedMemory24h int64

	// ProjectedMemory7d is the projected memory in 7 days at current rate
	ProjectedMemory7d int64
}

// LeakAlert contains alert information for a detected leak
type LeakAlert struct {
	// Title is the alert title
	Title string

	// Message is the detailed alert message
	Message string

	// Severity is the alert severity
	Severity LeakSeverity

	// Workload identifies the affected workload
	Workload string

	// Container identifies the affected container
	Container string

	// DetectedAt is when the leak was detected
	DetectedAt time.Time

	// GrowthRate is the memory growth rate (human readable)
	GrowthRate string

	// ProjectedImpact describes the projected impact
	ProjectedImpact string

	// SuggestedActions lists suggested remediation actions
	SuggestedActions []string
}

// NewDetector creates a new memory leak detector with default settings
func NewDetector() *Detector {
	return &Detector{
		MinSamples:           20,
		MinDuration:          1 * time.Hour,
		SlopeThreshold:       1024 * 1024,     // 1 MB/hour
		ResetThreshold:       0.15,            // 15% drop = reset
		MaxResets:            2,               // Max 2 resets to still consider leak
		ConsistencyThreshold: 0.7,             // R² >= 0.7 for consistent trend
		GrowthRateThreshold:  0.1,             // 10% growth minimum
	}
}

// Analyze analyzes memory samples to detect potential leaks
func (d *Detector) Analyze(samples []MemorySample) *LeakAnalysis {
	analysis := &LeakAnalysis{
		IsLeak:   false,
		Severity: SeverityNone,
	}

	// Validate input
	if len(samples) < d.MinSamples {
		analysis.Description = fmt.Sprintf("Insufficient samples for analysis (%d < %d required)",
			len(samples), d.MinSamples)
		return analysis
	}

	// Sort samples by timestamp
	sortedSamples := make([]MemorySample, len(samples))
	copy(sortedSamples, samples)
	sort.Slice(sortedSamples, func(i, j int) bool {
		return sortedSamples[i].Timestamp.Before(sortedSamples[j].Timestamp)
	})

	// Check duration
	duration := sortedSamples[len(sortedSamples)-1].Timestamp.Sub(sortedSamples[0].Timestamp)
	if duration < d.MinDuration {
		analysis.Description = fmt.Sprintf("Insufficient time span for analysis (%v < %v required)",
			duration.Round(time.Minute), d.MinDuration)
		return analysis
	}

	// Calculate statistics
	stats := d.calculateStatistics(sortedSamples)
	analysis.Statistics = stats

	// Detect leak based on multiple criteria
	d.detectLeak(analysis, sortedSamples)

	return analysis
}

// calculateStatistics calculates detailed statistics from samples
func (d *Detector) calculateStatistics(samples []MemorySample) LeakStatistics {
	n := len(samples)
	stats := LeakStatistics{
		SampleCount: n,
		Duration:    samples[n-1].Timestamp.Sub(samples[0].Timestamp),
		StartMemory: samples[0].Bytes,
		EndMemory:   samples[n-1].Bytes,
		MinMemory:   samples[0].Bytes,
		MaxMemory:   samples[0].Bytes,
	}

	// Calculate min, max, mean
	var sum float64
	for _, s := range samples {
		sum += float64(s.Bytes)
		if s.Bytes < stats.MinMemory {
			stats.MinMemory = s.Bytes
		}
		if s.Bytes > stats.MaxMemory {
			stats.MaxMemory = s.Bytes
		}
	}
	stats.MeanMemory = sum / float64(n)

	// Calculate linear regression for slope and R²
	stats.Slope, stats.RSquared = d.calculateLinearRegression(samples)
	stats.SlopePerDay = stats.Slope * 24

	// Calculate growth percentage
	if stats.StartMemory > 0 {
		stats.GrowthPercent = float64(stats.EndMemory-stats.StartMemory) / float64(stats.StartMemory) * 100
	}

	// Count resets (significant drops in memory)
	stats.ResetCount = d.countResets(samples)

	// Project future memory
	if stats.Slope > 0 {
		stats.ProjectedMemory24h = stats.EndMemory + int64(stats.Slope*24)
		stats.ProjectedMemory7d = stats.EndMemory + int64(stats.Slope*24*7)
	} else {
		stats.ProjectedMemory24h = stats.EndMemory
		stats.ProjectedMemory7d = stats.EndMemory
	}

	return stats
}

// calculateLinearRegression performs linear regression on memory samples
// Returns slope (bytes/hour) and R² (coefficient of determination)
func (d *Detector) calculateLinearRegression(samples []MemorySample) (slope, rSquared float64) {
	n := float64(len(samples))
	if n < 2 {
		return 0, 0
	}

	// Convert timestamps to hours from start
	startTime := samples[0].Timestamp
	var sumX, sumY, sumXY, sumX2, sumY2 float64

	for _, s := range samples {
		x := s.Timestamp.Sub(startTime).Hours()
		y := float64(s.Bytes)

		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
		sumY2 += y * y
	}

	// Calculate slope
	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		return 0, 0
	}
	slope = (n*sumXY - sumX*sumY) / denominator

	// Calculate R²
	meanY := sumY / n
	var ssTotal, ssResidual float64
	intercept := (sumY - slope*sumX) / n

	for _, s := range samples {
		x := s.Timestamp.Sub(startTime).Hours()
		y := float64(s.Bytes)
		predicted := intercept + slope*x

		ssTotal += (y - meanY) * (y - meanY)
		ssResidual += (y - predicted) * (y - predicted)
	}

	if ssTotal > 0 {
		rSquared = 1 - (ssResidual / ssTotal)
	}

	// Clamp R² to [0, 1]
	if rSquared < 0 {
		rSquared = 0
	}
	if rSquared > 1 {
		rSquared = 1
	}

	return slope, rSquared
}

// countResets counts significant memory drops (potential GC or restarts)
func (d *Detector) countResets(samples []MemorySample) int {
	resets := 0
	peakMemory := samples[0].Bytes

	for i := 1; i < len(samples); i++ {
		current := samples[i].Bytes

		// Update peak
		if current > peakMemory {
			peakMemory = current
		}

		// Check for reset (significant drop from peak)
		if peakMemory > 0 {
			dropPercent := float64(peakMemory-current) / float64(peakMemory)
			if dropPercent >= d.ResetThreshold {
				resets++
				peakMemory = current // Reset peak after a drop
			}
		}
	}

	return resets
}

// detectLeak determines if a memory leak is present and its severity
func (d *Detector) detectLeak(analysis *LeakAnalysis, samples []MemorySample) {
	stats := analysis.Statistics

	// Check if slope is positive and significant
	if stats.Slope <= d.SlopeThreshold {
		analysis.Description = fmt.Sprintf("Memory is stable or decreasing (slope: %.2f MB/hour)",
			stats.Slope/(1024*1024))
		return
	}

	// Check if trend is consistent (high R²)
	if stats.RSquared < d.ConsistencyThreshold {
		analysis.Description = fmt.Sprintf("Memory increases but inconsistently (R²: %.2f < %.2f). Likely normal GC behavior.",
			stats.RSquared, d.ConsistencyThreshold)
		return
	}

	// Check for resets - if too many, it's probably normal GC
	if stats.ResetCount > d.MaxResets {
		analysis.Description = fmt.Sprintf("Memory pattern shows %d resets (drops), indicating normal GC behavior.",
			stats.ResetCount)
		return
	}

	// Check if growth is significant
	if stats.GrowthPercent < d.GrowthRateThreshold*100 {
		analysis.Description = fmt.Sprintf("Memory growth (%.1f%%) is below threshold (%.1f%%).",
			stats.GrowthPercent, d.GrowthRateThreshold*100)
		return
	}

	// At this point, we have detected a leak
	analysis.IsLeak = true

	// Determine severity based on growth rate
	mbPerHour := stats.Slope / (1024 * 1024)
	analysis.Severity = d.classifySeverity(mbPerHour, stats.GrowthPercent, stats.Duration)

	// Calculate confidence based on R² and sample count
	sampleConfidence := math.Min(float64(stats.SampleCount)/100, 1.0) * 30
	rSquaredConfidence := stats.RSquared * 50
	growthConfidence := math.Min(stats.GrowthPercent/50, 1.0) * 20
	analysis.Confidence = sampleConfidence + rSquaredConfidence + growthConfidence

	// Generate description
	analysis.Description = fmt.Sprintf(
		"MEMORY LEAK DETECTED: Memory is growing at %.2f MB/hour (%.1f%% over %v) with %.0f%% consistency. "+
			"Only %d memory resets observed.",
		mbPerHour,
		stats.GrowthPercent,
		stats.Duration.Round(time.Minute),
		stats.RSquared*100,
		stats.ResetCount,
	)

	// Generate recommendation
	analysis.Recommendation = d.generateRecommendation(analysis)

	// Generate alert
	analysis.Alert = d.generateAlert(analysis)
}

// classifySeverity determines leak severity based on growth rate
func (d *Detector) classifySeverity(mbPerHour, growthPercent float64, duration time.Duration) LeakSeverity {
	// Calculate growth rate per hour
	hoursElapsed := duration.Hours()
	if hoursElapsed <= 0 {
		hoursElapsed = 1
	}
	growthPerHour := growthPercent / hoursElapsed

	// Classify based on MB/hour and growth rate
	switch {
	case mbPerHour >= 100 || growthPerHour >= 10:
		return SeverityCritical // Very fast leak
	case mbPerHour >= 50 || growthPerHour >= 5:
		return SeverityHigh // Fast leak
	case mbPerHour >= 10 || growthPerHour >= 2:
		return SeverityMedium // Moderate leak
	default:
		return SeverityLow // Slow leak
	}
}

// generateRecommendation generates action recommendations based on analysis
func (d *Detector) generateRecommendation(analysis *LeakAnalysis) string {
	switch analysis.Severity {
	case SeverityCritical:
		return "URGENT: Do not scale resources. Immediately investigate the memory leak. " +
			"Consider restarting the workload as a temporary measure while investigating root cause. " +
			"Check for: unclosed connections, growing caches, event listener leaks, circular references."

	case SeverityHigh:
		return "Do not scale resources. Investigate memory leak within 24 hours. " +
			"Enable heap profiling and analyze memory allocation patterns. " +
			"Review recent code changes that may have introduced the leak."

	case SeverityMedium:
		return "Do not scale resources without investigating. Memory leak detected. " +
			"Schedule investigation within the week. Consider enabling memory profiling " +
			"and reviewing application memory management patterns."

	case SeverityLow:
		return "Possible slow memory leak detected. Monitor closely and investigate when convenient. " +
			"The leak is slow enough that it may not cause immediate issues, but should be addressed. " +
			"Consider extending the monitoring period to confirm the pattern."

	default:
		return "No action required."
	}
}

// generateAlert generates an alert for the detected leak
func (d *Detector) generateAlert(analysis *LeakAnalysis) *LeakAlert {
	if !analysis.IsLeak {
		return nil
	}

	stats := analysis.Statistics
	mbPerHour := stats.Slope / (1024 * 1024)
	mbPerDay := stats.SlopePerDay / (1024 * 1024)

	alert := &LeakAlert{
		Title:      fmt.Sprintf("Memory Leak Detected - %s Severity", analysis.Severity),
		Severity:   analysis.Severity,
		DetectedAt: time.Now(),
		GrowthRate: fmt.Sprintf("%.2f MB/hour (%.2f MB/day)", mbPerHour, mbPerDay),
	}

	// Build message
	alert.Message = fmt.Sprintf(
		"Memory leak detected with %.0f%% confidence.\n\n"+
			"Growth Rate: %s\n"+
			"Growth: %.1f%% over %v\n"+
			"Current Memory: %s\n"+
			"Projected (24h): %s\n"+
			"Projected (7d): %s\n"+
			"Trend Consistency: %.0f%%\n"+
			"Memory Resets: %d",
		analysis.Confidence,
		alert.GrowthRate,
		stats.GrowthPercent,
		stats.Duration.Round(time.Minute),
		formatBytes(stats.EndMemory),
		formatBytes(stats.ProjectedMemory24h),
		formatBytes(stats.ProjectedMemory7d),
		stats.RSquared*100,
		stats.ResetCount,
	)

	// Projected impact
	if stats.ProjectedMemory7d > stats.EndMemory*2 {
		alert.ProjectedImpact = fmt.Sprintf("Memory will double within 7 days at current rate. "+
			"Projected to reach %s.", formatBytes(stats.ProjectedMemory7d))
	} else {
		alert.ProjectedImpact = fmt.Sprintf("Memory will reach %s in 7 days at current rate.",
			formatBytes(stats.ProjectedMemory7d))
	}

	// Suggested actions based on severity
	alert.SuggestedActions = d.getSuggestedActions(analysis.Severity)

	return alert
}

// getSuggestedActions returns suggested remediation actions based on severity
func (d *Detector) getSuggestedActions(severity LeakSeverity) []string {
	common := []string{
		"Enable heap profiling to identify memory allocation hotspots",
		"Review recent code changes for potential memory leaks",
		"Check for unclosed resources (connections, file handles, channels)",
	}

	switch severity {
	case SeverityCritical:
		return append([]string{
			"IMMEDIATE: Consider rolling restart of affected pods",
			"Enable debug logging for memory allocations",
			"Prepare for potential OOM if not addressed within hours",
		}, common...)

	case SeverityHigh:
		return append([]string{
			"Schedule investigation within 24 hours",
			"Consider enabling memory limits if not set",
			"Prepare rollback plan for recent deployments",
		}, common...)

	case SeverityMedium:
		return append([]string{
			"Schedule investigation within the week",
			"Increase monitoring frequency for this workload",
		}, common...)

	case SeverityLow:
		return append([]string{
			"Continue monitoring to confirm pattern",
			"Consider extending analysis period for more data",
		}, common...)

	default:
		return common
	}
}

// AnalyzeWithLimit analyzes samples and projects time to OOM based on memory limit
func (d *Detector) AnalyzeWithLimit(samples []MemorySample, memoryLimit int64) *LeakAnalysis {
	analysis := d.Analyze(samples)

	// Calculate time to OOM if we have a leak and a limit
	if analysis.IsLeak && memoryLimit > 0 && analysis.Statistics.Slope > 0 {
		remainingBytes := memoryLimit - analysis.Statistics.EndMemory
		if remainingBytes > 0 {
			hoursToOOM := float64(remainingBytes) / analysis.Statistics.Slope
			analysis.Statistics.TimeToOOM = time.Duration(hoursToOOM * float64(time.Hour))

			// Add time to OOM to alert
			if analysis.Alert != nil {
				analysis.Alert.Message += fmt.Sprintf("\n\nEstimated Time to OOM: %v",
					analysis.Statistics.TimeToOOM.Round(time.Minute))

				if analysis.Statistics.TimeToOOM < 24*time.Hour {
					analysis.Alert.SuggestedActions = append(
						[]string{fmt.Sprintf("WARNING: OOM expected in %v - take immediate action",
							analysis.Statistics.TimeToOOM.Round(time.Minute))},
						analysis.Alert.SuggestedActions...,
					)
				}
			}
		}
	}

	return analysis
}

// formatBytes formats bytes into human-readable string
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}

// FormatAnalysisSummary returns a formatted summary of the analysis
func (a *LeakAnalysis) FormatAnalysisSummary() string {
	stats := a.Statistics

	summary := fmt.Sprintf(`Memory Leak Analysis
====================
Leak Detected: %v
Severity: %s
Confidence: %.0f%%

%s

Statistics:
  Samples Analyzed: %d
  Analysis Duration: %v
  Memory Start: %s
  Memory End: %s
  Memory Growth: %.1f%%
  Growth Rate: %.2f MB/hour (%.2f MB/day)
  Trend Consistency (R²): %.2f
  Memory Resets: %d

Projections:
  Memory in 24h: %s
  Memory in 7d: %s
`,
		a.IsLeak,
		a.Severity,
		a.Confidence,
		a.Description,
		stats.SampleCount,
		stats.Duration.Round(time.Minute),
		formatBytes(stats.StartMemory),
		formatBytes(stats.EndMemory),
		stats.GrowthPercent,
		stats.Slope/(1024*1024),
		stats.SlopePerDay/(1024*1024),
		stats.RSquared,
		stats.ResetCount,
		formatBytes(stats.ProjectedMemory24h),
		formatBytes(stats.ProjectedMemory7d),
	)

	if a.IsLeak {
		summary += fmt.Sprintf("\nRecommendation:\n  %s\n", a.Recommendation)

		if a.Alert != nil && len(a.Alert.SuggestedActions) > 0 {
			summary += "\nSuggested Actions:\n"
			for i, action := range a.Alert.SuggestedActions {
				summary += fmt.Sprintf("  %d. %s\n", i+1, action)
			}
		}
	}

	return summary
}

// ShouldPreventScaling returns true if scaling should be blocked due to leak
func (a *LeakAnalysis) ShouldPreventScaling() (bool, string) {
	if !a.IsLeak {
		return false, ""
	}

	switch a.Severity {
	case SeverityCritical, SeverityHigh:
		return true, fmt.Sprintf("Memory leak detected (%s severity). "+
			"Scaling blocked - investigate leak before adding resources.", a.Severity)
	case SeverityMedium:
		return true, fmt.Sprintf("Possible memory leak detected (%s severity). "+
			"Scaling blocked - verify this is not a leak before adding resources.", a.Severity)
	case SeverityLow:
		return false, fmt.Sprintf("Slow memory leak detected (%s severity). "+
			"Scaling allowed but monitoring recommended.", a.Severity)
	default:
		return false, ""
	}
}
