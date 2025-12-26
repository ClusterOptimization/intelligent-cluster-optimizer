# Intelligent Cluster Optimizer - Architecture

## Table of Contents

- [Overview](#overview)
- [System Architecture](#system-architecture)
- [Core Components](#core-components)
- [Algorithms](#algorithms)
- [Data Flow](#data-flow)
- [Security Architecture](#security-architecture)
- [Integration Points](#integration-points)

## Overview

The Intelligent Cluster Optimizer is a Kubernetes controller that automatically optimizes resource allocations (CPU, memory, replicas) for workloads based on actual usage patterns, SLA requirements, and cost considerations.

### Design Principles

1. **Safety First**: Multiple safety mechanisms (circuit breakers, health checks, rollback)
2. **Non-Invasive**: Works alongside existing Kubernetes features (HPA, PDB)
3. **Observable**: Comprehensive metrics and logging
4. **Configurable**: Profile-based and fine-grained configuration
5. **GitOps-Compatible**: Export recommendations to version control

## System Architecture

```
┌──────────────────────────────────────────────────────────────────┐
│                        Kubernetes Cluster                         │
│                                                                    │
│  ┌────────────────┐                    ┌──────────────────────┐  │
│  │  OptimizerConfig │                  │  Workloads           │  │
│  │  (CRD)          │                  │  (Deployments, etc)  │  │
│  └────────┬───────┘                    └──────────┬───────────┘  │
│           │                                       │               │
│           │ Watch                     Metrics API │               │
│           │                                       │               │
│  ┌────────▼────────────────────────────┐         │               │
│  │  Optimizer Controller               │         │               │
│  │                                     │◄────────┘               │
│  │  ┌─────────────────────────────┐   │                         │
│  │  │  Reconciliation Loop        │   │                         │
│  │  │                             │   │                         │
│  │  │  1. Metrics Collection      │   │                         │
│  │  │  2. Analysis & Prediction   │   │                         │
│  │  │  3. Recommendation          │   │                         │
│  │  │  4. Validation              │   │                         │
│  │  │  5. Application             │   │                         │
│  │  └─────────────────────────────┘   │                         │
│  │                                     │                         │
│  │  ┌─────────────┐  ┌──────────────┐ │                         │
│  │  │ Policy      │  │ SLA          │ │                         │
│  │  │ Engine      │  │ Monitor      │ │                         │
│  │  └─────────────┘  └──────────────┘ │                         │
│  └──────────────────┬──────────────────┘                         │
│                     │                                             │
│           ┌─────────▼──────────┐                                 │
│           │  Prometheus        │                                 │
│           │  (Metrics)         │                                 │
│           └────────────────────┘                                 │
└──────────────────────────────────────────────────────────────────┘
        │                                                │
        │ GitOps Export                                  │ Events
        ▼                                                ▼
┌────────────────┐                              ┌────────────────┐
│  Git Repository│                              │  Event Stream  │
│  (Kustomize/   │                              │  (Audit Logs)  │
│   Helm)        │                              └────────────────┘
└────────────────┘
```

## Core Components

### 1. Controller (pkg/controller)

The main reconciliation loop that orchestrates all optimization activities.

**Key Files:**
- `reconciler.go`: Main reconciliation logic
- `profiles.go`: Environment profile management

**Responsibilities:**
- Watch OptimizerConfig resources
- Trigger optimization cycles
- Coordinate all subsystems
- Update status and emit events

### 2. Metrics Collection (pkg/metrics)

Collects real-time resource usage from Kubernetes Metrics API.

**Key Files:**
- `collector.go`: Metrics API client
- `prometheus.go`: Prometheus exporter
- `storage.go`: Time-series storage

**Metrics Collected:**
- CPU usage (millicores)
- Memory usage (bytes)
- Pod/container lifecycle events
- Historical patterns

### 3. Resource Optimizer (pkg/optimizer)

Analyzes metrics and generates resource recommendations.

**Key Files:**
- `resource.go`: Core recommendation engine
- `pareto.go`: Multi-objective optimization

**Algorithms:**
- Percentile-based sizing (P50, P90, P95, P99)
- Safety margin application
- Multi-objective Pareto optimization (cost vs performance vs reliability)

**Example Recommendation:**
```go
type ResourceRecommendation struct {
    CPU ResourceAmount {
        Current: 1000m
        Recommended: 500m
        Confidence: 0.92
    }
    Memory: ResourceAmount {
        Current: 2Gi
        Recommended: 1.5Gi
        Confidence: 0.88
    }
    Replicas: {
        Current: 3
        Recommended: 2
        Confidence: 0.85
    }
}
```

### 4. Policy Engine (pkg/policy)

Expression-based policy evaluation for governance.

**Key Files:**
- `engine.go`: Policy evaluation engine
- `expressions.go`: Expression parser

**Features:**
- Expression language (via expr-lang/expr)
- Built-in functions and operators
- Policy violation detection

**Example Policy:**
```yaml
policies:
  - name: "min-cpu"
    expression: "recommendation.cpu.millicores >= 100"
    action: "block"
  - name: "max-memory"
    expression: "recommendation.memory.bytes <= 16 * 1024 * 1024 * 1024"
    action: "block"
```

### 5. SLA Monitor (pkg/sla)

Monitors Service Level Agreements and system health.

**Key Files:**
- `monitor.go`: SLA monitoring
- `health_checker.go`: Health scoring
- `control_chart.go`: Statistical process control

**SLA Types:**
- Latency (P95, P99)
- Error rate
- Availability
- Throughput
- Custom metrics

**Health Scoring:**
```go
score := 100.0
for each violation {
    penalty := violation.Severity * 35.0  // Max 35 points
    score -= penalty
}
score -= outliers * 2.0  // 2 points per outlier
```

**Control Charts:**
- 3-Sigma method (Shewhart charts)
- Robust outlier detection (MAD - Median Absolute Deviation)
- Trend detection

### 6. Prediction Engine (pkg/prediction)

Time-series forecasting for proactive scaling.

**Key Files:**
- `holtwinters.go`: Holt-Winters triple exponential smoothing

**Algorithm:**
- Level, trend, and seasonal components
- Adaptive to changing patterns
- Peak load prediction

**Formula:**
```
Level:   L_t = α(Y_t - S_{t-s}) + (1-α)(L_{t-1} + T_{t-1})
Trend:   T_t = β(L_t - L_{t-1}) + (1-β)T_{t-1}
Seasonal: S_t = γ(Y_t - L_t) + (1-γ)S_{t-s}
Forecast: F_{t+m} = (L_t + mT_t)S_{t-s+m}
```

### 7. GitOps Integration (pkg/gitops)

Exports recommendations to GitOps formats.

**Key Files:**
- `kustomize.go`: Kustomize patch generation
- `helm.go`: Helm values generation

**Formats:**
1. **Kustomize Strategic Merge**:
   ```yaml
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     name: myapp
   spec:
     template:
       spec:
         containers:
         - name: app
           resources:
             requests:
               cpu: 500m
               memory: 1.5Gi
   ```

2. **Kustomize JSON6902**:
   ```yaml
   - op: replace
     path: /spec/template/spec/containers/0/resources/requests/cpu
     value: 500m
   ```

3. **Helm Values**:
   ```yaml
   resources:
     requests:
       cpu: 500m
       memory: 1.5Gi
   ```

### 8. Circuit Breaker (pkg/circuitbreaker)

Prevents cascading failures during optimization.

**Key Files:**
- `circuitbreaker.go`: State machine implementation

**States:**
- **Closed**: Normal operation
- **Open**: Blocking all operations (after N consecutive failures)
- **Half-Open**: Testing if system recovered

**State Transitions:**
```
Closed --[error threshold exceeded]--> Open
Open --[timeout expired]--> Half-Open
Half-Open --[success threshold met]--> Closed
Half-Open --[any failure]--> Open
```

## Algorithms

### Resource Recommendation Algorithm

```go
func CalculateRecommendation(metrics []Metric, config RecommendationConfig) Recommendation {
    // 1. Filter metrics by time window
    filtered := filterByWindow(metrics, config.HistoryDuration)

    // 2. Calculate percentile
    value := calculatePercentile(filtered, config.CPUPercentile)

    // 3. Apply safety margin
    recommended := value * config.SafetyMargin

    // 4. Enforce thresholds
    if recommended < config.Min {
        recommended = config.Min
    }
    if recommended > config.Max {
        recommended = config.Max
    }

    // 5. Calculate confidence
    confidence := calculateConfidence(len(filtered), variance(filtered))

    return Recommendation{
        Value: recommended,
        Confidence: confidence,
    }
}
```

### Pareto Multi-Objective Optimization

Finds optimal solutions balancing multiple objectives:
- **Cost**: Lower resource usage = lower cost
- **Performance**: Higher resources = better performance
- **Reliability**: Safety margins = fewer OOM kills

```go
func FindParetoFrontier(solutions []Solution) []Solution {
    var frontier []Solution

    for i, solution := range solutions {
        dominated := false

        for j, other := range solutions {
            if i == j {
                continue
            }

            // Check if 'other' dominates 'solution'
            if dominates(other, solution) {
                dominated = true
                break
            }
        }

        if !dominated {
            frontier = append(frontier, solution)
        }
    }

    return frontier
}
```

### SLA Violation Detection

```go
func CheckSLA(metrics []Metric, sla SLADefinition) []Violation {
    var violations []Violation

    // Calculate metric value
    value := calculateMetric(metrics, sla.Type, sla.Percentile)

    // Check threshold
    threshold := sla.Target + sla.Threshold
    if value > threshold {
        severity := (value - threshold) / threshold
        violations = append(violations, Violation{
            SLA: sla,
            ActualValue: value,
            Threshold: threshold,
            Severity: min(severity, 1.0),
        })
    }

    return violations
}
```

## Data Flow

### Optimization Cycle

1. **Trigger**: Timer, config change, or manual trigger
2. **Collect Metrics**: Query Kubernetes Metrics API
3. **Analyze**: Run algorithms (percentile, prediction, Pareto)
4. **Validate**:
   - Check policies (policy engine)
   - Check SLA health (SLA monitor)
   - Check HPA conflicts
   - Check PDB violations
5. **Decision**:
   - If healthy && policies pass → Continue
   - If blocked → Record event and skip
6. **Apply** (if not dry-run):
   - Update workload resources
   - Record event
   - Export to GitOps (if enabled)
7. **Monitor**:
   - Post-optimization health check
   - Rollback if degraded
   - Update metrics

### Metrics Flow

```
Kubernetes Metrics API
    ↓
MetricsCollector.GetPodMetrics()
    ↓
MetricsStorage.Store()
    ↓
TimeSeriesDB (in-memory)
    ↓
Analyzer.Analyze()
    ↓
ResourceRecommendation
```

## Security Architecture

### RBAC Permissions

Minimum required permissions:
- **Read**: Pods, Metrics, HPAs, PDBs
- **Write**: Deployments, StatefulSets, DaemonSets
- **Full**: OptimizerConfigs (CRD)

### Security Features

1. **Non-Root User**: Runs as UID 65532
2. **Read-Only Root Filesystem**: No write access to container FS
3. **No Privileged Escalation**: Capabilities dropped
4. **Seccomp Profile**: Runtime security
5. **Network Policies**: Restrict egress/ingress
6. **Secret Management**: Git credentials in Kubernetes secrets

### Admission Control

Webhook validator ensures:
- Valid cron expressions
- Resource limits make sense (min < max)
- Regex patterns compile
- Duration formats are valid

## Integration Points

### Kubernetes

- **Metrics API**: Resource usage data
- **Events**: Audit trail
- **CRDs**: Configuration management
- **RBAC**: Security

### Prometheus

- **Metrics Export**: `/metrics` endpoint
- **ServiceMonitor**: Automatic scraping (Prometheus Operator)
- **Grafana Integration**: Pre-built dashboards

### GitOps

- **Kustomize**: Patch generation
- **Helm**: Values override
- **Git**: Commit and PR creation

### CI/CD

- **GitHub Actions**: Automated testing
- **Container Registry**: Image distribution
- **Kubernetes**: Deployment automation

## Performance Considerations

### Controller Scalability

- **Leader Election**: Single active controller
- **Work Queue**: Batching and rate limiting
- **Caching**: Informer caches reduce API calls

### Metrics Storage

- **In-Memory**: Fast access, limited retention
- **Retention**: Configurable (default 24h-7d)
- **Sampling**: Adjustable sample rate

### Optimization Frequency

- **Default**: Every 5 minutes
- **Configurable**: Via OptimizerConfig
- **Maintenance Windows**: Scheduled updates only

## Future Architecture

Planned enhancements:
1. **ML-based Prediction**: Deep learning models
2. **Multi-Cluster**: Federation support
3. **Custom Metrics**: User-defined metrics
4. **Auto-Scaling Integration**: Dynamic HPA targets
5. **Cost Attribution**: Per-team/app cost tracking
