package policy

import (
	"testing"
	"time"
)

func TestLoadPoliciesFromBytes(t *testing.T) {
	policyYAML := `
defaultAction: deny

policies:
  - name: test-policy
    description: A test policy
    condition: workload.namespace == 'production'
    action: allow
    priority: 100
    enabled: true
`

	engine := NewEngine()
	err := engine.LoadPoliciesFromBytes([]byte(policyYAML))
	if err != nil {
		t.Fatalf("Failed to load policies: %v", err)
	}

	if len(engine.GetPolicies()) != 1 {
		t.Errorf("Expected 1 policy, got %d", len(engine.GetPolicies()))
	}

	if engine.GetDefaultAction() != "deny" {
		t.Errorf("Expected default action 'deny', got '%s'", engine.GetDefaultAction())
	}
}

func TestLoadPoliciesValidation(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
		errMsg  string
	}{
		{
			name: "missing policy name",
			yaml: `
policies:
  - condition: workload.namespace == 'prod'
    action: allow
`,
			wantErr: true,
			errMsg:  "has no name",
		},
		{
			name: "missing condition",
			yaml: `
policies:
  - name: test
    action: allow
`,
			wantErr: true,
			errMsg:  "has no condition",
		},
		{
			name: "missing action",
			yaml: `
policies:
  - name: test
    condition: workload.namespace == 'prod'
`,
			wantErr: true,
			errMsg:  "has no action",
		},
		{
			name: "invalid action type",
			yaml: `
policies:
  - name: test
    condition: workload.namespace == 'prod'
    action: invalid-action
`,
			wantErr: true,
			errMsg:  "invalid action",
		},
		{
			name: "set-min-memory missing parameter",
			yaml: `
policies:
  - name: test
    condition: workload.namespace == 'prod'
    action: set-min-memory
    enabled: true
`,
			wantErr: true,
			errMsg:  "requires 'min-memory' parameter",
		},
		{
			name: "set-min-cpu invalid value",
			yaml: `
policies:
  - name: test
    condition: workload.namespace == 'prod'
    action: set-min-cpu
    parameters:
      min-cpu: "invalid"
    enabled: true
`,
			wantErr: true,
			errMsg:  "invalid min-cpu value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewEngine()
			err := engine.LoadPoliciesFromBytes([]byte(tt.yaml))

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestEvaluateSimpleConditions(t *testing.T) {
	tests := []struct {
		name           string
		policy         string
		ctx            EvaluationContext
		expectedAction string
		shouldMatch    bool
	}{
		{
			name: "namespace match",
			policy: `
policies:
  - name: prod-protection
    condition: workload.namespace == 'production'
    action: deny
    enabled: true
`,
			ctx: EvaluationContext{
				Workload: WorkloadInfo{
					Namespace: "production",
					Name:      "api-server",
				},
			},
			expectedAction: "deny",
			shouldMatch:    true,
		},
		{
			name: "namespace no match",
			policy: `
defaultAction: allow
policies:
  - name: prod-protection
    condition: workload.namespace == 'production'
    action: deny
    enabled: true
`,
			ctx: EvaluationContext{
				Workload: WorkloadInfo{
					Namespace: "staging",
					Name:      "api-server",
				},
			},
			expectedAction: "allow",
			shouldMatch:    false,
		},
		{
			name: "label match",
			policy: `
policies:
  - name: critical-protection
    condition: workload.labels['tier'] == 'critical'
    action: deny
    enabled: true
`,
			ctx: EvaluationContext{
				Workload: WorkloadInfo{
					Labels: map[string]string{
						"tier": "critical",
					},
				},
			},
			expectedAction: "deny",
			shouldMatch:    true,
		},
		{
			name: "confidence threshold",
			policy: `
policies:
  - name: low-confidence-block
    condition: recommendation.confidence < 50
    action: deny
    enabled: true
`,
			ctx: EvaluationContext{
				Recommendation: RecommendationInfo{
					Confidence: 30.0,
				},
			},
			expectedAction: "deny",
			shouldMatch:    true,
		},
		{
			name: "change type scaledown",
			policy: `
policies:
  - name: prevent-scaledown
    condition: recommendation.changeType == 'scaledown'
    action: skip-scaledown
    enabled: true
`,
			ctx: EvaluationContext{
				Recommendation: RecommendationInfo{
					ChangeType: "scaledown",
				},
			},
			expectedAction: "deny",
			shouldMatch:    true,
		},
		{
			name: "business hours check",
			policy: `
policies:
  - name: business-hours-protection
    condition: time.isBusinessHours == true
    action: deny
    enabled: true
`,
			ctx: EvaluationContext{
				Time: TimeInfo{
					IsBusinessHours: true,
				},
			},
			expectedAction: "deny",
			shouldMatch:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewEngine()
			err := engine.LoadPoliciesFromBytes([]byte(tt.policy))
			if err != nil {
				t.Fatalf("Failed to load policy: %v", err)
			}

			decision, err := engine.Evaluate(tt.ctx)
			if err != nil {
				t.Fatalf("Evaluation failed: %v", err)
			}

			if decision.Action != tt.expectedAction {
				t.Errorf("Expected action '%s', got '%s'", tt.expectedAction, decision.Action)
			}
		})
	}
}

func TestEvaluateComplexConditions(t *testing.T) {
	policyYAML := `
policies:
  - name: complex-rule
    condition: workload.namespace == 'production' && workload.labels['tier'] == 'critical' && recommendation.changeType == 'scaledown'
    action: deny
    priority: 100
    enabled: true

  - name: percentage-check
    condition: recommendation.cpuChangePercent > 100
    action: deny
    priority: 90
    enabled: true
`

	engine := NewEngine()
	err := engine.LoadPoliciesFromBytes([]byte(policyYAML))
	if err != nil {
		t.Fatalf("Failed to load policies: %v", err)
	}

	// Test case 1: All conditions match
	ctx1 := EvaluationContext{
		Workload: WorkloadInfo{
			Namespace: "production",
			Labels: map[string]string{
				"tier": "critical",
			},
		},
		Recommendation: RecommendationInfo{
			ChangeType: "scaledown",
		},
	}

	decision, err := engine.Evaluate(ctx1)
	if err != nil {
		t.Fatalf("Evaluation failed: %v", err)
	}
	if decision.Action != "deny" {
		t.Errorf("Expected action 'deny', got '%s'", decision.Action)
	}
	if decision.MatchedPolicy != "complex-rule" {
		t.Errorf("Expected policy 'complex-rule', got '%s'", decision.MatchedPolicy)
	}

	// Test case 2: Percentage check
	ctx2 := EvaluationContext{
		Workload: WorkloadInfo{
			Namespace: "staging",
		},
		Recommendation: RecommendationInfo{
			CPUChangePercent: 150.0,
		},
	}

	decision, err = engine.Evaluate(ctx2)
	if err != nil {
		t.Fatalf("Evaluation failed: %v", err)
	}
	if decision.Action != "deny" {
		t.Errorf("Expected action 'deny', got '%s'", decision.Action)
	}
	if decision.MatchedPolicy != "percentage-check" {
		t.Errorf("Expected policy 'percentage-check', got '%s'", decision.MatchedPolicy)
	}
}

func TestPriorityOrdering(t *testing.T) {
	policyYAML := `
policies:
  - name: low-priority
    condition: workload.namespace == 'production'
    action: allow
    priority: 10
    enabled: true

  - name: high-priority
    condition: workload.namespace == 'production'
    action: deny
    priority: 100
    enabled: true
`

	engine := NewEngine()
	err := engine.LoadPoliciesFromBytes([]byte(policyYAML))
	if err != nil {
		t.Fatalf("Failed to load policies: %v", err)
	}

	ctx := EvaluationContext{
		Workload: WorkloadInfo{
			Namespace: "production",
		},
	}

	decision, err := engine.Evaluate(ctx)
	if err != nil {
		t.Fatalf("Evaluation failed: %v", err)
	}

	// High priority policy should match first
	if decision.MatchedPolicy != "high-priority" {
		t.Errorf("Expected 'high-priority' to match, got '%s'", decision.MatchedPolicy)
	}
	if decision.Action != "deny" {
		t.Errorf("Expected action 'deny', got '%s'", decision.Action)
	}
}

func TestDisabledPolicies(t *testing.T) {
	policyYAML := `
defaultAction: allow
policies:
  - name: disabled-rule
    condition: workload.namespace == 'production'
    action: deny
    enabled: false
`

	engine := NewEngine()
	err := engine.LoadPoliciesFromBytes([]byte(policyYAML))
	if err != nil {
		t.Fatalf("Failed to load policies: %v", err)
	}

	ctx := EvaluationContext{
		Workload: WorkloadInfo{
			Namespace: "production",
		},
	}

	decision, err := engine.Evaluate(ctx)
	if err != nil {
		t.Fatalf("Evaluation failed: %v", err)
	}

	// Disabled policy should not match, default action should apply
	if decision.Action != "allow" {
		t.Errorf("Expected default action 'allow', got '%s'", decision.Action)
	}
	if decision.MatchedPolicy != "" {
		t.Errorf("Expected no matched policy, got '%s'", decision.MatchedPolicy)
	}
}

func TestResourceModification(t *testing.T) {
	tests := []struct {
		name               string
		policy             string
		ctx                EvaluationContext
		expectedAction     string
		expectModification bool
	}{
		{
			name: "set minimum memory - needs increase",
			policy: `
policies:
  - name: min-memory
    condition: workload.labels['app'] == 'database'
    action: set-min-memory
    parameters:
      min-memory: "1Gi"
    enabled: true
`,
			ctx: EvaluationContext{
				Workload: WorkloadInfo{
					Labels: map[string]string{"app": "database"},
				},
				Recommendation: RecommendationInfo{
					RecommendedMemory: 512 * 1024 * 1024, // 512Mi
				},
			},
			expectedAction:     "modify",
			expectModification: true,
		},
		{
			name: "set minimum memory - already above minimum",
			policy: `
policies:
  - name: min-memory
    condition: workload.labels['app'] == 'database'
    action: set-min-memory
    parameters:
      min-memory: "512Mi"
    enabled: true
`,
			ctx: EvaluationContext{
				Workload: WorkloadInfo{
					Labels: map[string]string{"app": "database"},
				},
				Recommendation: RecommendationInfo{
					RecommendedMemory: 1024 * 1024 * 1024, // 1Gi
				},
			},
			expectedAction:     "allow",
			expectModification: false,
		},
		{
			name: "set maximum CPU - needs decrease",
			policy: `
policies:
  - name: max-cpu
    condition: workload.labels['tier'] == 'batch'
    action: set-max-cpu
    parameters:
      max-cpu: "500m"
    enabled: true
`,
			ctx: EvaluationContext{
				Workload: WorkloadInfo{
					Labels: map[string]string{"tier": "batch"},
				},
				Recommendation: RecommendationInfo{
					RecommendedCPU: 1000, // 1000m
				},
			},
			expectedAction:     "modify",
			expectModification: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewEngine()
			err := engine.LoadPoliciesFromBytes([]byte(tt.policy))
			if err != nil {
				t.Fatalf("Failed to load policy: %v", err)
			}

			decision, err := engine.Evaluate(tt.ctx)
			if err != nil {
				t.Fatalf("Evaluation failed: %v", err)
			}

			if decision.Action != tt.expectedAction {
				t.Errorf("Expected action '%s', got '%s'", tt.expectedAction, decision.Action)
			}

			if tt.expectModification {
				if decision.ModifiedRecommendation == nil {
					t.Error("Expected ModifiedRecommendation, got nil")
				} else if len(decision.ModifiedRecommendation.Modifications) == 0 {
					t.Error("Expected modifications, got none")
				}
			}
		})
	}
}

func TestTimeBasedPolicies(t *testing.T) {
	policyYAML := `
policies:
  - name: business-hours-scaledown-prevention
    condition: time.isBusinessHours && recommendation.changeType == 'scaledown'
    action: skip-scaledown
    enabled: true

  - name: weekend-optimization
    condition: time.isWeekend
    action: allow
    priority: 50
    enabled: true
`

	engine := NewEngine()
	err := engine.LoadPoliciesFromBytes([]byte(policyYAML))
	if err != nil {
		t.Fatalf("Failed to load policies: %v", err)
	}

	// Business hours scaledown
	ctx1 := EvaluationContext{
		Time: TimeInfo{
			Now:             time.Now(),
			IsBusinessHours: true,
		},
		Recommendation: RecommendationInfo{
			ChangeType: "scaledown",
		},
	}

	decision, err := engine.Evaluate(ctx1)
	if err != nil {
		t.Fatalf("Evaluation failed: %v", err)
	}
	if decision.Action != "deny" {
		t.Errorf("Expected scaledown to be denied during business hours, got '%s'", decision.Action)
	}

	// Weekend optimization
	ctx2 := EvaluationContext{
		Time: TimeInfo{
			IsWeekend: true,
		},
		Recommendation: RecommendationInfo{
			ChangeType: "scaleup",
		},
	}

	decision, err = engine.Evaluate(ctx2)
	if err != nil {
		t.Fatalf("Evaluation failed: %v", err)
	}
	if decision.Action != "allow" {
		t.Errorf("Expected action 'allow' on weekend, got '%s'", decision.Action)
	}
}

func TestCacheInvalidation(t *testing.T) {
	engine := NewEngine()

	policyYAML := `
policies:
  - name: test
    condition: workload.namespace == 'production'
    action: allow
    enabled: true
`

	err := engine.LoadPoliciesFromBytes([]byte(policyYAML))
	if err != nil {
		t.Fatalf("Failed to load policies: %v", err)
	}

	ctx := EvaluationContext{
		Workload: WorkloadInfo{
			Namespace: "production",
		},
	}

	// First evaluation - will compile expression
	_, err = engine.Evaluate(ctx)
	if err != nil {
		t.Fatalf("First evaluation failed: %v", err)
	}

	// Check cache has entry
	if len(engine.compiledPrograms) != 1 {
		t.Errorf("Expected 1 cached program, got %d", len(engine.compiledPrograms))
	}

	// Clear cache
	engine.ClearCache()

	// Check cache is empty
	if len(engine.compiledPrograms) != 0 {
		t.Errorf("Expected 0 cached programs after clear, got %d", len(engine.compiledPrograms))
	}

	// Second evaluation - will recompile
	_, err = engine.Evaluate(ctx)
	if err != nil {
		t.Fatalf("Second evaluation failed: %v", err)
	}

	// Cache should have entry again
	if len(engine.compiledPrograms) != 1 {
		t.Errorf("Expected 1 cached program after re-evaluation, got %d", len(engine.compiledPrograms))
	}
}

func TestAllActionTypes(t *testing.T) {
	tests := []struct {
		name           string
		action         string
		parameters     map[string]string
		ctx            EvaluationContext
		expectedAction string
		expectModify   bool
	}{
		{
			name:           "action skip",
			action:         ActionSkip,
			expectedAction: ActionDeny,
		},
		{
			name:   "action skip-scaleup when scaleup",
			action: ActionSkipScaleUp,
			ctx: EvaluationContext{
				Recommendation: RecommendationInfo{
					ChangeType: "scaleup",
				},
			},
			expectedAction: ActionDeny,
		},
		{
			name:   "action skip-scaleup when not scaleup",
			action: ActionSkipScaleUp,
			ctx: EvaluationContext{
				Recommendation: RecommendationInfo{
					ChangeType: "scaledown",
				},
			},
			expectedAction: ActionAllow,
		},
		{
			name:   "action set-min-cpu",
			action: ActionSetMinCPU,
			parameters: map[string]string{
				"min-cpu": "1000m",
			},
			ctx: EvaluationContext{
				Recommendation: RecommendationInfo{
					RecommendedCPU: 500,
				},
			},
			expectedAction: "modify",
			expectModify:   true,
		},
		{
			name:   "action set-max-memory",
			action: ActionSetMaxMemory,
			parameters: map[string]string{
				"max-memory": "512Mi",
			},
			ctx: EvaluationContext{
				Recommendation: RecommendationInfo{
					RecommendedMemory: 1024 * 1024 * 1024,
				},
			},
			expectedAction: "modify",
			expectModify:   true,
		},
		{
			name:           "action require-approval",
			action:         ActionRequireApproval,
			expectedAction: "require-approval",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewEngine()
			policy := Policy{
				Name:       "test-policy",
				Condition:  "workload.namespace == 'test'",
				Action:     tt.action,
				Parameters: tt.parameters,
				Enabled:    true,
			}

			decision, err := engine.applyAction(policy, tt.ctx)
			if err != nil {
				t.Fatalf("applyAction failed: %v", err)
			}

			if decision.Action != tt.expectedAction {
				t.Errorf("Expected action '%s', got '%s'", tt.expectedAction, decision.Action)
			}

			if tt.expectModify {
				if decision.ModifiedRecommendation == nil {
					t.Error("Expected ModifiedRecommendation, got nil")
				} else if len(decision.ModifiedRecommendation.Modifications) == 0 {
					t.Error("Expected modifications, got none")
				}
			}
		})
	}
}

func TestLoadPoliciesFromFile(t *testing.T) {
	// Test loading production policies
	engine := NewEngine()
	err := engine.LoadPolicies("examples/production-policies.yaml")
	if err != nil {
		t.Fatalf("Failed to load production policies: %v", err)
	}

	policies := engine.GetPolicies()
	if len(policies) != 10 {
		t.Errorf("Expected 10 production policies, got %d", len(policies))
	}

	// Verify policies are sorted by priority
	for i := 1; i < len(policies); i++ {
		if policies[i].Priority > policies[i-1].Priority {
			t.Errorf("Policies not sorted by priority: %d at index %d, %d at index %d",
				policies[i-1].Priority, i-1, policies[i].Priority, i)
		}
	}

	// Test loading development policies
	engine2 := NewEngine()
	err = engine2.LoadPolicies("examples/development-policies.yaml")
	if err != nil {
		t.Fatalf("Failed to load development policies: %v", err)
	}

	devPolicies := engine2.GetPolicies()
	if len(devPolicies) != 4 {
		t.Errorf("Expected 4 development policies, got %d", len(devPolicies))
	}
}

func TestLoadPoliciesFromNonExistentFile(t *testing.T) {
	engine := NewEngine()
	err := engine.LoadPolicies("nonexistent-file.yaml")
	if err == nil {
		t.Error("Expected error loading nonexistent file, got nil")
	}
}

func TestResourceModificationErrors(t *testing.T) {
	engine := NewEngine()

	tests := []struct {
		name        string
		policy      Policy
		ctx         EvaluationContext
		wantErr     bool
		errContains string
	}{
		{
			name: "set-min-cpu missing parameter",
			policy: Policy{
				Action:     ActionSetMinCPU,
				Parameters: map[string]string{},
			},
			wantErr:     true,
			errContains: "missing parameter",
		},
		{
			name: "set-min-cpu invalid value",
			policy: Policy{
				Action: ActionSetMinCPU,
				Parameters: map[string]string{
					"min-cpu": "invalid",
				},
			},
			wantErr:     true,
			errContains: "invalid",
		},
		{
			name: "unknown action",
			policy: Policy{
				Action: "unknown-action",
			},
			wantErr:     true,
			errContains: "unknown action",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := engine.applyAction(tt.policy, tt.ctx)

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

func TestEvaluationErrors(t *testing.T) {
	tests := []struct {
		name                  string
		policy                string
		ctx                   EvaluationContext
		expectPolicySkipped   bool
		expectedDefaultAction string
	}{
		{
			name: "invalid expression syntax skips policy",
			policy: `
defaultAction: deny
policies:
  - name: bad-syntax
    condition: workload.namespace ==
    action: allow
    enabled: true
`,
			expectPolicySkipped:   true,
			expectedDefaultAction: "deny",
		},
		{
			name: "condition returns non-boolean skips policy",
			policy: `
defaultAction: allow
policies:
  - name: non-boolean
    condition: workload.namespace
    action: deny
    enabled: true
`,
			ctx: EvaluationContext{
				Workload: WorkloadInfo{
					Namespace: "test",
				},
			},
			expectPolicySkipped:   true,
			expectedDefaultAction: "allow",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewEngine()
			err := engine.LoadPoliciesFromBytes([]byte(tt.policy))

			if err != nil {
				t.Fatalf("Unexpected load error: %v", err)
			}

			decision, err := engine.Evaluate(tt.ctx)
			if err != nil {
				t.Fatalf("Unexpected evaluation error: %v", err)
			}

			if tt.expectPolicySkipped {
				// When policy evaluation fails, it should be skipped and default action used
				if decision.MatchedPolicy != "" {
					t.Errorf("Expected no matched policy, got '%s'", decision.MatchedPolicy)
				}
				if decision.Action != tt.expectedDefaultAction {
					t.Errorf("Expected default action '%s', got '%s'", tt.expectedDefaultAction, decision.Action)
				}
			}
		})
	}
}

func TestComplexResourceModifications(t *testing.T) {
	policyYAML := `
policies:
  - name: combined-limits
    condition: workload.namespace == 'test'
    action: set-max-cpu
    parameters:
      max-cpu: "2"
    priority: 100
    enabled: true

  - name: min-memory
    condition: workload.namespace == 'test'
    action: set-min-memory
    parameters:
      min-memory: "256Mi"
    priority: 90
    enabled: true
`

	engine := NewEngine()
	err := engine.LoadPoliciesFromBytes([]byte(policyYAML))
	if err != nil {
		t.Fatalf("Failed to load policies: %v", err)
	}

	// Test max CPU enforcement with cores instead of millicores
	ctx := EvaluationContext{
		Workload: WorkloadInfo{
			Namespace: "test",
		},
		Recommendation: RecommendationInfo{
			RecommendedCPU: 3000, // 3 cores in millicores
		},
	}

	decision, err := engine.Evaluate(ctx)
	if err != nil {
		t.Fatalf("Evaluation failed: %v", err)
	}

	if decision.Action != "modify" {
		t.Errorf("Expected action 'modify', got '%s'", decision.Action)
	}

	if decision.ModifiedRecommendation == nil {
		t.Fatal("Expected ModifiedRecommendation, got nil")
	}

	if decision.ModifiedRecommendation.CPURequest == nil {
		t.Error("Expected CPU modification, got nil")
	} else {
		// Should be capped at 2000m (2 cores)
		if *decision.ModifiedRecommendation.CPURequest != 2000 {
			t.Errorf("Expected CPU capped at 2000m, got %dm", *decision.ModifiedRecommendation.CPURequest)
		}
	}
}
