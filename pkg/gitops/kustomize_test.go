package gitops

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

func TestGenerateStrategicMerge(t *testing.T) {
	gen := NewKustomizeGenerator()

	tests := []struct {
		name    string
		rec     ResourceRecommendation
		wantErr bool
		verify  func(t *testing.T, patch string)
	}{
		{
			name: "basic deployment with requests only",
			rec: ResourceRecommendation{
				Namespace:         "production",
				Name:              "api-server",
				Kind:              "Deployment",
				ContainerName:     "api",
				RecommendedCPU:    500,
				RecommendedMemory: 512 * 1024 * 1024,
				SetLimits:         false,
			},
			wantErr: false,
			verify: func(t *testing.T, patch string) {
				var p KustomizePatch
				if err := yaml.Unmarshal([]byte(patch), &p); err != nil {
					t.Fatalf("Failed to unmarshal patch: %v", err)
				}

				if p.APIVersion != "apps/v1" {
					t.Errorf("Expected apiVersion apps/v1, got %s", p.APIVersion)
				}
				if p.Kind != "Deployment" {
					t.Errorf("Expected kind Deployment, got %s", p.Kind)
				}
				if p.Metadata.Name != "api-server" {
					t.Errorf("Expected name api-server, got %s", p.Metadata.Name)
				}
				if p.Metadata.Namespace != "production" {
					t.Errorf("Expected namespace production, got %s", p.Metadata.Namespace)
				}

				// Verify spec contains resources
				if p.Spec == nil {
					t.Fatal("Spec is nil")
				}
			},
		},
		{
			name: "statefulset with limits",
			rec: ResourceRecommendation{
				Namespace:         "database",
				Name:              "postgres",
				Kind:              "StatefulSet",
				ContainerName:     "postgres",
				RecommendedCPU:    2000,
				RecommendedMemory: 4 * 1024 * 1024 * 1024,
				SetLimits:         true,
			},
			wantErr: false,
			verify: func(t *testing.T, patch string) {
				if !strings.Contains(patch, "limits") {
					t.Error("Expected patch to contain limits")
				}
				if !strings.Contains(patch, "requests") {
					t.Error("Expected patch to contain requests")
				}
			},
		},
		{
			name: "daemonset with millicores",
			rec: ResourceRecommendation{
				Namespace:         "kube-system",
				Name:              "fluentd",
				Kind:              "DaemonSet",
				ContainerName:     "fluentd",
				RecommendedCPU:    100,
				RecommendedMemory: 128 * 1024 * 1024,
				SetLimits:         false,
			},
			wantErr: false,
			verify: func(t *testing.T, patch string) {
				if !strings.Contains(patch, "100m") {
					t.Error("Expected CPU to be formatted as 100m")
				}
			},
		},
		{
			name: "missing namespace",
			rec: ResourceRecommendation{
				Name:              "api-server",
				Kind:              "Deployment",
				ContainerName:     "api",
				RecommendedCPU:    500,
				RecommendedMemory: 512 * 1024 * 1024,
			},
			wantErr: true,
		},
		{
			name: "missing name",
			rec: ResourceRecommendation{
				Namespace:         "production",
				Kind:              "Deployment",
				ContainerName:     "api",
				RecommendedCPU:    500,
				RecommendedMemory: 512 * 1024 * 1024,
			},
			wantErr: true,
		},
		{
			name: "negative CPU",
			rec: ResourceRecommendation{
				Namespace:         "production",
				Name:              "api-server",
				Kind:              "Deployment",
				ContainerName:     "api",
				RecommendedCPU:    -500,
				RecommendedMemory: 512 * 1024 * 1024,
			},
			wantErr: true,
		},
		{
			name: "zero memory",
			rec: ResourceRecommendation{
				Namespace:         "production",
				Name:              "api-server",
				Kind:              "Deployment",
				ContainerName:     "api",
				RecommendedCPU:    500,
				RecommendedMemory: 0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patch, err := gen.GenerateStrategicMerge(tt.rec)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if patch == "" {
				t.Error("Expected non-empty patch")
			}

			if tt.verify != nil {
				tt.verify(t, patch)
			}
		})
	}
}

func TestGenerateJSON6902(t *testing.T) {
	gen := NewKustomizeGenerator()

	tests := []struct {
		name    string
		rec     ResourceRecommendation
		wantErr bool
		verify  func(t *testing.T, patch string)
	}{
		{
			name: "basic JSON 6902 patch",
			rec: ResourceRecommendation{
				Namespace:         "production",
				Name:              "api-server",
				Kind:              "Deployment",
				ContainerName:     "api",
				RecommendedCPU:    500,
				RecommendedMemory: 512 * 1024 * 1024,
				SetLimits:         false,
			},
			wantErr: false,
			verify: func(t *testing.T, patch string) {
				var patches []JSON6902Patch
				if err := json.Unmarshal([]byte(patch), &patches); err != nil {
					t.Fatalf("Failed to unmarshal JSON: %v", err)
				}

				// Should have 2 patches (CPU and memory requests)
				if len(patches) != 2 {
					t.Errorf("Expected 2 patches, got %d", len(patches))
				}

				// Verify all patches are replace operations
				for _, p := range patches {
					if p.Op != "replace" {
						t.Errorf("Expected op 'replace', got '%s'", p.Op)
					}
					if p.Path == "" {
						t.Error("Path should not be empty")
					}
					if p.Value == nil {
						t.Error("Value should not be nil")
					}
				}
			},
		},
		{
			name: "JSON 6902 with limits",
			rec: ResourceRecommendation{
				Namespace:         "production",
				Name:              "api-server",
				Kind:              "Deployment",
				ContainerName:     "api",
				RecommendedCPU:    1000,
				RecommendedMemory: 1024 * 1024 * 1024,
				SetLimits:         true,
			},
			wantErr: false,
			verify: func(t *testing.T, patch string) {
				var patches []JSON6902Patch
				if err := json.Unmarshal([]byte(patch), &patches); err != nil {
					t.Fatalf("Failed to unmarshal JSON: %v", err)
				}

				// Should have 4 patches (CPU/memory requests + limits)
				if len(patches) != 4 {
					t.Errorf("Expected 4 patches, got %d", len(patches))
				}

				// Verify paths contain both requests and limits
				hasRequests := false
				hasLimits := false
				for _, p := range patches {
					if strings.Contains(p.Path, "requests") {
						hasRequests = true
					}
					if strings.Contains(p.Path, "limits") {
						hasLimits = true
					}
				}

				if !hasRequests {
					t.Error("Expected patches to contain requests paths")
				}
				if !hasLimits {
					t.Error("Expected patches to contain limits paths")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patch, err := gen.GenerateJSON6902(tt.rec)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if tt.verify != nil {
				tt.verify(t, patch)
			}
		})
	}
}

func TestGenerateKustomization(t *testing.T) {
	gen := NewKustomizeGenerator()

	tests := []struct {
		name       string
		patchFiles []string
		wantErr    bool
		verify     func(t *testing.T, kustomization string)
	}{
		{
			name:       "single patch file",
			patchFiles: []string{"patch-production-api-server-0.yaml"},
			wantErr:    false,
			verify: func(t *testing.T, kustomization string) {
				if !strings.Contains(kustomization, "patch-production-api-server-0.yaml") {
					t.Error("Expected kustomization to contain patch filename")
				}
				if !strings.Contains(kustomization, "kustomize.config.k8s.io/v1beta1") {
					t.Error("Expected kustomization to contain apiVersion")
				}
			},
		},
		{
			name: "multiple patch files",
			patchFiles: []string{
				"patch-production-api-server-0.yaml",
				"patch-database-postgres-0.yaml",
				"patch-cache-redis-0.yaml",
			},
			wantErr: false,
			verify: func(t *testing.T, kustomization string) {
				for _, file := range []string{
					"patch-production-api-server-0.yaml",
					"patch-database-postgres-0.yaml",
					"patch-cache-redis-0.yaml",
				} {
					if !strings.Contains(kustomization, file) {
						t.Errorf("Expected kustomization to contain %s", file)
					}
				}
			},
		},
		{
			name:       "empty patch files",
			patchFiles: []string{},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kustomization, err := gen.GenerateKustomization(tt.patchFiles)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if tt.verify != nil {
				tt.verify(t, kustomization)
			}
		})
	}
}

func TestFormatCPU(t *testing.T) {
	tests := []struct {
		millicores int64
		expected   string
	}{
		{100, "100m"},
		{500, "500m"},
		{999, "999m"},
		{1000, "1.00"},
		{1500, "1.50"},
		{2000, "2.00"},
		{2500, "2.50"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatCPU(tt.millicores)
			if result != tt.expected {
				t.Errorf("formatCPU(%d) = %s, want %s", tt.millicores, result, tt.expected)
			}
		})
	}
}

func TestFormatMemory(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{100, "100"},
		{1024, "1Ki"},
		{1024 * 1024, "1Mi"},
		{512 * 1024 * 1024, "512Mi"},
		{1024 * 1024 * 1024, "1Gi"},
		{2 * 1024 * 1024 * 1024, "2Gi"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatMemory(tt.bytes)
			if result != tt.expected {
				t.Errorf("formatMemory(%d) = %s, want %s", tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestGetAPIVersion(t *testing.T) {
	tests := []struct {
		kind     string
		expected string
	}{
		{"Deployment", "apps/v1"},
		{"StatefulSet", "apps/v1"},
		{"DaemonSet", "apps/v1"},
		{"ReplicaSet", "apps/v1"},
		{"Pod", "v1"},
		{"CronJob", "batch/v1"},
		{"Job", "batch/v1"},
		{"Unknown", "apps/v1"},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			result := getAPIVersion(tt.kind)
			if result != tt.expected {
				t.Errorf("getAPIVersion(%s) = %s, want %s", tt.kind, result, tt.expected)
			}
		})
	}
}

func TestValidateRecommendation(t *testing.T) {
	tests := []struct {
		name    string
		rec     ResourceRecommendation
		wantErr bool
	}{
		{
			name: "valid recommendation",
			rec: ResourceRecommendation{
				Namespace:         "default",
				Name:              "app",
				Kind:              "Deployment",
				ContainerName:     "app",
				RecommendedCPU:    500,
				RecommendedMemory: 512 * 1024 * 1024,
			},
			wantErr: false,
		},
		{
			name: "missing namespace",
			rec: ResourceRecommendation{
				Name:              "app",
				Kind:              "Deployment",
				ContainerName:     "app",
				RecommendedCPU:    500,
				RecommendedMemory: 512 * 1024 * 1024,
			},
			wantErr: true,
		},
		{
			name: "missing name",
			rec: ResourceRecommendation{
				Namespace:         "default",
				Kind:              "Deployment",
				ContainerName:     "app",
				RecommendedCPU:    500,
				RecommendedMemory: 512 * 1024 * 1024,
			},
			wantErr: true,
		},
		{
			name: "missing kind",
			rec: ResourceRecommendation{
				Namespace:         "default",
				Name:              "app",
				ContainerName:     "app",
				RecommendedCPU:    500,
				RecommendedMemory: 512 * 1024 * 1024,
			},
			wantErr: true,
		},
		{
			name: "missing container name",
			rec: ResourceRecommendation{
				Namespace:         "default",
				Name:              "app",
				Kind:              "Deployment",
				RecommendedCPU:    500,
				RecommendedMemory: 512 * 1024 * 1024,
			},
			wantErr: true,
		},
		{
			name: "zero CPU",
			rec: ResourceRecommendation{
				Namespace:         "default",
				Name:              "app",
				Kind:              "Deployment",
				ContainerName:     "app",
				RecommendedCPU:    0,
				RecommendedMemory: 512 * 1024 * 1024,
			},
			wantErr: true,
		},
		{
			name: "negative memory",
			rec: ResourceRecommendation{
				Namespace:         "default",
				Name:              "app",
				Kind:              "Deployment",
				ContainerName:     "app",
				RecommendedCPU:    500,
				RecommendedMemory: -100,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRecommendation(tt.rec)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}
