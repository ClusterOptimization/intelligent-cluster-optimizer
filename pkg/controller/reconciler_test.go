package controller

import (
	"context"
	"testing"
	"time"

	optimizerv1alpha1 "intelligent-cluster-optimizer/pkg/apis/optimizer/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestReconciler_DisabledConfig(t *testing.T) {
	r := NewReconciler()
	config := &optimizerv1alpha1.OptimizerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-config",
			Namespace: "default",
		},
		Spec: optimizerv1alpha1.OptimizerConfigSpec{
			Enabled:          false,
			TargetNamespaces: []string{"default"},
			Strategy:         optimizerv1alpha1.StrategyBalanced,
		},
	}

	result, err := r.Reconcile(context.Background(), config)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	if !result.Updated {
		t.Error("Expected status to be updated")
	}

	if config.Status.Phase != optimizerv1alpha1.OptimizerPhasePaused {
		t.Errorf("Expected phase Paused, got %s", config.Status.Phase)
	}
}

func TestReconciler_InitialReconcile(t *testing.T) {
	r := NewReconciler()
	config := &optimizerv1alpha1.OptimizerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-config",
			Namespace:  "default",
			Generation: 1,
		},
		Spec: optimizerv1alpha1.OptimizerConfigSpec{
			Enabled:          true,
			TargetNamespaces: []string{"default"},
			Strategy:         optimizerv1alpha1.StrategyBalanced,
		},
	}

	result, err := r.Reconcile(context.Background(), config)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	if !result.Updated {
		t.Error("Expected status to be updated")
	}

	if config.Status.Phase != optimizerv1alpha1.OptimizerPhasePending {
		t.Errorf("Expected phase Pending, got %s", config.Status.Phase)
	}
}

func TestReconciler_CircuitBreakerOpen(t *testing.T) {
	r := NewReconciler()
	config := &optimizerv1alpha1.OptimizerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-config",
			Namespace:  "default",
			Generation: 1,
		},
		Spec: optimizerv1alpha1.OptimizerConfigSpec{
			Enabled:          true,
			TargetNamespaces: []string{"default"},
			Strategy:         optimizerv1alpha1.StrategyBalanced,
			CircuitBreaker: &optimizerv1alpha1.CircuitBreakerConfig{
				Enabled:          true,
				ErrorThreshold:   5,
				SuccessThreshold: 3,
			},
		},
		Status: optimizerv1alpha1.OptimizerConfigStatus{
			Phase:        optimizerv1alpha1.OptimizerPhaseActive,
			CircuitState: optimizerv1alpha1.CircuitStateOpen,
		},
	}

	result, err := r.Reconcile(context.Background(), config)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	if !result.Updated {
		t.Error("Expected status to be updated")
	}

	if config.Status.Phase != optimizerv1alpha1.OptimizerPhaseCircuitOpen {
		t.Errorf("Expected phase CircuitOpen, got %s", config.Status.Phase)
	}

	if result.RequeueAfter != 5*time.Minute {
		t.Errorf("Expected requeue after 5 minutes, got %v", result.RequeueAfter)
	}
}

func TestReconciler_ActiveReconcile(t *testing.T) {
	r := NewReconciler()
	config := &optimizerv1alpha1.OptimizerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-config",
			Namespace:  "default",
			Generation: 2,
		},
		Spec: optimizerv1alpha1.OptimizerConfigSpec{
			Enabled:          true,
			TargetNamespaces: []string{"default"},
			Strategy:         optimizerv1alpha1.StrategyBalanced,
			DryRun:           false,
		},
		Status: optimizerv1alpha1.OptimizerConfigStatus{
			Phase:        optimizerv1alpha1.OptimizerPhaseActive,
			CircuitState: optimizerv1alpha1.CircuitStateClosed,
		},
	}

	result, err := r.Reconcile(context.Background(), config)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	if !result.Updated {
		t.Error("Expected status to be updated")
	}

	if config.Status.Phase != optimizerv1alpha1.OptimizerPhaseActive {
		t.Errorf("Expected phase Active, got %s", config.Status.Phase)
	}

	if config.Status.ObservedGeneration != 2 {
		t.Errorf("Expected ObservedGeneration 2, got %d", config.Status.ObservedGeneration)
	}

	if config.Status.LastRecommendationTime == nil {
		t.Error("Expected LastRecommendationTime to be set")
	}

	if result.RequeueAfter != 30*time.Second {
		t.Errorf("Expected requeue after 30 seconds, got %v", result.RequeueAfter)
	}

	foundReadyCondition := false
	for _, cond := range config.Status.Conditions {
		if cond.Type == optimizerv1alpha1.ConditionTypeReady {
			foundReadyCondition = true
			if cond.Status != optimizerv1alpha1.ConditionTrue {
				t.Errorf("Expected Ready condition to be True, got %s", cond.Status)
			}
		}
	}

	if !foundReadyCondition {
		t.Error("Expected Ready condition to be set")
	}
}

func TestReconciler_DryRunMode(t *testing.T) {
	r := NewReconciler()
	config := &optimizerv1alpha1.OptimizerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-config",
			Namespace:  "default",
			Generation: 1,
		},
		Spec: optimizerv1alpha1.OptimizerConfigSpec{
			Enabled:          true,
			TargetNamespaces: []string{"default"},
			Strategy:         optimizerv1alpha1.StrategyBalanced,
			DryRun:           true,
		},
		Status: optimizerv1alpha1.OptimizerConfigStatus{
			Phase: optimizerv1alpha1.OptimizerPhaseActive,
		},
	}

	_, err := r.Reconcile(context.Background(), config)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	if config.Status.Phase != optimizerv1alpha1.OptimizerPhaseActive {
		t.Errorf("Expected phase Active even in dry-run, got %s", config.Status.Phase)
	}
}

func TestReconciler_UpdateCondition(t *testing.T) {
	r := NewReconciler()
	config := &optimizerv1alpha1.OptimizerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-config",
			Namespace: "default",
		},
	}

	err := r.updateCondition(config, optimizerv1alpha1.ConditionTypeReady, optimizerv1alpha1.ConditionTrue, "TestReason", "Test message")
	if err != nil {
		t.Fatalf("updateCondition failed: %v", err)
	}

	if len(config.Status.Conditions) != 1 {
		t.Fatalf("Expected 1 condition, got %d", len(config.Status.Conditions))
	}

	cond := config.Status.Conditions[0]
	if cond.Type != optimizerv1alpha1.ConditionTypeReady {
		t.Errorf("Expected condition type Ready, got %s", cond.Type)
	}
	if cond.Status != optimizerv1alpha1.ConditionTrue {
		t.Errorf("Expected condition status True, got %s", cond.Status)
	}
	if cond.Reason != "TestReason" {
		t.Errorf("Expected reason TestReason, got %s", cond.Reason)
	}

	err = r.updateCondition(config, optimizerv1alpha1.ConditionTypeReady, optimizerv1alpha1.ConditionFalse, "NewReason", "New message")
	if err != nil {
		t.Fatalf("updateCondition failed: %v", err)
	}

	if len(config.Status.Conditions) != 1 {
		t.Fatalf("Expected 1 condition after update, got %d", len(config.Status.Conditions))
	}

	cond = config.Status.Conditions[0]
	if cond.Status != optimizerv1alpha1.ConditionFalse {
		t.Errorf("Expected updated condition status False, got %s", cond.Status)
	}
}
