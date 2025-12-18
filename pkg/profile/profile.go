package profile

import (
	"fmt"
	"time"
)

// EnvironmentType represents the type of environment
type EnvironmentType string

const (
	// EnvironmentProduction is for production workloads (conservative)
	EnvironmentProduction EnvironmentType = "production"
	// EnvironmentStaging is for staging/pre-prod workloads (balanced)
	EnvironmentStaging EnvironmentType = "staging"
	// EnvironmentDevelopment is for development workloads (aggressive)
	EnvironmentDevelopment EnvironmentType = "development"
	// EnvironmentTest is for test workloads (very aggressive)
	EnvironmentTest EnvironmentType = "test"
	// EnvironmentCustom allows fully custom configuration
	EnvironmentCustom EnvironmentType = "custom"
)

// Profile defines optimization behavior for a specific environment
type Profile struct {
	// Name is the profile identifier
	Name string

	// Environment is the type of environment this profile is for
	Environment EnvironmentType

	// Description explains the profile's purpose
	Description string

	// Settings contains the actual optimization parameters
	Settings ProfileSettings
}

// ProfileSettings contains all configurable optimization parameters
type ProfileSettings struct {
	// Strategy is the optimization strategy (aggressive, balanced, conservative)
	Strategy string

	// CPUPercentile is the percentile to use for CPU recommendations (50-99)
	CPUPercentile int

	// MemoryPercentile is the percentile to use for memory recommendations (50-99)
	MemoryPercentile int

	// SafetyMargin is the multiplier applied to recommendations (e.g., 1.2 = 20% buffer)
	SafetyMargin float64

	// MinSamples is the minimum samples required before making recommendations
	MinSamples int

	// HistoryDuration is how far back to look for metrics
	HistoryDuration time.Duration

	// MinConfidence is the minimum confidence score required to apply recommendations
	MinConfidence float64

	// ApplyDelay is how long to wait after generating a recommendation before applying
	ApplyDelay time.Duration

	// MaxChangePercent is the maximum allowed change per update (0 = unlimited)
	MaxChangePercent float64

	// RequireApproval indicates whether manual approval is needed before applying
	RequireApproval bool

	// RollbackOnError indicates whether to automatically rollback on errors
	RollbackOnError bool

	// CircuitBreakerEnabled controls circuit breaker activation
	CircuitBreakerEnabled bool

	// CircuitBreakerThreshold is the error count before opening circuit
	CircuitBreakerThreshold int

	// DryRunByDefault starts in dry-run mode
	DryRunByDefault bool

	// MinResourceLimits defines minimum resource values
	MinCPUMillicores    int64
	MinMemoryMegabytes  int64

	// MaxResourceLimits defines maximum resource values
	MaxCPUMillicores    int64
	MaxMemoryMegabytes  int64
}

// ProfileManager manages environment profiles
type ProfileManager struct {
	profiles map[string]*Profile
}

// NewProfileManager creates a new profile manager with default profiles
func NewProfileManager() *ProfileManager {
	pm := &ProfileManager{
		profiles: make(map[string]*Profile),
	}

	// Register default profiles
	pm.RegisterProfile(ProductionProfile())
	pm.RegisterProfile(StagingProfile())
	pm.RegisterProfile(DevelopmentProfile())
	pm.RegisterProfile(TestProfile())

	return pm
}

// RegisterProfile registers a profile
func (pm *ProfileManager) RegisterProfile(p *Profile) {
	pm.profiles[p.Name] = p
}

// GetProfile returns a profile by name
func (pm *ProfileManager) GetProfile(name string) (*Profile, error) {
	if p, ok := pm.profiles[name]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("profile not found: %s", name)
}

// GetProfileByEnvironment returns the default profile for an environment type
func (pm *ProfileManager) GetProfileByEnvironment(env EnvironmentType) (*Profile, error) {
	for _, p := range pm.profiles {
		if p.Environment == env {
			return p, nil
		}
	}
	return nil, fmt.Errorf("no profile found for environment: %s", env)
}

// ListProfiles returns all registered profiles
func (pm *ProfileManager) ListProfiles() []*Profile {
	profiles := make([]*Profile, 0, len(pm.profiles))
	for _, p := range pm.profiles {
		profiles = append(profiles, p)
	}
	return profiles
}

// ProductionProfile returns the production environment profile (Conservative)
func ProductionProfile() *Profile {
	return &Profile{
		Name:        "production",
		Environment: EnvironmentProduction,
		Description: "Conservative optimization for production workloads. Prioritizes stability over cost savings.",
		Settings: ProfileSettings{
			Strategy:         "conservative",
			CPUPercentile:    99, // Use P99 for safety
			MemoryPercentile: 99,
			SafetyMargin:     1.4, // 40% buffer
			MinSamples:       200,
			HistoryDuration:  7 * 24 * time.Hour, // 1 week of data
			MinConfidence:    80.0,               // High confidence required
			ApplyDelay:       24 * time.Hour,     // Wait 24h before applying
			MaxChangePercent: 20.0,               // Max 20% change at a time
			RequireApproval:  true,               // Require manual approval
			RollbackOnError:  true,

			CircuitBreakerEnabled:   true,
			CircuitBreakerThreshold: 2, // Open after 2 errors

			DryRunByDefault: true, // Start in dry-run

			MinCPUMillicores:   100,           // 100m minimum
			MinMemoryMegabytes: 128,           // 128Mi minimum
			MaxCPUMillicores:   16000,         // 16 cores max
			MaxMemoryMegabytes: 64 * 1024,     // 64Gi max
		},
	}
}

// StagingProfile returns the staging environment profile (Balanced)
func StagingProfile() *Profile {
	return &Profile{
		Name:        "staging",
		Environment: EnvironmentStaging,
		Description: "Balanced optimization for staging/pre-production workloads.",
		Settings: ProfileSettings{
			Strategy:         "balanced",
			CPUPercentile:    95, // P95
			MemoryPercentile: 95,
			SafetyMargin:     1.2, // 20% buffer
			MinSamples:       100,
			HistoryDuration:  3 * 24 * time.Hour, // 3 days of data
			MinConfidence:    60.0,
			ApplyDelay:       4 * time.Hour, // Wait 4h before applying
			MaxChangePercent: 30.0,          // Max 30% change
			RequireApproval:  false,
			RollbackOnError:  true,

			CircuitBreakerEnabled:   true,
			CircuitBreakerThreshold: 3,

			DryRunByDefault: false,

			MinCPUMillicores:   50,            // 50m minimum
			MinMemoryMegabytes: 64,            // 64Mi minimum
			MaxCPUMillicores:   8000,          // 8 cores max
			MaxMemoryMegabytes: 32 * 1024,     // 32Gi max
		},
	}
}

// DevelopmentProfile returns the development environment profile (Aggressive)
func DevelopmentProfile() *Profile {
	return &Profile{
		Name:        "development",
		Environment: EnvironmentDevelopment,
		Description: "Aggressive optimization for development workloads. Prioritizes cost savings.",
		Settings: ProfileSettings{
			Strategy:         "aggressive",
			CPUPercentile:    90, // P90
			MemoryPercentile: 90,
			SafetyMargin:     1.1, // 10% buffer
			MinSamples:       50,
			HistoryDuration:  24 * time.Hour, // 1 day of data
			MinConfidence:    40.0,           // Lower confidence OK
			ApplyDelay:       1 * time.Hour,  // Wait 1h before applying
			MaxChangePercent: 50.0,           // Allow larger changes
			RequireApproval:  false,
			RollbackOnError:  true,

			CircuitBreakerEnabled:   true,
			CircuitBreakerThreshold: 5,

			DryRunByDefault: false,

			MinCPUMillicores:   10,            // 10m minimum
			MinMemoryMegabytes: 32,            // 32Mi minimum
			MaxCPUMillicores:   4000,          // 4 cores max
			MaxMemoryMegabytes: 16 * 1024,     // 16Gi max
		},
	}
}

// TestProfile returns the test environment profile (Very Aggressive)
func TestProfile() *Profile {
	return &Profile{
		Name:        "test",
		Environment: EnvironmentTest,
		Description: "Very aggressive optimization for test/ephemeral workloads. Maximum cost savings.",
		Settings: ProfileSettings{
			Strategy:         "aggressive",
			CPUPercentile:    80, // P80 - more aggressive
			MemoryPercentile: 85,
			SafetyMargin:     1.05, // 5% buffer only
			MinSamples:       20,
			HistoryDuration:  6 * time.Hour, // Just 6 hours
			MinConfidence:    20.0,          // Very low confidence OK
			ApplyDelay:       15 * time.Minute,
			MaxChangePercent: 0, // No limit
			RequireApproval:  false,
			RollbackOnError:  false, // Don't bother rolling back tests

			CircuitBreakerEnabled:   false, // No circuit breaker
			CircuitBreakerThreshold: 10,

			DryRunByDefault: false,

			MinCPUMillicores:   5,             // 5m minimum
			MinMemoryMegabytes: 16,            // 16Mi minimum
			MaxCPUMillicores:   2000,          // 2 cores max
			MaxMemoryMegabytes: 8 * 1024,      // 8Gi max
		},
	}
}

// CustomProfile creates a custom profile with provided settings
func CustomProfile(name, description string, settings ProfileSettings) *Profile {
	return &Profile{
		Name:        name,
		Environment: EnvironmentCustom,
		Description: description,
		Settings:    settings,
	}
}

// Merge merges profile settings with overrides (overrides take precedence)
func (s *ProfileSettings) Merge(overrides *ProfileSettings) ProfileSettings {
	result := *s // Copy base settings

	if overrides == nil {
		return result
	}

	// Only override non-zero values
	if overrides.Strategy != "" {
		result.Strategy = overrides.Strategy
	}
	if overrides.CPUPercentile > 0 {
		result.CPUPercentile = overrides.CPUPercentile
	}
	if overrides.MemoryPercentile > 0 {
		result.MemoryPercentile = overrides.MemoryPercentile
	}
	if overrides.SafetyMargin > 0 {
		result.SafetyMargin = overrides.SafetyMargin
	}
	if overrides.MinSamples > 0 {
		result.MinSamples = overrides.MinSamples
	}
	if overrides.HistoryDuration > 0 {
		result.HistoryDuration = overrides.HistoryDuration
	}
	if overrides.MinConfidence > 0 {
		result.MinConfidence = overrides.MinConfidence
	}
	if overrides.ApplyDelay > 0 {
		result.ApplyDelay = overrides.ApplyDelay
	}
	if overrides.MaxChangePercent > 0 {
		result.MaxChangePercent = overrides.MaxChangePercent
	}
	if overrides.CircuitBreakerThreshold > 0 {
		result.CircuitBreakerThreshold = overrides.CircuitBreakerThreshold
	}
	if overrides.MinCPUMillicores > 0 {
		result.MinCPUMillicores = overrides.MinCPUMillicores
	}
	if overrides.MinMemoryMegabytes > 0 {
		result.MinMemoryMegabytes = overrides.MinMemoryMegabytes
	}
	if overrides.MaxCPUMillicores > 0 {
		result.MaxCPUMillicores = overrides.MaxCPUMillicores
	}
	if overrides.MaxMemoryMegabytes > 0 {
		result.MaxMemoryMegabytes = overrides.MaxMemoryMegabytes
	}

	// Boolean fields - check explicitly as false is valid
	result.RequireApproval = overrides.RequireApproval
	result.RollbackOnError = overrides.RollbackOnError
	result.CircuitBreakerEnabled = overrides.CircuitBreakerEnabled
	result.DryRunByDefault = overrides.DryRunByDefault

	return result
}

// Validate validates profile settings
func (s *ProfileSettings) Validate() error {
	if s.CPUPercentile < 50 || s.CPUPercentile > 99 {
		return fmt.Errorf("CPUPercentile must be between 50 and 99, got %d", s.CPUPercentile)
	}
	if s.MemoryPercentile < 50 || s.MemoryPercentile > 99 {
		return fmt.Errorf("MemoryPercentile must be between 50 and 99, got %d", s.MemoryPercentile)
	}
	if s.SafetyMargin < 1.0 || s.SafetyMargin > 3.0 {
		return fmt.Errorf("SafetyMargin must be between 1.0 and 3.0, got %.2f", s.SafetyMargin)
	}
	if s.MinSamples < 1 {
		return fmt.Errorf("MinSamples must be at least 1, got %d", s.MinSamples)
	}
	if s.MinConfidence < 0 || s.MinConfidence > 100 {
		return fmt.Errorf("MinConfidence must be between 0 and 100, got %.2f", s.MinConfidence)
	}
	if s.MaxChangePercent < 0 {
		return fmt.Errorf("MaxChangePercent must be non-negative, got %.2f", s.MaxChangePercent)
	}

	validStrategies := map[string]bool{
		"aggressive":   true,
		"balanced":     true,
		"conservative": true,
	}
	if !validStrategies[s.Strategy] {
		return fmt.Errorf("invalid strategy: %s", s.Strategy)
	}

	return nil
}

// String returns a human-readable summary of the profile
func (p *Profile) String() string {
	return fmt.Sprintf("Profile[%s] (%s): %s - Strategy=%s, SafetyMargin=%.0f%%, P%d/P%d CPU/Memory",
		p.Name, p.Environment, p.Description,
		p.Settings.Strategy,
		(p.Settings.SafetyMargin-1)*100,
		p.Settings.CPUPercentile,
		p.Settings.MemoryPercentile)
}

// Summary returns a brief summary for logging
func (p *Profile) Summary() string {
	return fmt.Sprintf("%s (%s): %s, P%d/%dP, %.0f%% margin",
		p.Name,
		p.Settings.Strategy,
		p.Environment,
		p.Settings.CPUPercentile,
		p.Settings.MemoryPercentile,
		(p.Settings.SafetyMargin-1)*100)
}
