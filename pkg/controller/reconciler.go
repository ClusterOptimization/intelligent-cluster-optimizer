package controller

import (
	"context"
	"time"

	optimizerv1alpha1 "intelligent-cluster-optimizer/pkg/apis/optimizer/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

type ReconcileResult struct {
	RequeueAfter time.Duration
	Updated      bool
}

type Reconciler struct {
	// TODO: Add analyzer, executor, safety checkers when implemented
}

func NewReconciler() *Reconciler {
	return &Reconciler{}
}

func (r *Reconciler) Reconcile(ctx context.Context, config *optimizerv1alpha1.OptimizerConfig) (*ReconcileResult, error) {
	result := &ReconcileResult{}

	if !config.Spec.Enabled {
		klog.V(3).Infof("OptimizerConfig %s/%s is disabled, skipping", config.Namespace, config.Name)
		if err := r.updatePhase(config, optimizerv1alpha1.OptimizerPhasePaused, "Disabled"); err != nil {
			return result, err
		}
		result.Updated = true
		return result, nil
	}

	if config.Status.Phase == "" {
		if err := r.updatePhase(config, optimizerv1alpha1.OptimizerPhasePending, "Initializing"); err != nil {
			return result, err
		}
		result.Updated = true
		return result, nil
	}

	if config.Spec.CircuitBreaker != nil && config.Spec.CircuitBreaker.Enabled {
		if config.Status.CircuitState == optimizerv1alpha1.CircuitStateOpen {
			klog.V(3).Infof("Circuit breaker is open for %s/%s", config.Namespace, config.Name)
			if err := r.updatePhase(config, optimizerv1alpha1.OptimizerPhaseCircuitOpen, "Circuit breaker open"); err != nil {
				return result, err
			}
			result.Updated = true
			result.RequeueAfter = 5 * time.Minute
			return result, nil
		}
	}

	if config.Spec.DryRun {
		klog.V(3).Infof("OptimizerConfig %s/%s in dry-run mode", config.Namespace, config.Name)
	}

	if err := r.updatePhase(config, optimizerv1alpha1.OptimizerPhaseActive, "Reconciliation successful"); err != nil {
		return result, err
	}

	if err := r.updateCondition(config, optimizerv1alpha1.ConditionTypeReady, optimizerv1alpha1.ConditionTrue, "ReconcileSuccess", "OptimizerConfig reconciled successfully"); err != nil {
		return result, err
	}

	config.Status.ObservedGeneration = config.Generation
	now := metav1.NewTime(time.Now())
	config.Status.LastRecommendationTime = &now

	result.Updated = true
	result.RequeueAfter = 30 * time.Second

	return result, nil
}

func (r *Reconciler) updatePhase(config *optimizerv1alpha1.OptimizerConfig, phase optimizerv1alpha1.OptimizerPhase, message string) error {
	config.Status.Phase = phase
	klog.V(4).Infof("Updated phase to %s: %s", phase, message)
	return nil
}

func (r *Reconciler) updateCondition(
	config *optimizerv1alpha1.OptimizerConfig,
	conditionType optimizerv1alpha1.OptimizerConditionType,
	status optimizerv1alpha1.ConditionStatus,
	reason, message string,
) error {
	now := metav1.NewTime(time.Now())
	condition := optimizerv1alpha1.OptimizerCondition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: now,
		Reason:             reason,
		Message:            message,
	}

	found := false
	for i, cond := range config.Status.Conditions {
		if cond.Type == conditionType {
			if cond.Status != status {
				config.Status.Conditions[i] = condition
			}
			found = true
			break
		}
	}

	if !found {
		config.Status.Conditions = append(config.Status.Conditions, condition)
	}

	return nil
}
