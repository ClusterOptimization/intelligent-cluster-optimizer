package policy

import (
	"fmt"
	"os"
	"sort"
	"sync"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"gopkg.in/yaml.v2"
	"k8s.io/klog/v2"
)

// Engine is the main policy evaluation engine
type Engine struct {
	// policies contains all loaded policies
	policies PolicySet

	// compiledPrograms caches compiled expressions for performance
	compiledPrograms map[string]*vm.Program

	// mu protects concurrent access to compiledPrograms
	mu sync.RWMutex
}

// NewEngine creates a new policy engine
func NewEngine() *Engine {
	return &Engine{
		policies: PolicySet{
			Policies:      []Policy{},
			DefaultAction: ActionAllow,
		},
		compiledPrograms: make(map[string]*vm.Program),
	}
}

// LoadPolicies loads policies from a YAML file
func (e *Engine) LoadPolicies(filepath string) error {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to read policy file: %w", err)
	}

	var policySet PolicySet
	if err := yaml.Unmarshal(data, &policySet); err != nil {
		return fmt.Errorf("failed to unmarshal policies: %w", err)
	}

	// Set default action if not specified
	if policySet.DefaultAction == "" {
		policySet.DefaultAction = ActionAllow
	}

	// Validate policies
	for i, p := range policySet.Policies {
		if p.Name == "" {
			return fmt.Errorf("policy at index %d has no name", i)
		}
		if p.Condition == "" {
			return fmt.Errorf("policy %s has no condition", p.Name)
		}
		if p.Action == "" {
			return fmt.Errorf("policy %s has no action", p.Name)
		}

		// Validate action type
		if !isValidAction(p.Action) {
			return fmt.Errorf("policy %s has invalid action: %s", p.Name, p.Action)
		}

		// Validate action-specific parameters
		if err := validateActionParameters(p); err != nil {
			return fmt.Errorf("policy %s: %w", p.Name, err)
		}
	}

	// Sort policies by priority (higher priority first)
	sort.Slice(policySet.Policies, func(i, j int) bool {
		return policySet.Policies[i].Priority > policySet.Policies[j].Priority
	})

	e.policies = policySet
	klog.Infof("Loaded %d policies with default action: %s", len(policySet.Policies), policySet.DefaultAction)

	return nil
}

// LoadPoliciesFromBytes loads policies from YAML bytes (useful for testing)
func (e *Engine) LoadPoliciesFromBytes(data []byte) error {
	var policySet PolicySet
	if err := yaml.Unmarshal(data, &policySet); err != nil {
		return fmt.Errorf("failed to unmarshal policies: %w", err)
	}

	if policySet.DefaultAction == "" {
		policySet.DefaultAction = ActionAllow
	}

	// Validate policies
	for i, p := range policySet.Policies {
		if p.Name == "" {
			return fmt.Errorf("policy at index %d has no name", i)
		}
		if p.Condition == "" {
			return fmt.Errorf("policy %s has no condition", p.Name)
		}
		if p.Action == "" {
			return fmt.Errorf("policy %s has no action", p.Name)
		}

		// Validate action type
		if !isValidAction(p.Action) {
			return fmt.Errorf("policy %s has invalid action: %s", p.Name, p.Action)
		}

		// Validate action-specific parameters
		if err := validateActionParameters(p); err != nil {
			return fmt.Errorf("policy %s: %w", p.Name, err)
		}
	}

	sort.Slice(policySet.Policies, func(i, j int) bool {
		return policySet.Policies[i].Priority > policySet.Policies[j].Priority
	})

	e.policies = policySet
	return nil
}

// Evaluate evaluates all policies against the given context
func (e *Engine) Evaluate(ctx EvaluationContext) (*PolicyDecision, error) {
	// Evaluate policies in priority order
	for _, policy := range e.policies.Policies {
		// Skip disabled policies
		if !policy.Enabled {
			continue
		}

		// Evaluate the condition
		matches, err := e.evaluateCondition(policy.Condition, ctx)
		if err != nil {
			klog.Warningf("Failed to evaluate policy %s: %v", policy.Name, err)
			continue
		}

		// If condition matches, apply the action
		if matches {
			klog.V(2).Infof("Policy %s matched for workload %s/%s",
				policy.Name, ctx.Workload.Namespace, ctx.Workload.Name)

			decision, err := e.applyAction(policy, ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to apply action for policy %s: %w", policy.Name, err)
			}

			decision.MatchedPolicy = policy.Name
			decision.Reason = fmt.Sprintf("Policy '%s' matched: %s", policy.Name, policy.Description)

			return decision, nil
		}
	}

	// No policy matched, use default action
	klog.V(2).Infof("No policy matched for workload %s/%s, using default action: %s",
		ctx.Workload.Namespace, ctx.Workload.Name, e.policies.DefaultAction)

	return &PolicyDecision{
		Action: e.policies.DefaultAction,
		Reason: "No policy matched, using default action",
	}, nil
}

// evaluateCondition evaluates a policy condition expression
func (e *Engine) evaluateCondition(condition string, ctx EvaluationContext) (bool, error) {
	// Check cache first
	e.mu.RLock()
	program, exists := e.compiledPrograms[condition]
	e.mu.RUnlock()

	// Create environment map for expr with lowercase keys
	env := map[string]interface{}{
		"workload":       ctx.Workload.ToExprEnv(),
		"recommendation": ctx.Recommendation.ToExprEnv(),
		"time":           ctx.Time.ToExprEnv(),
		"cluster":        ctx.Cluster.ToExprEnv(),
		"custom":         ctx.Custom,
	}

	if !exists {
		// Compile the expression
		compiled, err := expr.Compile(condition, expr.Env(env), expr.AsBool())
		if err != nil {
			return false, fmt.Errorf("failed to compile condition: %w", err)
		}

		// Cache the compiled program
		e.mu.Lock()
		e.compiledPrograms[condition] = compiled
		e.mu.Unlock()

		program = compiled
	}

	// Run the program
	output, err := expr.Run(program, env)
	if err != nil {
		return false, fmt.Errorf("failed to evaluate condition: %w", err)
	}

	result, ok := output.(bool)
	if !ok {
		return false, fmt.Errorf("condition did not evaluate to boolean: %T", output)
	}

	return result, nil
}

// applyAction applies the policy action and returns a decision
func (e *Engine) applyAction(policy Policy, ctx EvaluationContext) (*PolicyDecision, error) {
	switch policy.Action {
	case ActionAllow:
		return &PolicyDecision{
			Action: ActionAllow,
		}, nil

	case ActionDeny:
		return &PolicyDecision{
			Action: ActionDeny,
		}, nil

	case ActionSkip:
		return &PolicyDecision{
			Action: ActionDeny,
		}, nil

	case ActionSkipScaleDown:
		if ctx.Recommendation.ChangeType == "scaledown" {
			return &PolicyDecision{
				Action: ActionDeny,
			}, nil
		}
		return &PolicyDecision{
			Action: ActionAllow,
		}, nil

	case ActionSkipScaleUp:
		if ctx.Recommendation.ChangeType == "scaleup" {
			return &PolicyDecision{
				Action: ActionDeny,
			}, nil
		}
		return &PolicyDecision{
			Action: ActionAllow,
		}, nil

	case ActionSetMinCPU:
		return e.applyResourceModification(policy, ctx, "cpu", "min")

	case ActionSetMaxCPU:
		return e.applyResourceModification(policy, ctx, "cpu", "max")

	case ActionSetMinMemory:
		return e.applyResourceModification(policy, ctx, "memory", "min")

	case ActionSetMaxMemory:
		return e.applyResourceModification(policy, ctx, "memory", "max")

	case ActionRequireApproval:
		return &PolicyDecision{
			Action: "require-approval",
		}, nil

	default:
		return nil, fmt.Errorf("unknown action: %s", policy.Action)
	}
}

// applyResourceModification applies min/max resource constraints
func (e *Engine) applyResourceModification(policy Policy, ctx EvaluationContext,
	resourceType string, limitType string) (*PolicyDecision, error) {

	paramKey := fmt.Sprintf("%s-%s", limitType, resourceType)
	limitValue, ok := policy.Parameters[paramKey]
	if !ok {
		return nil, fmt.Errorf("missing parameter %s for action %s", paramKey, policy.Action)
	}

	// Parse the resource value
	limitBytes, err := parseResourceValue(limitValue, resourceType)
	if err != nil {
		return nil, fmt.Errorf("invalid %s value: %w", paramKey, err)
	}

	// Create modified recommendation
	modified := &ModifiedRecommendation{
		Modifications: []string{},
	}

	// Apply the modification based on resource type and limit type
	if resourceType == "cpu" {
		currentRecommendation := ctx.Recommendation.RecommendedCPU

		if limitType == "min" && currentRecommendation < limitBytes {
			modified.CPURequest = &limitBytes
			modified.Modifications = append(modified.Modifications,
				fmt.Sprintf("Increased CPU from %dm to %dm (policy minimum)",
					currentRecommendation, limitBytes))
		} else if limitType == "max" && currentRecommendation > limitBytes {
			modified.CPURequest = &limitBytes
			modified.Modifications = append(modified.Modifications,
				fmt.Sprintf("Decreased CPU from %dm to %dm (policy maximum)",
					currentRecommendation, limitBytes))
		}
	} else if resourceType == "memory" {
		currentRecommendation := ctx.Recommendation.RecommendedMemory

		if limitType == "min" && currentRecommendation < limitBytes {
			modified.MemoryRequest = &limitBytes
			modified.Modifications = append(modified.Modifications,
				fmt.Sprintf("Increased memory from %d to %d bytes (policy minimum)",
					currentRecommendation, limitBytes))
		} else if limitType == "max" && currentRecommendation > limitBytes {
			modified.MemoryRequest = &limitBytes
			modified.Modifications = append(modified.Modifications,
				fmt.Sprintf("Decreased memory from %d to %d bytes (policy maximum)",
					currentRecommendation, limitBytes))
		}
	}

	// If modifications were made, return modify decision
	if len(modified.Modifications) > 0 {
		return &PolicyDecision{
			Action:                 "modify",
			ModifiedRecommendation: modified,
		}, nil
	}

	// No modification needed, allow the original recommendation
	return &PolicyDecision{
		Action: ActionAllow,
	}, nil
}

// GetPolicies returns all loaded policies
func (e *Engine) GetPolicies() []Policy {
	return e.policies.Policies
}

// GetDefaultAction returns the default action
func (e *Engine) GetDefaultAction() string {
	return e.policies.DefaultAction
}

// ClearCache clears the compiled expression cache
func (e *Engine) ClearCache() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.compiledPrograms = make(map[string]*vm.Program)
}
