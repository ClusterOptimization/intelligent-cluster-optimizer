package webhook

import (
	"testing"

	optimizerv1alpha1 "intelligent-cluster-optimizer/pkg/apis/optimizer/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestValidator_ValidateTargetNamespaces(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name        string
		namespaces  []string
		shouldError bool
	}{
		{
			name:        "valid single namespace",
			namespaces:  []string{"default"},
			shouldError: false,
		},
		{
			name:        "valid multiple namespaces",
			namespaces:  []string{"default", "kube-system", "prod-app"},
			shouldError: false,
		},
		{
			name:        "empty namespace list",
			namespaces:  []string{},
			shouldError: true,
		},
		{
			name:        "empty namespace string",
			namespaces:  []string{"default", ""},
			shouldError: true,
		},
		{
			name:        "invalid uppercase namespace",
			namespaces:  []string{"Default"},
			shouldError: true,
		},
		{
			name:        "invalid underscore in namespace",
			namespaces:  []string{"my_namespace"},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &optimizerv1alpha1.OptimizerConfig{
				Spec: optimizerv1alpha1.OptimizerConfigSpec{
					TargetNamespaces: tt.namespaces,
				},
			}

			err := validator.ValidateCreate(config)
			if tt.shouldError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestValidator_ValidateMaintenanceWindows(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name        string
		windows     []optimizerv1alpha1.MaintenanceWindow
		shouldError bool
	}{
		{
			name: "valid maintenance window",
			windows: []optimizerv1alpha1.MaintenanceWindow{
				{
					Schedule: "0 2 * * *", // 2 AM every day
					Duration: "2h",
					Timezone: "UTC",
				},
			},
			shouldError: false,
		},
		{
			name: "valid with timezone",
			windows: []optimizerv1alpha1.MaintenanceWindow{
				{
					Schedule: "0 0 * * 6", // Midnight on Saturdays
					Duration: "4h",
					Timezone: "America/New_York",
				},
			},
			shouldError: false,
		},
		{
			name: "invalid cron expression",
			windows: []optimizerv1alpha1.MaintenanceWindow{
				{
					Schedule: "invalid cron",
					Duration: "2h",
				},
			},
			shouldError: true,
		},
		{
			name: "invalid duration format",
			windows: []optimizerv1alpha1.MaintenanceWindow{
				{
					Schedule: "0 2 * * *",
					Duration: "2 hours", // Should be "2h"
				},
			},
			shouldError: true,
		},
		{
			name: "negative duration",
			windows: []optimizerv1alpha1.MaintenanceWindow{
				{
					Schedule: "0 2 * * *",
					Duration: "-1h",
				},
			},
			shouldError: true,
		},
		{
			name: "invalid timezone",
			windows: []optimizerv1alpha1.MaintenanceWindow{
				{
					Schedule: "0 2 * * *",
					Duration: "2h",
					Timezone: "Invalid/Timezone",
				},
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &optimizerv1alpha1.OptimizerConfig{
				Spec: optimizerv1alpha1.OptimizerConfigSpec{
					TargetNamespaces:   []string{"default"},
					MaintenanceWindows: tt.windows,
				},
			}

			err := validator.ValidateCreate(config)
			if tt.shouldError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestValidator_ValidateResourceThresholds(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name        string
		thresholds  *optimizerv1alpha1.ResourceThresholds
		shouldError bool
	}{
		{
			name: "valid CPU and memory thresholds",
			thresholds: &optimizerv1alpha1.ResourceThresholds{
				CPU: &optimizerv1alpha1.ResourceLimit{
					Min: "10m",
					Max: "4",
				},
				Memory: &optimizerv1alpha1.ResourceLimit{
					Min: "64Mi",
					Max: "8Gi",
				},
			},
			shouldError: false,
		},
		{
			name: "invalid CPU format",
			thresholds: &optimizerv1alpha1.ResourceThresholds{
				CPU: &optimizerv1alpha1.ResourceLimit{
					Min: "10cores", // Invalid format
					Max: "4",
				},
			},
			shouldError: true,
		},
		{
			name: "min greater than max",
			thresholds: &optimizerv1alpha1.ResourceThresholds{
				CPU: &optimizerv1alpha1.ResourceLimit{
					Min: "8",
					Max: "4",
				},
			},
			shouldError: true,
		},
		{
			name: "negative min value",
			thresholds: &optimizerv1alpha1.ResourceThresholds{
				Memory: &optimizerv1alpha1.ResourceLimit{
					Min: "-100Mi",
					Max: "1Gi",
				},
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &optimizerv1alpha1.OptimizerConfig{
				Spec: optimizerv1alpha1.OptimizerConfigSpec{
					TargetNamespaces:   []string{"default"},
					ResourceThresholds: tt.thresholds,
				},
			}

			err := validator.ValidateCreate(config)
			if tt.shouldError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestValidator_ValidateRecommendationConfig(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name        string
		config      *optimizerv1alpha1.RecommendationConfig
		shouldError bool
	}{
		{
			name: "valid recommendation config",
			config: &optimizerv1alpha1.RecommendationConfig{
				CPUPercentile:    95,
				MemoryPercentile: 90,
				MinSamples:       100,
				SafetyMargin:     1.2,
				HistoryDuration:  "24h",
			},
			shouldError: false,
		},
		{
			name: "invalid CPU percentile too low",
			config: &optimizerv1alpha1.RecommendationConfig{
				CPUPercentile: 40,
			},
			shouldError: true,
		},
		{
			name: "invalid memory percentile too high",
			config: &optimizerv1alpha1.RecommendationConfig{
				MemoryPercentile: 100,
			},
			shouldError: true,
		},
		{
			name: "invalid min samples",
			config: &optimizerv1alpha1.RecommendationConfig{
				MinSamples: 5,
			},
			shouldError: true,
		},
		{
			name: "invalid safety margin too low",
			config: &optimizerv1alpha1.RecommendationConfig{
				SafetyMargin: 0.5,
			},
			shouldError: true,
		},
		{
			name: "invalid safety margin too high",
			config: &optimizerv1alpha1.RecommendationConfig{
				SafetyMargin: 5.0,
			},
			shouldError: true,
		},
		{
			name: "invalid history duration format",
			config: &optimizerv1alpha1.RecommendationConfig{
				HistoryDuration: "24 hours",
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &optimizerv1alpha1.OptimizerConfig{
				Spec: optimizerv1alpha1.OptimizerConfigSpec{
					TargetNamespaces: []string{"default"},
					Recommendations:  tt.config,
				},
			}

			err := validator.ValidateCreate(config)
			if tt.shouldError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestValidator_ValidateProfileOverrides(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name        string
		overrides   *optimizerv1alpha1.ProfileOverrides
		shouldError bool
	}{
		{
			name: "valid profile overrides",
			overrides: &optimizerv1alpha1.ProfileOverrides{
				MinConfidence:    ptrFloat64(80.0),
				MaxChangePercent: ptrFloat64(25.0),
				ApplyDelay:       "1h",
			},
			shouldError: false,
		},
		{
			name: "invalid min confidence too high",
			overrides: &optimizerv1alpha1.ProfileOverrides{
				MinConfidence: ptrFloat64(150.0),
			},
			shouldError: true,
		},
		{
			name: "invalid max change percent negative",
			overrides: &optimizerv1alpha1.ProfileOverrides{
				MaxChangePercent: ptrFloat64(-10.0),
			},
			shouldError: true,
		},
		{
			name: "invalid apply delay format",
			overrides: &optimizerv1alpha1.ProfileOverrides{
				ApplyDelay: "one hour",
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &optimizerv1alpha1.OptimizerConfig{
				Spec: optimizerv1alpha1.OptimizerConfigSpec{
					TargetNamespaces: []string{"default"},
					ProfileOverrides: tt.overrides,
				},
			}

			err := validator.ValidateCreate(config)
			if tt.shouldError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestValidator_ValidateUpdateStrategy(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name        string
		strategy    *optimizerv1alpha1.UpdateStrategy
		shouldError bool
	}{
		{
			name: "valid rolling update with percentage",
			strategy: &optimizerv1alpha1.UpdateStrategy{
				Type: optimizerv1alpha1.UpdateStrategyRollingUpdate,
				RollingUpdate: &optimizerv1alpha1.RollingUpdateConfig{
					MaxUnavailable: "25%",
					MaxSurge:       "25%",
				},
			},
			shouldError: false,
		},
		{
			name: "valid rolling update with number",
			strategy: &optimizerv1alpha1.UpdateStrategy{
				Type: optimizerv1alpha1.UpdateStrategyRollingUpdate,
				RollingUpdate: &optimizerv1alpha1.RollingUpdateConfig{
					MaxUnavailable: "1",
					MaxSurge:       "2",
				},
			},
			shouldError: false,
		},
		{
			name: "invalid percentage too high",
			strategy: &optimizerv1alpha1.UpdateStrategy{
				Type: optimizerv1alpha1.UpdateStrategyRollingUpdate,
				RollingUpdate: &optimizerv1alpha1.RollingUpdateConfig{
					MaxUnavailable: "150%",
				},
			},
			shouldError: true,
		},
		{
			name: "invalid format",
			strategy: &optimizerv1alpha1.UpdateStrategy{
				Type: optimizerv1alpha1.UpdateStrategyRollingUpdate,
				RollingUpdate: &optimizerv1alpha1.RollingUpdateConfig{
					MaxSurge: "invalid",
				},
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &optimizerv1alpha1.OptimizerConfig{
				Spec: optimizerv1alpha1.OptimizerConfigSpec{
					TargetNamespaces: []string{"default"},
					UpdateStrategy:   tt.strategy,
				},
			}

			err := validator.ValidateCreate(config)
			if tt.shouldError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestValidator_ValidateCircuitBreaker(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name        string
		cb          *optimizerv1alpha1.CircuitBreakerConfig
		shouldError bool
	}{
		{
			name: "valid circuit breaker",
			cb: &optimizerv1alpha1.CircuitBreakerConfig{
				Enabled:          true,
				ErrorThreshold:   5,
				SuccessThreshold: 3,
				Timeout:          "5m",
			},
			shouldError: false,
		},
		{
			name: "invalid error threshold too high",
			cb: &optimizerv1alpha1.CircuitBreakerConfig{
				ErrorThreshold: 25,
			},
			shouldError: true,
		},
		{
			name: "invalid success threshold too high",
			cb: &optimizerv1alpha1.CircuitBreakerConfig{
				SuccessThreshold: 15,
			},
			shouldError: true,
		},
		{
			name: "invalid timeout format",
			cb: &optimizerv1alpha1.CircuitBreakerConfig{
				Timeout: "5 minutes",
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &optimizerv1alpha1.OptimizerConfig{
				Spec: optimizerv1alpha1.OptimizerConfigSpec{
					TargetNamespaces: []string{"default"},
					CircuitBreaker:   tt.cb,
				},
			}

			err := validator.ValidateCreate(config)
			if tt.shouldError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestValidator_ValidateGitOpsExport(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name        string
		gitops      *optimizerv1alpha1.GitOpsExportConfig
		shouldError bool
	}{
		{
			name: "valid gitops without auto commit",
			gitops: &optimizerv1alpha1.GitOpsExportConfig{
				Enabled:    true,
				Format:     optimizerv1alpha1.GitOpsFormatKustomize,
				OutputPath: "./gitops-exports",
			},
			shouldError: false,
		},
		{
			name: "valid gitops with auto commit",
			gitops: &optimizerv1alpha1.GitOpsExportConfig{
				Enabled:    true,
				AutoCommit: true,
				OutputPath: "./gitops",
				GitRepository: &optimizerv1alpha1.GitRepositoryConfig{
					URL:    "https://github.com/user/repo.git",
					Branch: "main",
				},
			},
			shouldError: false,
		},
		{
			name: "valid gitops with SSH URL",
			gitops: &optimizerv1alpha1.GitOpsExportConfig{
				Enabled:    true,
				AutoCommit: true,
				OutputPath: "./gitops",
				GitRepository: &optimizerv1alpha1.GitRepositoryConfig{
					URL: "git@github.com:user/repo.git",
				},
			},
			shouldError: false,
		},
		{
			name: "empty output path",
			gitops: &optimizerv1alpha1.GitOpsExportConfig{
				Enabled:    true,
				OutputPath: "",
			},
			shouldError: true,
		},
		{
			name: "auto commit without git repository",
			gitops: &optimizerv1alpha1.GitOpsExportConfig{
				Enabled:    true,
				AutoCommit: true,
				OutputPath: "./gitops",
			},
			shouldError: true,
		},
		{
			name: "invalid git URL",
			gitops: &optimizerv1alpha1.GitOpsExportConfig{
				Enabled:    true,
				AutoCommit: true,
				OutputPath: "./gitops",
				GitRepository: &optimizerv1alpha1.GitRepositoryConfig{
					URL: "not-a-valid-url",
				},
			},
			shouldError: true,
		},
		{
			name: "create PR without title",
			gitops: &optimizerv1alpha1.GitOpsExportConfig{
				Enabled:    true,
				AutoCommit: true,
				OutputPath: "./gitops",
				GitRepository: &optimizerv1alpha1.GitRepositoryConfig{
					URL:      "https://github.com/user/repo.git",
					CreatePR: true,
					PRTitle:  "",
				},
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &optimizerv1alpha1.OptimizerConfig{
				Spec: optimizerv1alpha1.OptimizerConfigSpec{
					TargetNamespaces: []string{"default"},
					GitOpsExport:     tt.gitops,
				},
			}

			err := validator.ValidateCreate(config)
			if tt.shouldError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestValidator_ValidateExcludeWorkloads(t *testing.T) {
	validator := NewValidator()

	tests := []struct {
		name        string
		patterns    []string
		shouldError bool
	}{
		{
			name:        "valid regex patterns",
			patterns:    []string{"^test-.*", ".*-temp$", "debug.*"},
			shouldError: false,
		},
		{
			name:        "invalid regex pattern",
			patterns:    []string{"[invalid"},
			shouldError: true,
		},
		{
			name:        "empty pattern",
			patterns:    []string{"valid-.*", ""},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &optimizerv1alpha1.OptimizerConfig{
				Spec: optimizerv1alpha1.OptimizerConfigSpec{
					TargetNamespaces: []string{"default"},
					ExcludeWorkloads: tt.patterns,
				},
			}

			err := validator.ValidateCreate(config)
			if tt.shouldError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestValidator_ValidateUpdate(t *testing.T) {
	validator := NewValidator()

	oldConfig := &optimizerv1alpha1.OptimizerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-config",
			Namespace: "default",
		},
		Spec: optimizerv1alpha1.OptimizerConfigSpec{
			TargetNamespaces: []string{"default"},
		},
	}

	newConfig := &optimizerv1alpha1.OptimizerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-config",
			Namespace: "default",
		},
		Spec: optimizerv1alpha1.OptimizerConfigSpec{
			TargetNamespaces: []string{"default", "production"},
		},
	}

	err := validator.ValidateUpdate(oldConfig, newConfig)
	if err != nil {
		t.Errorf("Expected no error for valid update, got: %v", err)
	}
}

func TestValidator_ValidateDelete(t *testing.T) {
	validator := NewValidator()

	config := &optimizerv1alpha1.OptimizerConfig{
		Spec: optimizerv1alpha1.OptimizerConfigSpec{
			TargetNamespaces: []string{"default"},
		},
	}

	err := validator.ValidateDelete(config)
	if err != nil {
		t.Errorf("Expected no error for deletion, got: %v", err)
	}
}

// Helper function to create float64 pointer
func ptrFloat64(v float64) *float64 {
	return &v
}
