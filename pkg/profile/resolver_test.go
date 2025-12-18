package profile

import (
	"testing"
	"time"

	optimizerv1alpha1 "intelligent-cluster-optimizer/pkg/apis/optimizer/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestResolverWithProductionProfile(t *testing.T) {
	resolver := NewResolver()

	config := &optimizerv1alpha1.OptimizerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-config",
			Namespace: "default",
		},
		Spec: optimizerv1alpha1.OptimizerConfigSpec{
			Profile:          optimizerv1alpha1.ProfileProduction,
			TargetNamespaces: []string{"production"},
		},
	}

	resolved, err := resolver.Resolve(config)
	if err != nil {
		t.Fatalf("failed to resolve: %v", err)
	}

	if resolved.ProfileName != "production" {
		t.Errorf("expected profile name 'production', got %s", resolved.ProfileName)
	}
	if resolved.Strategy != "conservative" {
		t.Errorf("expected strategy 'conservative', got %s", resolved.Strategy)
	}
	if resolved.CPUPercentile != 99 {
		t.Errorf("expected CPU percentile 99, got %d", resolved.CPUPercentile)
	}
	if resolved.SafetyMargin < 1.3 {
		t.Errorf("expected safety margin >= 1.3, got %.2f", resolved.SafetyMargin)
	}
	if resolved.MinConfidence < 70 {
		t.Errorf("expected min confidence >= 70, got %.2f", resolved.MinConfidence)
	}
	if !resolved.RequireApproval {
		t.Error("expected require approval for production")
	}
}

func TestResolverWithDevelopmentProfile(t *testing.T) {
	resolver := NewResolver()

	config := &optimizerv1alpha1.OptimizerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-config",
			Namespace: "default",
		},
		Spec: optimizerv1alpha1.OptimizerConfigSpec{
			Profile:          optimizerv1alpha1.ProfileDevelopment,
			TargetNamespaces: []string{"dev"},
		},
	}

	resolved, err := resolver.Resolve(config)
	if err != nil {
		t.Fatalf("failed to resolve: %v", err)
	}

	if resolved.ProfileName != "development" {
		t.Errorf("expected profile name 'development', got %s", resolved.ProfileName)
	}
	if resolved.Strategy != "aggressive" {
		t.Errorf("expected strategy 'aggressive', got %s", resolved.Strategy)
	}
	if resolved.CPUPercentile > 95 {
		t.Errorf("expected CPU percentile <= 95, got %d", resolved.CPUPercentile)
	}
	if resolved.SafetyMargin > 1.2 {
		t.Errorf("expected safety margin <= 1.2, got %.2f", resolved.SafetyMargin)
	}
	if resolved.RequireApproval {
		t.Error("did not expect require approval for development")
	}
}

func TestResolverWithProfileOverrides(t *testing.T) {
	resolver := NewResolver()

	minConfidence := 90.0
	requireApproval := false

	config := &optimizerv1alpha1.OptimizerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-config",
			Namespace: "default",
		},
		Spec: optimizerv1alpha1.OptimizerConfigSpec{
			Profile:          optimizerv1alpha1.ProfileProduction,
			TargetNamespaces: []string{"production"},
			ProfileOverrides: &optimizerv1alpha1.ProfileOverrides{
				MinConfidence:   &minConfidence,
				RequireApproval: &requireApproval,
				ApplyDelay:      "48h",
			},
		},
	}

	resolved, err := resolver.Resolve(config)
	if err != nil {
		t.Fatalf("failed to resolve: %v", err)
	}

	// Overridden values
	if resolved.MinConfidence != 90.0 {
		t.Errorf("expected min confidence 90.0, got %.2f", resolved.MinConfidence)
	}
	if resolved.RequireApproval {
		t.Error("expected require approval to be false (overridden)")
	}
	if resolved.ApplyDelay != 48*time.Hour {
		t.Errorf("expected apply delay 48h, got %v", resolved.ApplyDelay)
	}

	// Base values should remain from production profile
	if resolved.Strategy != "conservative" {
		t.Errorf("expected strategy 'conservative' (not overridden), got %s", resolved.Strategy)
	}
}

func TestResolverWithNoProfile(t *testing.T) {
	resolver := NewResolver()

	config := &optimizerv1alpha1.OptimizerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-config",
			Namespace: "default",
		},
		Spec: optimizerv1alpha1.OptimizerConfigSpec{
			Strategy:         optimizerv1alpha1.StrategyAggressive,
			TargetNamespaces: []string{"app"},
			DryRun:           true,
			Recommendations: &optimizerv1alpha1.RecommendationConfig{
				CPUPercentile:    90,
				MemoryPercentile: 92,
				SafetyMargin:     1.15,
				MinSamples:       50,
				HistoryDuration:  "12h",
			},
		},
	}

	resolved, err := resolver.Resolve(config)
	if err != nil {
		t.Fatalf("failed to resolve: %v", err)
	}

	if resolved.ProfileName != "custom" {
		t.Errorf("expected profile name 'custom', got %s", resolved.ProfileName)
	}
	if resolved.Strategy != "aggressive" {
		t.Errorf("expected strategy 'aggressive', got %s", resolved.Strategy)
	}
	if resolved.CPUPercentile != 90 {
		t.Errorf("expected CPU percentile 90, got %d", resolved.CPUPercentile)
	}
	if resolved.MemoryPercentile != 92 {
		t.Errorf("expected memory percentile 92, got %d", resolved.MemoryPercentile)
	}
	if resolved.SafetyMargin != 1.15 {
		t.Errorf("expected safety margin 1.15, got %.2f", resolved.SafetyMargin)
	}
	if resolved.MinSamples != 50 {
		t.Errorf("expected min samples 50, got %d", resolved.MinSamples)
	}
	if resolved.HistoryDuration != 12*time.Hour {
		t.Errorf("expected history duration 12h, got %v", resolved.HistoryDuration)
	}
	if !resolved.DryRun {
		t.Error("expected dry run to be true")
	}
}

func TestResolverSpecOverridesProfile(t *testing.T) {
	resolver := NewResolver()

	config := &optimizerv1alpha1.OptimizerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-config",
			Namespace: "default",
		},
		Spec: optimizerv1alpha1.OptimizerConfigSpec{
			Profile:          optimizerv1alpha1.ProfileProduction,
			TargetNamespaces: []string{"production"},
			DryRun:           false, // Override production's default of true
			Recommendations: &optimizerv1alpha1.RecommendationConfig{
				CPUPercentile: 97, // Override production's 99
			},
		},
	}

	resolved, err := resolver.Resolve(config)
	if err != nil {
		t.Fatalf("failed to resolve: %v", err)
	}

	// Spec overrides
	if resolved.CPUPercentile != 97 {
		t.Errorf("expected CPU percentile 97 (spec override), got %d", resolved.CPUPercentile)
	}
	if resolved.DryRun {
		t.Error("expected dry run to be false (spec override)")
	}

	// Profile base values
	if resolved.MemoryPercentile != 99 {
		t.Errorf("expected memory percentile 99 (from profile), got %d", resolved.MemoryPercentile)
	}
}

func TestShouldApplyRecommendation(t *testing.T) {
	tests := []struct {
		name          string
		settings      ResolvedSettings
		confidence    float64
		changePercent float64
		shouldApply   bool
		expectedReason string
	}{
		{
			name: "dry run mode",
			settings: ResolvedSettings{
				DryRun:        true,
				MinConfidence: 50,
			},
			confidence:    80,
			changePercent: 10,
			shouldApply:   false,
			expectedReason: "dry-run",
		},
		{
			name: "confidence too low",
			settings: ResolvedSettings{
				DryRun:        false,
				MinConfidence: 70,
			},
			confidence:    60,
			changePercent: 10,
			shouldApply:   false,
			expectedReason: "confidence",
		},
		{
			name: "change too large",
			settings: ResolvedSettings{
				DryRun:           false,
				MinConfidence:    50,
				MaxChangePercent: 20,
			},
			confidence:    80,
			changePercent: 30,
			shouldApply:   false,
			expectedReason: "change",
		},
		{
			name: "requires approval",
			settings: ResolvedSettings{
				DryRun:          false,
				MinConfidence:   50,
				RequireApproval: true,
			},
			confidence:    80,
			changePercent: 10,
			shouldApply:   false,
			expectedReason: "approval",
		},
		{
			name: "should apply",
			settings: ResolvedSettings{
				DryRun:           false,
				MinConfidence:    50,
				MaxChangePercent: 30,
				RequireApproval:  false,
			},
			confidence:    80,
			changePercent: 20,
			shouldApply:   true,
		},
		{
			name: "no max change limit",
			settings: ResolvedSettings{
				DryRun:           false,
				MinConfidence:    50,
				MaxChangePercent: 0, // No limit
				RequireApproval:  false,
			},
			confidence:    80,
			changePercent: 100,
			shouldApply:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apply, reason := tt.settings.ShouldApplyRecommendation(tt.confidence, tt.changePercent)
			if apply != tt.shouldApply {
				t.Errorf("expected shouldApply=%v, got %v (reason: %s)", tt.shouldApply, apply, reason)
			}
			if !tt.shouldApply && tt.expectedReason != "" {
				if !containsSubstring(reason, tt.expectedReason) {
					t.Errorf("expected reason to contain '%s', got '%s'", tt.expectedReason, reason)
				}
			}
		})
	}
}

func TestGetEffectiveStrategy(t *testing.T) {
	tests := []struct {
		strategy string
		expected optimizerv1alpha1.OptimizationStrategy
	}{
		{"aggressive", optimizerv1alpha1.StrategyAggressive},
		{"balanced", optimizerv1alpha1.StrategyBalanced},
		{"conservative", optimizerv1alpha1.StrategyConservative},
		{"unknown", optimizerv1alpha1.StrategyBalanced}, // Default to balanced
		{"", optimizerv1alpha1.StrategyBalanced},
	}

	for _, tt := range tests {
		t.Run(tt.strategy, func(t *testing.T) {
			settings := ResolvedSettings{Strategy: tt.strategy}
			result := settings.GetEffectiveStrategy()
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestResolvedSettingsSummary(t *testing.T) {
	settings := ResolvedSettings{
		ProfileName:      "production",
		Strategy:         "conservative",
		CPUPercentile:    99,
		MemoryPercentile: 99,
		SafetyMargin:     1.4,
		MinConfidence:    80.0,
		DryRun:           true,
	}

	summary := settings.Summary()

	if summary == "" {
		t.Error("expected non-empty summary")
	}
	if !containsSubstring(summary, "production") {
		t.Error("expected summary to contain profile name")
	}
	if !containsSubstring(summary, "conservative") {
		t.Error("expected summary to contain strategy")
	}
}

func TestParseCPUToMillicores(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"100m", 100},
		{"500m", 500},
		{"1", 1000},
		{"2", 2000},
		{"0.5", 500},
		{"1.5", 1500},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseCPUToMillicores(tt.input)
			if result != tt.expected {
				t.Errorf("parseCPUToMillicores(%s) = %d, expected %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseMemoryToMegabytes(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"128Mi", 128},
		{"256Mi", 256},
		{"1Gi", 1024},
		{"2Gi", 2048},
		{"1G", 1000},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseMemoryToMegabytes(tt.input)
			if result != tt.expected {
				t.Errorf("parseMemoryToMegabytes(%s) = %d, expected %d", tt.input, result, tt.expected)
			}
		})
	}
}
