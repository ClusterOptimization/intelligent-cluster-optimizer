package timepattern

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// Analyzer detects time-based usage patterns in workload metrics
type Analyzer struct {
	// MinDataPoints is the minimum data points needed per hour for analysis
	MinDataPoints int

	// PeakThresholdRatio defines how much higher than average constitutes a "peak"
	// e.g., 1.5 means 50% above average
	PeakThresholdRatio float64

	// OffPeakThresholdRatio defines how much lower than average constitutes "off-peak"
	// e.g., 0.5 means 50% below average
	OffPeakThresholdRatio float64

	// MinPeakDuration is the minimum consecutive hours to consider a peak pattern
	MinPeakDuration int

	// SignificantVariationCV is the coefficient of variation threshold for significant patterns
	// Below this, usage is considered "flat" with no time-based pattern
	SignificantVariationCV float64
}

// TimePattern represents detected time-based usage patterns
type TimePattern struct {
	// HasPattern indicates whether a significant time pattern was detected
	HasPattern bool

	// PatternType describes the type of pattern detected
	PatternType PatternType

	// Description is a human-readable description of the pattern
	Description string

	// PeakHours are the hours (0-23) with above-average usage
	PeakHours []int

	// OffPeakHours are the hours (0-23) with below-average usage
	OffPeakHours []int

	// PeakDays are the days of week (0=Sunday, 6=Saturday) with above-average usage
	PeakDays []time.Weekday

	// OffPeakDays are the days of week with below-average usage
	OffPeakDays []time.Weekday

	// HourlyStats contains statistics for each hour
	HourlyStats [24]HourStats

	// DailyStats contains statistics for each day of week
	DailyStats [7]DayStats

	// OverallStats contains overall statistics
	OverallStats OverallStats

	// ScalingRecommendation contains the recommended scaling schedule
	ScalingRecommendation *ScalingSchedule
}

// PatternType describes the type of time-based pattern
type PatternType string

const (
	// PatternNone indicates no significant time pattern
	PatternNone PatternType = "none"

	// PatternBusinessHours indicates typical 9-5 business hours pattern
	PatternBusinessHours PatternType = "business_hours"

	// PatternNightBatch indicates nighttime batch processing pattern
	PatternNightBatch PatternType = "night_batch"

	// PatternWeekdayOnly indicates weekday-only usage
	PatternWeekdayOnly PatternType = "weekday_only"

	// PatternWeekendPeak indicates weekend peak usage
	PatternWeekendPeak PatternType = "weekend_peak"

	// PatternMorningSpike indicates morning spike pattern
	PatternMorningSpike PatternType = "morning_spike"

	// PatternEveningSpike indicates evening spike pattern
	PatternEveningSpike PatternType = "evening_spike"

	// PatternCustom indicates a custom/irregular pattern
	PatternCustom PatternType = "custom"
)

// HourStats contains statistics for a specific hour
type HourStats struct {
	Hour         int
	SampleCount  int
	MeanCPU      float64
	MeanMemory   float64
	MaxCPU       int64
	MaxMemory    int64
	MinCPU       int64
	MinMemory    int64
	StdDevCPU    float64
	StdDevMemory float64
	IsPeak       bool
	IsOffPeak    bool
}

// DayStats contains statistics for a specific day of week
type DayStats struct {
	Day          time.Weekday
	SampleCount  int
	MeanCPU      float64
	MeanMemory   float64
	MaxCPU       int64
	MaxMemory    int64
	IsPeak       bool
	IsOffPeak    bool
}

// OverallStats contains overall usage statistics
type OverallStats struct {
	TotalSamples     int
	MeanCPU          float64
	MeanMemory       float64
	MaxCPU           int64
	MaxMemory        int64
	MinCPU           int64
	MinMemory        int64
	StdDevCPU        float64
	StdDevMemory     float64
	CoefficientOfVar float64 // CV = StdDev / Mean
	PeakToPeakRatio  float64 // Max / Min ratio
}

// ScalingSchedule represents a recommended scaling schedule
type ScalingSchedule struct {
	// Enabled indicates if schedule-based scaling is recommended
	Enabled bool

	// Reason explains why schedule-based scaling is recommended
	Reason string

	// Schedules contains the scaling schedules
	Schedules []ScheduleEntry

	// EstimatedSavingsPercent is the estimated resource savings
	EstimatedSavingsPercent float64
}

// ScheduleEntry represents a single scaling schedule entry
type ScheduleEntry struct {
	// Name is a human-readable name for this schedule
	Name string

	// CronSchedule is the cron expression for when this applies
	CronSchedule string

	// Duration is how long this schedule applies
	Duration time.Duration

	// CPUMultiplier is the multiplier to apply to base CPU (1.0 = no change)
	CPUMultiplier float64

	// MemoryMultiplier is the multiplier to apply to base memory
	MemoryMultiplier float64

	// Description explains this schedule entry
	Description string
}

// Sample represents a single metric sample for analysis
type Sample struct {
	Timestamp time.Time
	CPU       int64 // millicores
	Memory    int64 // bytes
}

// NewAnalyzer creates a new time pattern analyzer with default settings
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		MinDataPoints:          3,    // At least 3 samples per hour
		PeakThresholdRatio:     1.3,  // 30% above average = peak
		OffPeakThresholdRatio:  0.7,  // 30% below average = off-peak
		MinPeakDuration:        2,    // At least 2 consecutive hours
		SignificantVariationCV: 0.25, // 25% CV threshold for patterns
	}
}

// Analyze analyzes samples to detect time-based patterns
func (a *Analyzer) Analyze(samples []Sample) *TimePattern {
	pattern := &TimePattern{
		HasPattern:  false,
		PatternType: PatternNone,
	}

	if len(samples) == 0 {
		pattern.Description = "No data available for analysis"
		return pattern
	}

	// Calculate hourly and daily statistics
	a.calculateHourlyStats(samples, pattern)
	a.calculateDailyStats(samples, pattern)
	a.calculateOverallStats(samples, pattern)

	// Detect if there's a significant pattern
	if pattern.OverallStats.CoefficientOfVar < a.SignificantVariationCV {
		pattern.Description = fmt.Sprintf("Flat usage pattern (CV=%.2f%%) - no time-based scaling recommended",
			pattern.OverallStats.CoefficientOfVar*100)
		return pattern
	}

	// Identify peak and off-peak hours
	a.identifyPeakHours(pattern)
	a.identifyPeakDays(pattern)

	// Classify the pattern
	a.classifyPattern(pattern)

	// Generate scaling recommendation
	pattern.ScalingRecommendation = a.generateScalingRecommendation(pattern)

	return pattern
}

// calculateHourlyStats calculates statistics for each hour of day
func (a *Analyzer) calculateHourlyStats(samples []Sample, pattern *TimePattern) {
	// Group samples by hour
	hourlyData := make(map[int][]Sample)
	for _, s := range samples {
		hour := s.Timestamp.Hour()
		hourlyData[hour] = append(hourlyData[hour], s)
	}

	// Calculate stats for each hour
	for hour := 0; hour < 24; hour++ {
		data := hourlyData[hour]
		stats := HourStats{Hour: hour, SampleCount: len(data)}

		if len(data) > 0 {
			var sumCPU, sumMemory float64
			stats.MinCPU, stats.MinMemory = data[0].CPU, data[0].Memory
			stats.MaxCPU, stats.MaxMemory = data[0].CPU, data[0].Memory

			for _, s := range data {
				sumCPU += float64(s.CPU)
				sumMemory += float64(s.Memory)
				if s.CPU < stats.MinCPU {
					stats.MinCPU = s.CPU
				}
				if s.CPU > stats.MaxCPU {
					stats.MaxCPU = s.CPU
				}
				if s.Memory < stats.MinMemory {
					stats.MinMemory = s.Memory
				}
				if s.Memory > stats.MaxMemory {
					stats.MaxMemory = s.Memory
				}
			}

			stats.MeanCPU = sumCPU / float64(len(data))
			stats.MeanMemory = sumMemory / float64(len(data))

			// Calculate standard deviation
			var sumSqDiffCPU, sumSqDiffMemory float64
			for _, s := range data {
				sumSqDiffCPU += math.Pow(float64(s.CPU)-stats.MeanCPU, 2)
				sumSqDiffMemory += math.Pow(float64(s.Memory)-stats.MeanMemory, 2)
			}
			stats.StdDevCPU = math.Sqrt(sumSqDiffCPU / float64(len(data)))
			stats.StdDevMemory = math.Sqrt(sumSqDiffMemory / float64(len(data)))
		}

		pattern.HourlyStats[hour] = stats
	}
}

// calculateDailyStats calculates statistics for each day of week
func (a *Analyzer) calculateDailyStats(samples []Sample, pattern *TimePattern) {
	// Group samples by day of week
	dailyData := make(map[time.Weekday][]Sample)
	for _, s := range samples {
		day := s.Timestamp.Weekday()
		dailyData[day] = append(dailyData[day], s)
	}

	// Calculate stats for each day
	for day := time.Sunday; day <= time.Saturday; day++ {
		data := dailyData[day]
		stats := DayStats{Day: day, SampleCount: len(data)}

		if len(data) > 0 {
			var sumCPU, sumMemory float64
			stats.MaxCPU, stats.MaxMemory = data[0].CPU, data[0].Memory

			for _, s := range data {
				sumCPU += float64(s.CPU)
				sumMemory += float64(s.Memory)
				if s.CPU > stats.MaxCPU {
					stats.MaxCPU = s.CPU
				}
				if s.Memory > stats.MaxMemory {
					stats.MaxMemory = s.Memory
				}
			}

			stats.MeanCPU = sumCPU / float64(len(data))
			stats.MeanMemory = sumMemory / float64(len(data))
		}

		pattern.DailyStats[day] = stats
	}
}

// calculateOverallStats calculates overall statistics
func (a *Analyzer) calculateOverallStats(samples []Sample, pattern *TimePattern) {
	if len(samples) == 0 {
		return
	}

	stats := &pattern.OverallStats
	stats.TotalSamples = len(samples)
	stats.MinCPU, stats.MinMemory = samples[0].CPU, samples[0].Memory
	stats.MaxCPU, stats.MaxMemory = samples[0].CPU, samples[0].Memory

	var sumCPU, sumMemory float64
	for _, s := range samples {
		sumCPU += float64(s.CPU)
		sumMemory += float64(s.Memory)
		if s.CPU < stats.MinCPU {
			stats.MinCPU = s.CPU
		}
		if s.CPU > stats.MaxCPU {
			stats.MaxCPU = s.CPU
		}
		if s.Memory < stats.MinMemory {
			stats.MinMemory = s.Memory
		}
		if s.Memory > stats.MaxMemory {
			stats.MaxMemory = s.Memory
		}
	}

	stats.MeanCPU = sumCPU / float64(len(samples))
	stats.MeanMemory = sumMemory / float64(len(samples))

	// Calculate standard deviation
	var sumSqDiffCPU, sumSqDiffMemory float64
	for _, s := range samples {
		sumSqDiffCPU += math.Pow(float64(s.CPU)-stats.MeanCPU, 2)
		sumSqDiffMemory += math.Pow(float64(s.Memory)-stats.MeanMemory, 2)
	}
	stats.StdDevCPU = math.Sqrt(sumSqDiffCPU / float64(len(samples)))
	stats.StdDevMemory = math.Sqrt(sumSqDiffMemory / float64(len(samples)))

	// Calculate coefficient of variation (using CPU as primary metric)
	if stats.MeanCPU > 0 {
		stats.CoefficientOfVar = stats.StdDevCPU / stats.MeanCPU
	}

	// Calculate peak-to-peak ratio
	if stats.MinCPU > 0 {
		stats.PeakToPeakRatio = float64(stats.MaxCPU) / float64(stats.MinCPU)
	}
}

// identifyPeakHours identifies peak and off-peak hours
func (a *Analyzer) identifyPeakHours(pattern *TimePattern) {
	meanCPU := pattern.OverallStats.MeanCPU
	peakThreshold := meanCPU * a.PeakThresholdRatio
	offPeakThreshold := meanCPU * a.OffPeakThresholdRatio

	for hour := 0; hour < 24; hour++ {
		stats := &pattern.HourlyStats[hour]
		if stats.SampleCount < a.MinDataPoints {
			continue
		}

		if stats.MeanCPU >= peakThreshold {
			stats.IsPeak = true
			pattern.PeakHours = append(pattern.PeakHours, hour)
		} else if stats.MeanCPU <= offPeakThreshold {
			stats.IsOffPeak = true
			pattern.OffPeakHours = append(pattern.OffPeakHours, hour)
		}
	}

	sort.Ints(pattern.PeakHours)
	sort.Ints(pattern.OffPeakHours)
}

// identifyPeakDays identifies peak and off-peak days
func (a *Analyzer) identifyPeakDays(pattern *TimePattern) {
	meanCPU := pattern.OverallStats.MeanCPU
	peakThreshold := meanCPU * a.PeakThresholdRatio
	offPeakThreshold := meanCPU * a.OffPeakThresholdRatio

	for day := time.Sunday; day <= time.Saturday; day++ {
		stats := &pattern.DailyStats[day]
		if stats.SampleCount < a.MinDataPoints {
			continue
		}

		if stats.MeanCPU >= peakThreshold {
			stats.IsPeak = true
			pattern.PeakDays = append(pattern.PeakDays, day)
		} else if stats.MeanCPU <= offPeakThreshold {
			stats.IsOffPeak = true
			pattern.OffPeakDays = append(pattern.OffPeakDays, day)
		}
	}
}

// classifyPattern classifies the detected pattern
func (a *Analyzer) classifyPattern(pattern *TimePattern) {
	pattern.HasPattern = true

	// Check for business hours pattern (roughly 8-18)
	if a.isBusinessHoursPattern(pattern) {
		pattern.PatternType = PatternBusinessHours
		pattern.Description = "Business hours pattern detected: higher usage during typical work hours (8 AM - 6 PM)"
		return
	}

	// Check for night batch pattern
	if a.isNightBatchPattern(pattern) {
		pattern.PatternType = PatternNightBatch
		pattern.Description = "Night batch pattern detected: higher usage during nighttime hours"
		return
	}

	// Check for weekday-only pattern
	if a.isWeekdayOnlyPattern(pattern) {
		pattern.PatternType = PatternWeekdayOnly
		pattern.Description = "Weekday pattern detected: significantly lower usage on weekends"
		return
	}

	// Check for weekend peak pattern
	if a.isWeekendPeakPattern(pattern) {
		pattern.PatternType = PatternWeekendPeak
		pattern.Description = "Weekend peak pattern detected: higher usage on weekends"
		return
	}

	// Check for morning spike
	if a.isMorningSpikePattern(pattern) {
		pattern.PatternType = PatternMorningSpike
		pattern.Description = fmt.Sprintf("Morning spike detected: peak usage around %s",
			formatHourRange(pattern.PeakHours))
		return
	}

	// Check for evening spike
	if a.isEveningSpikePattern(pattern) {
		pattern.PatternType = PatternEveningSpike
		pattern.Description = fmt.Sprintf("Evening spike detected: peak usage around %s",
			formatHourRange(pattern.PeakHours))
		return
	}

	// Default to custom pattern
	if len(pattern.PeakHours) > 0 || len(pattern.OffPeakHours) > 0 {
		pattern.PatternType = PatternCustom
		pattern.Description = fmt.Sprintf("Custom usage pattern: peaks at %s, low usage at %s",
			formatHourRange(pattern.PeakHours), formatHourRange(pattern.OffPeakHours))
	} else {
		pattern.HasPattern = false
		pattern.PatternType = PatternNone
		pattern.Description = "No clear time-based pattern detected"
	}
}

// Pattern detection helpers
func (a *Analyzer) isBusinessHoursPattern(pattern *TimePattern) bool {
	// Check if peaks are mostly between 8-18
	businessPeaks := 0
	for _, hour := range pattern.PeakHours {
		if hour >= 8 && hour <= 18 {
			businessPeaks++
		}
	}
	return len(pattern.PeakHours) >= 3 && float64(businessPeaks)/float64(len(pattern.PeakHours)) >= 0.7
}

func (a *Analyzer) isNightBatchPattern(pattern *TimePattern) bool {
	// Check if peaks are mostly between 0-6 or 22-23
	nightPeaks := 0
	for _, hour := range pattern.PeakHours {
		if hour <= 6 || hour >= 22 {
			nightPeaks++
		}
	}
	return len(pattern.PeakHours) >= 2 && float64(nightPeaks)/float64(len(pattern.PeakHours)) >= 0.7
}

func (a *Analyzer) isWeekdayOnlyPattern(pattern *TimePattern) bool {
	// Check if weekends are off-peak
	weekendOffPeak := 0
	for _, day := range pattern.OffPeakDays {
		if day == time.Saturday || day == time.Sunday {
			weekendOffPeak++
		}
	}
	return weekendOffPeak >= 2
}

func (a *Analyzer) isWeekendPeakPattern(pattern *TimePattern) bool {
	// Check if weekends are peak
	weekendPeak := 0
	for _, day := range pattern.PeakDays {
		if day == time.Saturday || day == time.Sunday {
			weekendPeak++
		}
	}
	return weekendPeak >= 2
}

func (a *Analyzer) isMorningSpikePattern(pattern *TimePattern) bool {
	// Check if peaks are concentrated in morning (6-11)
	morningPeaks := 0
	for _, hour := range pattern.PeakHours {
		if hour >= 6 && hour <= 11 {
			morningPeaks++
		}
	}
	return len(pattern.PeakHours) >= 2 && len(pattern.PeakHours) <= 4 &&
		float64(morningPeaks)/float64(len(pattern.PeakHours)) >= 0.7
}

func (a *Analyzer) isEveningSpikePattern(pattern *TimePattern) bool {
	// Check if peaks are concentrated in evening (17-22)
	eveningPeaks := 0
	for _, hour := range pattern.PeakHours {
		if hour >= 17 && hour <= 22 {
			eveningPeaks++
		}
	}
	return len(pattern.PeakHours) >= 2 && len(pattern.PeakHours) <= 4 &&
		float64(eveningPeaks)/float64(len(pattern.PeakHours)) >= 0.7
}

// generateScalingRecommendation generates scaling schedule recommendation
func (a *Analyzer) generateScalingRecommendation(pattern *TimePattern) *ScalingSchedule {
	if !pattern.HasPattern {
		return &ScalingSchedule{
			Enabled: false,
			Reason:  "No significant time-based pattern detected",
		}
	}

	schedule := &ScalingSchedule{
		Enabled:   true,
		Schedules: []ScheduleEntry{},
	}

	// Calculate potential savings
	if len(pattern.OffPeakHours) > 0 && pattern.OverallStats.MeanCPU > 0 {
		// Calculate average off-peak usage
		var offPeakSum float64
		for _, hour := range pattern.OffPeakHours {
			offPeakSum += pattern.HourlyStats[hour].MeanCPU
		}
		avgOffPeak := offPeakSum / float64(len(pattern.OffPeakHours))
		offPeakRatio := avgOffPeak / pattern.OverallStats.MeanCPU

		// Savings = (1 - offPeakRatio) * (offPeakHours / 24)
		offPeakFraction := float64(len(pattern.OffPeakHours)) / 24.0
		schedule.EstimatedSavingsPercent = (1 - offPeakRatio) * offPeakFraction * 100
	}

	// Generate schedule entries based on pattern type
	switch pattern.PatternType {
	case PatternBusinessHours:
		schedule.Reason = "Business hours pattern - scale down outside 8 AM - 6 PM"
		schedule.Schedules = append(schedule.Schedules,
			ScheduleEntry{
				Name:             "business-hours",
				CronSchedule:     "0 8 * * 1-5", // 8 AM Mon-Fri
				Duration:         10 * time.Hour,
				CPUMultiplier:    1.0,
				MemoryMultiplier: 1.0,
				Description:      "Full resources during business hours",
			},
			ScheduleEntry{
				Name:             "off-hours",
				CronSchedule:     "0 18 * * 1-5", // 6 PM Mon-Fri
				Duration:         14 * time.Hour,
				CPUMultiplier:    0.5,
				MemoryMultiplier: 0.7,
				Description:      "Reduced resources outside business hours",
			},
			ScheduleEntry{
				Name:             "weekend",
				CronSchedule:     "0 0 * * 0,6", // Midnight Sat, Sun
				Duration:         24 * time.Hour,
				CPUMultiplier:    0.3,
				MemoryMultiplier: 0.5,
				Description:      "Minimal resources on weekends",
			},
		)

	case PatternNightBatch:
		schedule.Reason = "Night batch pattern - scale up during batch processing hours"
		schedule.Schedules = append(schedule.Schedules,
			ScheduleEntry{
				Name:             "batch-window",
				CronSchedule:     "0 22 * * *", // 10 PM daily
				Duration:         8 * time.Hour,
				CPUMultiplier:    1.5,
				MemoryMultiplier: 1.3,
				Description:      "Increased resources for batch processing",
			},
			ScheduleEntry{
				Name:             "daytime",
				CronSchedule:     "0 6 * * *", // 6 AM daily
				Duration:         16 * time.Hour,
				CPUMultiplier:    0.5,
				MemoryMultiplier: 0.7,
				Description:      "Reduced resources during daytime",
			},
		)

	case PatternWeekdayOnly:
		schedule.Reason = "Weekday-only pattern - scale down on weekends"
		schedule.Schedules = append(schedule.Schedules,
			ScheduleEntry{
				Name:             "weekday",
				CronSchedule:     "0 0 * * 1", // Monday midnight
				Duration:         120 * time.Hour,
				CPUMultiplier:    1.0,
				MemoryMultiplier: 1.0,
				Description:      "Full resources on weekdays",
			},
			ScheduleEntry{
				Name:             "weekend",
				CronSchedule:     "0 0 * * 6", // Saturday midnight
				Duration:         48 * time.Hour,
				CPUMultiplier:    0.3,
				MemoryMultiplier: 0.5,
				Description:      "Minimal resources on weekends",
			},
		)

	case PatternMorningSpike:
		schedule.Reason = fmt.Sprintf("Morning spike pattern - scale up during peak hours (%s)",
			formatHourRange(pattern.PeakHours))
		peakStart := pattern.PeakHours[0]
		peakEnd := pattern.PeakHours[len(pattern.PeakHours)-1]
		schedule.Schedules = append(schedule.Schedules,
			ScheduleEntry{
				Name:             "morning-peak",
				CronSchedule:     fmt.Sprintf("0 %d * * *", peakStart),
				Duration:         time.Duration(peakEnd-peakStart+1) * time.Hour,
				CPUMultiplier:    1.3,
				MemoryMultiplier: 1.2,
				Description:      "Increased resources during morning peak",
			},
			ScheduleEntry{
				Name:             "off-peak",
				CronSchedule:     fmt.Sprintf("0 %d * * *", (peakEnd+1)%24),
				Duration:         time.Duration(24-(peakEnd-peakStart+1)) * time.Hour,
				CPUMultiplier:    0.6,
				MemoryMultiplier: 0.7,
				Description:      "Reduced resources outside peak hours",
			},
		)

	case PatternEveningSpike:
		schedule.Reason = fmt.Sprintf("Evening spike pattern - scale up during peak hours (%s)",
			formatHourRange(pattern.PeakHours))
		peakStart := pattern.PeakHours[0]
		peakEnd := pattern.PeakHours[len(pattern.PeakHours)-1]
		schedule.Schedules = append(schedule.Schedules,
			ScheduleEntry{
				Name:             "evening-peak",
				CronSchedule:     fmt.Sprintf("0 %d * * *", peakStart),
				Duration:         time.Duration(peakEnd-peakStart+1) * time.Hour,
				CPUMultiplier:    1.3,
				MemoryMultiplier: 1.2,
				Description:      "Increased resources during evening peak",
			},
			ScheduleEntry{
				Name:             "off-peak",
				CronSchedule:     fmt.Sprintf("0 %d * * *", (peakEnd+1)%24),
				Duration:         time.Duration(24-(peakEnd-peakStart+1)) * time.Hour,
				CPUMultiplier:    0.6,
				MemoryMultiplier: 0.7,
				Description:      "Reduced resources outside peak hours",
			},
		)

	default:
		schedule.Reason = "Custom pattern detected - review peak hours for optimization"
		if len(pattern.PeakHours) > 0 && len(pattern.OffPeakHours) > 0 {
			schedule.Schedules = append(schedule.Schedules,
				ScheduleEntry{
					Name:             "peak-hours",
					CronSchedule:     fmt.Sprintf("0 %d * * *", pattern.PeakHours[0]),
					Duration:         time.Duration(len(pattern.PeakHours)) * time.Hour,
					CPUMultiplier:    1.2,
					MemoryMultiplier: 1.1,
					Description:      "Increased resources during detected peak hours",
				},
				ScheduleEntry{
					Name:             "off-peak-hours",
					CronSchedule:     fmt.Sprintf("0 %d * * *", pattern.OffPeakHours[0]),
					Duration:         time.Duration(len(pattern.OffPeakHours)) * time.Hour,
					CPUMultiplier:    0.6,
					MemoryMultiplier: 0.7,
					Description:      "Reduced resources during detected off-peak hours",
				},
			)
		}
	}

	return schedule
}

// formatHourRange formats a slice of hours into a human-readable range
func formatHourRange(hours []int) string {
	if len(hours) == 0 {
		return "none"
	}
	if len(hours) == 1 {
		return fmt.Sprintf("%d:00", hours[0])
	}

	// Find consecutive ranges
	ranges := []string{}
	start := hours[0]
	end := hours[0]

	for i := 1; i < len(hours); i++ {
		if hours[i] == end+1 {
			end = hours[i]
		} else {
			if start == end {
				ranges = append(ranges, fmt.Sprintf("%d:00", start))
			} else {
				ranges = append(ranges, fmt.Sprintf("%d:00-%d:00", start, end))
			}
			start = hours[i]
			end = hours[i]
		}
	}

	// Add last range
	if start == end {
		ranges = append(ranges, fmt.Sprintf("%d:00", start))
	} else {
		ranges = append(ranges, fmt.Sprintf("%d:00-%d:00", start, end))
	}

	if len(ranges) == 1 {
		return ranges[0]
	}
	return fmt.Sprintf("%s", ranges)
}

// FormatPatternSummary returns a formatted summary of the pattern
func (p *TimePattern) FormatPatternSummary() string {
	if !p.HasPattern {
		return p.Description
	}

	summary := fmt.Sprintf(`Time Pattern Analysis
=====================
Pattern Type: %s
Description: %s

Peak Hours: %s
Off-Peak Hours: %s
Peak Days: %v
Off-Peak Days: %v

Overall Statistics:
  Total Samples: %d
  Mean CPU: %.0fm
  Peak-to-Peak Ratio: %.2fx
  Coefficient of Variation: %.1f%%
`,
		p.PatternType,
		p.Description,
		formatHourRange(p.PeakHours),
		formatHourRange(p.OffPeakHours),
		p.PeakDays,
		p.OffPeakDays,
		p.OverallStats.TotalSamples,
		p.OverallStats.MeanCPU,
		p.OverallStats.PeakToPeakRatio,
		p.OverallStats.CoefficientOfVar*100,
	)

	if p.ScalingRecommendation != nil && p.ScalingRecommendation.Enabled {
		summary += fmt.Sprintf(`
Scaling Recommendation:
  Reason: %s
  Estimated Savings: %.1f%%
  Schedules:
`,
			p.ScalingRecommendation.Reason,
			p.ScalingRecommendation.EstimatedSavingsPercent,
		)
		for _, s := range p.ScalingRecommendation.Schedules {
			summary += fmt.Sprintf("    - %s: %s (CPU: %.0f%%, Memory: %.0f%%)\n",
				s.Name, s.CronSchedule, s.CPUMultiplier*100, s.MemoryMultiplier*100)
		}
	}

	return summary
}
