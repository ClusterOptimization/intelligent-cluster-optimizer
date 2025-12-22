package policy

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// isValidAction checks if the action is a valid action type
func isValidAction(action string) bool {
	validActions := map[string]bool{
		ActionAllow:           true,
		ActionDeny:            true,
		ActionSkip:            true,
		ActionSkipScaleDown:   true,
		ActionSkipScaleUp:     true,
		ActionSetMinCPU:       true,
		ActionSetMaxCPU:       true,
		ActionSetMinMemory:    true,
		ActionSetMaxMemory:    true,
		ActionRequireApproval: true,
	}
	return validActions[action]
}

// validateActionParameters validates that required parameters are present for an action
func validateActionParameters(policy Policy) error {
	switch policy.Action {
	case ActionSetMinCPU:
		if _, ok := policy.Parameters["min-cpu"]; !ok {
			return fmt.Errorf("action set-min-cpu requires 'min-cpu' parameter")
		}
		if _, err := parseResourceValue(policy.Parameters["min-cpu"], "cpu"); err != nil {
			return fmt.Errorf("invalid min-cpu value: %w", err)
		}

	case ActionSetMaxCPU:
		if _, ok := policy.Parameters["max-cpu"]; !ok {
			return fmt.Errorf("action set-max-cpu requires 'max-cpu' parameter")
		}
		if _, err := parseResourceValue(policy.Parameters["max-cpu"], "cpu"); err != nil {
			return fmt.Errorf("invalid max-cpu value: %w", err)
		}

	case ActionSetMinMemory:
		if _, ok := policy.Parameters["min-memory"]; !ok {
			return fmt.Errorf("action set-min-memory requires 'min-memory' parameter")
		}
		if _, err := parseResourceValue(policy.Parameters["min-memory"], "memory"); err != nil {
			return fmt.Errorf("invalid min-memory value: %w", err)
		}

	case ActionSetMaxMemory:
		if _, ok := policy.Parameters["max-memory"]; !ok {
			return fmt.Errorf("action set-max-memory requires 'max-memory' parameter")
		}
		if _, err := parseResourceValue(policy.Parameters["max-memory"], "memory"); err != nil {
			return fmt.Errorf("invalid max-memory value: %w", err)
		}
	}

	return nil
}

// parseResourceValue parses a Kubernetes resource value (e.g., "100m", "512Mi") to int64
func parseResourceValue(value string, resourceType string) (int64, error) {
	value = strings.TrimSpace(value)

	if resourceType == "cpu" {
		return parseCPUValue(value)
	} else if resourceType == "memory" {
		return parseMemoryValue(value)
	}

	return 0, fmt.Errorf("unknown resource type: %s", resourceType)
}

// parseCPUValue parses CPU values like "100m" (millicores) or "1" (cores)
func parseCPUValue(value string) (int64, error) {
	value = strings.TrimSpace(value)

	// Match patterns like "100m" or "1.5"
	re := regexp.MustCompile(`^(\d+\.?\d*)([m]?)$`)
	matches := re.FindStringSubmatch(value)

	if matches == nil {
		return 0, fmt.Errorf("invalid CPU format: %s", value)
	}

	num, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid CPU number: %w", err)
	}

	// If it has 'm' suffix, it's already in millicores
	if matches[2] == "m" {
		return int64(num), nil
	}

	// Otherwise, it's in cores, convert to millicores
	return int64(num * 1000), nil
}

// parseMemoryValue parses memory values like "512Mi", "1Gi", "1024"
func parseMemoryValue(value string) (int64, error) {
	value = strings.TrimSpace(value)

	// Match patterns like "512Mi" or "1Gi" or "1024"
	re := regexp.MustCompile(`^(\d+\.?\d*)([KMGTP]i?)?$`)
	matches := re.FindStringSubmatch(value)

	if matches == nil {
		return 0, fmt.Errorf("invalid memory format: %s", value)
	}

	num, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory number: %w", err)
	}

	// Parse the unit suffix
	unit := matches[2]
	multiplier := int64(1)

	switch unit {
	case "":
		// Assume bytes if no unit
		multiplier = 1
	case "K", "Ki":
		multiplier = 1024
	case "M", "Mi":
		multiplier = 1024 * 1024
	case "G", "Gi":
		multiplier = 1024 * 1024 * 1024
	case "T", "Ti":
		multiplier = 1024 * 1024 * 1024 * 1024
	case "P", "Pi":
		multiplier = 1024 * 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unknown memory unit: %s", unit)
	}

	return int64(num * float64(multiplier)), nil
}

// FormatCPU formats CPU millicores to human-readable string
func FormatCPU(millicores int64) string {
	if millicores < 1000 {
		return fmt.Sprintf("%dm", millicores)
	}
	return fmt.Sprintf("%.2f", float64(millicores)/1000.0)
}

// FormatMemory formats bytes to human-readable string
func FormatMemory(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2fTi", float64(bytes)/TB)
	case bytes >= GB:
		return fmt.Sprintf("%.2fGi", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2fMi", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2fKi", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d", bytes)
	}
}
