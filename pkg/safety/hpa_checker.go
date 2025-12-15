package safety

import (
	"context"
	"fmt"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type HPAConflictResult struct {
	HasConflict     bool
	ConflictingHPA  string
	ConflictMetrics []string
	Message         string
}

type HPAChecker struct {
	kubeClient kubernetes.Interface
}

func NewHPAChecker(kubeClient kubernetes.Interface) *HPAChecker {
	return &HPAChecker{
		kubeClient: kubeClient,
	}
}

func (h *HPAChecker) CheckHPAConflict(ctx context.Context, namespace, kind, name string) (*HPAConflictResult, error) {
	hpaList, err := h.kubeClient.AutoscalingV2().HorizontalPodAutoscalers(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list HPAs: %v", err)
	}

	for _, hpa := range hpaList.Items {
		if h.isTargetingWorkload(&hpa, kind, name) {
			conflictingMetrics := h.getConflictingMetrics(&hpa)
			if len(conflictingMetrics) > 0 {
				klog.V(3).Infof("HPA conflict detected: %s targets %s/%s with metrics %v",
					hpa.Name, kind, name, conflictingMetrics)
				return &HPAConflictResult{
					HasConflict:     true,
					ConflictingHPA:  hpa.Name,
					ConflictMetrics: conflictingMetrics,
					Message: fmt.Sprintf("HPA %s manages %s metrics for %s/%s",
						hpa.Name, conflictingMetrics, kind, name),
				}, nil
			}
		}
	}

	return &HPAConflictResult{
		HasConflict: false,
		Message:     "No HPA conflicts detected",
	}, nil
}

func (h *HPAChecker) isTargetingWorkload(hpa *autoscalingv2.HorizontalPodAutoscaler, kind, name string) bool {
	if hpa.Spec.ScaleTargetRef.Name != name {
		return false
	}

	targetKind := hpa.Spec.ScaleTargetRef.Kind
	switch kind {
	case "Deployment":
		return targetKind == "Deployment"
	case "StatefulSet":
		return targetKind == "StatefulSet"
	case "DaemonSet":
		return targetKind == "DaemonSet"
	default:
		return false
	}
}

func (h *HPAChecker) getConflictingMetrics(hpa *autoscalingv2.HorizontalPodAutoscaler) []string {
	conflicts := make([]string, 0)
	seen := make(map[string]bool)

	for _, metric := range hpa.Spec.Metrics {
		if metric.Type == autoscalingv2.ResourceMetricSourceType {
			resourceName := string(metric.Resource.Name)
			if resourceName == "cpu" || resourceName == "memory" {
				if !seen[resourceName] {
					conflicts = append(conflicts, resourceName)
					seen[resourceName] = true
				}
			}
		}
	}

	return conflicts
}

func (h *HPAChecker) GetHPATargetRef(ctx context.Context, namespace, hpaName string) (*autoscalingv2.CrossVersionObjectReference, error) {
	hpa, err := h.kubeClient.AutoscalingV2().HorizontalPodAutoscalers(namespace).Get(ctx, hpaName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return &hpa.Spec.ScaleTargetRef, nil
}
