# Intelligent Cluster Optimizer

A Kubernetes-native resource optimization system that automatically right-sizes workloads based on historical usage patterns, reducing costs while maintaining performance and reliability.

## Project Overview

This project implements an intelligent resource optimizer for Kubernetes clusters. It collects metrics, analyzes usage patterns, detects anomalies, and generates (or auto-applies) resource recommendations for workloads.

### Key Goals
- **Cost Reduction**: Right-size over-provisioned workloads
- **Reliability**: Detect memory leaks, prevent OOM kills
- **Safety**: Conservative defaults, circuit breakers, rollback capability
- **Intelligence**: Time-based patterns, confidence scoring, environment profiles

---

## Architecture

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Metrics        │────▶│  Analysis       │────▶│  Recommendation │
│  Collection     │     │  Engine         │     │  Engine         │
└─────────────────┘     └─────────────────┘     └─────────────────┘
        │                       │                       │
        ▼                       ▼                       ▼
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Anomaly        │     │  Prediction     │     │  Pareto         │
│  Detection      │     │  (Holt-Winters) │     │  Optimizer      │
└─────────────────┘     └─────────────────┘     └─────────────────┘
                                                        │
                                ┌───────────────────────┤
                                ▼                       ▼
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Policy         │────▶│  SLA            │────▶│  Safety         │
│  Engine         │     │  Monitor        │     │  Checks         │
└─────────────────┘     └─────────────────┘     └─────────────────┘
                                                        │
                                                        ▼
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Rollback       │◀────│  Applier        │     │  GitOps         │
│  Manager        │     │                 │     │  Exporter       │
└─────────────────┘     └─────────────────┘     └─────────────────┘
```

---

## Implementation Status

### Core Features

| Feature | Status | Description |
|---------|--------|-------------|
| Recommendation Engine | ✅ Done | P95/P99 percentile-based resource recommendations |
| Cost Estimator | ✅ Done | Calculates potential savings (hourly/monthly/yearly) |
| OOM Detection | ✅ Done | Detects OOM-killed containers, boosts memory, prioritizes |
| Confidence Scoring | ✅ Done | 0-100% score based on data quality (5 weighted factors) |
| Recommendation Expiry | ✅ Done | TTL-based expiry prevents stale recommendations |
| Memory Leak Detection | ✅ Done | Linear regression slope analysis with R² consistency |
| Time Pattern Analyzer | ✅ Done | Detects business hours, night batch, spike patterns |
| Environment Profiles | ✅ Done | Production/Staging/Development/Test presets |
| Circuit Breaker | ✅ Done | Stops scaling after repeated failures |
| Emergency Rollback | ✅ Done | Reverts changes if health checks fail |
| HPA/PDB Conflict Check | ✅ Done | Avoids conflicts with existing autoscalers |

### Advanced Analytics

| Feature | Status | Description |
|---------|--------|-------------|
| Anomaly Detection | ✅ Done | Multi-method consensus (Z-Score, IQR, Moving Average) |
| Time Series Prediction | ✅ Done | Holt-Winters forecasting for proactive scaling |
| Pareto Optimization | ✅ Done | Multi-objective optimization (cost, performance, reliability, efficiency, stability) |

### Policy & Governance

| Feature | Status | Description |
|---------|--------|-------------|
| Policy Engine | ✅ Done | Expression-based policies with YAML configuration |
| SLA Monitoring | ✅ Done | Latency, error rate, availability, throughput tracking |
| Health Checker | ✅ Done | Control chart-based health assessment |

### GitOps Integration

| Feature | Status | Description |
|---------|--------|-------------|
| Kustomize Export | ✅ Done | Strategic merge and JSON 6902 patch generation |
| Helm Export | ✅ Done | Values.yaml generation for Helm charts |

### Infrastructure

| Component | Status | Description |
|-----------|--------|-------------|
| Kubernetes Controller | ✅ Done | Reconciliation loop watching OptimizerConfig CRD |
| Custom Resource (CRD) | ✅ Done | OptimizerConfig for declarative configuration |
| Metrics Collector | ✅ Done | Collects pod CPU/memory from metrics API |
| In-Memory Storage | ✅ Done | Stores historical metrics |
| Vertical Scaler | ✅ Done | Patches deployment resource specs |
| Event Recorder | ✅ Done | Records Kubernetes events for audit |

### Testing

| Test Type | Status | Coverage |
|-----------|--------|----------|
| Unit Tests | ✅ Done | All major packages |
| Integration Tests | ✅ Done | CSV-based end-to-end scenarios |
| Stress Tests | ❌ Pending | Large-scale performance testing |

---

## Project Structure

```
intelligent-cluster-optimizer/
├── cmd/
│   ├── controller/      # Main Kubernetes controller
│   ├── collector/       # Standalone metrics collector
│   └── optctl/          # CLI tool
├── config/
│   └── crd/             # Kubernetes CRD definitions
├── pkg/
│   ├── apis/            # Custom Resource types
│   ├── controller/      # Kubernetes controller logic
│   ├── recommendation/  # Core recommendation engine
│   ├── leakdetector/    # Memory leak detection
│   ├── timepattern/     # Time-based pattern analysis
│   ├── profile/         # Environment profiles
│   ├── safety/          # Safety checks (OOM, HPA, PDB, circuit breaker)
│   ├── rollback/        # Emergency rollback system
│   ├── cost/            # Cost calculation
│   ├── metrics/         # Metrics collection
│   ├── storage/         # Metrics storage
│   ├── applier/         # Change application
│   ├── scaler/          # Vertical scaling
│   ├── scheduler/       # Maintenance windows
│   ├── anomaly/         # Statistical anomaly detection
│   ├── prediction/      # Time series forecasting (Holt-Winters)
│   ├── pareto/          # Multi-objective Pareto optimization
│   ├── policy/          # Policy engine with expression evaluation
│   ├── sla/             # SLA monitoring and health checking
│   ├── gitops/          # GitOps export (Kustomize, Helm)
│   └── events/          # Kubernetes event broadcasting
└── go.mod
```

---

## How It Works

### 1. Data Collection
- Scrapes CPU/memory metrics from Kubernetes metrics API
- Stores 24 hours of historical data per container
- Detects anomalies using statistical methods (Z-Score, IQR, Moving Average)

### 2. Analysis
- **Leak Detection**: Analyzes memory slope; blocks scaling if leak detected
- **Pattern Detection**: Identifies business hours, night batch, spike patterns
- **Anomaly Detection**: Multi-method consensus for outlier detection
- **Time Series Prediction**: Holt-Winters forecasting for proactive scaling
- **Profile Resolution**: Applies environment-specific settings (prod vs dev)

### 3. Recommendation Generation
- Calculates P95/P99 percentiles from historical usage
- Applies safety margin (1.2x default)
- Boosts memory for OOM-affected containers
- Scores confidence based on data quality
- Estimates cost savings
- **Pareto Optimization**: Generates multiple solutions balancing:
  - Cost (minimize resource spend)
  - Performance (headroom above average usage)
  - Reliability (buffer for peak loads)
  - Efficiency (resource utilization)
  - Stability (minimize change frequency)

### 4. Policy Evaluation
- Evaluates YAML-defined policies with expression-based conditions
- Supports actions: allow, deny, skip, modify, require-approval
- Enforces resource limits (min/max CPU/memory)
- Priority-based policy ordering

### 5. SLA Monitoring
- Tracks latency, error rate, availability, throughput SLAs
- Percentile-based latency checks (P95, P99)
- Control chart-based health assessment
- Blocks scaling during SLA violations

### 6. Safety Checks
- Verifies no HPA/PDB conflicts
- Checks circuit breaker state
- Validates recommendation confidence threshold
- Enforces MaxChangePercent limits

### 7. Application
- Patches deployment resource requests/limits
- Monitors health for rollback window
- Records events for audit trail

### 8. GitOps Export
- Exports recommendations as Kustomize patches (strategic merge or JSON 6902)
- Generates Helm values.yaml for GitOps workflows
- Supports PR-based review processes

---

## Test Scenarios

The integration tests validate these real-world scenarios:

| Scenario | Input Pattern | Expected Behavior |
|----------|---------------|-------------------|
| Memory Leak | Continuously growing memory | Block scaling, alert |
| Stable Usage | Normal GC sawtooth, low usage | Recommend scale down |
| Business Hours | High 9-5, low otherwise | Recommend schedule-based scaling |
| High Usage | Consistently near limits | Recommend scale up |

---

## Running Tests

```bash
# Run all tests
go test ./pkg/...

# Run integration tests only
go test ./pkg/recommendation/... -run Integration -v

# Run specific package tests
go test ./pkg/leakdetector/... -v
go test ./pkg/timepattern/... -v
go test ./pkg/safety/... -v
go test ./pkg/anomaly/... -v
go test ./pkg/prediction/... -v
go test ./pkg/pareto/... -v
go test ./pkg/policy/... -v
go test ./pkg/sla/... -v
go test ./pkg/gitops/... -v
```

---

## Pending Work

- [ ] Stress tests (10k workloads, 1M samples)
- [ ] Prometheus metrics integration
- [ ] Webhook notifications (Slack, PagerDuty)
- [ ] Web dashboard for visualization
- [ ] Helm chart for production deployment

---

## Technologies

- **Language**: Go 1.21+
- **Framework**: Kubernetes controller-runtime
- **APIs**: Kubernetes metrics API, custom CRDs
- **Policy Engine**: expr-lang/expr for expression evaluation
- **Testing**: Go testing, table-driven tests, CSV test data
- **GitOps**: Kustomize patches, Helm values generation
