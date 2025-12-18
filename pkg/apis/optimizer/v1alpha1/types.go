package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=optconfig;oc
// +kubebuilder:printcolumn:name="Strategy",type=string,JSONPath=`.spec.strategy`
// +kubebuilder:printcolumn:name="Dry-Run",type=boolean,JSONPath=`.spec.dryRun`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// OptimizerConfig is the Schema for the optimizerconfigs API
type OptimizerConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OptimizerConfigSpec   `json:"spec"`
	Status OptimizerConfigStatus `json:"status,omitempty"`
}

// OptimizerConfigSpec defines the desired state of OptimizerConfig
type OptimizerConfigSpec struct {
	// Enabled controls whether the optimizer is active for this configuration
	// +optional
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// TargetNamespaces is a list of namespaces to monitor and optimize
	// +required
	// +kubebuilder:validation:MinItems=1
	TargetNamespaces []string `json:"targetNamespaces"`

	// Profile selects a predefined optimization profile (production, staging, development, test)
	// When set, this overrides the Strategy field with profile-specific settings
	// +optional
	// +kubebuilder:validation:Enum=production;staging;development;test;custom
	Profile EnvironmentProfile `json:"profile,omitempty"`

	// ProfileOverrides allows overriding specific profile settings
	// Only used when Profile is set
	// +optional
	ProfileOverrides *ProfileOverrides `json:"profileOverrides,omitempty"`

	// Strategy defines the optimization strategy (aggressive, balanced, conservative)
	// Ignored if Profile is set (profile determines strategy)
	// +optional
	// +kubebuilder:validation:Enum=aggressive;balanced;conservative
	// +kubebuilder:default=balanced
	Strategy OptimizationStrategy `json:"strategy,omitempty"`

	// DryRun mode only logs recommendations without applying changes
	// +optional
	// +kubebuilder:default=false
	DryRun bool `json:"dryRun"`

	// MaintenanceWindows defines when updates are allowed to be applied
	// +optional
	MaintenanceWindows []MaintenanceWindow `json:"maintenanceWindows,omitempty"`

	// ResourceThresholds defines min/max resource limits for recommendations
	// +optional
	ResourceThresholds *ResourceThresholds `json:"resourceThresholds,omitempty"`

	// Recommendations configures how recommendations are calculated
	// +optional
	Recommendations *RecommendationConfig `json:"recommendations,omitempty"`

	// UpdateStrategy defines how updates are applied to workloads
	// +optional
	UpdateStrategy *UpdateStrategy `json:"updateStrategy,omitempty"`

	// HPAAwareness configures HPA conflict detection
	// +optional
	HPAAwareness *HPAAwareness `json:"hpaAwareness,omitempty"`

	// PDBAwareness configures PodDisruptionBudget validation
	// +optional
	PDBAwareness *PDBAwareness `json:"pdbAwareness,omitempty"`

	// CircuitBreaker configures the circuit breaker for failure protection
	// +optional
	CircuitBreaker *CircuitBreakerConfig `json:"circuitBreaker,omitempty"`

	// TargetResources defines which resource types to optimize
	// +optional
	// +kubebuilder:default={deployments,statefulsets}
	TargetResources []TargetResourceType `json:"targetResources,omitempty"`

	// ExcludeWorkloads is a list of workload name patterns to exclude (regex)
	// +optional
	ExcludeWorkloads []string `json:"excludeWorkloads,omitempty"`
}

// OptimizationStrategy defines how aggressive the optimizer should be
// +kubebuilder:validation:Enum=aggressive;balanced;conservative
type OptimizationStrategy string

const (
	// Aggressive strategy applies recommendations quickly with minimal safety margin
	StrategyAggressive OptimizationStrategy = "aggressive"
	// Balanced strategy balances optimization with safety
	StrategyBalanced OptimizationStrategy = "balanced"
	// Conservative strategy prioritizes safety over optimization
	StrategyConservative OptimizationStrategy = "conservative"
)

// EnvironmentProfile defines predefined optimization profiles for different environments
// +kubebuilder:validation:Enum=production;staging;development;test;custom
type EnvironmentProfile string

const (
	// ProfileProduction uses conservative settings optimized for production stability
	ProfileProduction EnvironmentProfile = "production"
	// ProfileStaging uses balanced settings for staging/pre-prod environments
	ProfileStaging EnvironmentProfile = "staging"
	// ProfileDevelopment uses aggressive settings for dev environments
	ProfileDevelopment EnvironmentProfile = "development"
	// ProfileTest uses very aggressive settings for test/ephemeral workloads
	ProfileTest EnvironmentProfile = "test"
	// ProfileCustom allows fully custom configuration
	ProfileCustom EnvironmentProfile = "custom"
)

// ProfileOverrides allows overriding specific settings from a profile
type ProfileOverrides struct {
	// MinConfidence overrides the minimum confidence score required (0-100)
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	MinConfidence *float64 `json:"minConfidence,omitempty"`

	// MaxChangePercent overrides the maximum allowed change percentage
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	MaxChangePercent *float64 `json:"maxChangePercent,omitempty"`

	// RequireApproval overrides whether manual approval is required
	// +optional
	RequireApproval *bool `json:"requireApproval,omitempty"`

	// ApplyDelay overrides how long to wait before applying (e.g., "1h", "24h")
	// +optional
	ApplyDelay string `json:"applyDelay,omitempty"`

	// DryRun overrides whether to start in dry-run mode
	// +optional
	DryRun *bool `json:"dryRun,omitempty"`
}

// MaintenanceWindow defines a time window when updates are allowed
type MaintenanceWindow struct {
	// Schedule is a cron expression for when the window starts (UTC)
	// +required
	Schedule string `json:"schedule"`

	// Duration is how long the maintenance window lasts (e.g., "2h", "30m")
	// +required
	Duration string `json:"duration"`

	// Timezone specifies the timezone for the schedule (IANA format)
	// +optional
	// +kubebuilder:default=UTC
	Timezone string `json:"timezone,omitempty"`
}

// ResourceThresholds defines min/max resource boundaries
type ResourceThresholds struct {
	// CPU threshold configuration
	// +optional
	CPU *ResourceLimit `json:"cpu,omitempty"`

	// Memory threshold configuration
	// +optional
	Memory *ResourceLimit `json:"memory,omitempty"`
}

// ResourceLimit defines min and max for a resource
type ResourceLimit struct {
	// Min is the minimum allowed value (e.g., "10m", "64Mi")
	// +optional
	// +kubebuilder:default="10m"
	Min string `json:"min,omitempty"`

	// Max is the maximum allowed value (e.g., "16", "64Gi")
	// +optional
	// +kubebuilder:default="16"
	Max string `json:"max,omitempty"`
}

// RecommendationConfig defines how recommendations are calculated
type RecommendationConfig struct {
	// CPUPercentile is the percentile to use for CPU (50-99)
	// +optional
	// +kubebuilder:validation:Minimum=50
	// +kubebuilder:validation:Maximum=99
	// +kubebuilder:default=95
	CPUPercentile int `json:"cpuPercentile,omitempty"`

	// MemoryPercentile is the percentile to use for memory (50-99)
	// +optional
	// +kubebuilder:validation:Minimum=50
	// +kubebuilder:validation:Maximum=99
	// +kubebuilder:default=95
	MemoryPercentile int `json:"memoryPercentile,omitempty"`

	// MinSamples is the minimum number of samples required
	// +optional
	// +kubebuilder:validation:Minimum=10
	// +kubebuilder:default=100
	MinSamples int `json:"minSamples,omitempty"`

	// SafetyMargin is a multiplier for the recommendation (e.g., 1.2 = 20% buffer)
	// +optional
	// +kubebuilder:validation:Minimum=1.0
	// +kubebuilder:validation:Maximum=3.0
	// +kubebuilder:default=1.2
	SafetyMargin float64 `json:"safetyMargin,omitempty"`

	// HistoryDuration is how far back to look for metrics (e.g., "24h", "7d")
	// +optional
	// +kubebuilder:default="24h"
	HistoryDuration string `json:"historyDuration,omitempty"`
}

// UpdateStrategy defines how updates are applied
type UpdateStrategy struct {
	// Type is the update strategy type
	// +optional
	// +kubebuilder:validation:Enum=InPlace;RollingUpdate
	// +kubebuilder:default=InPlace
	Type UpdateStrategyType `json:"type,omitempty"`

	// RollingUpdate configuration (only used if Type=RollingUpdate)
	// +optional
	RollingUpdate *RollingUpdateConfig `json:"rollingUpdate,omitempty"`
}

// UpdateStrategyType defines the update strategy
// +kubebuilder:validation:Enum=InPlace;RollingUpdate
type UpdateStrategyType string

const (
	// UpdateStrategyInPlace updates resources in-place without pod restarts
	UpdateStrategyInPlace UpdateStrategyType = "InPlace"
	// UpdateStrategyRollingUpdate performs a rolling update
	UpdateStrategyRollingUpdate UpdateStrategyType = "RollingUpdate"
)

// RollingUpdateConfig defines rolling update parameters
type RollingUpdateConfig struct {
	// MaxUnavailable is the max number/percentage of pods that can be unavailable
	// +optional
	// +kubebuilder:default="25%"
	MaxUnavailable string `json:"maxUnavailable,omitempty"`

	// MaxSurge is the max number/percentage of pods that can be created above desired
	// +optional
	// +kubebuilder:default="25%"
	MaxSurge string `json:"maxSurge,omitempty"`
}

// HPAAwareness configures HPA conflict detection
type HPAAwareness struct {
	// Enabled controls whether to check for HPA conflicts
	// +optional
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// ConflictPolicy defines how to handle HPA conflicts
	// +optional
	// +kubebuilder:validation:Enum=Skip;Override;Warn
	// +kubebuilder:default=Skip
	ConflictPolicy HPAConflictPolicy `json:"conflictPolicy,omitempty"`
}

// HPAConflictPolicy defines how to handle HPA conflicts
// +kubebuilder:validation:Enum=Skip;Override;Warn
type HPAConflictPolicy string

const (
	// HPAConflictPolicySkip skips workloads that have HPAs
	HPAConflictPolicySkip HPAConflictPolicy = "Skip"
	// HPAConflictPolicyOverride overrides HPA settings (dangerous)
	HPAConflictPolicyOverride HPAConflictPolicy = "Override"
	// HPAConflictPolicyWarn logs warnings but continues
	HPAConflictPolicyWarn HPAConflictPolicy = "Warn"
)

// PDBAwareness configures PDB validation
type PDBAwareness struct {
	// Enabled controls whether to validate against PDBs
	// +optional
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// RespectMinAvailable controls whether to respect PDB minAvailable
	// +optional
	// +kubebuilder:default=true
	RespectMinAvailable bool `json:"respectMinAvailable,omitempty"`
}

// CircuitBreakerConfig defines circuit breaker parameters
type CircuitBreakerConfig struct {
	// Enabled controls whether the circuit breaker is active
	// +optional
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// ErrorThreshold is the number of consecutive errors before opening
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=20
	// +kubebuilder:default=5
	ErrorThreshold int `json:"errorThreshold,omitempty"`

	// SuccessThreshold is the number of consecutive successes to close
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=10
	// +kubebuilder:default=3
	SuccessThreshold int `json:"successThreshold,omitempty"`

	// Timeout is how long to wait before attempting half-open state
	// +optional
	// +kubebuilder:default="5m"
	Timeout string `json:"timeout,omitempty"`
}

// TargetResourceType defines which resource types to optimize
// +kubebuilder:validation:Enum=deployments;statefulsets;daemonsets
type TargetResourceType string

const (
	// TargetResourceDeployments targets Deployment resources
	TargetResourceDeployments TargetResourceType = "deployments"
	// TargetResourceStatefulSets targets StatefulSet resources
	TargetResourceStatefulSets TargetResourceType = "statefulsets"
	// TargetResourceDaemonSets targets DaemonSet resources
	TargetResourceDaemonSets TargetResourceType = "daemonsets"
)

// OptimizerConfigStatus defines the observed state of OptimizerConfig
type OptimizerConfigStatus struct {
	// Phase represents the current operational phase
	// +optional
	Phase OptimizerPhase `json:"phase,omitempty"`

	// ObservedGeneration is the generation observed by the controller
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// LastRecommendationTime is when the last recommendation was generated
	// +optional
	LastRecommendationTime *metav1.Time `json:"lastRecommendationTime,omitempty"`

	// LastUpdateTime is when the last update was applied
	// +optional
	LastUpdateTime *metav1.Time `json:"lastUpdateTime,omitempty"`

	// CircuitState is the current circuit breaker state
	// +optional
	// +kubebuilder:default=Closed
	CircuitState CircuitState `json:"circuitState,omitempty"`

	// ConsecutiveErrors tracks consecutive error count
	// +optional
	ConsecutiveErrors int `json:"consecutiveErrors,omitempty"`

	// ConsecutiveSuccesses tracks consecutive success count
	// +optional
	ConsecutiveSuccesses int `json:"consecutiveSuccesses,omitempty"`

	// TotalRecommendations is the total number of recommendations generated
	// +optional
	TotalRecommendations int64 `json:"totalRecommendations,omitempty"`

	// TotalUpdatesApplied is the total number of successful updates
	// +optional
	TotalUpdatesApplied int64 `json:"totalUpdatesApplied,omitempty"`

	// TotalUpdatesFailed is the total number of failed updates
	// +optional
	TotalUpdatesFailed int64 `json:"totalUpdatesFailed,omitempty"`

	// Conditions represent the latest available observations
	// +optional
	Conditions []OptimizerCondition `json:"conditions,omitempty"`

	// ActiveMaintenanceWindow indicates if currently in a maintenance window
	// +optional
	ActiveMaintenanceWindow bool `json:"activeMaintenanceWindow,omitempty"`

	// NextMaintenanceWindow is when the next maintenance window starts
	// +optional
	NextMaintenanceWindow *metav1.Time `json:"nextMaintenanceWindow,omitempty"`
}

// OptimizerPhase represents the current operational phase
// +kubebuilder:validation:Enum=Pending;Active;Paused;CircuitOpen;Error
type OptimizerPhase string

const (
	// OptimizerPhasePending means the optimizer is initializing
	OptimizerPhasePending OptimizerPhase = "Pending"
	// OptimizerPhaseActive means the optimizer is running normally
	OptimizerPhaseActive OptimizerPhase = "Active"
	// OptimizerPhasePaused means the optimizer is paused
	OptimizerPhasePaused OptimizerPhase = "Paused"
	// OptimizerPhaseCircuitOpen means circuit breaker is open
	OptimizerPhaseCircuitOpen OptimizerPhase = "CircuitOpen"
	// OptimizerPhaseError means the optimizer encountered an error
	OptimizerPhaseError OptimizerPhase = "Error"
)

// CircuitState represents the circuit breaker state
// +kubebuilder:validation:Enum=Closed;Open;HalfOpen
type CircuitState string

const (
	// CircuitStateClosed means circuit is closed (normal operation)
	CircuitStateClosed CircuitState = "Closed"
	// CircuitStateOpen means circuit is open (blocking operations)
	CircuitStateOpen CircuitState = "Open"
	// CircuitStateHalfOpen means circuit is testing if service recovered
	CircuitStateHalfOpen CircuitState = "HalfOpen"
)

// OptimizerCondition contains details for the current condition
type OptimizerCondition struct {
	// Type of the condition
	// +required
	Type OptimizerConditionType `json:"type"`

	// Status of the condition (True, False, Unknown)
	// +required
	Status ConditionStatus `json:"status"`

	// LastTransitionTime is the last time the condition transitioned
	// +required
	LastTransitionTime metav1.Time `json:"lastTransitionTime"`

	// Reason is a machine-readable reason for the condition's last transition
	// +optional
	Reason string `json:"reason,omitempty"`

	// Message is a human-readable message indicating details
	// +optional
	Message string `json:"message,omitempty"`
}

// OptimizerConditionType represents condition types
// +kubebuilder:validation:Enum=Ready;HPAConflict;PDBViolation;MaintenanceWindow;CircuitBreakerOpen;MetricsAvailable
type OptimizerConditionType string

const (
	// ConditionTypeReady indicates the optimizer is ready
	ConditionTypeReady OptimizerConditionType = "Ready"
	// ConditionTypeHPAConflict indicates an HPA conflict was detected
	ConditionTypeHPAConflict OptimizerConditionType = "HPAConflict"
	// ConditionTypePDBViolation indicates a PDB violation would occur
	ConditionTypePDBViolation OptimizerConditionType = "PDBViolation"
	// ConditionTypeMaintenanceWindow indicates currently in maintenance window
	ConditionTypeMaintenanceWindow OptimizerConditionType = "MaintenanceWindow"
	// ConditionTypeCircuitBreakerOpen indicates circuit breaker is open
	ConditionTypeCircuitBreakerOpen OptimizerConditionType = "CircuitBreakerOpen"
	// ConditionTypeMetricsAvailable indicates metrics are available
	ConditionTypeMetricsAvailable OptimizerConditionType = "MetricsAvailable"
)

// ConditionStatus represents the status of a condition
// +kubebuilder:validation:Enum=True;False;Unknown
type ConditionStatus string

const (
	// ConditionTrue means the condition is true
	ConditionTrue ConditionStatus = "True"
	// ConditionFalse means the condition is false
	ConditionFalse ConditionStatus = "False"
	// ConditionUnknown means the condition status is unknown
	ConditionUnknown ConditionStatus = "Unknown"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// OptimizerConfigList contains a list of OptimizerConfig
type OptimizerConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OptimizerConfig `json:"items"`
}
