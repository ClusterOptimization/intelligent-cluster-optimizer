package pareto

import (
	"fmt"
	"time"

	"k8s.io/klog/v2"
)

// RecommendationHelper integrates Pareto optimization with the recommendation engine
type RecommendationHelper struct {
	optimizer *Optimizer
}

// NewRecommendationHelper creates a new recommendation helper
func NewRecommendationHelper() *RecommendationHelper {
	return &RecommendationHelper{
		optimizer: NewOptimizer(),
	}
}

// NewRecommendationHelperWithOptimizer creates a helper with a custom optimizer
func NewRecommendationHelperWithOptimizer(opt *Optimizer) *RecommendationHelper {
	return &RecommendationHelper{
		optimizer: opt,
	}
}

// WorkloadMetrics contains the metrics needed for Pareto optimization
type WorkloadMetrics struct {
	Namespace    string
	WorkloadName string

	// Current allocation
	CurrentCPU    int64 // millicores
	CurrentMemory int64 // bytes

	// Usage statistics
	AvgCPU  int64
	AvgMemory int64
	PeakCPU   int64
	PeakMemory int64
	P95CPU    int64
	P95Memory int64
	P99CPU    int64
	P99Memory int64

	// Metadata
	Confidence  float64
	SampleCount int
}

// ParetoRecommendation represents the result of Pareto-based recommendation
type ParetoRecommendation struct {
	Namespace    string
	WorkloadName string
	GeneratedAt  time.Time

	// Best overall solution
	BestSolution *Solution

	// All Pareto-optimal solutions
	ParetoFrontier []*Solution

	// Profile-specific recommendations
	ProductionChoice  *Solution
	DevelopmentChoice *Solution
	PerformanceChoice *Solution

	// Trade-off information
	TradeOffs []TradeOff

	// Summary
	Summary string
}

// GenerateRecommendation generates a Pareto-optimal recommendation for a workload
func (h *RecommendationHelper) GenerateRecommendation(metrics *WorkloadMetrics) (*ParetoRecommendation, error) {
	if metrics == nil {
		return nil, fmt.Errorf("metrics cannot be nil")
	}

	// Generate candidate solutions
	solutions := h.optimizer.GenerateSolutionSet(
		metrics.Namespace,
		metrics.WorkloadName,
		metrics.CurrentCPU,
		metrics.CurrentMemory,
		metrics.AvgCPU,
		metrics.AvgMemory,
		metrics.PeakCPU,
		metrics.PeakMemory,
		metrics.P95CPU,
		metrics.P95Memory,
		metrics.P99CPU,
		metrics.P99Memory,
		metrics.Confidence,
	)

	// Run Pareto optimization
	result := h.optimizer.Optimize(solutions)

	// Build recommendation
	rec := &ParetoRecommendation{
		Namespace:      metrics.Namespace,
		WorkloadName:   metrics.WorkloadName,
		GeneratedAt:    time.Now(),
		BestSolution:   result.BestSolution,
		ParetoFrontier: result.ParetoFrontier,
		TradeOffs:      result.TradeOffs,
	}

	// Get profile-specific choices
	rec.ProductionChoice = h.optimizer.SelectBestForProfile(result, "production")
	rec.DevelopmentChoice = h.optimizer.SelectBestForProfile(result, "development")
	rec.PerformanceChoice = h.optimizer.SelectBestForProfile(result, "performance")

	// Generate summary
	rec.Summary = h.generateSummary(rec, metrics)

	klog.V(3).Infof("Pareto recommendation for %s/%s: best=%s, frontier size=%d",
		metrics.Namespace, metrics.WorkloadName,
		rec.BestSolution.ID, len(rec.ParetoFrontier))

	return rec, nil
}

// generateSummary creates a human-readable summary
func (h *RecommendationHelper) generateSummary(rec *ParetoRecommendation, metrics *WorkloadMetrics) string {
	if rec.BestSolution == nil {
		return "No optimal solution found"
	}

	cpuChange := float64(rec.BestSolution.CPURequest-metrics.CurrentCPU) / float64(metrics.CurrentCPU) * 100
	memChange := float64(rec.BestSolution.MemoryRequest-metrics.CurrentMemory) / float64(metrics.CurrentMemory) * 100

	action := "MAINTAIN"
	if cpuChange < -10 || memChange < -10 {
		action = "SCALE DOWN"
	} else if cpuChange > 10 || memChange > 10 {
		action = "SCALE UP"
	}

	return fmt.Sprintf(
		"%s: %s strategy (CPU: %dm -> %dm [%.1f%%], Memory: %dMi -> %dMi [%.1f%%]). "+
			"Pareto frontier has %d optimal solutions.",
		action,
		rec.BestSolution.ID,
		metrics.CurrentCPU,
		rec.BestSolution.CPURequest,
		cpuChange,
		metrics.CurrentMemory/(1024*1024),
		rec.BestSolution.MemoryRequest/(1024*1024),
		memChange,
		len(rec.ParetoFrontier),
	)
}

// CompareStrategies returns a comparison of all strategies on the Pareto frontier
func (h *RecommendationHelper) CompareStrategies(rec *ParetoRecommendation) []StrategyComparison {
	var comparisons []StrategyComparison

	for _, sol := range rec.ParetoFrontier {
		comp := StrategyComparison{
			Strategy:      sol.ID,
			CPURequest:    sol.CPURequest,
			MemoryRequest: sol.MemoryRequest,
			OverallScore:  sol.OverallScore,
			ParetoRank:    sol.ParetoRank,
		}

		if obj, ok := sol.Objectives[ObjectiveCost]; ok {
			comp.CostScore = obj.Normalized
			comp.HourlyCost = obj.Value
		}
		if obj, ok := sol.Objectives[ObjectivePerformance]; ok {
			comp.PerformanceScore = obj.Normalized
		}
		if obj, ok := sol.Objectives[ObjectiveReliability]; ok {
			comp.ReliabilityScore = obj.Normalized
		}
		if obj, ok := sol.Objectives[ObjectiveEfficiency]; ok {
			comp.EfficiencyScore = obj.Normalized
		}

		comparisons = append(comparisons, comp)
	}

	return comparisons
}

// StrategyComparison provides a simplified view of a strategy
type StrategyComparison struct {
	Strategy         string
	CPURequest       int64
	MemoryRequest    int64
	OverallScore     float64
	ParetoRank       int
	HourlyCost       float64
	CostScore        float64
	PerformanceScore float64
	ReliabilityScore float64
	EfficiencyScore  float64
}

// GetRecommendationForProfile returns the best solution for a specific profile
func (h *RecommendationHelper) GetRecommendationForProfile(rec *ParetoRecommendation, profile string) *Solution {
	switch profile {
	case "production":
		return rec.ProductionChoice
	case "development", "test":
		return rec.DevelopmentChoice
	case "performance":
		return rec.PerformanceChoice
	default:
		return rec.BestSolution
	}
}

// AnalyzeTradeOffs provides detailed trade-off analysis between strategies
func (h *RecommendationHelper) AnalyzeTradeOffs(rec *ParetoRecommendation) string {
	if len(rec.TradeOffs) == 0 {
		return "No trade-offs to analyze (single optimal solution)"
	}

	result := fmt.Sprintf("Trade-off Analysis for %s/%s:\n", rec.Namespace, rec.WorkloadName)
	result += fmt.Sprintf("Pareto frontier contains %d optimal solutions\n\n", len(rec.ParetoFrontier))

	for i, to := range rec.TradeOffs {
		result += fmt.Sprintf("Trade-off %d: %s vs %s\n", i+1, to.SolutionA, to.SolutionB)

		if len(to.Improvements) > 0 {
			result += "  " + to.SolutionA + " is better for: "
			for objType, diff := range to.Improvements {
				result += fmt.Sprintf("%s (+%.2f) ", objType, diff)
			}
			result += "\n"
		}

		if len(to.Degradations) > 0 {
			result += "  " + to.SolutionB + " is better for: "
			for objType, diff := range to.Degradations {
				result += fmt.Sprintf("%s (+%.2f) ", objType, diff)
			}
			result += "\n"
		}
		result += "\n"
	}

	return result
}

// SetObjectiveWeights updates the optimization weights
func (h *RecommendationHelper) SetObjectiveWeights(cost, performance, reliability, efficiency, stability float64) {
	h.optimizer.CostWeight = cost
	h.optimizer.PerformanceWeight = performance
	h.optimizer.ReliabilityWeight = reliability
	h.optimizer.EfficiencyWeight = efficiency
	h.optimizer.StabilityWeight = stability
}

// SetCostParameters updates the cost model
func (h *RecommendationHelper) SetCostParameters(cpuPerCore, memoryPerGB float64) {
	h.optimizer.CPUCostPerCore = cpuPerCore
	h.optimizer.MemoryCostPerGB = memoryPerGB
}
