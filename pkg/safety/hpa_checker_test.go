package safety

import (
	"context"
	"testing"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestHPAChecker_NoHPA(t *testing.T) {
	client := fake.NewSimpleClientset()
	checker := NewHPAChecker(client)

	result, err := checker.CheckHPAConflict(context.Background(), "default", "Deployment", "test-app")
	if err != nil {
		t.Fatalf("CheckHPAConflict failed: %v", err)
	}

	if result.HasConflict {
		t.Error("Expected no conflict when no HPA exists")
	}
}

func TestHPAChecker_HPAWithCPUMetric(t *testing.T) {
	hpa := createTestHPA("test-hpa", "default", "Deployment", "test-app", []string{"cpu"})
	client := fake.NewSimpleClientset(hpa)
	checker := NewHPAChecker(client)

	result, err := checker.CheckHPAConflict(context.Background(), "default", "Deployment", "test-app")
	if err != nil {
		t.Fatalf("CheckHPAConflict failed: %v", err)
	}

	if !result.HasConflict {
		t.Error("Expected conflict when HPA manages CPU")
	}

	if result.ConflictingHPA != "test-hpa" {
		t.Errorf("Expected conflicting HPA 'test-hpa', got %s", result.ConflictingHPA)
	}

	if len(result.ConflictMetrics) != 1 || result.ConflictMetrics[0] != "cpu" {
		t.Errorf("Expected conflict metrics [cpu], got %v", result.ConflictMetrics)
	}
}

func TestHPAChecker_HPAWithMemoryMetric(t *testing.T) {
	hpa := createTestHPA("test-hpa", "default", "Deployment", "test-app", []string{"memory"})
	client := fake.NewSimpleClientset(hpa)
	checker := NewHPAChecker(client)

	result, err := checker.CheckHPAConflict(context.Background(), "default", "Deployment", "test-app")
	if err != nil {
		t.Fatalf("CheckHPAConflict failed: %v", err)
	}

	if !result.HasConflict {
		t.Error("Expected conflict when HPA manages memory")
	}

	if len(result.ConflictMetrics) != 1 || result.ConflictMetrics[0] != "memory" {
		t.Errorf("Expected conflict metrics [memory], got %v", result.ConflictMetrics)
	}
}

func TestHPAChecker_HPAWithBothMetrics(t *testing.T) {
	hpa := createTestHPA("test-hpa", "default", "Deployment", "test-app", []string{"cpu", "memory"})
	client := fake.NewSimpleClientset(hpa)
	checker := NewHPAChecker(client)

	result, err := checker.CheckHPAConflict(context.Background(), "default", "Deployment", "test-app")
	if err != nil {
		t.Fatalf("CheckHPAConflict failed: %v", err)
	}

	if !result.HasConflict {
		t.Error("Expected conflict when HPA manages both CPU and memory")
	}

	if len(result.ConflictMetrics) != 2 {
		t.Errorf("Expected 2 conflict metrics, got %v", result.ConflictMetrics)
	}
}

func TestHPAChecker_HPAWithCustomMetrics(t *testing.T) {
	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hpa",
			Namespace: "default",
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				Kind: "Deployment",
				Name: "test-app",
			},
			MinReplicas: int32Ptr(1),
			MaxReplicas: 10,
			Metrics: []autoscalingv2.MetricSpec{
				{
					Type: autoscalingv2.PodsMetricSourceType,
					Pods: &autoscalingv2.PodsMetricSource{
						Metric: autoscalingv2.MetricIdentifier{
							Name: "http_requests",
						},
						Target: autoscalingv2.MetricTarget{
							Type:         autoscalingv2.AverageValueMetricType,
							AverageValue: resource.NewQuantity(1000, resource.DecimalSI),
						},
					},
				},
			},
		},
	}
	client := fake.NewSimpleClientset(hpa)
	checker := NewHPAChecker(client)

	result, err := checker.CheckHPAConflict(context.Background(), "default", "Deployment", "test-app")
	if err != nil {
		t.Fatalf("CheckHPAConflict failed: %v", err)
	}

	if result.HasConflict {
		t.Error("Expected no conflict when HPA uses only custom metrics")
	}
}

func TestHPAChecker_DifferentWorkload(t *testing.T) {
	hpa := createTestHPA("test-hpa", "default", "Deployment", "other-app", []string{"cpu"})
	client := fake.NewSimpleClientset(hpa)
	checker := NewHPAChecker(client)

	result, err := checker.CheckHPAConflict(context.Background(), "default", "Deployment", "test-app")
	if err != nil {
		t.Fatalf("CheckHPAConflict failed: %v", err)
	}

	if result.HasConflict {
		t.Error("Expected no conflict when HPA targets different workload")
	}
}

func TestHPAChecker_DifferentNamespace(t *testing.T) {
	hpa := createTestHPA("test-hpa", "other-namespace", "Deployment", "test-app", []string{"cpu"})
	client := fake.NewSimpleClientset(hpa)
	checker := NewHPAChecker(client)

	result, err := checker.CheckHPAConflict(context.Background(), "default", "Deployment", "test-app")
	if err != nil {
		t.Fatalf("CheckHPAConflict failed: %v", err)
	}

	if result.HasConflict {
		t.Error("Expected no conflict when HPA is in different namespace")
	}
}

func TestHPAChecker_StatefulSet(t *testing.T) {
	hpa := createTestHPA("test-hpa", "default", "StatefulSet", "test-statefulset", []string{"cpu"})
	client := fake.NewSimpleClientset(hpa)
	checker := NewHPAChecker(client)

	result, err := checker.CheckHPAConflict(context.Background(), "default", "StatefulSet", "test-statefulset")
	if err != nil {
		t.Fatalf("CheckHPAConflict failed: %v", err)
	}

	if !result.HasConflict {
		t.Error("Expected conflict for StatefulSet with HPA")
	}
}

func TestHPAChecker_GetHPATargetRef(t *testing.T) {
	hpa := createTestHPA("test-hpa", "default", "Deployment", "test-app", []string{"cpu"})
	client := fake.NewSimpleClientset(hpa)
	checker := NewHPAChecker(client)

	targetRef, err := checker.GetHPATargetRef(context.Background(), "default", "test-hpa")
	if err != nil {
		t.Fatalf("GetHPATargetRef failed: %v", err)
	}

	if targetRef.Kind != "Deployment" {
		t.Errorf("Expected Kind 'Deployment', got %s", targetRef.Kind)
	}

	if targetRef.Name != "test-app" {
		t.Errorf("Expected Name 'test-app', got %s", targetRef.Name)
	}
}

func createTestHPA(name, namespace, kind, targetName string, metrics []string) *autoscalingv2.HorizontalPodAutoscaler {
	metricSpecs := make([]autoscalingv2.MetricSpec, 0, len(metrics))
	for _, metric := range metrics {
		metricSpecs = append(metricSpecs, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceName(metric),
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: int32Ptr(80),
				},
			},
		})
	}

	return &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				Kind: kind,
				Name: targetName,
			},
			MinReplicas: int32Ptr(1),
			MaxReplicas: 10,
			Metrics:     metricSpecs,
		},
	}
}

func int32Ptr(i int32) *int32 {
	return &i
}
