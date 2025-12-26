package policy

import (
	"time"
)

// Policy represents a single optimization policy rule
type Policy struct {
	// Name is the unique identifier for this policy
	Name string `json:"name" yaml:"name"`

	// Description explains what this policy does
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// Condition is an expression that must evaluate to true for the policy to apply
	// Example: "workload.labels['app-type'] == 'database'"
	Condition string `json:"condition" yaml:"condition"`

	// Action defines what to do when the condition matches
	// Possible values: "allow", "deny", "skip", "skip-scaledown", "skip-scaleup",
	//                  "set-min-cpu", "set-max-cpu", "set-min-memory", "set-max-memory"
	Action string `json:"action" yaml:"action"`

	// Parameters contains action-specific parameters
	// Example: {"min-memory": "512Mi"} for "set-min-memory" action
	Parameters map[string]string `json:"parameters,omitempty" yaml:"parameters,omitempty"`

	// Priority determines evaluation order (higher priority = evaluated first)
	Priority int `json:"priority,omitempty" yaml:"priority,omitempty"`

	// Enabled allows temporarily disabling a policy without removing it
	Enabled bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

// PolicySet is a collection of policies
type PolicySet struct {
	// Policies is the list of policy rules
	Policies []Policy `json:"policies" yaml:"policies"`

	// DefaultAction is taken when no policy matches
	// Default: "allow"
	DefaultAction string `json:"defaultAction,omitempty" yaml:"defaultAction,omitempty"`
}

// EvaluationContext contains all data available for policy evaluation
type EvaluationContext struct {
	// Workload information
	Workload WorkloadInfo `json:"workload"`

	// Recommendation being evaluated
	Recommendation RecommendationInfo `json:"recommendation"`

	// Time information
	Time TimeInfo `json:"time"`

	// Cluster information
	Cluster ClusterInfo `json:"cluster"`

	// Custom labels and annotations
	Custom map[string]interface{} `json:"custom,omitempty"`
}

// WorkloadInfo contains workload metadata for policy evaluation
type WorkloadInfo struct {
	// Namespace where the workload runs
	Namespace string `json:"namespace"`

	// Name of the workload
	Name string `json:"name"`

	// Kind of workload (Deployment, StatefulSet, DaemonSet)
	Kind string `json:"kind"`

	// Labels attached to the workload
	Labels map[string]string `json:"labels"`

	// Annotations attached to the workload
	Annotations map[string]string `json:"annotations"`

	// Replicas count
	Replicas int32 `json:"replicas"`

	// CurrentCPU in millicores
	CurrentCPU int64 `json:"currentCPU"`

	// CurrentMemory in bytes
	CurrentMemory int64 `json:"currentMemory"`
}

// ToExprEnv converts WorkloadInfo to a map for expr evaluation with lowercase keys
func (w WorkloadInfo) ToExprEnv() map[string]interface{} {
	return map[string]interface{}{
		"namespace":     w.Namespace,
		"name":          w.Name,
		"kind":          w.Kind,
		"labels":        w.Labels,
		"annotations":   w.Annotations,
		"replicas":      w.Replicas,
		"currentCPU":    w.CurrentCPU,
		"currentMemory": w.CurrentMemory,
	}
}

// RecommendationInfo contains the recommendation being evaluated
type RecommendationInfo struct {
	// RecommendedCPU in millicores
	RecommendedCPU int64 `json:"recommendedCPU"`

	// RecommendedMemory in bytes
	RecommendedMemory int64 `json:"recommendedMemory"`

	// Confidence score (0-100)
	Confidence float64 `json:"confidence"`

	// ChangeType indicates the type of change ("scaleup", "scaledown", "nochange")
	ChangeType string `json:"changeType"`

	// CPUChangePercent is the percentage change in CPU
	CPUChangePercent float64 `json:"cpuChangePercent"`

	// MemoryChangePercent is the percentage change in memory
	MemoryChangePercent float64 `json:"memoryChangePercent"`
}

// ToExprEnv converts RecommendationInfo to a map for expr evaluation
func (r RecommendationInfo) ToExprEnv() map[string]interface{} {
	return map[string]interface{}{
		"recommendedCPU":      r.RecommendedCPU,
		"recommendedMemory":   r.RecommendedMemory,
		"confidence":          r.Confidence,
		"changeType":          r.ChangeType,
		"cpuChangePercent":    r.CPUChangePercent,
		"memoryChangePercent": r.MemoryChangePercent,
	}
}

// TimeInfo contains time-related information for time-based policies
type TimeInfo struct {
	// Now is the current time
	Now time.Time `json:"now"`

	// Hour is the current hour (0-23)
	Hour int `json:"hour"`

	// Weekday is the current day of week (0-6, 0=Sunday)
	Weekday int `json:"weekday"`

	// IsBusinessHours indicates if current time is business hours (9-17)
	IsBusinessHours bool `json:"isBusinessHours"`

	// IsWeekend indicates if current day is Saturday or Sunday
	IsWeekend bool `json:"isWeekend"`
}

// ToExprEnv converts TimeInfo to a map for expr evaluation
func (t TimeInfo) ToExprEnv() map[string]interface{} {
	return map[string]interface{}{
		"now":             t.Now,
		"hour":            t.Hour,
		"weekday":         t.Weekday,
		"isBusinessHours": t.IsBusinessHours,
		"isWeekend":       t.IsWeekend,
	}
}

// ClusterInfo contains cluster-level information
type ClusterInfo struct {
	// TotalNodes is the total number of nodes in the cluster
	TotalNodes int `json:"totalNodes"`

	// AvailableCPU is total available CPU in the cluster (millicores)
	AvailableCPU int64 `json:"availableCPU,omitempty"`

	// AvailableMemory is total available memory in the cluster (bytes)
	AvailableMemory int64 `json:"availableMemory,omitempty"`

	// Environment indicates the cluster environment (production, staging, development)
	Environment string `json:"environment,omitempty"`
}

// ToExprEnv converts ClusterInfo to a map for expr evaluation
func (c ClusterInfo) ToExprEnv() map[string]interface{} {
	return map[string]interface{}{
		"totalNodes":      c.TotalNodes,
		"availableCPU":    c.AvailableCPU,
		"availableMemory": c.AvailableMemory,
		"environment":     c.Environment,
	}
}

// PolicyDecision represents the result of policy evaluation
type PolicyDecision struct {
	// Action is the final decision ("allow", "deny", "modify")
	Action string

	// Reason explains why this decision was made
	Reason string

	// MatchedPolicy is the name of the policy that matched (if any)
	MatchedPolicy string

	// ModifiedRecommendation contains modified resource values (if action is "modify")
	ModifiedRecommendation *ModifiedRecommendation
}

// ModifiedRecommendation contains resource modifications applied by policies
type ModifiedRecommendation struct {
	// CPURequest is the modified CPU request (nil means no change)
	CPURequest *int64

	// MemoryRequest is the modified memory request (nil means no change)
	MemoryRequest *int64

	// Modifications lists all modifications applied
	Modifications []string
}

// ActionType constants for common policy actions
const (
	ActionAllow           = "allow"
	ActionDeny            = "deny"
	ActionSkip            = "skip"
	ActionSkipScaleDown   = "skip-scaledown"
	ActionSkipScaleUp     = "skip-scaleup"
	ActionSetMinCPU       = "set-min-cpu"
	ActionSetMaxCPU       = "set-max-cpu"
	ActionSetMinMemory    = "set-min-memory"
	ActionSetMaxMemory    = "set-max-memory"
	ActionRequireApproval = "require-approval"
)
