package recommendation

import (
	"fmt"
	"sort"
	"time"

	optimizerv1alpha1 "intelligent-cluster-optimizer/pkg/apis/optimizer/v1alpha1"
	"intelligent-cluster-optimizer/pkg/cost"
	"intelligent-cluster-optimizer/pkg/models"

	"k8s.io/klog/v2"
)

// Engine generates resource recommendations based on historical metrics
type Engine struct {
	// Default configuration values
	defaultCPUPercentile    int
	defaultMemoryPercentile int
	defaultSafetyMargin     float64
	defaultMinSamples       int
	defaultHistoryDuration  time.Duration

	// Cost calculator for savings estimation
	costCalculator *cost.Calculator

	// Confidence calculator for scoring recommendations
	confidenceCalculator *ConfidenceCalculator

	// Expected interval between metric samples (for gap detection)
	expectedSampleInterval time.Duration
}

// NewEngine creates a new recommendation engine with sensible defaults
func NewEngine() *Engine {
	return &Engine{
		defaultCPUPercentile:    95,
		defaultMemoryPercentile: 95,
		defaultSafetyMargin:     1.2,
		defaultMinSamples:       10,
		defaultHistoryDuration:  24 * time.Hour,
		costCalculator:          cost.NewCalculator(nil), // Use default pricing
		confidenceCalculator:    NewConfidenceCalculator(),
		expectedSampleInterval:  30 * time.Second, // Default: metrics collected every 30s
	}
}

// NewEngineWithPricing creates a recommendation engine with custom pricing
func NewEngineWithPricing(pricingPreset string) *Engine {
	e := NewEngine()
	e.costCalculator = cost.NewCalculatorWithPreset(pricingPreset)
	return e
}

// SetPricingModel updates the pricing model for cost calculations
func (e *Engine) SetPricingModel(pricing *cost.PricingModel) {
	e.costCalculator = cost.NewCalculator(pricing)
}

// ContainerRecommendation represents a recommendation for a single container
type ContainerRecommendation struct {
	ContainerName     string
	CurrentCPU        int64 // millicores
	CurrentMemory     int64 // bytes
	RecommendedCPU    int64 // millicores
	RecommendedMemory int64 // bytes
	SampleCount       int
	CPUPercentile     int
	MemoryPercentile  int
	Confidence        float64 // 0-100 overall confidence score

	// Detailed confidence scoring
	ConfidenceDetails *ConfidenceScore

	// Cost estimation
	EstimatedSavings *cost.SavingsEstimate

	// OOM information
	HasOOMHistory    bool
	OOMCount         int
	OOMBoostApplied  float64 // Memory boost multiplier applied due to OOM history
	OOMPriority      string  // Priority level based on OOM frequency
}

// WorkloadRecommendation represents recommendations for all containers in a workload
type WorkloadRecommendation struct {
	Namespace    string
	WorkloadKind string
	WorkloadName string
	Containers   []ContainerRecommendation
	GeneratedAt  time.Time

	// Cost estimation for entire workload
	TotalEstimatedSavings *cost.SavingsEstimate

	// OOM summary for workload
	HasOOMHistory bool
	TotalOOMCount int
	OOMPriority   string // Overall priority based on OOM history
}

// MetricsProvider is an interface for retrieving historical metrics
type MetricsProvider interface {
	GetMetricsByNamespace(namespace string, since time.Duration) []models.PodMetric
	GetMetricsByWorkload(namespace, workloadName string, since time.Duration) []models.PodMetric
}

// OOMInfoProvider is an interface for retrieving OOM information
type OOMInfoProvider interface {
	GetMemoryBoostFactor(namespace, workloadName, containerName string) float64
	GetOOMHistory(namespace, workloadName string) *OOMHistoryInfo
}

// OOMHistoryInfo contains OOM history for a workload (simplified for recommendation use)
type OOMHistoryInfo struct {
	HasOOMHistory bool
	TotalOOMCount int
	Priority      string
	ContainerOOMs map[string]ContainerOOMDetails
}

// ContainerOOMDetails contains OOM details for a container
type ContainerOOMDetails struct {
	OOMCount         int
	RecommendedBoost float64
	Priority         string
}

// GenerateRecommendations generates recommendations for workloads in the given namespaces
func (e *Engine) GenerateRecommendations(
	provider MetricsProvider,
	config *optimizerv1alpha1.OptimizerConfig,
) ([]WorkloadRecommendation, error) {
	return e.GenerateRecommendationsWithOOM(provider, nil, config)
}

// GenerateRecommendationsWithOOM generates recommendations with OOM-aware memory adjustments
func (e *Engine) GenerateRecommendationsWithOOM(
	provider MetricsProvider,
	oomProvider OOMInfoProvider,
	config *optimizerv1alpha1.OptimizerConfig,
) ([]WorkloadRecommendation, error) {
	var recommendations []WorkloadRecommendation

	// Get configuration or use defaults
	cpuPercentile := e.defaultCPUPercentile
	memoryPercentile := e.defaultMemoryPercentile
	safetyMargin := e.defaultSafetyMargin
	minSamples := e.defaultMinSamples
	historyDuration := e.defaultHistoryDuration

	if config.Spec.Recommendations != nil {
		if config.Spec.Recommendations.CPUPercentile > 0 {
			cpuPercentile = config.Spec.Recommendations.CPUPercentile
		}
		if config.Spec.Recommendations.MemoryPercentile > 0 {
			memoryPercentile = config.Spec.Recommendations.MemoryPercentile
		}
		if config.Spec.Recommendations.SafetyMargin > 0 {
			safetyMargin = config.Spec.Recommendations.SafetyMargin
		}
		if config.Spec.Recommendations.MinSamples > 0 {
			minSamples = config.Spec.Recommendations.MinSamples
		}
		if config.Spec.Recommendations.HistoryDuration != "" {
			if d, err := time.ParseDuration(config.Spec.Recommendations.HistoryDuration); err == nil {
				historyDuration = d
			}
		}
	}

	// Apply strategy-based adjustments
	cpuPercentile, memoryPercentile, safetyMargin = e.applyStrategy(
		config.Spec.Strategy,
		cpuPercentile,
		memoryPercentile,
		safetyMargin,
	)

	klog.V(4).Infof("Generating recommendations with: CPU P%d, Memory P%d, SafetyMargin %.2f, MinSamples %d, History %v",
		cpuPercentile, memoryPercentile, safetyMargin, minSamples, historyDuration)

	// Process each target namespace
	for _, namespace := range config.Spec.TargetNamespaces {
		metrics := provider.GetMetricsByNamespace(namespace, historyDuration)
		if len(metrics) == 0 {
			klog.V(3).Infof("No metrics found for namespace %s", namespace)
			continue
		}

		// Group metrics by workload (pod name prefix before the hash)
		workloadMetrics := e.groupByWorkload(metrics)

		for workloadName, containerMetrics := range workloadMetrics {
			// Get OOM info for this workload if provider is available
			var oomInfo *OOMHistoryInfo
			if oomProvider != nil {
				oomInfo = oomProvider.GetOOMHistory(namespace, workloadName)
			}

			rec := e.generateWorkloadRecommendationWithOOM(
				namespace,
				workloadName,
				containerMetrics,
				cpuPercentile,
				memoryPercentile,
				safetyMargin,
				minSamples,
				config.Spec.ResourceThresholds,
				oomInfo,
			)
			if rec != nil {
				recommendations = append(recommendations, *rec)
			}
		}
	}

	// Sort recommendations: OOM-affected workloads first, then by priority
	sortRecommendationsByPriority(recommendations)

	return recommendations, nil
}

// sortRecommendationsByPriority sorts recommendations with OOM-affected workloads first
func sortRecommendationsByPriority(recs []WorkloadRecommendation) {
	// Simple bubble sort for now - could use sort.Slice for larger lists
	for i := 0; i < len(recs); i++ {
		for j := i + 1; j < len(recs); j++ {
			if shouldSwap(recs[i], recs[j]) {
				recs[i], recs[j] = recs[j], recs[i]
			}
		}
	}
}

// shouldSwap returns true if b should come before a (b has higher priority)
func shouldSwap(a, b WorkloadRecommendation) bool {
	// OOM workloads come first
	if b.HasOOMHistory && !a.HasOOMHistory {
		return true
	}
	if a.HasOOMHistory && !b.HasOOMHistory {
		return false
	}

	// Both have OOM or neither has OOM - sort by OOM count
	if a.HasOOMHistory && b.HasOOMHistory {
		return b.TotalOOMCount > a.TotalOOMCount
	}

	return false
}

// applyStrategy adjusts percentile and safety margin based on optimization strategy
func (e *Engine) applyStrategy(
	strategy optimizerv1alpha1.OptimizationStrategy,
	cpuPercentile, memoryPercentile int,
	safetyMargin float64,
) (int, int, float64) {
	switch strategy {
	case optimizerv1alpha1.StrategyAggressive:
		// Lower percentiles and safety margin for more aggressive optimization
		return max(cpuPercentile-10, 50), max(memoryPercentile-5, 50), maxFloat(safetyMargin-0.1, 1.0)
	case optimizerv1alpha1.StrategyConservative:
		// Higher percentiles and safety margin for safer optimization
		return min(cpuPercentile+4, 99), min(memoryPercentile+4, 99), minFloat(safetyMargin+0.2, 2.0)
	default: // balanced
		return cpuPercentile, memoryPercentile, safetyMargin
	}
}

// groupByWorkload groups metrics by workload name (container level)
func (e *Engine) groupByWorkload(metrics []models.PodMetric) map[string]map[string][]containerSample {
	// workloadName -> containerName -> samples
	result := make(map[string]map[string][]containerSample)

	for _, pm := range metrics {
		// Extract workload name from pod name (remove hash suffix)
		workloadName := extractWorkloadName(pm.PodName)

		if _, exists := result[workloadName]; !exists {
			result[workloadName] = make(map[string][]containerSample)
		}

		for _, cm := range pm.Containers {
			sample := containerSample{
				timestamp:     pm.Timestamp,
				usageCPU:      cm.UsageCPU,
				usageMemory:   cm.UsageMemory,
				requestCPU:    cm.RequestCPU,
				requestMemory: cm.RequestMemory,
			}
			result[workloadName][cm.ContainerName] = append(
				result[workloadName][cm.ContainerName],
				sample,
			)
		}
	}

	return result
}

type containerSample struct {
	timestamp     time.Time
	usageCPU      int64
	usageMemory   int64
	requestCPU    int64
	requestMemory int64
}

// generateWorkloadRecommendation generates recommendations for a single workload
func (e *Engine) generateWorkloadRecommendation(
	namespace, workloadName string,
	containerMetrics map[string][]containerSample,
	cpuPercentile, memoryPercentile int,
	safetyMargin float64,
	minSamples int,
	thresholds *optimizerv1alpha1.ResourceThresholds,
) *WorkloadRecommendation {
	return e.generateWorkloadRecommendationWithOOM(
		namespace, workloadName, containerMetrics,
		cpuPercentile, memoryPercentile, safetyMargin,
		minSamples, thresholds, nil,
	)
}

// generateWorkloadRecommendationWithOOM generates recommendations with OOM-aware memory adjustments
func (e *Engine) generateWorkloadRecommendationWithOOM(
	namespace, workloadName string,
	containerMetrics map[string][]containerSample,
	cpuPercentile, memoryPercentile int,
	safetyMargin float64,
	minSamples int,
	thresholds *optimizerv1alpha1.ResourceThresholds,
	oomInfo *OOMHistoryInfo,
) *WorkloadRecommendation {
	var containerRecs []ContainerRecommendation
	var totalOOMCount int
	hasOOMHistory := false

	for containerName, samples := range containerMetrics {
		if len(samples) < minSamples {
			klog.V(4).Infof("Skipping container %s/%s/%s: insufficient samples (%d < %d)",
				namespace, workloadName, containerName, len(samples), minSamples)
			continue
		}

		// Get OOM info for this specific container
		var containerOOM *ContainerOOMDetails
		if oomInfo != nil && oomInfo.ContainerOOMs != nil {
			if oom, ok := oomInfo.ContainerOOMs[containerName]; ok {
				containerOOM = &oom
				hasOOMHistory = true
				totalOOMCount += oom.OOMCount
			}
		}

		rec := e.generateContainerRecommendationWithOOM(
			containerName,
			samples,
			cpuPercentile,
			memoryPercentile,
			safetyMargin,
			minSamples,
			thresholds,
			containerOOM,
		)
		if rec != nil {
			containerRecs = append(containerRecs, *rec)
		}
	}

	if len(containerRecs) == 0 {
		return nil
	}

	// Calculate total workload savings
	totalSavings := e.aggregateContainerSavings(containerRecs)

	// Determine overall OOM priority
	oomPriority := "None"
	if oomInfo != nil {
		oomPriority = oomInfo.Priority
		if oomInfo.HasOOMHistory {
			hasOOMHistory = true
		}
		if oomInfo.TotalOOMCount > totalOOMCount {
			totalOOMCount = oomInfo.TotalOOMCount
		}
	}

	if hasOOMHistory {
		klog.V(3).Infof("Workload %s/%s has OOM history: count=%d, priority=%s",
			namespace, workloadName, totalOOMCount, oomPriority)
	}

	return &WorkloadRecommendation{
		Namespace:             namespace,
		WorkloadKind:          "Deployment", // Default, could be enhanced to detect actual kind
		WorkloadName:          workloadName,
		Containers:            containerRecs,
		GeneratedAt:           time.Now(),
		TotalEstimatedSavings: totalSavings,
		HasOOMHistory:         hasOOMHistory,
		TotalOOMCount:         totalOOMCount,
		OOMPriority:           oomPriority,
	}
}

// aggregateContainerSavings combines savings from all containers in a workload
func (e *Engine) aggregateContainerSavings(containers []ContainerRecommendation) *cost.SavingsEstimate {
	var totalCPUSavings, totalMemorySavings float64
	var totalCurrentCost, totalRecommendedCost float64

	for _, c := range containers {
		if c.EstimatedSavings != nil {
			totalCPUSavings += c.EstimatedSavings.CPUSavingsPerHour
			totalMemorySavings += c.EstimatedSavings.MemorySavingsPerHour
			totalCurrentCost += c.EstimatedSavings.CurrentCost.TotalPerHour
			totalRecommendedCost += c.EstimatedSavings.RecommendedCost.TotalPerHour
		}
	}

	totalSavingsPerHour := totalCPUSavings + totalMemorySavings
	var percentageReduction float64
	if totalCurrentCost > 0 {
		percentageReduction = (totalSavingsPerHour / totalCurrentCost) * 100
	}

	return &cost.SavingsEstimate{
		CurrentCost: cost.ResourceCost{
			TotalPerHour:  totalCurrentCost,
			TotalPerDay:   totalCurrentCost * 24,
			TotalPerMonth: totalCurrentCost * 24 * 30,
			TotalPerYear:  totalCurrentCost * 24 * 365,
		},
		RecommendedCost: cost.ResourceCost{
			TotalPerHour:  totalRecommendedCost,
			TotalPerDay:   totalRecommendedCost * 24,
			TotalPerMonth: totalRecommendedCost * 24 * 30,
			TotalPerYear:  totalRecommendedCost * 24 * 365,
		},
		CPUSavingsPerHour:    totalCPUSavings,
		MemorySavingsPerHour: totalMemorySavings,
		TotalSavingsPerHour:  totalSavingsPerHour,
		SavingsPerDay:        totalSavingsPerHour * 24,
		SavingsPerMonth:      totalSavingsPerHour * 24 * 30,
		SavingsPerYear:       totalSavingsPerHour * 24 * 365,
		PercentageReduction:  percentageReduction,
	}
}

// generateContainerRecommendation generates a recommendation for a single container
func (e *Engine) generateContainerRecommendation(
	containerName string,
	samples []containerSample,
	cpuPercentile, memoryPercentile int,
	safetyMargin float64,
	minSamples int,
	thresholds *optimizerv1alpha1.ResourceThresholds,
) *ContainerRecommendation {
	return e.generateContainerRecommendationWithOOM(
		containerName, samples, cpuPercentile, memoryPercentile,
		safetyMargin, minSamples, thresholds, nil,
	)
}

// generateContainerRecommendationWithOOM generates a recommendation with OOM-aware memory boost
func (e *Engine) generateContainerRecommendationWithOOM(
	containerName string,
	samples []containerSample,
	cpuPercentile, memoryPercentile int,
	safetyMargin float64,
	minSamples int,
	thresholds *optimizerv1alpha1.ResourceThresholds,
	oomInfo *ContainerOOMDetails,
) *ContainerRecommendation {
	// Extract CPU and memory values along with timestamps
	cpuValues := make([]int64, len(samples))
	memoryValues := make([]int64, len(samples))
	timestamps := make([]time.Time, len(samples))

	var currentCPU, currentMemory int64
	for i, s := range samples {
		cpuValues[i] = s.usageCPU
		memoryValues[i] = s.usageMemory
		timestamps[i] = s.timestamp
		// Use the most recent request values as "current"
		currentCPU = s.requestCPU
		currentMemory = s.requestMemory
	}

	// Calculate percentiles
	cpuP := calculatePercentile(cpuValues, cpuPercentile)
	memoryP := calculatePercentile(memoryValues, memoryPercentile)

	// Apply safety margin
	recommendedCPU := int64(float64(cpuP) * safetyMargin)
	recommendedMemory := int64(float64(memoryP) * safetyMargin)

	// Apply OOM boost to memory if container has OOM history
	var oomBoostApplied float64 = 1.0
	hasOOMHistory := false
	oomCount := 0
	oomPriority := "None"

	if oomInfo != nil {
		hasOOMHistory = true
		oomCount = oomInfo.OOMCount
		oomPriority = oomInfo.Priority

		// Apply OOM boost - this ensures we don't reduce memory below current if there's OOM history
		if oomInfo.RecommendedBoost > 1.0 {
			oomBoostApplied = oomInfo.RecommendedBoost
			boostedMemory := int64(float64(recommendedMemory) * oomInfo.RecommendedBoost)

			// If OOM occurred, never recommend less memory than current
			if boostedMemory < currentMemory {
				boostedMemory = currentMemory
				klog.V(3).Infof("Container %s has OOM history - not reducing memory below current (%d bytes)",
					containerName, currentMemory)
			}

			klog.V(3).Infof("Container %s: applying OOM boost %.2fx to memory: %d -> %d bytes (OOM count: %d, priority: %s)",
				containerName, oomInfo.RecommendedBoost, recommendedMemory, boostedMemory, oomCount, oomPriority)

			recommendedMemory = boostedMemory
		}
	}

	// Apply thresholds
	recommendedCPU = e.applyThresholds(recommendedCPU, thresholds, "cpu")
	recommendedMemory = e.applyThresholds(recommendedMemory, thresholds, "memory")

	// Calculate detailed confidence score using all metrics
	// We use CPU values for confidence calculation as they typically have more variance
	confidenceDetails := e.confidenceCalculator.CalculateFromSamples(
		timestamps,
		cpuValues,
		e.expectedSampleInterval,
	)

	// Use the detailed score as the main confidence value
	confidence := confidenceDetails.Score

	// Calculate cost savings
	savings := e.costCalculator.EstimateSavings(
		currentCPU, recommendedCPU,
		currentMemory, recommendedMemory,
	)

	oomLogSuffix := ""
	if hasOOMHistory {
		oomLogSuffix = fmt.Sprintf(", OOM: count=%d boost=%.2fx priority=%s", oomCount, oomBoostApplied, oomPriority)
	}

	klog.V(4).Infof("Container %s: CPU P%d=%dm (recommended: %dm), Memory P%d=%dMi (recommended: %dMi), confidence=%.2f, savings=$%.4f/hour%s",
		containerName,
		cpuPercentile, cpuP, recommendedCPU,
		memoryPercentile, memoryP/(1024*1024), recommendedMemory/(1024*1024),
		confidence, savings.TotalSavingsPerHour, oomLogSuffix)

	return &ContainerRecommendation{
		ContainerName:       containerName,
		CurrentCPU:          currentCPU,
		CurrentMemory:       currentMemory,
		RecommendedCPU:      recommendedCPU,
		RecommendedMemory:   recommendedMemory,
		SampleCount:         len(samples),
		CPUPercentile:       cpuPercentile,
		MemoryPercentile:    memoryPercentile,
		Confidence:          confidence,
		ConfidenceDetails:   &confidenceDetails,
		EstimatedSavings:    &savings,
		HasOOMHistory:       hasOOMHistory,
		OOMCount:            oomCount,
		OOMBoostApplied:     oomBoostApplied,
		OOMPriority:         oomPriority,
	}
}

// calculatePercentile calculates the nth percentile of a slice of int64 values
// This implementation uses the nearest-rank method
func calculatePercentile(values []int64, percentile int) int64 {
	if len(values) == 0 {
		return 0
	}

	// Make a copy to avoid modifying the original slice
	sorted := make([]int64, len(values))
	copy(sorted, values)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	// Calculate the index using nearest-rank method
	// P = (n * percentile) / 100, rounded up
	n := len(sorted)
	rank := int(float64(n*percentile)/100.0 + 0.5)
	if rank < 1 {
		rank = 1
	}
	if rank > n {
		rank = n
	}

	return sorted[rank-1]
}

// calculateConfidence returns a confidence score based on sample count
// More samples = higher confidence, up to 1.0
func calculateConfidence(sampleCount, minSamples int) float64 {
	if sampleCount < minSamples {
		return 0.0
	}

	// Confidence increases logarithmically with sample count
	// At minSamples: ~0.5
	// At 2x minSamples: ~0.7
	// At 10x minSamples: ~0.9
	// At 100x minSamples: ~1.0
	ratio := float64(sampleCount) / float64(minSamples)
	confidence := 1.0 - (1.0 / (1.0 + ratio*0.5))

	if confidence > 1.0 {
		confidence = 1.0
	}
	return confidence
}

// applyThresholds ensures the recommendation is within configured bounds
func (e *Engine) applyThresholds(value int64, thresholds *optimizerv1alpha1.ResourceThresholds, resourceType string) int64 {
	if thresholds == nil {
		return value
	}

	var minVal, maxVal int64

	switch resourceType {
	case "cpu":
		if thresholds.CPU != nil {
			minVal = parseCPUToMillicores(thresholds.CPU.Min)
			maxVal = parseCPUToMillicores(thresholds.CPU.Max)
		}
	case "memory":
		if thresholds.Memory != nil {
			minVal = parseMemoryToBytes(thresholds.Memory.Min)
			maxVal = parseMemoryToBytes(thresholds.Memory.Max)
		}
	}

	if minVal > 0 && value < minVal {
		return minVal
	}
	if maxVal > 0 && value > maxVal {
		return maxVal
	}

	return value
}

// parseCPUToMillicores converts CPU string (e.g., "100m", "1") to millicores
func parseCPUToMillicores(cpu string) int64 {
	if cpu == "" {
		return 0
	}

	var value int64
	if _, err := fmt.Sscanf(cpu, "%dm", &value); err == nil {
		return value
	}
	if _, err := fmt.Sscanf(cpu, "%d", &value); err == nil {
		return value * 1000 // cores to millicores
	}

	var floatValue float64
	if _, err := fmt.Sscanf(cpu, "%f", &floatValue); err == nil {
		return int64(floatValue * 1000)
	}

	return 0
}

// parseMemoryToBytes converts memory string (e.g., "128Mi", "1Gi") to bytes
func parseMemoryToBytes(memory string) int64 {
	if memory == "" {
		return 0
	}

	var value int64

	// Try different suffixes
	suffixes := map[string]int64{
		"Ki": 1024,
		"Mi": 1024 * 1024,
		"Gi": 1024 * 1024 * 1024,
		"Ti": 1024 * 1024 * 1024 * 1024,
		"K":  1000,
		"M":  1000 * 1000,
		"G":  1000 * 1000 * 1000,
		"T":  1000 * 1000 * 1000 * 1000,
	}

	for suffix, multiplier := range suffixes {
		pattern := "%d" + suffix
		if _, err := fmt.Sscanf(memory, pattern, &value); err == nil {
			return value * multiplier
		}
	}

	// Plain bytes
	if _, err := fmt.Sscanf(memory, "%d", &value); err == nil {
		return value
	}

	return 0
}

// extractWorkloadName extracts the workload name from a pod name
// e.g., "nginx-deployment-5d7b8c7d9f-abc12" -> "nginx-deployment"
func extractWorkloadName(podName string) string {
	// Pod names typically follow the pattern: <deployment-name>-<replicaset-hash>-<pod-hash>
	// We want to extract the deployment name

	parts := splitPodName(podName)
	if len(parts) <= 2 {
		return podName
	}

	// Remove the last two segments (replicaset hash and pod hash)
	return joinParts(parts[:len(parts)-2])
}

func splitPodName(name string) []string {
	var parts []string
	current := ""
	for _, c := range name {
		if c == '-' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func joinParts(parts []string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += "-"
		}
		result += p
	}
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
