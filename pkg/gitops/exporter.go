package gitops

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DefaultExporter is the default implementation of Exporter
type DefaultExporter struct {
	kustomizeGen KustomizeGenerator
	helmGen      HelmGenerator
}

// NewExporter creates a new exporter
func NewExporter() Exporter {
	return &DefaultExporter{
		kustomizeGen: NewKustomizeGenerator(),
		helmGen:      NewHelmGenerator(),
	}
}

// Export exports recommendations in the specified format
func (e *DefaultExporter) Export(recommendations []ResourceRecommendation, config ExportConfig) (*ExportResult, error) {
	if err := e.ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	result := &ExportResult{
		Files:     make(map[string]string),
		Timestamp: time.Now(),
	}

	var err error

	switch config.Format {
	case FormatKustomize:
		err = e.exportKustomize(recommendations, config, result)
	case FormatKustomizeJSON6902:
		err = e.exportKustomizeJSON6902(recommendations, config, result)
	case FormatHelm:
		err = e.exportHelm(recommendations, config, result)
	default:
		return nil, fmt.Errorf("unsupported format: %s", config.Format)
	}

	if err != nil {
		result.Errors = append(result.Errors, err)
		return result, err
	}

	// Write files to disk if OutputPath is specified
	if config.OutputPath != "" {
		if err := e.writeFiles(result.Files, config.OutputPath); err != nil {
			result.Errors = append(result.Errors, err)
			return result, err
		}
	}

	return result, nil
}

// ValidateConfig validates the export configuration
func (e *DefaultExporter) ValidateConfig(config ExportConfig) error {
	if config.Format == "" {
		return fmt.Errorf("format is required")
	}

	// Validate format
	validFormats := map[ExportFormat]bool{
		FormatKustomize:         true,
		FormatKustomizeJSON6902: true,
		FormatHelm:              true,
	}

	if !validFormats[config.Format] {
		return fmt.Errorf("invalid format: %s", config.Format)
	}

	// Validate Git config if provided
	if config.GitConfig != nil {
		if config.GitConfig.RepositoryURL == "" {
			return fmt.Errorf("git repository URL is required")
		}
	}

	return nil
}

// exportKustomize exports as Kustomize strategic merge patches
func (e *DefaultExporter) exportKustomize(recommendations []ResourceRecommendation, config ExportConfig, result *ExportResult) error {
	patchFiles := []string{}

	for i, rec := range recommendations {
		patch, err := e.kustomizeGen.GenerateStrategicMerge(rec)
		if err != nil {
			return fmt.Errorf("failed to generate patch for %s/%s: %w", rec.Namespace, rec.Name, err)
		}

		filename := fmt.Sprintf("patch-%s-%s-%d.yaml", rec.Namespace, rec.Name, i)
		result.Files[filename] = patch
		patchFiles = append(patchFiles, filename)
	}

	// Generate kustomization.yaml only if there are patches
	if len(patchFiles) > 0 {
		kustomization, err := e.kustomizeGen.GenerateKustomization(patchFiles)
		if err != nil {
			return fmt.Errorf("failed to generate kustomization.yaml: %w", err)
		}

		result.Files["kustomization.yaml"] = kustomization
	}

	return nil
}

// exportKustomizeJSON6902 exports as Kustomize JSON 6902 patches
func (e *DefaultExporter) exportKustomizeJSON6902(recommendations []ResourceRecommendation, config ExportConfig, result *ExportResult) error {
	for i, rec := range recommendations {
		patch, err := e.kustomizeGen.GenerateJSON6902(rec)
		if err != nil {
			return fmt.Errorf("failed to generate JSON 6902 patch for %s/%s: %w", rec.Namespace, rec.Name, err)
		}

		filename := fmt.Sprintf("patch-%s-%s-%d.json", rec.Namespace, rec.Name, i)
		result.Files[filename] = patch
	}

	return nil
}

// exportHelm exports as Helm values
func (e *DefaultExporter) exportHelm(recommendations []ResourceRecommendation, config ExportConfig, result *ExportResult) error {
	values, err := e.helmGen.GenerateValues(recommendations)
	if err != nil {
		return fmt.Errorf("failed to generate Helm values: %w", err)
	}

	result.Files["values.yaml"] = values

	return nil
}

// writeFiles writes files to the specified directory
func (e *DefaultExporter) writeFiles(files map[string]string, outputPath string) error {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	for filename, content := range files {
		filePath := filepath.Join(outputPath, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write file %s: %w", filename, err)
		}
	}

	return nil
}

// ExportToString exports recommendations to a string (for testing/preview)
func ExportToString(recommendations []ResourceRecommendation, format ExportFormat) (string, error) {
	exporter := NewExporter()

	config := ExportConfig{
		Format: format,
	}

	result, err := exporter.Export(recommendations, config)
	if err != nil {
		return "", err
	}

	// Concatenate all files
	var output string
	for filename, content := range result.Files {
		output += fmt.Sprintf("# File: %s\n", filename)
		output += content
		output += "\n\n"
	}

	return output, nil
}
