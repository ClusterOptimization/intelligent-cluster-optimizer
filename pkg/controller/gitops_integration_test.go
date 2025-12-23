package controller

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	optimizerv1alpha1 "intelligent-cluster-optimizer/pkg/apis/optimizer/v1alpha1"
	"intelligent-cluster-optimizer/pkg/models"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"
)

func TestGitOpsIntegration_KustomizeExport(t *testing.T) {
	// Create temp directory for GitOps exports
	tmpDir := t.TempDir()

	// Create reconciler
	kubeClient := fake.NewSimpleClientset()
	eventRecorder := record.NewFakeRecorder(100)
	reconciler := NewReconciler(kubeClient, eventRecorder)

	// Populate metrics storage with sample data
	metrics := []models.PodMetric{
		{
			Namespace:  "production",
			PodName:    "api-server-abc123",
			Timestamp:  metav1.Now().Time,
			Containers: []models.ContainerMetric{
				{
					ContainerName: "api",
					UsageCPU:      400,  // 400m
					UsageMemory:   400 * 1024 * 1024, // 400Mi
					RequestCPU:    1000, // 1000m (current request)
					RequestMemory: 1024 * 1024 * 1024, // 1Gi (current request)
				},
			},
		},
	}

	// Add metrics to storage
	for _, metric := range metrics {
		reconciler.GetMetricsStorage().Add(metric)
	}

	// Create OptimizerConfig with GitOps export enabled
	config := &optimizerv1alpha1.OptimizerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-gitops",
			Namespace: "default",
		},
		Spec: optimizerv1alpha1.OptimizerConfigSpec{
			Enabled: true,
			TargetNamespaces: []string{"production"},
			Strategy: optimizerv1alpha1.StrategyBalanced,
			DryRun: true, // Use dry-run to test export without applying changes
			GitOpsExport: &optimizerv1alpha1.GitOpsExportConfig{
				Enabled: true,
				Format: optimizerv1alpha1.GitOpsFormatKustomize,
				OutputPath: tmpDir,
			},
			Recommendations: &optimizerv1alpha1.RecommendationConfig{
				CPUPercentile:    95,
				MemoryPercentile: 95,
				SafetyMargin:     1.2,
				MinSamples:       1, // Low threshold for test
				HistoryDuration:  "1h",
			},
		},
		Status: optimizerv1alpha1.OptimizerConfigStatus{
			Phase: optimizerv1alpha1.OptimizerPhaseActive,
		},
	}

	// Reconcile
	ctx := context.Background()
	result, err := reconciler.Reconcile(ctx, config)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	if !result.Updated {
		t.Error("Expected reconciliation to update status")
	}

	// Verify GitOps files were created
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read output directory: %v", err)
	}

	if len(files) == 0 {
		t.Error("Expected GitOps files to be generated, but directory is empty")
	}

	// Check for kustomization.yaml
	kustomizationPath := filepath.Join(tmpDir, "kustomization.yaml")
	if _, err := os.Stat(kustomizationPath); os.IsNotExist(err) {
		t.Error("Expected kustomization.yaml to be generated")
	} else {
		// Read and verify content
		content, err := os.ReadFile(kustomizationPath)
		if err != nil {
			t.Fatalf("Failed to read kustomization.yaml: %v", err)
		}

		contentStr := string(content)
		if len(contentStr) == 0 {
			t.Error("kustomization.yaml is empty")
		}

		t.Logf("Generated kustomization.yaml:\n%s", contentStr)
	}

	// Check for patch files
	hasPatchFile := false
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".yaml" && file.Name() != "kustomization.yaml" {
			hasPatchFile = true
			t.Logf("Generated patch file: %s", file.Name())

			// Read patch content
			patchPath := filepath.Join(tmpDir, file.Name())
			content, err := os.ReadFile(patchPath)
			if err != nil {
				t.Errorf("Failed to read patch file %s: %v", file.Name(), err)
				continue
			}

			t.Logf("Patch content:\n%s", string(content))
		}
	}

	if !hasPatchFile {
		t.Error("Expected at least one patch file to be generated")
	}

	// Verify event was recorded
	select {
	case event := <-eventRecorder.Events:
		if event == "" {
			t.Error("Expected event to be recorded")
		}
		t.Logf("Recorded event: %s", event)
	default:
		t.Log("No event recorded (this might be expected if no changes needed)")
	}
}

func TestGitOpsIntegration_HelmExport(t *testing.T) {
	tmpDir := t.TempDir()

	kubeClient := fake.NewSimpleClientset()
	eventRecorder := record.NewFakeRecorder(100)
	reconciler := NewReconciler(kubeClient, eventRecorder)

	// Populate metrics with sample data
	metrics := []models.PodMetric{
		{
			Namespace: "production",
			PodName:   "database-xyz789",
			Timestamp: metav1.Now().Time,
			Containers: []models.ContainerMetric{
				{
					ContainerName: "postgres",
					UsageCPU:      1800, // 1.8 cores
					UsageMemory:   3 * 1024 * 1024 * 1024, // 3Gi
					RequestCPU:    2000, // 2 cores
					RequestMemory: 4 * 1024 * 1024 * 1024, // 4Gi
				},
			},
		},
	}

	for _, metric := range metrics {
		reconciler.GetMetricsStorage().Add(metric)
	}

	config := &optimizerv1alpha1.OptimizerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-helm-export",
			Namespace: "default",
		},
		Spec: optimizerv1alpha1.OptimizerConfigSpec{
			Enabled: true,
			TargetNamespaces: []string{"production"},
			Strategy: optimizerv1alpha1.StrategyBalanced,
			DryRun: true,
			GitOpsExport: &optimizerv1alpha1.GitOpsExportConfig{
				Enabled: true,
				Format: optimizerv1alpha1.GitOpsFormatHelm,
				OutputPath: tmpDir,
			},
			Recommendations: &optimizerv1alpha1.RecommendationConfig{
				CPUPercentile:    95,
				MemoryPercentile: 95,
				SafetyMargin:     1.2,
				MinSamples:       1,
				HistoryDuration:  "1h",
			},
		},
		Status: optimizerv1alpha1.OptimizerConfigStatus{
			Phase: optimizerv1alpha1.OptimizerPhaseActive,
		},
	}

	ctx := context.Background()
	result, err := reconciler.Reconcile(ctx, config)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	if !result.Updated {
		t.Error("Expected reconciliation to update status")
	}

	// Verify values.yaml was created
	valuesPath := filepath.Join(tmpDir, "values.yaml")
	if _, err := os.Stat(valuesPath); os.IsNotExist(err) {
		t.Error("Expected values.yaml to be generated")
	} else {
		content, err := os.ReadFile(valuesPath)
		if err != nil {
			t.Fatalf("Failed to read values.yaml: %v", err)
		}

		contentStr := string(content)
		if len(contentStr) == 0 {
			t.Error("values.yaml is empty")
		}

		t.Logf("Generated values.yaml:\n%s", contentStr)

		// Verify Helm values format
		if !contains(contentStr, "resources:") {
			t.Error("Expected 'resources:' section in Helm values")
		}
	}
}

func TestGitOpsIntegration_DisabledExport(t *testing.T) {
	tmpDir := t.TempDir()

	kubeClient := fake.NewSimpleClientset()
	eventRecorder := record.NewFakeRecorder(100)
	reconciler := NewReconciler(kubeClient, eventRecorder)

	config := &optimizerv1alpha1.OptimizerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-disabled-export",
			Namespace: "default",
		},
		Spec: optimizerv1alpha1.OptimizerConfigSpec{
			Enabled: true,
			TargetNamespaces: []string{"production"},
			DryRun: true,
			GitOpsExport: &optimizerv1alpha1.GitOpsExportConfig{
				Enabled: false, // Disabled
				OutputPath: tmpDir,
			},
		},
		Status: optimizerv1alpha1.OptimizerConfigStatus{
			Phase: optimizerv1alpha1.OptimizerPhaseActive,
		},
	}

	ctx := context.Background()
	_, err := reconciler.Reconcile(ctx, config)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	// Verify no files were created
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read output directory: %v", err)
	}

	if len(files) > 0 {
		t.Errorf("Expected no files to be generated when GitOps export is disabled, found %d files", len(files))
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
