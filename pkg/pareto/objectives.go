package pareto

import (
	"fmt"
	"math"
)

// Objective represents a single optimization objective
type Objective struct {
	Name        string
	Value       float64
	Weight      float64 // Importance weight (0-1)
	IsMinimize  bool    // true = minimize (like cost), false = maximize (like performance)
	Normalized  float64 // Normalized value (0-1)
	Description string
}

// ObjectiveType defines standard objective types
type ObjectiveType string

const (
	ObjectiveCost        ObjectiveType = "cost"
	ObjectivePerformance ObjectiveType = "performance"
	ObjectiveReliability ObjectiveType = "reliability"
	ObjectiveEfficiency  ObjectiveType = "efficiency"
	ObjectiveStability   ObjectiveType = "stability"
)

// Solution represents a candidate solution with multiple objectives
type Solution struct {
	ID           string
	Namespace    string
	WorkloadName string

	// Resource recommendations
	CPURequest    int64 // millicores
	MemoryRequest int64 // bytes
	CPULimit      int64
	MemoryLimit   int64

	// Objectives
	Objectives map[ObjectiveType]*Objective

	// Pareto analysis results
	DominatedBy      []string // IDs of solutions that dominate this one
	DominatesIDs     []string // IDs of solutions this one dominates
	ParetoRank       int      // 0 = Pareto optimal (frontier)
	CrowdingDistance float64  // For diversity preservation

	// Overall score (weighted sum of normalized objectives)
	OverallScore float64

	// Metadata
	Confidence float64
	Source     string // e.g., "p95", "p99", "prediction"
}

// NewSolution creates a new solution with the given resource recommendations
func NewSolution(id, namespace, workloadName string) *Solution {
	return &Solution{
		ID:           id,
		Namespace:    namespace,
		WorkloadName: workloadName,
		Objectives:   make(map[ObjectiveType]*Objective),
		DominatedBy:  []string{},
		DominatesIDs: []string{},
	}
}

// AddObjective adds an objective to the solution
func (s *Solution) AddObjective(objType ObjectiveType, name string, value float64, weight float64, isMinimize bool) {
	s.Objectives[objType] = &Objective{
		Name:       name,
		Value:      value,
		Weight:     weight,
		IsMinimize: isMinimize,
	}
}

// SetCostObjective sets the cost objective based on resource usage
func (s *Solution) SetCostObjective(cpuCostPerCore, memoryCostPerGB float64, weight float64) {
	// Calculate hourly cost
	cpuCores := float64(s.CPURequest) / 1000.0
	memoryGB := float64(s.MemoryRequest) / (1024 * 1024 * 1024)

	hourlyCost := cpuCores*cpuCostPerCore + memoryGB*memoryCostPerGB

	s.AddObjective(ObjectiveCost, "Cost", hourlyCost, weight, true) // Minimize cost
}

// SetPerformanceObjective sets the performance objective
// Higher headroom = better performance potential
func (s *Solution) SetPerformanceObjective(currentCPUUsage, currentMemoryUsage int64, weight float64) {
	// Performance is measured as headroom percentage
	cpuHeadroom := 0.0
	if s.CPULimit > 0 {
		cpuHeadroom = float64(s.CPULimit-currentCPUUsage) / float64(s.CPULimit) * 100
	}

	memoryHeadroom := 0.0
	if s.MemoryLimit > 0 {
		memoryHeadroom = float64(s.MemoryLimit-currentMemoryUsage) / float64(s.MemoryLimit) * 100
	}

	// Average headroom as performance score
	performance := (cpuHeadroom + memoryHeadroom) / 2

	s.AddObjective(ObjectivePerformance, "Performance", performance, weight, false) // Maximize performance
}

// SetReliabilityObjective sets the reliability objective
// Based on buffer above peak usage
func (s *Solution) SetReliabilityObjective(peakCPU, peakMemory int64, weight float64) {
	cpuBuffer := 0.0
	if peakCPU > 0 {
		cpuBuffer = float64(s.CPURequest-peakCPU) / float64(peakCPU) * 100
	}

	memoryBuffer := 0.0
	if peakMemory > 0 {
		memoryBuffer = float64(s.MemoryRequest-peakMemory) / float64(peakMemory) * 100
	}

	// Reliability score: higher buffer = more reliable
	reliability := math.Min(cpuBuffer, memoryBuffer)
	if reliability < 0 {
		reliability = 0
	}

	s.AddObjective(ObjectiveReliability, "Reliability", reliability, weight, false) // Maximize reliability
}

// SetEfficiencyObjective sets the efficiency objective
// Ratio of actual usage to allocated resources
func (s *Solution) SetEfficiencyObjective(avgCPUUsage, avgMemoryUsage int64, weight float64) {
	cpuEfficiency := 0.0
	if s.CPURequest > 0 {
		cpuEfficiency = float64(avgCPUUsage) / float64(s.CPURequest) * 100
	}

	memoryEfficiency := 0.0
	if s.MemoryRequest > 0 {
		memoryEfficiency = float64(avgMemoryUsage) / float64(s.MemoryRequest) * 100
	}

	// Efficiency: how well resources are utilized (target ~70-80%)
	efficiency := (cpuEfficiency + memoryEfficiency) / 2

	s.AddObjective(ObjectiveEfficiency, "Efficiency", efficiency, weight, false) // Maximize efficiency
}

// SetStabilityObjective sets the stability objective
// Based on confidence and change magnitude
func (s *Solution) SetStabilityObjective(confidence float64, changePercent float64, weight float64) {
	// Stability: high confidence + low change = stable
	// Penalize large changes
	changePenalty := math.Min(changePercent/100, 1.0) * 50
	stability := confidence - changePenalty
	if stability < 0 {
		stability = 0
	}

	s.AddObjective(ObjectiveStability, "Stability", stability, weight, false) // Maximize stability
}

// Dominates returns true if this solution dominates the other
// A solution dominates another if it's at least as good in all objectives
// and strictly better in at least one
func (s *Solution) Dominates(other *Solution) bool {
	if len(s.Objectives) == 0 || len(other.Objectives) == 0 {
		return false
	}

	atLeastAsGoodInAll := true
	strictlyBetterInOne := false

	for objType, obj := range s.Objectives {
		otherObj, exists := other.Objectives[objType]
		if !exists {
			continue
		}

		var isBetter, isWorse bool
		if obj.IsMinimize {
			isBetter = obj.Value < otherObj.Value
			isWorse = obj.Value > otherObj.Value
		} else {
			isBetter = obj.Value > otherObj.Value
			isWorse = obj.Value < otherObj.Value
		}

		if isWorse {
			atLeastAsGoodInAll = false
			break
		}
		if isBetter {
			strictlyBetterInOne = true
		}
	}

	return atLeastAsGoodInAll && strictlyBetterInOne
}

// CalculateOverallScore computes weighted sum of normalized objectives
func (s *Solution) CalculateOverallScore() {
	if len(s.Objectives) == 0 {
		s.OverallScore = 0
		return
	}

	var totalWeight, weightedSum float64
	for _, obj := range s.Objectives {
		score := obj.Normalized
		if obj.IsMinimize {
			score = 1 - score // Invert for minimization objectives
		}
		weightedSum += score * obj.Weight
		totalWeight += obj.Weight
	}

	if totalWeight > 0 {
		s.OverallScore = weightedSum / totalWeight
	}
}

// Summary returns a human-readable summary of the solution
func (s *Solution) Summary() string {
	return fmt.Sprintf(
		"Solution %s: CPU=%dm, Memory=%dMi, Score=%.2f, Rank=%d",
		s.ID,
		s.CPURequest,
		s.MemoryRequest/(1024*1024),
		s.OverallScore,
		s.ParetoRank,
	)
}

// ObjectiveSummary returns a summary of all objectives
func (s *Solution) ObjectiveSummary() string {
	result := fmt.Sprintf("Solution %s objectives:\n", s.ID)
	for objType, obj := range s.Objectives {
		direction := "maximize"
		if obj.IsMinimize {
			direction = "minimize"
		}
		result += fmt.Sprintf("  - %s (%s): %.2f (weight=%.2f, normalized=%.2f)\n",
			objType, direction, obj.Value, obj.Weight, obj.Normalized)
	}
	return result
}
