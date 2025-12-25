# Deployment Manifests

This directory contains Kubernetes deployment manifests for the Intelligent Cluster Optimizer.

## Components

- **namespace.yaml**: Creates the `intelligent-optimizer-system` namespace
- **serviceaccount.yaml**: ServiceAccount for the controller
- **rbac.yaml**: ClusterRole and ClusterRoleBinding with necessary permissions
- **deployment.yaml**: Controller deployment configuration
- **service.yaml**: Service for Prometheus metrics endpoint
- **servicemonitor.yaml**: ServiceMonitor for Prometheus Operator (optional)
- **kustomization.yaml**: Kustomize configuration for easy deployment

## Prerequisites

1. Kubernetes cluster (v1.20+)
2. Metrics Server installed (for resource recommendations)
3. (Optional) Prometheus Operator (for ServiceMonitor)

## Quick Start

### Using kubectl

```bash
# Deploy all resources
kubectl apply -f deploy/

# Or deploy in order
kubectl apply -f deploy/namespace.yaml
kubectl apply -f deploy/serviceaccount.yaml
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/deployment.yaml
kubectl apply -f deploy/service.yaml
```

### Using Kustomize

```bash
# Build and view manifests
kubectl kustomize deploy/

# Deploy using kustomize
kubectl apply -k deploy/
```

## Configuration

### Image

By default, the deployment uses `intelligent-cluster-optimizer:latest`. To use a different image:

**With Kustomize:**
```bash
cd deploy/
kustomize edit set image intelligent-cluster-optimizer=myregistry/intelligent-cluster-optimizer:v1.0.0
kubectl apply -k .
```

**With kubectl:**
Edit `deployment.yaml` and change the `image` field.

### Resource Limits

Default resource requests and limits:
- Requests: 100m CPU, 128Mi memory
- Limits: 500m CPU, 512Mi memory

Adjust these in `deployment.yaml` based on your cluster size and optimization workload.

### Replicas

The controller runs with 1 replica by default (with leader election). You can increase replicas for high availability:

```bash
kubectl scale deployment intelligent-optimizer-controller -n intelligent-optimizer-system --replicas=3
```

## Verification

Check if the controller is running:

```bash
# Check deployment status
kubectl get deployment -n intelligent-optimizer-system

# Check controller logs
kubectl logs -n intelligent-optimizer-system -l app.kubernetes.io/component=controller -f

# Check if metrics endpoint is accessible
kubectl port-forward -n intelligent-optimizer-system svc/intelligent-optimizer-metrics 8080:8080
# Then visit http://localhost:8080/metrics
```

## Security

The deployment follows Kubernetes security best practices:

- Runs as non-root user (UID 65532)
- Read-only root filesystem
- Drops all capabilities
- Uses seccomp profile
- Resource limits enforced

## Monitoring

### Prometheus Metrics

The controller exposes metrics on port 8080 at `/metrics`. If you have Prometheus Operator installed, the ServiceMonitor will automatically configure scraping.

**Manual Prometheus configuration:**
```yaml
scrape_configs:
  - job_name: 'intelligent-optimizer'
    kubernetes_sd_configs:
      - role: service
        namespaces:
          names:
            - intelligent-optimizer-system
    relabel_configs:
      - source_labels: [__meta_kubernetes_service_label_app_kubernetes_io_component]
        action: keep
        regex: controller
```

## Troubleshooting

### Controller not starting

1. Check RBAC permissions:
   ```bash
   kubectl auth can-i --as=system:serviceaccount:intelligent-optimizer-system:intelligent-optimizer-controller list pods --all-namespaces
   ```

2. Check controller logs:
   ```bash
   kubectl logs -n intelligent-optimizer-system deployment/intelligent-optimizer-controller
   ```

### Metrics not available

1. Ensure Metrics Server is installed:
   ```bash
   kubectl get deployment -n kube-system metrics-server
   ```

2. Test metrics API:
   ```bash
   kubectl top nodes
   kubectl top pods -A
   ```

## Uninstallation

```bash
# Using kubectl
kubectl delete -f deploy/

# Using kustomize
kubectl delete -k deploy/
```

## Next Steps

After deployment:

1. Create an OptimizerConfig resource (see `examples/` directory)
2. Monitor controller logs for recommendations
3. Check Prometheus metrics for optimization statistics
