package pareto

import (
	"math"
	"sort"
)

// Optimizer performs multi-objective Pareto optimization
type Optimizer struct {
	// Default objective weights
	CostWeight        float64
	PerformanceWeight float64
	ReliabilityWeight float64
	EfficiencyWeight  float64
	StabilityWeight   float64

	// Cost parameters (per hour)
	CPUCostPerCore   float64 // Cost per CPU core per hour
	MemoryCostPerGB  float64 // Cost per GB memory per hour
}

// NewOptimizer creates a new Pareto optimizer with default weights
func NewOptimizer() *Optimizer {
	return &Optimizer{
		// Default balanced weights
		CostWeight:        0.25,
		PerformanceWeight: 0.20,
		ReliabilityWeight: 0.25,
		EfficiencyWeight:  0.15,
		StabilityWeight:   0.15,

		// Default AWS-like costs
		CPUCostPerCore:  0.0336, // ~$24/month per core
		MemoryCostPerGB: 0.0045, // ~$3.24/month per GB
	}
}

// OptimizationResult contains the results of Pareto optimization
type OptimizationResult struct {
	// All solutions analyzed
	AllSolutions []*Solution

	// Pareto optimal solutions (frontier)
	ParetoFrontier []*Solution

	// Best solution based on weighted objectives
	BestSolution *Solution

	// Solutions grouped by rank
	RankedSolutions map[int][]*Solution

	// Trade-off analysis
	TradeOffs []TradeOff
}

// TradeOff describes a trade-off between two solutions
type TradeOff struct {
	SolutionA    string
	SolutionB    string
	Improvements map[ObjectiveType]float64 // Positive = A is better
	Degradations map[ObjectiveType]float64 // Positive = A is worse
	Summary      string
}

// Optimize performs Pareto optimization on a set of solutions
func (o *Optimizer) Optimize(solutions []*Solution) *OptimizationResult {
	if len(solutions) == 0 {
		return &OptimizationResult{
			RankedSolutions: make(map[int][]*Solution),
		}
	}

	result := &OptimizationResult{
		AllSolutions:    solutions,
		RankedSolutions: make(map[int][]*Solution),
	}

	// Step 1: Normalize all objectives across solutions
	o.normalizeObjectives(solutions)

	// Step 2: Calculate dominance relationships
	o.calculateDominance(solutions)

	// Step 3: Assign Pareto ranks using non-dominated sorting
	o.assignParetoRanks(solutions, result)

	// Step 4: Calculate crowding distance for diversity
	o.calculateCrowdingDistance(result.ParetoFrontier)

	// Step 5: Calculate overall scores
	for _, s := range solutions {
		s.CalculateOverallScore()
	}

	// Step 6: Find best solution (highest score on frontier)
	result.BestSolution = o.findBestSolution(result.ParetoFrontier)

	// Step 7: Analyze trade-offs between frontier solutions
	result.TradeOffs = o.analyzeTradeOffs(result.ParetoFrontier)

	return result
}

// normalizeObjectives normalizes objective values to [0, 1] range
func (o *Optimizer) normalizeObjectives(solutions []*Solution) {
	if len(solutions) == 0 {
		return
	}

	// Find min/max for each objective type
	minMax := make(map[ObjectiveType]struct{ min, max float64 })

	for _, s := range solutions {
		for objType, obj := range s.Objectives {
			mm, exists := minMax[objType]
			if !exists {
				mm = struct{ min, max float64 }{obj.Value, obj.Value}
			} else {
				if obj.Value < mm.min {
					mm.min = obj.Value
				}
				if obj.Value > mm.max {
					mm.max = obj.Value
				}
			}
			minMax[objType] = mm
		}
	}

	// Normalize each objective
	for _, s := range solutions {
		for objType, obj := range s.Objectives {
			mm := minMax[objType]
			if mm.max-mm.min > 0 {
				obj.Normalized = (obj.Value - mm.min) / (mm.max - mm.min)
			} else {
				obj.Normalized = 0.5 // All values are the same
			}
		}
	}
}

// calculateDominance computes which solutions dominate others
func (o *Optimizer) calculateDominance(solutions []*Solution) {
	n := len(solutions)

	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if i == j {
				continue
			}

			if solutions[i].Dominates(solutions[j]) {
				solutions[i].DominatesIDs = append(solutions[i].DominatesIDs, solutions[j].ID)
				solutions[j].DominatedBy = append(solutions[j].DominatedBy, solutions[i].ID)
			}
		}
	}
}

// assignParetoRanks uses non-dominated sorting to assign ranks
func (o *Optimizer) assignParetoRanks(solutions []*Solution, result *OptimizationResult) {
	remaining := make([]*Solution, len(solutions))
	copy(remaining, solutions)

	rank := 0
	for len(remaining) > 0 {
		// Find non-dominated solutions in current set
		var frontier []*Solution
		var nextRemaining []*Solution

		for _, s := range remaining {
			isDominated := false
			for _, other := range remaining {
				if s.ID != other.ID && other.Dominates(s) {
					isDominated = true
					break
				}
			}

			if !isDominated {
				s.ParetoRank = rank
				frontier = append(frontier, s)
			} else {
				nextRemaining = append(nextRemaining, s)
			}
		}

		result.RankedSolutions[rank] = frontier
		if rank == 0 {
			result.ParetoFrontier = frontier
		}

		remaining = nextRemaining
		rank++
	}
}

// calculateCrowdingDistance computes diversity metric for frontier solutions
func (o *Optimizer) calculateCrowdingDistance(frontier []*Solution) {
	if len(frontier) <= 2 {
		for _, s := range frontier {
			s.CrowdingDistance = math.MaxFloat64
		}
		return
	}

	// Initialize crowding distances
	for _, s := range frontier {
		s.CrowdingDistance = 0
	}

	// Get all objective types
	objTypes := make([]ObjectiveType, 0)
	if len(frontier) > 0 {
		for objType := range frontier[0].Objectives {
			objTypes = append(objTypes, objType)
		}
	}

	// Calculate crowding distance for each objective
	for _, objType := range objTypes {
		// Sort by this objective
		sorted := make([]*Solution, len(frontier))
		copy(sorted, frontier)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Objectives[objType].Value < sorted[j].Objectives[objType].Value
		})

		// Boundary solutions get infinite distance
		sorted[0].CrowdingDistance = math.MaxFloat64
		sorted[len(sorted)-1].CrowdingDistance = math.MaxFloat64

		// Calculate range
		minVal := sorted[0].Objectives[objType].Value
		maxVal := sorted[len(sorted)-1].Objectives[objType].Value
		objRange := maxVal - minVal

		if objRange == 0 {
			continue
		}

		// Add to crowding distance
		for i := 1; i < len(sorted)-1; i++ {
			if sorted[i].CrowdingDistance < math.MaxFloat64 {
				distance := (sorted[i+1].Objectives[objType].Value -
					sorted[i-1].Objectives[objType].Value) / objRange
				sorted[i].CrowdingDistance += distance
			}
		}
	}
}

// findBestSolution returns the highest-scoring solution from the frontier
func (o *Optimizer) findBestSolution(frontier []*Solution) *Solution {
	if len(frontier) == 0 {
		return nil
	}

	best := frontier[0]
	for _, s := range frontier[1:] {
		if s.OverallScore > best.OverallScore {
			best = s
		}
	}

	return best
}

// analyzeTradeOffs compares frontier solutions pairwise
func (o *Optimizer) analyzeTradeOffs(frontier []*Solution) []TradeOff {
	var tradeOffs []TradeOff

	for i := 0; i < len(frontier); i++ {
		for j := i + 1; j < len(frontier); j++ {
			to := o.compareTradeOff(frontier[i], frontier[j])
			if to != nil {
				tradeOffs = append(tradeOffs, *to)
			}
		}
	}

	return tradeOffs
}

// compareTradeOff compares two solutions and describes their trade-offs
func (o *Optimizer) compareTradeOff(a, b *Solution) *TradeOff {
	to := &TradeOff{
		SolutionA:    a.ID,
		SolutionB:    b.ID,
		Improvements: make(map[ObjectiveType]float64),
		Degradations: make(map[ObjectiveType]float64),
	}

	for objType, objA := range a.Objectives {
		objB, exists := b.Objectives[objType]
		if !exists {
			continue
		}

		var diff float64
		if objA.IsMinimize {
			diff = objB.Value - objA.Value // Positive = A is better (lower)
		} else {
			diff = objA.Value - objB.Value // Positive = A is better (higher)
		}

		if diff > 0 {
			to.Improvements[objType] = diff
		} else if diff < 0 {
			to.Degradations[objType] = -diff
		}
	}

	// Generate summary
	if len(to.Improvements) > 0 && len(to.Degradations) > 0 {
		to.Summary = "Trade-off: "
		for objType := range to.Improvements {
			to.Summary += string(objType) + " better in A, "
		}
		for objType := range to.Degradations {
			to.Summary += string(objType) + " better in B, "
		}
	}

	return to
}

// GenerateSolutionSet creates multiple candidate solutions for a workload
func (o *Optimizer) GenerateSolutionSet(
	namespace, workloadName string,
	currentCPU, currentMemory int64,
	avgCPU, avgMemory int64,
	peakCPU, peakMemory int64,
	p95CPU, p95Memory int64,
	p99CPU, p99Memory int64,
	confidence float64,
) []*Solution {
	var solutions []*Solution

	// Strategy 1: Conservative (P99 + 20% buffer)
	conservativeSol := o.createSolution("conservative", namespace, workloadName,
		int64(float64(p99CPU)*1.2), int64(float64(p99Memory)*1.2),
		avgCPU, avgMemory, peakCPU, peakMemory, confidence, 0)
	solutions = append(solutions, conservativeSol)

	// Strategy 2: Balanced (P95 + 10% buffer)
	balancedSol := o.createSolution("balanced", namespace, workloadName,
		int64(float64(p95CPU)*1.1), int64(float64(p95Memory)*1.1),
		avgCPU, avgMemory, peakCPU, peakMemory, confidence, 10)
	solutions = append(solutions, balancedSol)

	// Strategy 3: Aggressive (P95 exact)
	aggressiveSol := o.createSolution("aggressive", namespace, workloadName,
		p95CPU, p95Memory,
		avgCPU, avgMemory, peakCPU, peakMemory, confidence, 20)
	solutions = append(solutions, aggressiveSol)

	// Strategy 4: Cost-optimized (Average + 30% buffer)
	costOptSol := o.createSolution("cost-optimized", namespace, workloadName,
		int64(float64(avgCPU)*1.3), int64(float64(avgMemory)*1.3),
		avgCPU, avgMemory, peakCPU, peakMemory, confidence, 30)
	solutions = append(solutions, costOptSol)

	// Strategy 5: Performance (Peak + 25% buffer)
	perfSol := o.createSolution("performance", namespace, workloadName,
		int64(float64(peakCPU)*1.25), int64(float64(peakMemory)*1.25),
		avgCPU, avgMemory, peakCPU, peakMemory, confidence, 5)
	solutions = append(solutions, perfSol)

	// Strategy 6: Current (keep current allocation)
	currentSol := o.createSolution("current", namespace, workloadName,
		currentCPU, currentMemory,
		avgCPU, avgMemory, peakCPU, peakMemory, confidence, 0)
	solutions = append(solutions, currentSol)

	return solutions
}

// createSolution creates a solution with all objectives calculated
func (o *Optimizer) createSolution(
	strategy, namespace, workloadName string,
	cpuRequest, memoryRequest int64,
	avgCPU, avgMemory int64,
	peakCPU, peakMemory int64,
	confidence, changePercent float64,
) *Solution {
	sol := NewSolution(strategy, namespace, workloadName)
	sol.CPURequest = cpuRequest
	sol.MemoryRequest = memoryRequest
	sol.CPULimit = int64(float64(cpuRequest) * 1.5)    // Limit = 1.5x request
	sol.MemoryLimit = int64(float64(memoryRequest) * 1.2) // Memory limit = 1.2x request
	sol.Confidence = confidence
	sol.Source = strategy

	// Set all objectives
	sol.SetCostObjective(o.CPUCostPerCore, o.MemoryCostPerGB, o.CostWeight)
	sol.SetPerformanceObjective(avgCPU, avgMemory, o.PerformanceWeight)
	sol.SetReliabilityObjective(peakCPU, peakMemory, o.ReliabilityWeight)
	sol.SetEfficiencyObjective(avgCPU, avgMemory, o.EfficiencyWeight)
	sol.SetStabilityObjective(confidence, changePercent, o.StabilityWeight)

	return sol
}

// SelectBestForProfile selects the best solution based on optimization profile
func (o *Optimizer) SelectBestForProfile(result *OptimizationResult, profile string) *Solution {
	if len(result.ParetoFrontier) == 0 {
		return nil
	}

	// Adjust weights based on profile
	switch profile {
	case "production":
		// Prioritize reliability and stability
		return o.selectByObjective(result.ParetoFrontier, ObjectiveReliability)
	case "staging":
		// Balanced approach
		return result.BestSolution
	case "development":
		// Prioritize cost savings
		return o.selectByObjective(result.ParetoFrontier, ObjectiveCost)
	case "performance":
		// Prioritize performance
		return o.selectByObjective(result.ParetoFrontier, ObjectivePerformance)
	default:
		return result.BestSolution
	}
}

// selectByObjective returns the solution that's best for a specific objective
func (o *Optimizer) selectByObjective(frontier []*Solution, objType ObjectiveType) *Solution {
	if len(frontier) == 0 {
		return nil
	}

	best := frontier[0]
	for _, s := range frontier[1:] {
		obj := s.Objectives[objType]
		bestObj := best.Objectives[objType]

		if obj == nil || bestObj == nil {
			continue
		}

		var isBetter bool
		if obj.IsMinimize {
			isBetter = obj.Value < bestObj.Value
		} else {
			isBetter = obj.Value > bestObj.Value
		}

		if isBetter {
			best = s
		}
	}

	return best
}
