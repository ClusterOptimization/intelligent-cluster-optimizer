package controller

import (
	"context"
	"fmt"
	"time"

	optimizerv1alpha1 "intelligent-cluster-optimizer/pkg/apis/optimizer/v1alpha1"
	"intelligent-cluster-optimizer/pkg/applier"
	"intelligent-cluster-optimizer/pkg/events"
	"intelligent-cluster-optimizer/pkg/profile"
	"intelligent-cluster-optimizer/pkg/recommendation"
	"intelligent-cluster-optimizer/pkg/safety"
	"intelligent-cluster-optimizer/pkg/scheduler"
	"intelligent-cluster-optimizer/pkg/storage"

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
	kubeClient             kubernetes.Interface
	hpaChecker             *safety.HPAChecker
	pdbChecker             *safety.PDBChecker
	applier                *applier.Applier
	eventRecorder          record.EventRecorder
	optimizerEvents        *events.OptimizerEventRecorder
	maintenanceWindowCheck *scheduler.MaintenanceWindowChecker
	circuitBreaker         *safety.CircuitBreaker
	recommendationEngine   *recommendation.Engine
	metricsStorage         *storage.InMemoryStorage
	profileResolver        *profile.Resolver
}

func NewReconciler(kubeClient kubernetes.Interface, eventRecorder record.EventRecorder) *Reconciler {
	return &Reconciler{
		kubeClient:             kubeClient,
		hpaChecker:             safety.NewHPAChecker(kubeClient),
		pdbChecker:             safety.NewPDBChecker(kubeClient),
		applier:                applier.NewApplier(kubeClient, eventRecorder),
		eventRecorder:          eventRecorder,
		optimizerEvents:        events.NewOptimizerEventRecorder(eventRecorder),
		maintenanceWindowCheck: scheduler.NewMaintenanceWindowChecker(),
		circuitBreaker:         safety.NewCircuitBreaker(),
		recommendationEngine:   recommendation.NewEngine(),
		metricsStorage:         storage.NewStorage(),
		profileResolver:        profile.NewResolver(),
	}
}

// SetMetricsStorage allows injecting a shared metrics storage instance
func (r *Reconciler) SetMetricsStorage(store *storage.InMemoryStorage) {
	r.metricsStorage = store
}

// GetMetricsStorage returns the metrics storage for external population
func (r *Reconciler) GetMetricsStorage() *storage.InMemoryStorage {
	return r.metricsStorage
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
		if !r.circuitBreaker.ShouldAllow(config) {
			klog.V(3).Infof("Circuit breaker is open for %s/%s", config.Namespace, config.Name)
			r.optimizerEvents.RecordWarningEvent(config, events.ReasonCircuitBreakerOpen,
				"Circuit breaker open, skipping optimization")
			if err := r.updatePhase(config, optimizerv1alpha1.OptimizerPhaseCircuitOpen, "Circuit breaker open"); err != nil {
				return result, err
			}
			result.Updated = true
			result.RequeueAfter = 5 * time.Minute
			return result, nil
		}
	}

	inMaintenanceWindow := r.maintenanceWindowCheck.IsInMaintenanceWindow(config)
	config.Status.ActiveMaintenanceWindow = inMaintenanceWindow

	if nextWindow := r.maintenanceWindowCheck.GetNextMaintenanceWindow(config); nextWindow != nil {
		config.Status.NextMaintenanceWindow = &metav1.Time{Time: *nextWindow}
	}

	if err := r.updateCondition(config, optimizerv1alpha1.ConditionTypeMaintenanceWindow,
		boolToConditionStatus(inMaintenanceWindow), "MaintenanceWindowCheck",
		maintenanceWindowMessage(inMaintenanceWindow)); err != nil {
		return result, err
	}

	if !inMaintenanceWindow && len(config.Spec.MaintenanceWindows) > 0 && !config.Spec.DryRun {
		klog.V(3).Infof("OptimizerConfig %s/%s outside maintenance window, skipping live updates", config.Namespace, config.Name)
		r.optimizerEvents.RecordWarningEvent(config, events.ReasonMaintenanceWindowSkipped,
			"Skipping live updates outside maintenance window")
		result.Updated = true
		if config.Status.NextMaintenanceWindow != nil {
			timeUntilNext := time.Until(config.Status.NextMaintenanceWindow.Time)
			if timeUntilNext > 0 && timeUntilNext < 30*time.Minute {
				result.RequeueAfter = timeUntilNext
			} else {
				result.RequeueAfter = 5 * time.Minute
			}
		} else {
			result.RequeueAfter = 5 * time.Minute
		}
		return result, nil
	}

	mode := "LIVE"
	if config.Spec.DryRun {
		mode = "DRY-RUN"
		klog.Infof("OptimizerConfig %s/%s running in DRY-RUN mode - no changes will be applied", config.Namespace, config.Name)
	} else {
		klog.Infof("OptimizerConfig %s/%s running in LIVE mode - changes will be applied", config.Namespace, config.Name)
	}

	// Process recommendations with per-workload safety checks
	if err := r.processRecommendations(ctx, config, mode); err != nil {
		klog.Warningf("Failed to process recommendations: %v", err)
		if config.Spec.CircuitBreaker != nil && config.Spec.CircuitBreaker.Enabled {
			if stateChanged := r.circuitBreaker.RecordFailure(config, err); stateChanged {
				stateName := r.circuitBreaker.GetStateName(config.Status.CircuitState)
				klog.Warningf("Circuit breaker state changed to %s for %s/%s after error: %v",
					stateName, config.Namespace, config.Name, err)
				r.optimizerEvents.RecordWarningEvent(config, "CircuitBreakerStateChanged",
					"Circuit breaker state: "+stateName)
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

	if config.Spec.CircuitBreaker != nil && config.Spec.CircuitBreaker.Enabled {
		if stateChanged := r.circuitBreaker.RecordSuccess(config); stateChanged {
			stateName := r.circuitBreaker.GetStateName(config.Status.CircuitState)
			klog.Infof("Circuit breaker state changed to %s for %s/%s", stateName, config.Namespace, config.Name)
			r.optimizerEvents.RecordNormalEvent(config, "CircuitBreakerStateChanged",
				"Circuit breaker state: "+stateName)
		}
	}

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

	// Resolve profile settings for this config
	resolvedSettings, err := r.profileResolver.Resolve(config)
	if err != nil {
		klog.Warningf("Failed to resolve profile settings, using defaults: %v", err)
		// Continue with nil settings - will skip MaxChangePercent check
	}

	// Generate recommendations using the engine with P95/P99 percentile calculation
	recommendations, err := r.recommendationEngine.GenerateRecommendations(r.metricsStorage, config)
	if err != nil {
		return fmt.Errorf("failed to generate recommendations: %w", err)
	}

	if len(recommendations) == 0 {
		klog.V(3).Infof("[%s] No recommendations generated for %s/%s (insufficient metrics or no changes needed)",
			mode, config.Namespace, config.Name)
		return nil
	}

	// Calculate total estimated savings across all workloads
	var totalSavingsPerMonth float64
	for _, rec := range recommendations {
		if rec.TotalEstimatedSavings != nil {
			totalSavingsPerMonth += rec.TotalEstimatedSavings.SavingsPerMonth
		}
	}

	klog.V(3).Infof("[%s] Generated %d workload recommendations (estimated savings: $%.2f/month)",
		mode, len(recommendations), totalSavingsPerMonth)

	// Process each workload recommendation
	var appliedCount, skippedCount int
	for _, workloadRec := range recommendations {
		// SAFETY CHECK: Check HPA conflicts before processing this workload
		if config.Spec.HPAAwareness != nil && config.Spec.HPAAwareness.Enabled {
			hpaResult, err := r.hpaChecker.CheckHPAConflict(ctx, workloadRec.Namespace, workloadRec.WorkloadKind, workloadRec.WorkloadName)
			if err != nil {
				klog.Warningf("Failed to check HPA conflict for %s/%s/%s: %v", workloadRec.Namespace, workloadRec.WorkloadKind, workloadRec.WorkloadName, err)
			} else if hpaResult.HasConflict {
				policy := config.Spec.HPAAwareness.ConflictPolicy
				if policy == "" || policy == optimizerv1alpha1.HPAConflictPolicySkip {
					klog.V(3).Infof("[%s] Skipping %s/%s/%s: HPA conflict detected - %s",
						mode, workloadRec.Namespace, workloadRec.WorkloadKind, workloadRec.WorkloadName, hpaResult.Message)
					r.optimizerEvents.RecordWarningEvent(config, events.ReasonHPAConflictDetected,
						fmt.Sprintf("HPA conflict for %s/%s: %s", workloadRec.WorkloadKind, workloadRec.WorkloadName, hpaResult.Message))
					if err := r.updateCondition(config, optimizerv1alpha1.ConditionTypeHPAConflict, optimizerv1alpha1.ConditionTrue, "ConflictDetected", hpaResult.Message); err != nil {
						klog.Warningf("Failed to update condition: %v", err)
					}
					skippedCount++
					continue
				} else if policy == optimizerv1alpha1.HPAConflictPolicyWarn {
					klog.Warningf("[%s] HPA conflict for %s/%s/%s, proceeding with caution: %s",
						mode, workloadRec.Namespace, workloadRec.WorkloadKind, workloadRec.WorkloadName, hpaResult.Message)
					r.optimizerEvents.RecordWarningEvent(config, events.ReasonHPAConflictDetected,
						fmt.Sprintf("HPA conflict for %s/%s (proceeding): %s", workloadRec.WorkloadKind, workloadRec.WorkloadName, hpaResult.Message))
				}
			}
		}

		// SAFETY CHECK: Check PDB constraints before processing this workload
		if config.Spec.PDBAwareness != nil && config.Spec.PDBAwareness.Enabled && !config.Spec.DryRun {
			pdbResult, err := r.pdbChecker.CheckPDBSafety(ctx, workloadRec.Namespace, workloadRec.WorkloadKind, workloadRec.WorkloadName, 1)
			if err != nil {
				klog.Warningf("Failed to check PDB for %s/%s/%s: %v", workloadRec.Namespace, workloadRec.WorkloadKind, workloadRec.WorkloadName, err)
			} else if pdbResult.HasPDB && !pdbResult.IsSafe {
				if config.Spec.PDBAwareness.RespectMinAvailable {
					klog.V(3).Infof("[%s] Skipping %s/%s/%s: PDB violation detected - %s",
						mode, workloadRec.Namespace, workloadRec.WorkloadKind, workloadRec.WorkloadName, pdbResult.Message)
					r.optimizerEvents.RecordWarningEvent(config, events.ReasonPDBViolation,
						fmt.Sprintf("PDB violation for %s/%s: %s", workloadRec.WorkloadKind, workloadRec.WorkloadName, pdbResult.Message))
					if err := r.updateCondition(config, optimizerv1alpha1.ConditionTypePDBViolation, optimizerv1alpha1.ConditionTrue, "ViolationDetected", pdbResult.Message); err != nil {
						klog.Warningf("Failed to update condition: %v", err)
					}
					skippedCount++
					continue
				} else {
					klog.Warningf("[%s] PDB violation for %s/%s/%s, proceeding anyway: %s",
						mode, workloadRec.Namespace, workloadRec.WorkloadKind, workloadRec.WorkloadName, pdbResult.Message)
					r.optimizerEvents.RecordWarningEvent(config, events.ReasonPDBViolation,
						fmt.Sprintf("PDB violation for %s/%s (proceeding): %s", workloadRec.WorkloadKind, workloadRec.WorkloadName, pdbResult.Message))
				}
			}
		}

		for _, containerRec := range workloadRec.Containers {
			// Convert to applier format
			rec := &applier.ResourceRecommendation{
				Namespace:         workloadRec.Namespace,
				WorkloadKind:      workloadRec.WorkloadKind,
				WorkloadName:      workloadRec.WorkloadName,
				ContainerName:     containerRec.ContainerName,
				CurrentCPU:        formatCPU(containerRec.CurrentCPU),
				RecommendedCPU:    formatCPU(containerRec.RecommendedCPU),
				CurrentMemory:     formatMemory(containerRec.CurrentMemory),
				RecommendedMemory: formatMemory(containerRec.RecommendedMemory),
			}

			// Skip if no changes needed
			if !rec.HasChanges() {
				klog.V(4).Infof("[%s] Skipping %s/%s/%s: no changes needed",
					mode, rec.Namespace, rec.WorkloadName, rec.ContainerName)
				skippedCount++
				continue
			}

			// SAFETY CHECK: Enforce MaxChangePercent limit
			if resolvedSettings != nil {
				changePercent := containerRec.MaxChangePercent()
				shouldApply, reason := resolvedSettings.ShouldApplyRecommendation(containerRec.Confidence, changePercent)
				if !shouldApply {
					klog.V(3).Infof("[%s] Skipping %s/%s/%s: %s (change=%.1f%%, confidence=%.1f%%)",
						mode, rec.Namespace, rec.WorkloadName, rec.ContainerName, reason, changePercent, containerRec.Confidence)
					r.optimizerEvents.RecordWarningEvent(config, events.ReasonRecommendationSkipped,
						fmt.Sprintf("Skipped %s/%s: %s", rec.WorkloadName, rec.ContainerName, reason))
					skippedCount++
					continue
				}
			}

			// Log recommendation details with cost savings
			savingsInfo := ""
			if containerRec.EstimatedSavings != nil {
				savingsInfo = fmt.Sprintf(", savings=$%.4f/hour ($%.2f/month)",
					containerRec.EstimatedSavings.TotalSavingsPerHour,
					containerRec.EstimatedSavings.SavingsPerMonth)
			}
			klog.V(3).Infof("[%s] Recommendation for %s/%s/%s: CPU %s->%s (P%d), Memory %s->%s (P%d), confidence=%.2f, samples=%d%s",
				mode, rec.Namespace, rec.WorkloadName, rec.ContainerName,
				rec.CurrentCPU, rec.RecommendedCPU, containerRec.CPUPercentile,
				rec.CurrentMemory, rec.RecommendedMemory, containerRec.MemoryPercentile,
				containerRec.Confidence, containerRec.SampleCount, savingsInfo)

			applyResult, err := r.applier.Apply(ctx, rec, config.Spec.DryRun)
			if err != nil {
				klog.Warningf("[%s] Failed to apply recommendation for %s/%s/%s: %v",
					mode, rec.Namespace, rec.WorkloadName, rec.ContainerName, err)
				continue
			}

			if config.Spec.DryRun {
				klog.V(3).Infof("[DRY-RUN] Summary: %d changes would be applied to %s/%s",
					len(applyResult.Changes), applyResult.WorkloadKind, applyResult.WorkloadName)
				for _, change := range applyResult.Changes {
					klog.V(3).Infof("[DRY-RUN]   - %s", change)
				}
			} else if applyResult.Applied {
				appliedCount++
				klog.Infof("[LIVE] Successfully applied changes to %s/%s/%s",
					rec.Namespace, rec.WorkloadName, rec.ContainerName)
			}
		}
	}

	// Record events with savings information
	if config.Spec.DryRun {
		if len(recommendations) > 0 {
			r.optimizerEvents.RecordNormalEvent(config, events.ReasonDryRunSimulated,
				fmt.Sprintf("Dry-run: %d recommendations generated, %d skipped (estimated savings: $%.2f/month)",
					len(recommendations), skippedCount, totalSavingsPerMonth))
		}
	} else if appliedCount > 0 {
		r.optimizerEvents.RecordOptimizationEvent(config, events.ReasonOptimizationApplied,
			fmt.Sprintf("Applied %d resource optimizations (estimated savings: $%.2f/month)",
				appliedCount, totalSavingsPerMonth))
		config.Status.TotalUpdatesApplied += int64(appliedCount)
	}

	config.Status.TotalRecommendations += int64(len(recommendations))

	return nil
}

// formatCPU converts millicores to Kubernetes CPU format (e.g., 100 -> "100m", 1000 -> "1")
func formatCPU(millicores int64) string {
	if millicores >= 1000 && millicores%1000 == 0 {
		return fmt.Sprintf("%d", millicores/1000)
	}
	return fmt.Sprintf("%dm", millicores)
}

// formatMemory converts bytes to Kubernetes memory format (e.g., "128Mi", "1Gi")
func formatMemory(bytes int64) string {
	const (
		Ki = 1024
		Mi = Ki * 1024
		Gi = Mi * 1024
	)

	if bytes >= Gi && bytes%Gi == 0 {
		return fmt.Sprintf("%dGi", bytes/Gi)
	}
	if bytes >= Mi {
		return fmt.Sprintf("%dMi", bytes/Mi)
	}
	if bytes >= Ki {
		return fmt.Sprintf("%dKi", bytes/Ki)
	}
	return fmt.Sprintf("%d", bytes)
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

func boolToConditionStatus(b bool) optimizerv1alpha1.ConditionStatus {
	if b {
		return optimizerv1alpha1.ConditionTrue
	}
	return optimizerv1alpha1.ConditionFalse
}

func maintenanceWindowMessage(inWindow bool) string {
	if inWindow {
		return "Currently in maintenance window"
	}
	return "Outside maintenance window"
}
