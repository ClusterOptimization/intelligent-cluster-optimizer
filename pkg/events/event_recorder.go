package events

import (
	optimizerv1alpha1 "intelligent-cluster-optimizer/pkg/apis/optimizer/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
)

const (
	ReasonOptimizationApplied      = "OptimizationApplied"
	ReasonDryRunSimulated          = "DryRunSimulated"
	ReasonHPAConflictDetected      = "HPAConflictDetected"
	ReasonPDBViolation             = "PDBViolation"
	ReasonMaintenanceWindowSkipped = "MaintenanceWindowSkipped"
	ReasonCircuitBreakerOpen       = "CircuitBreakerOpen"
	ReasonScalingStarted           = "ScalingStarted"
	ReasonScalingCompleted         = "ScalingCompleted"
	ReasonScalingFailed            = "ScalingFailed"
)

type OptimizerEventRecorder struct {
	recorder record.EventRecorder
}

func NewOptimizerEventRecorder(recorder record.EventRecorder) *OptimizerEventRecorder {
	return &OptimizerEventRecorder{recorder: recorder}
}

func (e *OptimizerEventRecorder) RecordOptimizationEvent(config *optimizerv1alpha1.OptimizerConfig, reason, message string) {
	if e.recorder != nil {
		e.recorder.Event(config, corev1.EventTypeNormal, reason, message)
	}
}

func (e *OptimizerEventRecorder) RecordWarningEvent(config *optimizerv1alpha1.OptimizerConfig, reason, message string) {
	if e.recorder != nil {
		e.recorder.Event(config, corev1.EventTypeWarning, reason, message)
	}
}

func (e *OptimizerEventRecorder) RecordNormalEvent(config *optimizerv1alpha1.OptimizerConfig, reason, message string) {
	if e.recorder != nil {
		e.recorder.Event(config, corev1.EventTypeNormal, reason, message)
	}
}
