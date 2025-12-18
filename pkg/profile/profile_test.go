package profile

import (
	"testing"
	"time"
)

func TestProductionProfile(t *testing.T) {
	p := ProductionProfile()

	if p.Name != "production" {
		t.Errorf("expected name 'production', got %s", p.Name)
	}
	if p.Environment != EnvironmentProduction {
		t.Errorf("expected environment EnvironmentProduction, got %s", p.Environment)
	}
	if p.Settings.Strategy != "conservative" {
		t.Errorf("expected strategy 'conservative', got %s", p.Settings.Strategy)
	}

	// Production should have high safety margins
	if p.Settings.SafetyMargin < 1.3 {
		t.Errorf("production safety margin should be >= 1.3, got %.2f", p.Settings.SafetyMargin)
	}
	if p.Settings.CPUPercentile < 95 {
		t.Errorf("production CPU percentile should be >= 95, got %d", p.Settings.CPUPercentile)
	}
	if p.Settings.MemoryPercentile < 95 {
		t.Errorf("production memory percentile should be >= 95, got %d", p.Settings.MemoryPercentile)
	}
	if p.Settings.MinConfidence < 70 {
		t.Errorf("production min confidence should be >= 70, got %.2f", p.Settings.MinConfidence)
	}
	if !p.Settings.RequireApproval {
		t.Error("production should require approval")
	}
	if !p.Settings.DryRunByDefault {
		t.Error("production should default to dry-run")
	}
	if p.Settings.CircuitBreakerThreshold > 3 {
		t.Errorf("production circuit breaker threshold should be <= 3, got %d", p.Settings.CircuitBreakerThreshold)
	}
}

func TestDevelopmentProfile(t *testing.T) {
	p := DevelopmentProfile()

	if p.Name != "development" {
		t.Errorf("expected name 'development', got %s", p.Name)
	}
	if p.Environment != EnvironmentDevelopment {
		t.Errorf("expected environment EnvironmentDevelopment, got %s", p.Environment)
	}
	if p.Settings.Strategy != "aggressive" {
		t.Errorf("expected strategy 'aggressive', got %s", p.Settings.Strategy)
	}

	// Development should have lower safety margins
	if p.Settings.SafetyMargin > 1.2 {
		t.Errorf("development safety margin should be <= 1.2, got %.2f", p.Settings.SafetyMargin)
	}
	if p.Settings.CPUPercentile > 95 {
		t.Errorf("development CPU percentile should be <= 95, got %d", p.Settings.CPUPercentile)
	}
	if p.Settings.MinConfidence > 50 {
		t.Errorf("development min confidence should be <= 50, got %.2f", p.Settings.MinConfidence)
	}
	if p.Settings.RequireApproval {
		t.Error("development should not require approval")
	}
	if p.Settings.DryRunByDefault {
		t.Error("development should not default to dry-run")
	}
}

func TestStagingProfile(t *testing.T) {
	p := StagingProfile()

	if p.Name != "staging" {
		t.Errorf("expected name 'staging', got %s", p.Name)
	}
	if p.Settings.Strategy != "balanced" {
		t.Errorf("expected strategy 'balanced', got %s", p.Settings.Strategy)
	}

	// Staging should be between production and development
	prod := ProductionProfile()
	dev := DevelopmentProfile()

	if p.Settings.SafetyMargin >= prod.Settings.SafetyMargin {
		t.Errorf("staging safety margin should be < production")
	}
	if p.Settings.SafetyMargin <= dev.Settings.SafetyMargin {
		t.Errorf("staging safety margin should be > development")
	}
}

func TestTestProfile(t *testing.T) {
	p := TestProfile()

	if p.Name != "test" {
		t.Errorf("expected name 'test', got %s", p.Name)
	}
	if p.Environment != EnvironmentTest {
		t.Errorf("expected environment EnvironmentTest, got %s", p.Environment)
	}
	if p.Settings.Strategy != "aggressive" {
		t.Errorf("expected strategy 'aggressive', got %s", p.Settings.Strategy)
	}

	// Test profile should be more aggressive than development
	dev := DevelopmentProfile()
	if p.Settings.SafetyMargin >= dev.Settings.SafetyMargin {
		t.Errorf("test safety margin should be < development")
	}
	if p.Settings.MinSamples >= dev.Settings.MinSamples {
		t.Errorf("test min samples should be < development")
	}
	if !dev.Settings.CircuitBreakerEnabled && p.Settings.CircuitBreakerEnabled {
		t.Error("test should have circuit breaker disabled if development has it disabled")
	}
}

func TestProfileManager(t *testing.T) {
	pm := NewProfileManager()

	t.Run("get production profile", func(t *testing.T) {
		p, err := pm.GetProfile("production")
		if err != nil {
			t.Fatalf("failed to get production profile: %v", err)
		}
		if p.Name != "production" {
			t.Errorf("expected production, got %s", p.Name)
		}
	})

	t.Run("get development profile", func(t *testing.T) {
		p, err := pm.GetProfile("development")
		if err != nil {
			t.Fatalf("failed to get development profile: %v", err)
		}
		if p.Name != "development" {
			t.Errorf("expected development, got %s", p.Name)
		}
	})

	t.Run("get nonexistent profile", func(t *testing.T) {
		_, err := pm.GetProfile("nonexistent")
		if err == nil {
			t.Error("expected error for nonexistent profile")
		}
	})

	t.Run("get by environment", func(t *testing.T) {
		p, err := pm.GetProfileByEnvironment(EnvironmentProduction)
		if err != nil {
			t.Fatalf("failed to get profile by environment: %v", err)
		}
		if p.Environment != EnvironmentProduction {
			t.Errorf("expected production environment, got %s", p.Environment)
		}
	})

	t.Run("list profiles", func(t *testing.T) {
		profiles := pm.ListProfiles()
		if len(profiles) < 4 {
			t.Errorf("expected at least 4 profiles, got %d", len(profiles))
		}
	})

	t.Run("register custom profile", func(t *testing.T) {
		custom := CustomProfile("my-custom", "My custom profile", ProfileSettings{
			Strategy:         "balanced",
			CPUPercentile:    93,
			MemoryPercentile: 93,
			SafetyMargin:     1.25,
			MinSamples:       75,
			HistoryDuration:  48 * time.Hour,
			MinConfidence:    55.0,
		})
		pm.RegisterProfile(custom)

		p, err := pm.GetProfile("my-custom")
		if err != nil {
			t.Fatalf("failed to get custom profile: %v", err)
		}
		if p.Settings.CPUPercentile != 93 {
			t.Errorf("expected CPU percentile 93, got %d", p.Settings.CPUPercentile)
		}
	})
}

func TestProfileSettingsMerge(t *testing.T) {
	base := ProfileSettings{
		Strategy:         "balanced",
		CPUPercentile:    95,
		MemoryPercentile: 95,
		SafetyMargin:     1.2,
		MinSamples:       100,
		HistoryDuration:  24 * time.Hour,
		MinConfidence:    60.0,
		RequireApproval:  false,
	}

	t.Run("merge with nil", func(t *testing.T) {
		result := base.Merge(nil)
		if result.CPUPercentile != 95 {
			t.Errorf("expected CPU percentile 95, got %d", result.CPUPercentile)
		}
	})

	t.Run("merge with overrides", func(t *testing.T) {
		overrides := &ProfileSettings{
			CPUPercentile:   99,
			MinConfidence:   80.0,
			RequireApproval: true,
		}
		result := base.Merge(overrides)

		if result.CPUPercentile != 99 {
			t.Errorf("expected CPU percentile 99, got %d", result.CPUPercentile)
		}
		if result.MemoryPercentile != 95 {
			t.Errorf("expected memory percentile 95 (unchanged), got %d", result.MemoryPercentile)
		}
		if result.MinConfidence != 80.0 {
			t.Errorf("expected min confidence 80, got %.2f", result.MinConfidence)
		}
		if !result.RequireApproval {
			t.Error("expected require approval to be true")
		}
	})
}

func TestProfileSettingsValidate(t *testing.T) {
	tests := []struct {
		name        string
		settings    ProfileSettings
		expectError bool
	}{
		{
			name: "valid settings",
			settings: ProfileSettings{
				Strategy:         "balanced",
				CPUPercentile:    95,
				MemoryPercentile: 95,
				SafetyMargin:     1.2,
				MinSamples:       10,
				MinConfidence:    50.0,
			},
			expectError: false,
		},
		{
			name: "invalid CPU percentile (too low)",
			settings: ProfileSettings{
				Strategy:         "balanced",
				CPUPercentile:    49,
				MemoryPercentile: 95,
				SafetyMargin:     1.2,
				MinSamples:       10,
				MinConfidence:    50.0,
			},
			expectError: true,
		},
		{
			name: "invalid CPU percentile (too high)",
			settings: ProfileSettings{
				Strategy:         "balanced",
				CPUPercentile:    100,
				MemoryPercentile: 95,
				SafetyMargin:     1.2,
				MinSamples:       10,
				MinConfidence:    50.0,
			},
			expectError: true,
		},
		{
			name: "invalid safety margin (too low)",
			settings: ProfileSettings{
				Strategy:         "balanced",
				CPUPercentile:    95,
				MemoryPercentile: 95,
				SafetyMargin:     0.9,
				MinSamples:       10,
				MinConfidence:    50.0,
			},
			expectError: true,
		},
		{
			name: "invalid safety margin (too high)",
			settings: ProfileSettings{
				Strategy:         "balanced",
				CPUPercentile:    95,
				MemoryPercentile: 95,
				SafetyMargin:     3.5,
				MinSamples:       10,
				MinConfidence:    50.0,
			},
			expectError: true,
		},
		{
			name: "invalid min samples",
			settings: ProfileSettings{
				Strategy:         "balanced",
				CPUPercentile:    95,
				MemoryPercentile: 95,
				SafetyMargin:     1.2,
				MinSamples:       0,
				MinConfidence:    50.0,
			},
			expectError: true,
		},
		{
			name: "invalid strategy",
			settings: ProfileSettings{
				Strategy:         "invalid",
				CPUPercentile:    95,
				MemoryPercentile: 95,
				SafetyMargin:     1.2,
				MinSamples:       10,
				MinConfidence:    50.0,
			},
			expectError: true,
		},
		{
			name: "invalid min confidence (negative)",
			settings: ProfileSettings{
				Strategy:         "balanced",
				CPUPercentile:    95,
				MemoryPercentile: 95,
				SafetyMargin:     1.2,
				MinSamples:       10,
				MinConfidence:    -10.0,
			},
			expectError: true,
		},
		{
			name: "invalid min confidence (too high)",
			settings: ProfileSettings{
				Strategy:         "balanced",
				CPUPercentile:    95,
				MemoryPercentile: 95,
				SafetyMargin:     1.2,
				MinSamples:       10,
				MinConfidence:    110.0,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.settings.Validate()
			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestProfileString(t *testing.T) {
	p := ProductionProfile()
	s := p.String()

	if s == "" {
		t.Error("expected non-empty string")
	}
	if !containsSubstring(s, "production") {
		t.Error("expected string to contain 'production'")
	}
	if !containsSubstring(s, "conservative") {
		t.Error("expected string to contain 'conservative'")
	}
}

func TestProfileSummary(t *testing.T) {
	p := DevelopmentProfile()
	s := p.Summary()

	if s == "" {
		t.Error("expected non-empty summary")
	}
	if !containsSubstring(s, "development") {
		t.Error("expected summary to contain 'development'")
	}
}

// Helper function
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
