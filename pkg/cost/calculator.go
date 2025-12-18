package cost

import (
	"fmt"
	"time"
)

// Calculator estimates cost savings from resource optimization
type Calculator struct {
	pricing *PricingModel
}

// PricingModel defines cloud resource pricing
type PricingModel struct {
	// Provider name (aws, gcp, azure, custom)
	Provider string

	// CPU pricing per core per hour (in USD)
	CPUPerCoreHour float64

	// Memory pricing per GB per hour (in USD)
	MemoryPerGBHour float64

	// Region for pricing (affects rates)
	Region string

	// PricingTier (on-demand, spot, reserved)
	Tier PricingTier
}

// PricingTier represents different pricing tiers
type PricingTier string

const (
	TierOnDemand PricingTier = "on-demand"
	TierSpot     PricingTier = "spot"
	TierReserved PricingTier = "reserved"
)

// DefaultPricingModels provides typical cloud pricing (approximate USD values)
var DefaultPricingModels = map[string]*PricingModel{
	"aws-us-east-1": {
		Provider:        "aws",
		Region:          "us-east-1",
		CPUPerCoreHour:  0.0416,  // ~$30/month per core
		MemoryPerGBHour: 0.0052,  // ~$3.75/month per GB
		Tier:            TierOnDemand,
	},
	"aws-us-east-1-spot": {
		Provider:        "aws",
		Region:          "us-east-1",
		CPUPerCoreHour:  0.0125,  // ~70% discount
		MemoryPerGBHour: 0.0016,
		Tier:            TierSpot,
	},
	"gcp-us-central1": {
		Provider:        "gcp",
		Region:          "us-central1",
		CPUPerCoreHour:  0.0350,
		MemoryPerGBHour: 0.0047,
		Tier:            TierOnDemand,
	},
	"azure-eastus": {
		Provider:        "azure",
		Region:          "eastus",
		CPUPerCoreHour:  0.0400,
		MemoryPerGBHour: 0.0050,
		Tier:            TierOnDemand,
	},
	"default": {
		Provider:        "generic",
		Region:          "default",
		CPUPerCoreHour:  0.040,   // Conservative estimate
		MemoryPerGBHour: 0.005,
		Tier:            TierOnDemand,
	},
}

// NewCalculator creates a new cost calculator with the specified pricing model
func NewCalculator(pricing *PricingModel) *Calculator {
	if pricing == nil {
		pricing = DefaultPricingModels["default"]
	}
	return &Calculator{pricing: pricing}
}

// NewCalculatorWithPreset creates a calculator using a preset pricing model
func NewCalculatorWithPreset(presetName string) *Calculator {
	pricing, ok := DefaultPricingModels[presetName]
	if !ok {
		pricing = DefaultPricingModels["default"]
	}
	return &Calculator{pricing: pricing}
}

// ResourceCost represents the cost of a resource allocation
type ResourceCost struct {
	// CPU cost per hour
	CPUCostPerHour float64

	// Memory cost per hour
	MemoryCostPerHour float64

	// Total cost per hour
	TotalPerHour float64

	// Projected costs
	TotalPerDay   float64
	TotalPerMonth float64
	TotalPerYear  float64
}

// SavingsEstimate represents estimated savings from optimization
type SavingsEstimate struct {
	// Current costs
	CurrentCost ResourceCost

	// Recommended costs
	RecommendedCost ResourceCost

	// Savings breakdown
	CPUSavingsPerHour    float64
	MemorySavingsPerHour float64
	TotalSavingsPerHour  float64

	// Projected savings
	SavingsPerDay   float64
	SavingsPerMonth float64
	SavingsPerYear  float64

	// Percentage reduction
	PercentageReduction float64

	// Resource changes
	CPUReduction    float64 // in millicores
	MemoryReduction int64   // in bytes
}

// ContainerSavings represents savings for a single container
type ContainerSavings struct {
	ContainerName   string
	Namespace       string
	WorkloadName    string
	Savings         SavingsEstimate
	ReplicaCount    int32
	TotalSavings    SavingsEstimate // Savings * ReplicaCount
}

// WorkloadSavings represents total savings for a workload
type WorkloadSavings struct {
	Namespace    string
	WorkloadKind string
	WorkloadName string
	ReplicaCount int32
	Containers   []ContainerSavings
	TotalSavings SavingsEstimate
}

// CalculateCost calculates the cost of resource allocation
// cpuMillicores: CPU in millicores (1000m = 1 core)
// memoryBytes: Memory in bytes
func (c *Calculator) CalculateCost(cpuMillicores int64, memoryBytes int64) ResourceCost {
	// Convert millicores to cores
	cpuCores := float64(cpuMillicores) / 1000.0

	// Convert bytes to GB
	memoryGB := float64(memoryBytes) / (1024 * 1024 * 1024)

	cpuCostPerHour := cpuCores * c.pricing.CPUPerCoreHour
	memoryCostPerHour := memoryGB * c.pricing.MemoryPerGBHour
	totalPerHour := cpuCostPerHour + memoryCostPerHour

	return ResourceCost{
		CPUCostPerHour:    cpuCostPerHour,
		MemoryCostPerHour: memoryCostPerHour,
		TotalPerHour:      totalPerHour,
		TotalPerDay:       totalPerHour * 24,
		TotalPerMonth:     totalPerHour * 24 * 30,
		TotalPerYear:      totalPerHour * 24 * 365,
	}
}

// EstimateSavings calculates savings between current and recommended resources
func (c *Calculator) EstimateSavings(
	currentCPU, recommendedCPU int64, // millicores
	currentMemory, recommendedMemory int64, // bytes
) SavingsEstimate {
	currentCost := c.CalculateCost(currentCPU, currentMemory)
	recommendedCost := c.CalculateCost(recommendedCPU, recommendedMemory)

	cpuSavingsPerHour := currentCost.CPUCostPerHour - recommendedCost.CPUCostPerHour
	memorySavingsPerHour := currentCost.MemoryCostPerHour - recommendedCost.MemoryCostPerHour
	totalSavingsPerHour := cpuSavingsPerHour + memorySavingsPerHour

	// Calculate percentage reduction (avoid division by zero)
	var percentageReduction float64
	if currentCost.TotalPerHour > 0 {
		percentageReduction = (totalSavingsPerHour / currentCost.TotalPerHour) * 100
	}

	return SavingsEstimate{
		CurrentCost:          currentCost,
		RecommendedCost:      recommendedCost,
		CPUSavingsPerHour:    cpuSavingsPerHour,
		MemorySavingsPerHour: memorySavingsPerHour,
		TotalSavingsPerHour:  totalSavingsPerHour,
		SavingsPerDay:        totalSavingsPerHour * 24,
		SavingsPerMonth:      totalSavingsPerHour * 24 * 30,
		SavingsPerYear:       totalSavingsPerHour * 24 * 365,
		PercentageReduction:  percentageReduction,
		CPUReduction:         float64(currentCPU - recommendedCPU),
		MemoryReduction:      currentMemory - recommendedMemory,
	}
}

// EstimateWorkloadSavings calculates total savings for a workload with multiple containers
func (c *Calculator) EstimateWorkloadSavings(
	namespace, workloadKind, workloadName string,
	replicaCount int32,
	containers []ContainerResourceChange,
) WorkloadSavings {
	var containerSavings []ContainerSavings
	var totalCPUSavings, totalMemorySavings float64
	var totalCurrentCost, totalRecommendedCost float64

	for _, container := range containers {
		savings := c.EstimateSavings(
			container.CurrentCPU, container.RecommendedCPU,
			container.CurrentMemory, container.RecommendedMemory,
		)

		// Scale by replica count
		scaledSavings := SavingsEstimate{
			CurrentCost:          scaleResourceCost(savings.CurrentCost, replicaCount),
			RecommendedCost:      scaleResourceCost(savings.RecommendedCost, replicaCount),
			CPUSavingsPerHour:    savings.CPUSavingsPerHour * float64(replicaCount),
			MemorySavingsPerHour: savings.MemorySavingsPerHour * float64(replicaCount),
			TotalSavingsPerHour:  savings.TotalSavingsPerHour * float64(replicaCount),
			SavingsPerDay:        savings.SavingsPerDay * float64(replicaCount),
			SavingsPerMonth:      savings.SavingsPerMonth * float64(replicaCount),
			SavingsPerYear:       savings.SavingsPerYear * float64(replicaCount),
			PercentageReduction:  savings.PercentageReduction,
			CPUReduction:         savings.CPUReduction * float64(replicaCount),
			MemoryReduction:      savings.MemoryReduction * int64(replicaCount),
		}

		containerSavings = append(containerSavings, ContainerSavings{
			ContainerName: container.ContainerName,
			Namespace:     namespace,
			WorkloadName:  workloadName,
			Savings:       savings,
			ReplicaCount:  replicaCount,
			TotalSavings:  scaledSavings,
		})

		totalCPUSavings += scaledSavings.CPUSavingsPerHour
		totalMemorySavings += scaledSavings.MemorySavingsPerHour
		totalCurrentCost += scaledSavings.CurrentCost.TotalPerHour
		totalRecommendedCost += scaledSavings.RecommendedCost.TotalPerHour
	}

	totalSavingsPerHour := totalCPUSavings + totalMemorySavings
	var percentageReduction float64
	if totalCurrentCost > 0 {
		percentageReduction = (totalSavingsPerHour / totalCurrentCost) * 100
	}

	return WorkloadSavings{
		Namespace:    namespace,
		WorkloadKind: workloadKind,
		WorkloadName: workloadName,
		ReplicaCount: replicaCount,
		Containers:   containerSavings,
		TotalSavings: SavingsEstimate{
			CurrentCost: ResourceCost{
				TotalPerHour:  totalCurrentCost,
				TotalPerDay:   totalCurrentCost * 24,
				TotalPerMonth: totalCurrentCost * 24 * 30,
				TotalPerYear:  totalCurrentCost * 24 * 365,
			},
			RecommendedCost: ResourceCost{
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
		},
	}
}

// ContainerResourceChange represents a resource change for a container
type ContainerResourceChange struct {
	ContainerName     string
	CurrentCPU        int64 // millicores
	RecommendedCPU    int64 // millicores
	CurrentMemory     int64 // bytes
	RecommendedMemory int64 // bytes
}

// scaleResourceCost multiplies a ResourceCost by a replica count
func scaleResourceCost(cost ResourceCost, replicas int32) ResourceCost {
	factor := float64(replicas)
	return ResourceCost{
		CPUCostPerHour:    cost.CPUCostPerHour * factor,
		MemoryCostPerHour: cost.MemoryCostPerHour * factor,
		TotalPerHour:      cost.TotalPerHour * factor,
		TotalPerDay:       cost.TotalPerDay * factor,
		TotalPerMonth:     cost.TotalPerMonth * factor,
		TotalPerYear:      cost.TotalPerYear * factor,
	}
}

// FormatSavings returns a human-readable summary of savings
func (s *SavingsEstimate) FormatSavings() string {
	if s.TotalSavingsPerHour <= 0 {
		return "No savings (recommendation increases cost or no change)"
	}

	return fmt.Sprintf(
		"Savings: $%.4f/hour ($%.2f/day, $%.2f/month, $%.2f/year) - %.1f%% reduction",
		s.TotalSavingsPerHour,
		s.SavingsPerDay,
		s.SavingsPerMonth,
		s.SavingsPerYear,
		s.PercentageReduction,
	)
}

// FormatCost returns a human-readable summary of cost
func (c *ResourceCost) FormatCost() string {
	return fmt.Sprintf(
		"$%.4f/hour ($%.2f/day, $%.2f/month, $%.2f/year)",
		c.TotalPerHour,
		c.TotalPerDay,
		c.TotalPerMonth,
		c.TotalPerYear,
	)
}

// SavingsReport represents a comprehensive savings report
type SavingsReport struct {
	GeneratedAt      time.Time
	PricingModel     string
	Workloads        []WorkloadSavings
	TotalSavings     SavingsEstimate
	TotalCurrentCost ResourceCost
	WorkloadCount    int
	ContainerCount   int
}

// GenerateReport creates a comprehensive savings report from multiple workloads
func (c *Calculator) GenerateReport(workloads []WorkloadSavings) SavingsReport {
	var totalCurrentCost, totalRecommendedCost float64
	var totalCPUSavings, totalMemorySavings float64
	containerCount := 0

	for _, w := range workloads {
		totalCurrentCost += w.TotalSavings.CurrentCost.TotalPerHour
		totalRecommendedCost += w.TotalSavings.RecommendedCost.TotalPerHour
		totalCPUSavings += w.TotalSavings.CPUSavingsPerHour
		totalMemorySavings += w.TotalSavings.MemorySavingsPerHour
		containerCount += len(w.Containers)
	}

	totalSavingsPerHour := totalCPUSavings + totalMemorySavings
	var percentageReduction float64
	if totalCurrentCost > 0 {
		percentageReduction = (totalSavingsPerHour / totalCurrentCost) * 100
	}

	return SavingsReport{
		GeneratedAt:  time.Now(),
		PricingModel: fmt.Sprintf("%s-%s (%s)", c.pricing.Provider, c.pricing.Region, c.pricing.Tier),
		Workloads:    workloads,
		TotalSavings: SavingsEstimate{
			CurrentCost: ResourceCost{
				TotalPerHour:  totalCurrentCost,
				TotalPerDay:   totalCurrentCost * 24,
				TotalPerMonth: totalCurrentCost * 24 * 30,
				TotalPerYear:  totalCurrentCost * 24 * 365,
			},
			RecommendedCost: ResourceCost{
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
		},
		TotalCurrentCost: ResourceCost{
			TotalPerHour:  totalCurrentCost,
			TotalPerDay:   totalCurrentCost * 24,
			TotalPerMonth: totalCurrentCost * 24 * 30,
			TotalPerYear:  totalCurrentCost * 24 * 365,
		},
		WorkloadCount:  len(workloads),
		ContainerCount: containerCount,
	}
}

// FormatReport returns a human-readable summary of the report
func (r *SavingsReport) FormatReport() string {
	return fmt.Sprintf(`
=== Cost Optimization Report ===
Generated: %s
Pricing Model: %s

Summary:
  Workloads Analyzed: %d
  Containers Analyzed: %d

Current Cost:
  %s

Projected Cost (After Optimization):
  %s

Estimated Savings:
  %s
================================
`,
		r.GeneratedAt.Format(time.RFC3339),
		r.PricingModel,
		r.WorkloadCount,
		r.ContainerCount,
		r.TotalCurrentCost.FormatCost(),
		r.TotalSavings.RecommendedCost.FormatCost(),
		r.TotalSavings.FormatSavings(),
	)
}
