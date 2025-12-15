package safety

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
)

func TestPDBChecker_NoPDB(t *testing.T) {
	deploy := createTestDeployment("test-app", "default", 3, 3, map[string]string{"app": "test"})
	client := fake.NewSimpleClientset(deploy)
	checker := NewPDBChecker(client)

	result, err := checker.CheckPDBSafety(context.Background(), "default", "Deployment", "test-app", 1)
	if err != nil {
		t.Fatalf("CheckPDBSafety failed: %v", err)
	}

	if result.HasPDB {
		t.Error("Expected no PDB")
	}

	if !result.IsSafe {
		t.Error("Expected safe when no PDB exists")
	}
}

func TestPDBChecker_MinAvailable_Safe(t *testing.T) {
	deploy := createTestDeployment("test-app", "default", 5, 5, map[string]string{"app": "test"})
	minAvail := intstr.FromInt(3)
	pdb := createTestPDB("test-pdb", "default", map[string]string{"app": "test"}, &minAvail, nil)
	client := fake.NewSimpleClientset(deploy, pdb)
	checker := NewPDBChecker(client)

	result, err := checker.CheckPDBSafety(context.Background(), "default", "Deployment", "test-app", 1)
	if err != nil {
		t.Fatalf("CheckPDBSafety failed: %v", err)
	}

	if !result.HasPDB {
		t.Error("Expected PDB to be found")
	}

	if !result.IsSafe {
		t.Error("Expected safe: 5 available - 1 disruption = 4, minAvailable=3")
	}

	if result.MinAvailable != 3 {
		t.Errorf("Expected MinAvailable=3, got %d", result.MinAvailable)
	}
}

func TestPDBChecker_MinAvailable_Unsafe(t *testing.T) {
	deploy := createTestDeployment("test-app", "default", 5, 5, map[string]string{"app": "test"})
	minAvail := intstr.FromInt(4)
	pdb := createTestPDB("test-pdb", "default", map[string]string{"app": "test"}, &minAvail, nil)
	client := fake.NewSimpleClientset(deploy, pdb)
	checker := NewPDBChecker(client)

	result, err := checker.CheckPDBSafety(context.Background(), "default", "Deployment", "test-app", 2)
	if err != nil {
		t.Fatalf("CheckPDBSafety failed: %v", err)
	}

	if result.IsSafe {
		t.Error("Expected unsafe: 5 available - 2 disruption = 3, minAvailable=4")
	}
}

func TestPDBChecker_MaxUnavailable_Safe(t *testing.T) {
	deploy := createTestDeployment("test-app", "default", 5, 5, map[string]string{"app": "test"})
	maxUnavail := intstr.FromInt(2)
	pdb := createTestPDB("test-pdb", "default", map[string]string{"app": "test"}, nil, &maxUnavail)
	client := fake.NewSimpleClientset(deploy, pdb)
	checker := NewPDBChecker(client)

	result, err := checker.CheckPDBSafety(context.Background(), "default", "Deployment", "test-app", 1)
	if err != nil {
		t.Fatalf("CheckPDBSafety failed: %v", err)
	}

	if !result.IsSafe {
		t.Error("Expected safe: 0 currently unavailable + 1 disruption = 1, maxUnavailable=2")
	}

	if result.MaxUnavailable != 2 {
		t.Errorf("Expected MaxUnavailable=2, got %d", result.MaxUnavailable)
	}
}

func TestPDBChecker_MaxUnavailable_Unsafe(t *testing.T) {
	deploy := createTestDeployment("test-app", "default", 5, 5, map[string]string{"app": "test"})
	maxUnavail := intstr.FromInt(1)
	pdb := createTestPDB("test-pdb", "default", map[string]string{"app": "test"}, nil, &maxUnavail)
	client := fake.NewSimpleClientset(deploy, pdb)
	checker := NewPDBChecker(client)

	result, err := checker.CheckPDBSafety(context.Background(), "default", "Deployment", "test-app", 2)
	if err != nil {
		t.Fatalf("CheckPDBSafety failed: %v", err)
	}

	if result.IsSafe {
		t.Error("Expected unsafe: 0 currently unavailable + 2 disruption = 2, maxUnavailable=1")
	}
}

func TestPDBChecker_MinAvailablePercentage(t *testing.T) {
	deploy := createTestDeployment("test-app", "default", 10, 10, map[string]string{"app": "test"})
	minAvail := intstr.FromString("50%")
	pdb := createTestPDB("test-pdb", "default", map[string]string{"app": "test"}, &minAvail, nil)
	client := fake.NewSimpleClientset(deploy, pdb)
	checker := NewPDBChecker(client)

	result, err := checker.CheckPDBSafety(context.Background(), "default", "Deployment", "test-app", 3)
	if err != nil {
		t.Fatalf("CheckPDBSafety failed: %v", err)
	}

	if result.MinAvailable != 5 {
		t.Errorf("Expected MinAvailable=5 (50%% of 10), got %d", result.MinAvailable)
	}

	if !result.IsSafe {
		t.Error("Expected safe: 10 available - 3 disruption = 7, minAvailable=5")
	}
}

func TestPDBChecker_MaxUnavailablePercentage(t *testing.T) {
	deploy := createTestDeployment("test-app", "default", 10, 10, map[string]string{"app": "test"})
	maxUnavail := intstr.FromString("30%")
	pdb := createTestPDB("test-pdb", "default", map[string]string{"app": "test"}, nil, &maxUnavail)
	client := fake.NewSimpleClientset(deploy, pdb)
	checker := NewPDBChecker(client)

	result, err := checker.CheckPDBSafety(context.Background(), "default", "Deployment", "test-app", 2)
	if err != nil {
		t.Fatalf("CheckPDBSafety failed: %v", err)
	}

	if result.MaxUnavailable != 3 {
		t.Errorf("Expected MaxUnavailable=3 (30%% of 10), got %d", result.MaxUnavailable)
	}

	if !result.IsSafe {
		t.Error("Expected safe: 0 currently unavailable + 2 disruption = 2, maxUnavailable=3")
	}
}

func TestPDBChecker_PartiallyAvailable(t *testing.T) {
	deploy := createTestDeployment("test-app", "default", 5, 3, map[string]string{"app": "test"})
	minAvail := intstr.FromInt(2)
	pdb := createTestPDB("test-pdb", "default", map[string]string{"app": "test"}, &minAvail, nil)
	client := fake.NewSimpleClientset(deploy, pdb)
	checker := NewPDBChecker(client)

	result, err := checker.CheckPDBSafety(context.Background(), "default", "Deployment", "test-app", 1)
	if err != nil {
		t.Fatalf("CheckPDBSafety failed: %v", err)
	}

	if !result.IsSafe {
		t.Error("Expected safe: 3 available - 1 disruption = 2, minAvailable=2")
	}

	result2, err := checker.CheckPDBSafety(context.Background(), "default", "Deployment", "test-app", 2)
	if err != nil {
		t.Fatalf("CheckPDBSafety failed: %v", err)
	}

	if result2.IsSafe {
		t.Error("Expected unsafe: 3 available - 2 disruption = 1, minAvailable=2")
	}
}

func TestPDBChecker_StatefulSet(t *testing.T) {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-sts",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: int32Ptr(3),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "test-sts"},
			},
		},
		Status: appsv1.StatefulSetStatus{
			Replicas:          3,
			AvailableReplicas: 3,
		},
	}
	minAvail := intstr.FromInt(2)
	pdb := createTestPDB("test-pdb", "default", map[string]string{"app": "test-sts"}, &minAvail, nil)
	client := fake.NewSimpleClientset(sts, pdb)
	checker := NewPDBChecker(client)

	result, err := checker.CheckPDBSafety(context.Background(), "default", "StatefulSet", "test-sts", 1)
	if err != nil {
		t.Fatalf("CheckPDBSafety failed: %v", err)
	}

	if !result.HasPDB {
		t.Error("Expected PDB to be found for StatefulSet")
	}

	if !result.IsSafe {
		t.Error("Expected safe for StatefulSet")
	}
}

func TestPDBChecker_CalculateSafeDisruptionBudget_MinAvailable(t *testing.T) {
	deploy := createTestDeployment("test-app", "default", 5, 5, map[string]string{"app": "test"})
	minAvail := intstr.FromInt(3)
	pdb := createTestPDB("test-pdb", "default", map[string]string{"app": "test"}, &minAvail, nil)
	client := fake.NewSimpleClientset(deploy, pdb)
	checker := NewPDBChecker(client)

	safeBudget, err := checker.CalculateSafeDisruptionBudget(context.Background(), "default", "Deployment", "test-app")
	if err != nil {
		t.Fatalf("CalculateSafeDisruptionBudget failed: %v", err)
	}

	if safeBudget != 2 {
		t.Errorf("Expected safe disruption budget of 2 (5 available - 3 minAvailable), got %d", safeBudget)
	}
}

func TestPDBChecker_CalculateSafeDisruptionBudget_MaxUnavailable(t *testing.T) {
	deploy := createTestDeployment("test-app", "default", 5, 5, map[string]string{"app": "test"})
	maxUnavail := intstr.FromInt(2)
	pdb := createTestPDB("test-pdb", "default", map[string]string{"app": "test"}, nil, &maxUnavail)
	client := fake.NewSimpleClientset(deploy, pdb)
	checker := NewPDBChecker(client)

	safeBudget, err := checker.CalculateSafeDisruptionBudget(context.Background(), "default", "Deployment", "test-app")
	if err != nil {
		t.Fatalf("CalculateSafeDisruptionBudget failed: %v", err)
	}

	if safeBudget != 2 {
		t.Errorf("Expected safe disruption budget of 2 (maxUnavailable - 0 current unavailable), got %d", safeBudget)
	}
}

func TestPDBChecker_CalculateSafeDisruptionBudget_NoPDB(t *testing.T) {
	deploy := createTestDeployment("test-app", "default", 5, 5, map[string]string{"app": "test"})
	client := fake.NewSimpleClientset(deploy)
	checker := NewPDBChecker(client)

	safeBudget, err := checker.CalculateSafeDisruptionBudget(context.Background(), "default", "Deployment", "test-app")
	if err != nil {
		t.Fatalf("CalculateSafeDisruptionBudget failed: %v", err)
	}

	if safeBudget != 5 {
		t.Errorf("Expected safe disruption budget to equal total replicas when no PDB, got %d", safeBudget)
	}
}

func createTestPDB(name, namespace string, selector map[string]string, minAvailable, maxUnavailable *intstr.IntOrString) *policyv1.PodDisruptionBudget {
	return &policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: policyv1.PodDisruptionBudgetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: selector,
			},
			MinAvailable:   minAvailable,
			MaxUnavailable: maxUnavailable,
		},
	}
}
