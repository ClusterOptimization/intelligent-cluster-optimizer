# Intelligent Cluster Optimizer - Usage Guide

## Table of Contents

- [Quick Start](#quick-start)
- [Installation](#installation)
- [CLI Tool (optctl)](#cli-tool-optctl)
- [Configuration](#configuration)
- [Common Use Cases](#common-use-cases)
- [Monitoring](#monitoring)
- [Troubleshooting](#troubleshooting)
- [Best Practices](#best-practices)

## Quick Start

### 1. Install the Optimizer

```bash
# Install using kubectl
kubectl apply -f deploy/

# Or using kustomize
kubectl apply -k deploy/
```

### 2. Create Your First OptimizerConfig

```bash
# Start with dry-run mode
kubectl apply -f examples/basic.yaml
```

### 3. Monitor Recommendations

```bash
# Watch controller logs
kubectl logs -n intelligent-optimizer-system \
  -l app.kubernetes.io/component=controller -f

# Check events
kubectl get events -n default \
  --field-selector involvedObject.kind=OptimizerConfig
```

### 4. Review and Apply

After reviewing recommendations in dry-run mode:

```bash
# Edit config to disable dry-run
kubectl edit optimizerconfig basic-optimizer

# Change: dryRun: false
```

## Installation

### Prerequisites

- Kubernetes 1.20+
- Metrics Server installed and working
- kubectl access with cluster-admin privileges

### Verify Prerequisites

```bash
# Check Kubernetes version
kubectl version --short

# Verify Metrics Server
kubectl top nodes
kubectl top pods -A

# If metrics-server is not installed:
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml
```

### Installation Methods

#### Method 1: Direct kubectl Apply

```bash
# Clone the repository
git clone https://github.com/k8s-resource-optimizer/intelligent-cluster-optimizer
cd intelligent-cluster-optimizer

# Deploy
kubectl apply -f deploy/
```

#### Method 2: Kustomize

```bash
# Using kubectl kustomize
kubectl apply -k deploy/

# With custom image
cd deploy/
kustomize edit set image intelligent-cluster-optimizer=myregistry/intelligent-cluster-optimizer:v1.0.0
kubectl apply -k .
```

#### Method 3: Helm (Coming Soon)

```bash
helm repo add intelligent-optimizer https://k8s-resource-optimizer.github.io/charts
helm install intelligent-optimizer intelligent-optimizer/intelligent-cluster-optimizer
```

### Verify Installation

```bash
# Check deployment status
kubectl get deployment -n intelligent-optimizer-system

# Check pods are running
kubectl get pods -n intelligent-optimizer-system

# Check logs
kubectl logs -n intelligent-optimizer-system \
  deployment/intelligent-optimizer-controller
```

## CLI Tool (optctl)

The `optctl` CLI provides commands for cluster monitoring, cost analysis, history tracking, and rollback operations.

### Building the CLI

```bash
go build -o bin/optctl ./cmd/optctl/
```

### Dashboard

Get a quick overview of your cluster with the dashboard command:

```bash
# Show cluster dashboard
optctl dashboard

# With specific pricing model
optctl --pricing=aws-us-east-1 dashboard
```

**What the Dashboard Shows:**

| Section | Information |
|---------|-------------|
| Cluster Overview | Nodes, namespaces, workloads, containers, replicas |
| Resource Summary | Total CPU and memory requests |
| Cost Summary | Hourly, daily, monthly, yearly costs |
| Top Workloads | Top 5 workloads by cost |
| Recent History | Last 5 optimization entries |
| Quick Commands | Common optctl commands reference |

**Sample Output:**
```
╔══════════════════════════════════════════════════════════════════════════════╗
║                    INTELLIGENT CLUSTER OPTIMIZER                              ║
║                           Dashboard v1.2.0                                    ║
╚══════════════════════════════════════════════════════════════════════════════╝

┌─ CLUSTER OVERVIEW ───────────────────────────────────────────────────────────┐
│  Nodes: 3    Namespaces: 5    Workloads: 12    Containers: 18    Replicas: 24 │
└──────────────────────────────────────────────────────────────────────────────┘

┌─ RESOURCE SUMMARY ───────────────────────────────────────────────────────────┐
│  Total CPU Requests:     4.50 cores                                          │
│  Total Memory Requests:  8.00 Gi                                             │
└──────────────────────────────────────────────────────────────────────────────┘

┌─ COST SUMMARY (aws-us-east-1) ───────────────────────────────────────────────┐
│  Hourly:   $0.25     Daily:    $6.00     Monthly:  $180.00    Yearly: $2190   │
└──────────────────────────────────────────────────────────────────────────────┘

┌─ TOP WORKLOADS BY COST ──────────────────────────────────────────────────────┐
│  1. prod/Deployment/api           $45.36/mo    500m CPU    512Mi Memory      │
│  2. prod/StatefulSet/postgres     $34.56/mo    1 core      2Gi Memory        │
└──────────────────────────────────────────────────────────────────────────────┘
```

### Cost Calculator

Calculate resource costs for your cluster using cloud provider pricing:

```bash
# Show available pricing models
optctl cost pricing

# Calculate costs for a namespace
optctl cost production

# Use AWS pricing model
optctl --pricing=aws-us-east-1 cost production

# Use spot instance pricing
optctl --pricing=aws-us-east-1-spot cost production

# Calculate costs across all namespaces
optctl --all-namespaces cost
```

**Available Pricing Models:**

| Model | Provider | Description |
|-------|----------|-------------|
| `default` | Generic | Conservative estimate |
| `aws-us-east-1` | AWS | On-demand pricing |
| `aws-us-east-1-spot` | AWS | Spot instance (~70% savings) |
| `gcp-us-central1` | GCP | On-demand pricing |
| `azure-eastus` | Azure | On-demand pricing |

**Sample Output:**
```
Resource Cost Report (Pricing: aws-us-east-1)
================================================================================
Namespaces: 1 | Workloads: 5 | Containers: 8 | Replicas: 12
--------------------------------------------------------------------------------
NAMESPACE  WORKLOAD              REPLICAS  CPU         MEMORY    COST/MONTH
prod       Deployment/api        3         500m        512 Mi    $45.36
prod       StatefulSet/postgres  1         1.00 cores  2.00 Gi   $34.56
--------------------------------------------------------------------------------
COST SUMMARY
--------------------------------------------------------------------------------
Total CPU:      2.50 cores
Total Memory:   4.00 Gi
Monthly Cost:   $89.86
Yearly Cost:    $1093.01
```

### History Tracking

View optimization history and previous configurations:

```bash
# Show all optimization history
optctl history

# Show history for specific workload
optctl --container=nginx history default/Deployment/nginx

# Use custom history file
optctl --history-file=/custom/path/history.json history
```

**Sample Output:**
```
Optimization History (3 entries across 2 workloads)
--------------------------------------------------------------------------------
WORKLOAD                  CONTAINER  CPU   MEMORY  TIMESTAMP         AGE
default/Deployment/nginx  nginx      200m  256Mi   2025-12-27 12:00  2h
default/Deployment/nginx  nginx      100m  128Mi   2025-12-27 10:00  4h
prod/StatefulSet/redis    redis      500m  512Mi   2025-12-26 08:00  1d
```

### Rollback Operations

Revert workloads to previous resource configurations:

```bash
# Rollback to previous configuration
optctl rollback default/Deployment/nginx

# Rollback specific container
optctl --container=app rollback prod/StatefulSet/redis
```

The rollback command:
1. Loads history from the persistent history file
2. Finds the previous configuration for the workload
3. Applies the previous resource requests
4. Updates the history file

### CLI Options

| Option | Description | Default |
|--------|-------------|---------|
| `--kubeconfig` | Path to kubeconfig | `~/.kube/config` |
| `--container` | Target container name | First container |
| `--pricing` | Pricing model for cost | `default` |
| `--all-namespaces` | Operate across all namespaces | `false` |
| `--history-file` | Path to history file | `/var/lib/optimizer/rollback-history.json` |
| `--json` | Output in JSON format | `false` |

## Configuration

### OptimizerConfig Basics

An OptimizerConfig is a Kubernetes custom resource that defines optimization behavior:

```yaml
apiVersion: optimizer.intelligent-cluster-optimizer.io/v1alpha1
kind: OptimizerConfig
metadata:
  name: my-optimizer
  namespace: default
spec:
  targetNamespaces:
    - default
  strategy: balanced
  dryRun: true
```

### Profile-Based Configuration

Choose a profile that matches your environment:

```yaml
spec:
  profile: production  # or: staging, development, test, custom
```

**Profile Characteristics:**

| Profile | Strategy | Safety Margin | Min Confidence | Apply Delay |
|---------|----------|---------------|----------------|-------------|
| production | conservative | 1.5x | 80% | 24h |
| staging | balanced | 1.3x | 70% | 12h |
| development | aggressive | 1.1x | 60% | 1h |
| test | aggressive | 1.05x | 50% | 0 |
| custom | (user defined) | (user defined) | (user defined) | (user defined) |

### Fine-Grained Configuration

#### Target Namespaces

```yaml
spec:
  targetNamespaces:
    - production
    - staging
    - pre-prod
```

#### Resource Thresholds

Prevent recommendations outside acceptable ranges:

```yaml
spec:
  resourceThresholds:
    cpu:
      min: "100m"    # Minimum 100 millicores
      max: "8"       # Maximum 8 cores
    memory:
      min: "256Mi"   # Minimum 256 MiB
      max: "16Gi"    # Maximum 16 GiB
```

#### Recommendation Settings

```yaml
spec:
  recommendations:
    cpuPercentile: 95        # Use P95 for CPU
    memoryPercentile: 95     # Use P95 for memory
    safetyMargin: 1.2        # Add 20% buffer
    historyDuration: "7d"    # Use 7 days of history
    minSamples: 1000         # Require 1000 data points
```

#### Maintenance Windows

Schedule when updates can be applied:

```yaml
spec:
  maintenanceWindows:
    # Every Sunday at 2 AM UTC
    - schedule: "0 2 * * 0"
      duration: "4h"
      timezone: "UTC"

    # Every Wednesday at 10 PM Eastern
    - schedule: "0 22 * * 3"
      duration: "2h"
      timezone: "America/New_York"
```

#### Update Strategy

```yaml
spec:
  updateStrategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: "25%"
      maxSurge: "25%"
```

#### HPA and PDB Awareness

```yaml
spec:
  # HPA Awareness
  hpaAwareness:
    enabled: true
    conflictPolicy: Skip  # Skip, Override, or Warn

  # PDB Awareness
  pdbAwareness:
    enabled: true
    respectMinAvailable: true
```

#### Circuit Breaker

```yaml
spec:
  circuitBreaker:
    enabled: true
    errorThreshold: 5       # Open after 5 failures
    successThreshold: 3     # Close after 3 successes
    timeout: "5m"           # Try again after 5 minutes
```

#### GitOps Export

```yaml
spec:
  gitOpsExport:
    enabled: true
    format: kustomize  # kustomize, kustomize-json6902, or helm
    outputPath: "./gitops-exports"
    autoCommit: true
    gitRepository:
      url: "https://github.com/myorg/k8s-manifests.git"
      branch: "optimizer-recommendations"
      baseBranch: "main"
      createPR: true
      prTitle: "Resource Optimization Recommendations"
      authSecretName: "git-credentials"
```

#### Workload Exclusion

```yaml
spec:
  excludeWorkloads:
    - "^kube-.*"           # Exclude kube-system workloads
    - ".*-monitoring$"     # Exclude monitoring workloads
    - "critical-app"       # Exclude specific workload
```

## Common Use Cases

### Use Case 1: Cost Optimization in Development

**Goal:** Reduce costs in dev/test environments

```yaml
apiVersion: optimizer.intelligent-cluster-optimizer.io/v1alpha1
kind: OptimizerConfig
metadata:
  name: dev-cost-optimizer
spec:
  profile: development
  targetNamespaces:
    - dev
    - test
  recommendations:
    cpuPercentile: 90
    memoryPercentile: 90
    safetyMargin: 1.1
  dryRun: false  # Apply immediately
```

**Expected Results:**
- 30-50% cost reduction
- Minimal performance impact
- Fast iteration cycles

### Use Case 2: Production Stability

**Goal:** Safe, gradual optimization in production

```yaml
apiVersion: optimizer.intelligent-cluster-optimizer.io/v1alpha1
kind: OptimizerConfig
metadata:
  name: prod-optimizer
spec:
  profile: production
  targetNamespaces:
    - production
  maintenanceWindows:
    - schedule: "0 2 * * 0"
      duration: "4h"
  profileOverrides:
    requireApproval: true
    applyDelay: "24h"
  circuitBreaker:
    enabled: true
  dryRun: false
```

**Expected Results:**
- 15-25% cost reduction
- High stability and safety
- Controlled rollout

### Use Case 3: GitOps Integration

**Goal:** Export recommendations to git for review

```yaml
apiVersion: optimizer.intelligent-cluster-optimizer.io/v1alpha1
kind: OptimizerConfig
metadata:
  name: gitops-optimizer
spec:
  targetNamespaces:
    - production
  dryRun: true  # Controller doesn't apply
  gitOpsExport:
    enabled: true
    format: kustomize
    autoCommit: true
    gitRepository:
      url: "https://github.com/myorg/k8s-manifests.git"
      createPR: true
```

**Workflow:**
1. Optimizer generates recommendations
2. Exports to git repository
3. Creates pull request
4. Team reviews and approves
5. ArgoCD/Flux applies changes

### Use Case 4: SLA-Aware Optimization

**Goal:** Optimize while maintaining SLAs

```yaml
apiVersion: optimizer.intelligent-cluster-optimizer.io/v1alpha1
kind: OptimizerConfig
metadata:
  name: sla-aware-optimizer
spec:
  targetNamespaces:
    - production
  recommendations:
    cpuPercentile: 95
    memoryPercentile: 95
    safetyMargin: 1.3
  # SLA monitoring built-in
  # Will block optimization if health degrades
```

## Monitoring

### Prometheus Metrics

The optimizer exposes metrics at `:8080/metrics`:

**Key Metrics:**
- `intelligent_optimizer_reconciliation_total`: Reconciliation attempts
- `intelligent_optimizer_reconciliation_duration_seconds`: Reconciliation time
- `intelligent_optimizer_cpu_recommendation_millicores`: CPU recommendations
- `intelligent_optimizer_memory_recommendation_mib`: Memory recommendations
- `intelligent_optimizer_sla_violations_total`: SLA violations detected
- `intelligent_optimizer_health_score`: System health score (0-100)

**Query Examples:**

```promql
# Reconciliation success rate
rate(intelligent_optimizer_reconciliation_total{result="success"}[5m])
/ rate(intelligent_optimizer_reconciliation_total[5m])

# Average CPU recommendation
avg(intelligent_optimizer_cpu_recommendation_millicores)

# SLA violation rate
rate(intelligent_optimizer_sla_violations_total[5m])
```

### Grafana Dashboard

Import the pre-built dashboard:

```bash
# Dashboard ID: TBD (coming soon)
```

**Panels:**
- Reconciliation rate and duration
- Resource recommendations over time
- SLA violations
- Health scores
- Policy evaluations

### Kubernetes Events

```bash
# View all optimizer events
kubectl get events -A --field-selector \
  involvedObject.kind=OptimizerConfig

# Watch events in real-time
kubectl get events -n default -w \
  --field-selector involvedObject.kind=OptimizerConfig
```

### Logs

```bash
# Controller logs (structured JSON in production)
kubectl logs -n intelligent-optimizer-system \
  deployment/intelligent-optimizer-controller -f

# Filter by level
kubectl logs -n intelligent-optimizer-system \
  deployment/intelligent-optimizer-controller -f \
  | grep '"level":"error"'

# Filter by config
kubectl logs -n intelligent-optimizer-system \
  deployment/intelligent-optimizer-controller -f \
  | grep '"config":"my-optimizer"'
```

## Troubleshooting

### No Recommendations Generated

**Symptoms:**
- Controller logs show no recommendation activity
- Events show no optimization attempts

**Common Causes:**

1. **Insufficient Metrics**
   ```bash
   # Verify metrics are available
   kubectl top pods -n <namespace>

   # Check history duration and min samples
   kubectl get optimizerconfig <name> -o yaml | grep -A 5 recommendations
   ```

2. **Outside Maintenance Window**
   ```bash
   # Check if in maintenance window
   kubectl get optimizerconfig <name> -o jsonpath='{.status.activeMaintenanceWindow}'

   # Check next window
   kubectl get optimizerconfig <name> -o jsonpath='{.status.nextMaintenanceWindow}'
   ```

3. **Workloads Excluded**
   ```bash
   # Check exclude patterns
   kubectl get optimizerconfig <name> -o jsonpath='{.spec.excludeWorkloads}'
   ```

### Recommendations Not Applied

**Symptoms:**
- Recommendations generated but not applied
- No resource changes observed

**Common Causes:**

1. **Dry-Run Mode**
   ```bash
   kubectl get optimizerconfig <name> -o jsonpath='{.spec.dryRun}'
   # Should be 'false' to apply changes
   ```

2. **Circuit Breaker Open**
   ```bash
   kubectl get optimizerconfig <name> -o jsonpath='{.status.circuitState}'
   # Should be 'Closed' for normal operation
   ```

3. **Policy Blocking**
   ```bash
   # Check policy evaluation events
   kubectl get events -n <namespace> | grep PolicyBlocked
   ```

4. **HPA Conflict**
   ```bash
   # Check HPA status
   kubectl get hpa -n <namespace>

   # Check conflict policy
   kubectl get optimizerconfig <name> -o jsonpath='{.spec.hpaAwareness.conflictPolicy}'
   ```

### High Error Rate

**Symptoms:**
- Many `intelligent_optimizer_reconciliation_errors_total` metrics
- Frequent error logs

**Common Causes:**

1. **RBAC Issues**
   ```bash
   # Test permissions
   kubectl auth can-i update deployments \
     --as=system:serviceaccount:intelligent-optimizer-system:intelligent-optimizer-controller \
     -n <namespace>
   ```

2. **API Server Issues**
   ```bash
   # Check API server health
   kubectl get --raw /healthz
   ```

3. **Metrics Server Down**
   ```bash
   kubectl get deployment -n kube-system metrics-server
   kubectl top nodes  # Should work
   ```

### Performance Issues

**Symptoms:**
- High reconciliation duration
- Controller using too much CPU/memory

**Solutions:**

1. **Reduce Reconciliation Frequency**
   ```yaml
   # In controller deployment
   args:
     - --reconcile-interval=10m  # Default: 5m
   ```

2. **Limit Target Namespaces**
   ```yaml
   spec:
     targetNamespaces:
       - production  # Only optimize necessary namespaces
   ```

3. **Adjust History Window**
   ```yaml
   spec:
     recommendations:
       historyDuration: "24h"  # Reduce from 7d if needed
   ```

## Best Practices

### 1. Start with Dry-Run

Always test in dry-run mode first:

```yaml
spec:
  dryRun: true
```

Review logs and events before enabling live updates.

### 2. Use Appropriate Profiles

Match profiles to environments:
- Production → `production` profile
- Staging → `staging` profile
- Development → `development` profile
- CI/CD → `test` profile

### 3. Set Maintenance Windows

For production, always use maintenance windows:

```yaml
spec:
  maintenanceWindows:
    - schedule: "0 2 * * 0"  # Sundays 2 AM
      duration: "4h"
```

### 4. Enable Safety Features

Always enable in production:

```yaml
spec:
  hpaAwareness:
    enabled: true
  pdbAwareness:
    enabled: true
  circuitBreaker:
    enabled: true
```

### 5. Monitor Health Metrics

Set up alerts on:
- `intelligent_optimizer_health_score < 70`
- `rate(intelligent_optimizer_sla_violations_total[5m]) > 0`
- `intelligent_optimizer_circuit_state != "Closed"`

### 6. Use GitOps for Production

For production changes, use GitOps workflow:

```yaml
spec:
  dryRun: true  # Don't apply directly
  gitOpsExport:
    enabled: true
    autoCommit: true
    gitRepository:
      createPR: true
```

### 7. Gradual Rollout

Start conservative, gradually tune:

1. Week 1: `safetyMargin: 2.0` (100% buffer)
2. Week 2: `safetyMargin: 1.5` (50% buffer)
3. Week 3: `safetyMargin: 1.3` (30% buffer)
4. Week 4+: `safetyMargin: 1.2` (20% buffer)

### 8. Exclude Critical Workloads

Always exclude:
- System components (kube-system)
- Monitoring (Prometheus, Grafana)
- Critical business apps (until validated)

```yaml
spec:
  excludeWorkloads:
    - "^kube-.*"
    - ".*-monitoring$"
    - "critical-payment-processor"
```

### 9. Regular Reviews

- Weekly: Review recommendations and metrics
- Monthly: Adjust safety margins based on data
- Quarterly: Re-evaluate profiles and thresholds

### 10. Document Changes

Use OptimizerConfig annotations to document decisions:

```yaml
metadata:
  annotations:
    description: "Production optimizer for microservices"
    owner: "platform-team"
    last-reviewed: "2025-01-15"
```

## Advanced Topics

### Custom Metrics Integration

(Coming in v1.1)

### Multi-Cluster Setup

(Coming in v1.2)

### Cost Attribution

(Coming in v1.3)

## Getting Help

- Documentation: https://docs.intelligent-cluster-optimizer.io
- GitHub Issues: https://github.com/k8s-resource-optimizer/intelligent-cluster-optimizer/issues
- Discussions: https://github.com/k8s-resource-optimizer/intelligent-cluster-optimizer/discussions
- Slack: #intelligent-cluster-optimizer (Kubernetes Slack)
