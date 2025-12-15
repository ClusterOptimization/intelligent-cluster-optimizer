package controller

import (
	"context"
	"time"

	optimizerv1alpha1 "intelligent-cluster-optimizer/pkg/apis/optimizer/v1alpha1"
	"intelligent-cluster-optimizer/pkg/applier"
	"intelligent-cluster-optimizer/pkg/safety"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
)

type ReconcileResult struct {
	RequeueAfter time.Duration
	Updated      bool
}

type Reconciler struct {
	kubeClient    kubernetes.Interface
	hpaChecker    *safety.HPAChecker
	pdbChecker    *safety.PDBChecker
	applier       *applier.Applier
	eventRecorder record.EventRecorder
}

func NewReconciler(kubeClient kubernetes.Interface, eventRecorder record.EventRecorder) *Reconciler {
	return &Reconciler{
		kubeClient:    kubeClient,
		hpaChecker:    safety.NewHPAChecker(kubeClient),
		pdbChecker:    safety.NewPDBChecker(kubeClient),
		applier:       applier.NewApplier(kubeClient, eventRecorder),
		eventRecorder: eventRecorder,
	}
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

	mode := "LIVE"
	if config.Spec.DryRun {
		mode = "DRY-RUN"
		klog.Infof("OptimizerConfig %s/%s running in DRY-RUN mode - no changes will be applied", config.Namespace, config.Name)
	} else {
		klog.Infof("OptimizerConfig %s/%s running in LIVE mode - changes will be applied", config.Namespace, config.Name)
	}

	if err := r.processRecommendations(ctx, config, mode); err != nil {
		klog.Warningf("Failed to process recommendations: %v", err)
	}

	if config.Spec.HPAAwareness != nil && config.Spec.HPAAwareness.Enabled {
		hasConflicts, err := r.checkHPAConflicts(ctx, config)
		if err != nil {
			klog.Warningf("Failed to check HPA conflicts: %v", err)
		} else if hasConflicts {
			result.Updated = true
			policy := config.Spec.HPAAwareness.ConflictPolicy
			if policy == "" || policy == optimizerv1alpha1.HPAConflictPolicySkip {
				klog.V(3).Infof("HPA conflicts detected for %s/%s, skipping optimization", config.Namespace, config.Name)
				return result, nil
			} else if policy == optimizerv1alpha1.HPAConflictPolicyWarn {
				klog.Warningf("HPA conflicts detected for %s/%s, proceeding with caution", config.Namespace, config.Name)
			}
		} else {
			if err := r.updateCondition(config, optimizerv1alpha1.ConditionTypeHPAConflict, optimizerv1alpha1.ConditionFalse, "NoConflict", "No HPA conflicts detected"); err != nil {
				return result, err
			}
		}
	}

	if config.Spec.PDBAwareness != nil && config.Spec.PDBAwareness.Enabled {
		hasViolations, err := r.checkPDBViolations(ctx, config)
		if err != nil {
			klog.Warningf("Failed to check PDB violations: %v", err)
		} else if hasViolations {
			result.Updated = true
			if config.Spec.PDBAwareness.RespectMinAvailable {
				klog.V(3).Infof("PDB violations detected for %s/%s, skipping optimization", config.Namespace, config.Name)
				return result, nil
			} else {
				klog.Warningf("PDB violations detected for %s/%s, proceeding anyway", config.Namespace, config.Name)
			}
		} else {
			if err := r.updateCondition(config, optimizerv1alpha1.ConditionTypePDBViolation, optimizerv1alpha1.ConditionFalse, "NoViolation", "No PDB violations detected"); err != nil {
				return result, err
			}
		}
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

func (r *Reconciler) checkHPAConflicts(ctx context.Context, config *optimizerv1alpha1.OptimizerConfig) (bool, error) {
	hasConflicts := false

	for _, ns := range config.Spec.TargetNamespaces {
		for _, resourceType := range config.Spec.TargetResources {
			kind := resourceTypeToKind(resourceType)
			result, err := r.hpaChecker.CheckHPAConflict(ctx, ns, kind, "")
			if err != nil {
				return false, err
			}
			if result.HasConflict {
				hasConflicts = true
				if err := r.updateCondition(config,
					optimizerv1alpha1.ConditionTypeHPAConflict,
					optimizerv1alpha1.ConditionTrue,
					"ConflictDetected",
					result.Message); err != nil {
					return false, err
				}
				klog.Warningf("HPA conflict in %s/%s: %s", ns, kind, result.Message)
			}
		}
	}

	return hasConflicts, nil
}

func (r *Reconciler) checkPDBViolations(ctx context.Context, config *optimizerv1alpha1.OptimizerConfig) (bool, error) {
	hasViolations := false
	plannedDisruption := int32(1)

	for _, ns := range config.Spec.TargetNamespaces {
		for _, resourceType := range config.Spec.TargetResources {
			kind := resourceTypeToKind(resourceType)
			result, err := r.pdbChecker.CheckPDBSafety(ctx, ns, kind, "", plannedDisruption)
			if err != nil {
				return false, err
			}
			if result.HasPDB && !result.IsSafe {
				hasViolations = true
				if err := r.updateCondition(config,
					optimizerv1alpha1.ConditionTypePDBViolation,
					optimizerv1alpha1.ConditionTrue,
					"ViolationDetected",
					result.Message); err != nil {
					return false, err
				}
				klog.Warningf("PDB violation in %s/%s: %s", ns, kind, result.Message)
			}
		}
	}

	return hasViolations, nil
}

func (r *Reconciler) processRecommendations(ctx context.Context, config *optimizerv1alpha1.OptimizerConfig, mode string) error {
	klog.V(4).Infof("[%s] Processing recommendations for OptimizerConfig %s/%s", mode, config.Namespace, config.Name)

	sampleRec := &applier.ResourceRecommendation{
		Namespace:         config.Spec.TargetNamespaces[0],
		WorkloadKind:      "Deployment",
		WorkloadName:      "example-app",
		ContainerName:     "app",
		CurrentCPU:        "100m",
		RecommendedCPU:    "200m",
		CurrentMemory:     "128Mi",
		RecommendedMemory: "256Mi",
	}

	applyResult, err := r.applier.Apply(ctx, sampleRec, config.Spec.DryRun)
	if err != nil {
		return err
	}

	if config.Spec.DryRun {
		klog.V(3).Infof("[DRY-RUN] Summary: %d changes would be applied to %s/%s",
			len(applyResult.Changes), applyResult.WorkloadKind, applyResult.WorkloadName)
		for _, change := range applyResult.Changes {
			klog.V(3).Infof("[DRY-RUN]   - %s", change)
		}
	} else if applyResult.Applied {
		klog.Infof("[LIVE] Successfully applied %d changes to %s/%s",
			len(applyResult.Changes), applyResult.WorkloadKind, applyResult.WorkloadName)
	}

	return nil
}

func resourceTypeToKind(resourceType optimizerv1alpha1.TargetResourceType) string {
	switch resourceType {
	case optimizerv1alpha1.TargetResourceDeployments:
		return "Deployment"
	case optimizerv1alpha1.TargetResourceStatefulSets:
		return "StatefulSet"
	case optimizerv1alpha1.TargetResourceDaemonSets:
		return "DaemonSet"
	default:
		return "Deployment"
	}
}
