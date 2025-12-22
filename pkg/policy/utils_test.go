package policy

import (
	"testing"
)

func TestParseCPUValue(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		wantErr  bool
	}{
		{"100m", 100, false},
		{"500m", 500, false},
		{"1", 1000, false},
		{"1.5", 1500, false},
		{"2.0", 2000, false},
		{"0.5", 500, false},
		{"invalid", 0, true},
		{"100x", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseCPUValue(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error for input '%s', got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for input '%s': %v", tt.input, err)
				return
			}

			if result != tt.expected {
				t.Errorf("For input '%s': expected %d, got %d", tt.input, tt.expected, result)
			}
		})
	}
}

func TestParseMemoryValue(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		wantErr  bool
	}{
		{"1024", 1024, false},
		{"1Ki", 1024, false},
		{"1K", 1024, false},
		{"1Mi", 1024 * 1024, false},
		{"1M", 1024 * 1024, false},
		{"1Gi", 1024 * 1024 * 1024, false},
		{"1G", 1024 * 1024 * 1024, false},
		{"512Mi", 512 * 1024 * 1024, false},
		{"2Gi", 2 * 1024 * 1024 * 1024, false},
		{"1.5Gi", int64(1.5 * 1024 * 1024 * 1024), false},
		{"invalid", 0, true},
		{"100X", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseMemoryValue(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error for input '%s', got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for input '%s': %v", tt.input, err)
				return
			}

			if result != tt.expected {
				t.Errorf("For input '%s': expected %d, got %d", tt.input, tt.expected, result)
			}
		})
	}
}

func TestParseResourceValue(t *testing.T) {
	tests := []struct {
		value        string
		resourceType string
		expected     int64
		wantErr      bool
	}{
		{"100m", "cpu", 100, false},
		{"1", "cpu", 1000, false},
		{"512Mi", "memory", 512 * 1024 * 1024, false},
		{"1Gi", "memory", 1024 * 1024 * 1024, false},
		{"invalid", "cpu", 0, true},
		{"100m", "unknown", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.value+"_"+tt.resourceType, func(t *testing.T) {
			result, err := parseResourceValue(tt.value, tt.resourceType)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestFormatCPU(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{100, "100m"},
		{500, "500m"},
		{1000, "1.00"},
		{1500, "1.50"},
		{2000, "2.00"},
	}

	for _, tt := range tests {
		result := FormatCPU(tt.input)
		if result != tt.expected {
			t.Errorf("FormatCPU(%d): expected %s, got %s", tt.input, tt.expected, result)
		}
	}
}

func TestFormatMemory(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{512, "512"},
		{1024, "1.00Ki"},
		{1024 * 1024, "1.00Mi"},
		{512 * 1024 * 1024, "512.00Mi"},
		{1024 * 1024 * 1024, "1.00Gi"},
		{2 * 1024 * 1024 * 1024, "2.00Gi"},
		{1024 * 1024 * 1024 * 1024, "1.00Ti"},
	}

	for _, tt := range tests {
		result := FormatMemory(tt.input)
		if result != tt.expected {
			t.Errorf("FormatMemory(%d): expected %s, got %s", tt.input, tt.expected, result)
		}
	}
}

func TestIsValidAction(t *testing.T) {
	validActions := []string{
		ActionAllow,
		ActionDeny,
		ActionSkip,
		ActionSkipScaleDown,
		ActionSkipScaleUp,
		ActionSetMinCPU,
		ActionSetMaxCPU,
		ActionSetMinMemory,
		ActionSetMaxMemory,
		ActionRequireApproval,
	}

	for _, action := range validActions {
		if !isValidAction(action) {
			t.Errorf("Action '%s' should be valid", action)
		}
	}

	invalidActions := []string{
		"invalid",
		"unknown-action",
		"",
		"allow-all",
	}

	for _, action := range invalidActions {
		if isValidAction(action) {
			t.Errorf("Action '%s' should be invalid", action)
		}
	}
}

func TestValidateActionParameters(t *testing.T) {
	tests := []struct {
		name    string
		policy  Policy
		wantErr bool
	}{
		{
			name: "set-min-cpu with valid parameters",
			policy: Policy{
				Action: ActionSetMinCPU,
				Parameters: map[string]string{
					"min-cpu": "500m",
				},
			},
			wantErr: false,
		},
		{
			name: "set-min-cpu missing parameter",
			policy: Policy{
				Action:     ActionSetMinCPU,
				Parameters: map[string]string{},
			},
			wantErr: true,
		},
		{
			name: "set-min-cpu invalid value",
			policy: Policy{
				Action: ActionSetMinCPU,
				Parameters: map[string]string{
					"min-cpu": "invalid",
				},
			},
			wantErr: true,
		},
		{
			name: "set-max-cpu with valid parameters",
			policy: Policy{
				Action: ActionSetMaxCPU,
				Parameters: map[string]string{
					"max-cpu": "2",
				},
			},
			wantErr: false,
		},
		{
			name: "set-max-cpu missing parameter",
			policy: Policy{
				Action:     ActionSetMaxCPU,
				Parameters: map[string]string{},
			},
			wantErr: true,
		},
		{
			name: "set-min-memory with valid parameters",
			policy: Policy{
				Action: ActionSetMinMemory,
				Parameters: map[string]string{
					"min-memory": "1Gi",
				},
			},
			wantErr: false,
		},
		{
			name: "set-min-memory missing parameter",
			policy: Policy{
				Action:     ActionSetMinMemory,
				Parameters: map[string]string{},
			},
			wantErr: true,
		},
		{
			name: "set-max-memory with valid parameters",
			policy: Policy{
				Action: ActionSetMaxMemory,
				Parameters: map[string]string{
					"max-memory": "2Gi",
				},
			},
			wantErr: false,
		},
		{
			name: "set-max-memory missing parameter",
			policy: Policy{
				Action:     ActionSetMaxMemory,
				Parameters: map[string]string{},
			},
			wantErr: true,
		},
		{
			name: "set-max-memory invalid value",
			policy: Policy{
				Action: ActionSetMaxMemory,
				Parameters: map[string]string{
					"max-memory": "bad-value",
				},
			},
			wantErr: true,
		},
		{
			name: "allow action - no parameters needed",
			policy: Policy{
				Action:     ActionAllow,
				Parameters: map[string]string{},
			},
			wantErr: false,
		},
		{
			name: "deny action - no parameters needed",
			policy: Policy{
				Action:     ActionDeny,
				Parameters: map[string]string{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateActionParameters(tt.policy)

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

func TestParseMemoryEdgeCases(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		wantErr  bool
	}{
		{"1Ti", 1024 * 1024 * 1024 * 1024, false},
		{"1T", 1024 * 1024 * 1024 * 1024, false},
		{"1Pi", 1024 * 1024 * 1024 * 1024 * 1024, false},
		{"1P", 1024 * 1024 * 1024 * 1024 * 1024, false},
		{"0.25Gi", int64(0.25 * 1024 * 1024 * 1024), false},
		{"100", 100, false},
		{"   512Mi   ", 512 * 1024 * 1024, false}, // whitespace trimming
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseMemoryValue(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error for input '%s', got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for input '%s': %v", tt.input, err)
				return
			}

			if result != tt.expected {
				t.Errorf("For input '%s': expected %d, got %d", tt.input, tt.expected, result)
			}
		})
	}
}

func TestParseCPUEdgeCases(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		wantErr  bool
	}{
		{"0.1", 100, false},
		{"0.25", 250, false},
		{"10", 10000, false},
		{"   100m   ", 100, false}, // whitespace trimming
		{"250m", 250, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseCPUValue(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error for input '%s', got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error for input '%s': %v", tt.input, err)
				return
			}

			if result != tt.expected {
				t.Errorf("For input '%s': expected %d, got %d", tt.input, tt.expected, result)
			}
		})
	}
}

func TestFormatEdgeCases(t *testing.T) {
	// Test CPU formatting edge cases
	cpuTests := []struct {
		input    int64
		expected string
	}{
		{1, "1m"},
		{10, "10m"},
		{999, "999m"},
		{1001, "1.00"},
		{2500, "2.50"},
	}

	for _, tt := range cpuTests {
		result := FormatCPU(tt.input)
		if result != tt.expected {
			t.Errorf("FormatCPU(%d): expected %s, got %s", tt.input, tt.expected, result)
		}
	}

	// Test Memory formatting edge cases
	memTests := []struct {
		input    int64
		expected string
	}{
		{0, "0"},
		{100, "100"},
		{1023, "1023"},
		{1536, "1.50Ki"},
		{1024 * 1024 * 1024 * 1024, "1.00Ti"},
	}

	for _, tt := range memTests {
		result := FormatMemory(tt.input)
		if result != tt.expected {
			t.Errorf("FormatMemory(%d): expected %s, got %s", tt.input, tt.expected, result)
		}
	}
}
