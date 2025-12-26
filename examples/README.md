# OptimizerConfig Examples

This directory contains example OptimizerConfig resources demonstrating different use cases and configurations.

## Quick Start

Apply an example configuration:

```bash
kubectl apply -f examples/basic.yaml
```

## Available Examples

### 1. Basic Configuration (`basic.yaml`)

**Use Case:** Getting started with the optimizer

**Features:**
- Targets `default` and `production` namespaces
- Balanced optimization strategy
- Dry-run mode enabled (safe to test)
- Optimizes Deployments and StatefulSets

**When to Use:**
- First time trying the optimizer
- Testing in a development cluster
- Learning how the optimizer works

```bash
kubectl apply -f examples/basic.yaml
```

### 2. Production Configuration (`production.yaml`)

**Use Case:** Production clusters requiring high stability

**Features:**
- Conservative production profile
- 85% confidence requirement
- Manual approval required
- 24-hour delay before applying changes
- Maintenance window scheduling (Sundays 2 AM)
- 50% safety margin on recommendations
- HPA and PDB conflict detection
- Circuit breaker protection
- Excludes critical system workloads

**When to Use:**
- Production environments
- Clusters running business-critical applications
- Compliance-sensitive deployments

```bash
kubectl apply -f examples/production.yaml
```

### 3. Development Configuration (`development.yaml`)

**Use Case:** Development and test environments

**Features:**
- Aggressive development profile
- Lower confidence threshold (60%)
- Immediate application of changes
- Minimal safety margin (10%)
- Shorter history window (24h)
- Fewer samples required (50)
- HPA and PDB checks disabled
- Lenient circuit breaker

**When to Use:**
- Development clusters
- Test environments
- CI/CD pipelines
- Cost optimization experiments

```bash
kubectl apply -f examples/development.yaml
```

### 4. GitOps-Enabled Configuration (`gitops-enabled.yaml`)

**Use Case:** GitOps workflows with automatic PR creation

**Features:**
- Kustomize export format
- Automatic git commits
- Pull request creation
- Dry-run mode (GitOps handles deployment)
- Conservative settings for production
- Git credentials from Kubernetes secret

**Prerequisites:**
```bash
# Create git credentials secret
kubectl create secret generic git-credentials \
  --from-literal=username=your-username \
  --from-literal=password=your-token \
  -n intelligent-optimizer-system
```

**When to Use:**
- GitOps-managed clusters (ArgoCD, Flux)
- Teams requiring review before changes
- Audit trail requirements
- Multi-cluster optimization

```bash
kubectl apply -f examples/gitops-enabled.yaml
```

### 5. Advanced Configuration (`advanced.yaml`)

**Use Case:** Complex production setups with all features

**Features:**
- Multiple maintenance windows
- Rolling update strategy
- 2 weeks of historical data
- Helm export format
- HPA awareness with warnings
- Comprehensive workload exclusions
- All resource types (Deployments, StatefulSets, DaemonSets)
- Fine-tuned thresholds and safety settings

**When to Use:**
- Large production clusters
- Multi-team environments
- Advanced optimization scenarios
- Custom integration requirements

```bash
kubectl apply -f examples/advanced.yaml
```

## Configuration Comparison

| Feature | Basic | Development | Production | GitOps | Advanced |
|---------|-------|-------------|------------|--------|----------|
| Dry Run | ✓ | ✗ | ✗ | ✓ | ✗ |
| Maintenance Windows | ✗ | ✗ | ✓ | ✗ | ✓ |
| HPA Awareness | ✗ | ✗ | ✓ | ✓ | ✓ |
| PDB Awareness | ✗ | ✗ | ✓ | ✓ | ✓ |
| Circuit Breaker | ✗ | ✓ | ✓ | ✓ | ✓ |
| GitOps Export | ✗ | ✗ | ✗ | ✓ | ✓ |
| Safety Margin | Default | 1.1x | 1.5x | 1.3x | 1.3x |
| Min Confidence | Default | 60% | 85% | Default | 80% |

## Customization Guide

### Adjusting Aggressiveness

**More Aggressive (Cost Savings):**
```yaml
recommendations:
  cpuPercentile: 90        # Lower percentile
  memoryPercentile: 90
  safetyMargin: 1.1        # Smaller buffer
```

**More Conservative (Stability):**
```yaml
recommendations:
  cpuPercentile: 99        # Higher percentile
  memoryPercentile: 99
  safetyMargin: 2.0        # Larger buffer
```

### Scheduling Maintenance Windows

```yaml
maintenanceWindows:
  # Every night at 2 AM
  - schedule: "0 2 * * *"
    duration: "2h"
    timezone: "UTC"

  # Weekends only
  - schedule: "0 0 * * 6,0"
    duration: "8h"
    timezone: "America/New_York"
```

### Excluding Workloads

```yaml
excludeWorkloads:
  # System workloads
  - "^kube-.*"

  # By label or annotation patterns
  - ".*-critical$"
  - "^legacy-.*"

  # Specific names
  - "important-app"
```

## Verification

After applying a configuration, verify it's working:

```bash
# Check OptimizerConfig status
kubectl get optimizerconfig -A

# View controller logs
kubectl logs -n intelligent-optimizer-system \
  -l app.kubernetes.io/component=controller -f

# Check for events
kubectl get events -n default --field-selector \
  involvedObject.kind=OptimizerConfig
```

## Troubleshooting

### Configuration Not Applying

1. **Check validation errors:**
   ```bash
   kubectl describe optimizerconfig <name>
   ```

2. **Verify controller is running:**
   ```bash
   kubectl get pods -n intelligent-optimizer-system
   ```

3. **Check controller logs:**
   ```bash
   kubectl logs -n intelligent-optimizer-system \
     deployment/intelligent-optimizer-controller
   ```

### No Recommendations Generated

- **Insufficient metrics:** Wait for enough data (check `minSamples`)
- **Metrics server not available:** Verify `kubectl top pods` works
- **Workloads excluded:** Check `excludeWorkloads` patterns
- **Outside maintenance window:** Check maintenance window schedule

## Best Practices

1. **Start with Dry-Run**
   - Always test with `dryRun: true` first
   - Review logs and events
   - Gradually enable live updates

2. **Use Appropriate Profiles**
   - `production`: Business-critical workloads
   - `staging`: Pre-production testing
   - `development`: Dev/test environments
   - `test`: Ephemeral/temporary workloads

3. **Set Conservative Thresholds Initially**
   - Start with higher safety margins (1.5-2.0)
   - Increase `minSamples` for stability
   - Use longer `historyDuration` (7-14 days)
   - Gradually tune based on results

4. **Monitor Impact**
   - Watch Prometheus metrics
   - Review application performance
   - Check cost savings vs. stability

5. **Enable Safety Features**
   - Always use circuit breaker in production
   - Enable HPA awareness
   - Enable PDB awareness
   - Set maintenance windows
   - Exclude critical workloads

## Next Steps

1. Choose an example that matches your use case
2. Customize the configuration for your environment
3. Apply with `kubectl apply -f`
4. Monitor the controller logs
5. Review recommendations in dry-run mode
6. Gradually enable live updates

For more information, see the main [ARCHITECTURE.md](../ARCHITECTURE.md) and [USAGE.md](../USAGE.md) documentation.
