# Policy Engine

The policy engine provides rule-based decision making for resource optimization recommendations. It allows administrators to define custom policies that control when and how optimizations are applied to workloads.

## Features

- Expression-based condition evaluation using [expr](https://github.com/expr-lang/expr)
- Priority-based policy ordering
- Resource modification (set min/max CPU and memory)
- Time-based policies (business hours, weekends)
- Label-based workload targeting
- Confidence threshold filtering
- Multiple action types (allow, deny, skip, modify)

## Policy Structure

Policies are defined in YAML format:

```yaml
defaultAction: allow  # Action when no policy matches

policies:
  - name: protect-critical-services
    description: Never scale down critical production services
    condition: workload.labels['tier'] == 'critical' && recommendation.changeType == 'scaledown'
    action: deny
    priority: 100
    enabled: true
```

### Policy Fields

- `name` (required): Unique identifier for the policy
- `description` (optional): Human-readable explanation
- `condition` (required): Expression that must evaluate to true for the policy to apply
- `action` (required): What to do when the condition matches
- `parameters` (optional): Action-specific configuration
- `priority` (optional): Evaluation order (higher = evaluated first)
- `enabled` (optional): Whether the policy is active

### Available Actions

1. `allow` - Approve the recommendation
2. `deny` - Block the recommendation
3. `skip` - Same as deny
4. `skip-scaledown` - Block only scale-down changes
5. `skip-scaleup` - Block only scale-up changes
6. `set-min-cpu` - Enforce minimum CPU (requires `min-cpu` parameter)
7. `set-max-cpu` - Enforce maximum CPU (requires `max-cpu` parameter)
8. `set-min-memory` - Enforce minimum memory (requires `min-memory` parameter)
9. `set-max-memory` - Enforce maximum memory (requires `max-memory` parameter)
10. `require-approval` - Mark for manual approval

## Expression Language

Policies use the [expr](https://github.com/expr-lang/expr) language for conditions. The following context is available:

### workload

Information about the Kubernetes workload being optimized:

```javascript
workload.namespace       // string: Namespace name
workload.name           // string: Workload name
workload.kind           // string: "Deployment", "StatefulSet", etc.
workload.labels         // map[string]string: Labels
workload.annotations    // map[string]string: Annotations
workload.replicas       // int32: Number of replicas
workload.currentCPU     // int64: Current CPU in millicores
workload.currentMemory  // int64: Current memory in bytes
```

### recommendation

The optimization recommendation being evaluated:

```javascript
recommendation.recommendedCPU      // int64: Recommended CPU in millicores
recommendation.recommendedMemory   // int64: Recommended memory in bytes
recommendation.confidence          // float64: Confidence score (0-100)
recommendation.changeType          // string: "scaleup", "scaledown", or "nochange"
recommendation.cpuChangePercent    // float64: CPU change percentage
recommendation.memoryChangePercent // float64: Memory change percentage
```

### time

Time-related information for scheduling policies:

```javascript
time.now              // time.Time: Current timestamp
time.hour             // int: Current hour (0-23)
time.weekday          // int: Day of week (0-6, 0=Sunday)
time.isBusinessHours  // bool: 9am-5pm weekdays
time.isWeekend        // bool: Saturday or Sunday
```

### cluster

Cluster-wide information:

```javascript
cluster.totalNodes      // int: Number of nodes
cluster.availableCPU    // int64: Available CPU in millicores
cluster.availableMemory // int64: Available memory in bytes
cluster.environment     // string: "production", "staging", "development"
```

## Usage Examples

### Basic Usage

```go
package main

import (
    "fmt"
    "intelligent-cluster-optimizer/pkg/policy"
)

func main() {
    // Create engine
    engine := policy.NewEngine()

    // Load policies from file
    err := engine.LoadPolicies("policies.yaml")
    if err != nil {
        panic(err)
    }

    // Create evaluation context
    ctx := policy.EvaluationContext{
        Workload: policy.WorkloadInfo{
            Namespace: "production",
            Name:      "api-server",
            Labels: map[string]string{
                "tier": "critical",
            },
        },
        Recommendation: policy.RecommendationInfo{
            RecommendedCPU:    500,
            RecommendedMemory: 1024 * 1024 * 1024,
            Confidence:        85.0,
            ChangeType:        "scaledown",
        },
    }

    // Evaluate policies
    decision, err := engine.Evaluate(ctx)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Action: %s\n", decision.Action)
    fmt.Printf("Reason: %s\n", decision.Reason)
    fmt.Printf("Matched Policy: %s\n", decision.MatchedPolicy)
}
```

### Policy Examples

#### Protect Critical Workloads

```yaml
policies:
  - name: protect-critical-services
    description: Never scale down critical production services
    condition: workload.labels['tier'] == 'critical' && recommendation.changeType == 'scaledown'
    action: deny
    priority: 100
    enabled: true
```

#### Minimum Memory for Databases

```yaml
policies:
  - name: minimum-memory-for-databases
    description: Ensure database workloads have at least 1Gi memory
    condition: workload.labels['app-type'] == 'database'
    action: set-min-memory
    parameters:
      min-memory: "1Gi"
    priority: 90
    enabled: true
```

#### Block Low Confidence Recommendations

```yaml
policies:
  - name: prevent-low-confidence-changes
    description: Block recommendations with low confidence scores
    condition: recommendation.confidence < 50
    action: deny
    priority: 95
    enabled: true
```

#### Business Hours Protection

```yaml
policies:
  - name: no-scaledown-during-business-hours
    description: Prevent scaling down during peak hours
    condition: time.isBusinessHours && recommendation.changeType == 'scaledown'
    action: skip-scaledown
    priority: 70
    enabled: true
```

#### Environment-Based Limits

```yaml
policies:
  - name: dev-resource-caps
    description: Limit resources in development environment
    condition: cluster.environment == 'development'
    action: set-max-memory
    parameters:
      max-memory: "512Mi"
    priority: 100
    enabled: true
```

#### Prevent Excessive Changes

```yaml
policies:
  - name: prevent-excessive-scaleup
    description: Block scale-ups over 200% increase
    condition: recommendation.changeType == 'scaleup' && (recommendation.cpuChangePercent > 200 || recommendation.memoryChangePercent > 200)
    action: deny
    priority: 65
    enabled: true
```

## Resource Value Formats

CPU values:
- Millicores: `100m`, `500m`, `1000m`
- Cores: `1`, `1.5`, `2.0`

Memory values:
- Bytes: `1024`
- Kibibytes: `1Ki` or `1K`
- Mebibytes: `512Mi` or `512M`
- Gibibytes: `1Gi` or `1G`
- Tebibytes: `1Ti` or `1T`

## Testing

The package includes comprehensive tests covering:

- Policy loading and validation
- Expression evaluation (simple and complex)
- Priority ordering
- Resource modifications
- Time-based policies
- Expression caching
- Resource value parsing

Run tests:

```bash
go test ./pkg/policy/...
```

## Example Policy Files

See the `examples/` directory for complete policy file examples:

- `production-policies.yaml` - Production environment policies
- `development-policies.yaml` - Development environment policies

## Performance

The policy engine includes expression caching to avoid recompiling expressions on every evaluation. Cache can be cleared manually if needed:

```go
engine.ClearCache()
```

## Thread Safety

The policy engine is thread-safe for concurrent evaluations. Expression cache access is protected by a read-write mutex.
