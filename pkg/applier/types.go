package applier

import (
	corev1 "k8s.io/api/core/v1"
)

type ResourceRecommendation struct {
	Namespace         string
	WorkloadKind      string
	WorkloadName      string
	ContainerName     string
	CurrentCPU        string
	RecommendedCPU    string
	CurrentMemory     string
	RecommendedMemory string
}

type ApplyResult struct {
	Applied      bool
	DryRun       bool
	WorkloadKind string
	WorkloadName string
	Namespace    string
	Changes      []string
	Error        error
}

func (r *ResourceRecommendation) HasChanges() bool {
	return r.CurrentCPU != r.RecommendedCPU || r.CurrentMemory != r.RecommendedMemory
}

func (r *ResourceRecommendation) GetResourceRequirements() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{},
		Limits:   corev1.ResourceList{},
	}
}
