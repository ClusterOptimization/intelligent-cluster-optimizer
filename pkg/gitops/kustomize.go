package gitops

import (
	"encoding/json"
	"fmt"

	"gopkg.in/yaml.v2"
)

// DefaultKustomizeGenerator is the default implementation of KustomizeGenerator
type DefaultKustomizeGenerator struct{}

// NewKustomizeGenerator creates a new Kustomize generator
func NewKustomizeGenerator() KustomizeGenerator {
	return &DefaultKustomizeGenerator{}
}

// GenerateStrategicMerge generates a strategic merge patch
func (g *DefaultKustomizeGenerator) GenerateStrategicMerge(rec ResourceRecommendation) (string, error) {
	if err := validateRecommendation(rec); err != nil {
		return "", err
	}

	// Build the patch structure
	patch := KustomizePatch{
		APIVersion: getAPIVersion(rec.Kind),
		Kind:       rec.Kind,
		Metadata: KustomizeMetadata{
			Name:      rec.Name,
			Namespace: rec.Namespace,
		},
		Spec: buildPatchSpec(rec),
	}

	// Convert to YAML
	yamlBytes, err := yaml.Marshal(patch)
	if err != nil {
		return "", fmt.Errorf("failed to marshal patch to YAML: %w", err)
	}

	return string(yamlBytes), nil
}

// GenerateJSON6902 generates a JSON 6902 patch
func (g *DefaultKustomizeGenerator) GenerateJSON6902(rec ResourceRecommendation) (string, error) {
	if err := validateRecommendation(rec); err != nil {
		return "", err
	}

	var patches []JSON6902Patch

	// Find container index (assume 0 if not specified, in real scenario would need to know)
	containerIndex := 0

	// CPU request patch
	cpuPath := fmt.Sprintf("/spec/template/spec/containers/%d/resources/requests/cpu", containerIndex)
	patches = append(patches, JSON6902Patch{
		Op:    "replace",
		Path:  cpuPath,
		Value: formatCPU(rec.RecommendedCPU),
	})

	// Memory request patch
	memPath := fmt.Sprintf("/spec/template/spec/containers/%d/resources/requests/memory", containerIndex)
	patches = append(patches, JSON6902Patch{
		Op:    "replace",
		Path:  memPath,
		Value: formatMemory(rec.RecommendedMemory),
	})

	// If SetLimits is true, also patch limits
	if rec.SetLimits {
		cpuLimitPath := fmt.Sprintf("/spec/template/spec/containers/%d/resources/limits/cpu", containerIndex)
		patches = append(patches, JSON6902Patch{
			Op:    "replace",
			Path:  cpuLimitPath,
			Value: formatCPU(rec.RecommendedCPU),
		})

		memLimitPath := fmt.Sprintf("/spec/template/spec/containers/%d/resources/limits/memory", containerIndex)
		patches = append(patches, JSON6902Patch{
			Op:    "replace",
			Path:  memLimitPath,
			Value: formatMemory(rec.RecommendedMemory),
		})
	}

	// Convert to JSON
	jsonBytes, err := json.MarshalIndent(patches, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal patch to JSON: %w", err)
	}

	return string(jsonBytes), nil
}

// GenerateKustomization generates a kustomization.yaml file
func (g *DefaultKustomizeGenerator) GenerateKustomization(patchFiles []string) (string, error) {
	if len(patchFiles) == 0 {
		return "", fmt.Errorf("no patch files provided")
	}

	kustomization := map[string]interface{}{
		"apiVersion": "kustomize.config.k8s.io/v1beta1",
		"kind":       "Kustomization",
		"patches":    patchFiles,
	}

	yamlBytes, err := yaml.Marshal(kustomization)
	if err != nil {
		return "", fmt.Errorf("failed to marshal kustomization to YAML: %w", err)
	}

	return string(yamlBytes), nil
}

// buildPatchSpec builds the spec section of the patch
func buildPatchSpec(rec ResourceRecommendation) interface{} {
	resources := map[string]interface{}{
		"requests": map[string]string{
			"cpu":    formatCPU(rec.RecommendedCPU),
			"memory": formatMemory(rec.RecommendedMemory),
		},
	}

	// Add limits if requested
	if rec.SetLimits {
		resources["limits"] = map[string]string{
			"cpu":    formatCPU(rec.RecommendedCPU),
			"memory": formatMemory(rec.RecommendedMemory),
		}
	}

	// Find the container in the list
	containers := []map[string]interface{}{
		{
			"name":      rec.ContainerName,
			"resources": resources,
		},
	}

	return map[string]interface{}{
		"template": map[string]interface{}{
			"spec": map[string]interface{}{
				"containers": containers,
			},
		},
	}
}

// getAPIVersion returns the API version for a given kind
func getAPIVersion(kind string) string {
	switch kind {
	case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet":
		return "apps/v1"
	case "Pod":
		return "v1"
	case "CronJob":
		return "batch/v1"
	case "Job":
		return "batch/v1"
	default:
		return "apps/v1"
	}
}

// formatCPU formats CPU value in Kubernetes format
func formatCPU(millicores int64) string {
	if millicores < 1000 {
		return fmt.Sprintf("%dm", millicores)
	}
	// Convert to cores with precision
	cores := float64(millicores) / 1000.0
	return fmt.Sprintf("%.2f", cores)
}

// formatMemory formats memory value in Kubernetes format
func formatMemory(bytes int64) string {
	const (
		Ki = 1024
		Mi = Ki * 1024
		Gi = Mi * 1024
	)

	switch {
	case bytes >= Gi:
		return fmt.Sprintf("%dGi", bytes/Gi)
	case bytes >= Mi:
		return fmt.Sprintf("%dMi", bytes/Mi)
	case bytes >= Ki:
		return fmt.Sprintf("%dKi", bytes/Ki)
	default:
		return fmt.Sprintf("%d", bytes)
	}
}

// validateRecommendation validates a resource recommendation
func validateRecommendation(rec ResourceRecommendation) error {
	if rec.Namespace == "" {
		return fmt.Errorf("namespace is required")
	}
	if rec.Name == "" {
		return fmt.Errorf("name is required")
	}
	if rec.Kind == "" {
		return fmt.Errorf("kind is required")
	}
	if rec.ContainerName == "" {
		return fmt.Errorf("container name is required")
	}
	if rec.RecommendedCPU <= 0 {
		return fmt.Errorf("recommended CPU must be positive")
	}
	if rec.RecommendedMemory <= 0 {
		return fmt.Errorf("recommended memory must be positive")
	}
	return nil
}
