# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 1.x.x   | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

If you discover a security vulnerability in this project, please report it responsibly:

1. **Do NOT** open a public GitHub issue for security vulnerabilities
2. Email the maintainers directly with details of the vulnerability
3. Include steps to reproduce the issue
4. Allow reasonable time for a fix before public disclosure

## Security Measures

This project implements several security measures:

### CI/CD Security Scanning

Our CI pipeline includes automated security scanning on every commit:

- **gosec**: Static analysis for Go security issues
- **govulncheck**: Checks for known vulnerabilities in dependencies
- **golangci-lint**: Includes security-focused linters

### Code Security Practices

#### Input Validation
- All user inputs are validated before processing
- File paths are sanitized using `filepath.Clean()` to prevent path traversal attacks
- Resource quantities are validated before applying to Kubernetes

#### File Permissions
- Configuration files: `0600` (owner read/write only)
- Output directories: `0750` (owner full, group read/execute)
- No world-readable files are created

#### Kubernetes Security
- Uses least-privilege RBAC roles
- Validates workload configurations before applying changes
- Implements circuit breakers to prevent runaway scaling
- PDB (Pod Disruption Budget) checks before any modifications

### Known Security Considerations

#### G115: Integer Overflow
Some integer conversions between `int` and `int32` are marked with `#nosec G115`. These are safe because:
- Values are bounded by Kubernetes API limits
- Percentile calculations are constrained by input ranges

#### G304: File Inclusion
Policy file reading is marked with `#nosec G304`. This is safe because:
- File paths are provided by cluster operators, not end users
- Paths are sanitized with `filepath.Clean()`
- Files are read in a controlled environment

## Secure Deployment

### Kubernetes RBAC

The optimizer requires specific RBAC permissions. Use least-privilege principles:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: optimizer-role
rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["apps"]
    resources: ["deployments", "statefulsets", "daemonsets"]
    verbs: ["get", "list", "watch", "patch"]
  - apiGroups: ["metrics.k8s.io"]
    resources: ["pods"]
    verbs: ["get", "list"]
  - apiGroups: ["policy"]
    resources: ["poddisruptionbudgets"]
    verbs: ["get", "list"]
  - apiGroups: ["autoscaling"]
    resources: ["horizontalpodautoscalers"]
    verbs: ["get", "list"]
```

### Network Policies

Restrict network access for the optimizer:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: optimizer-network-policy
spec:
  podSelector:
    matchLabels:
      app: optimizer
  policyTypes:
    - Ingress
    - Egress
  egress:
    - to:
        - namespaceSelector: {}
      ports:
        - protocol: TCP
          port: 443  # Kubernetes API
        - protocol: TCP
          port: 8443 # Metrics API
```

### Secrets Management

- Never commit secrets to the repository
- Use Kubernetes Secrets for sensitive configuration
- Consider using external secret managers (Vault, AWS Secrets Manager)

## Security Checklist for Contributors

Before submitting code:

- [ ] Run `gosec ./...` and address all findings
- [ ] Run `govulncheck ./...` to check for vulnerable dependencies
- [ ] Validate all external inputs
- [ ] Use `filepath.Clean()` for file path operations
- [ ] Use restrictive file permissions (0600 for files, 0750 for directories)
- [ ] Avoid hardcoded credentials or secrets
- [ ] Document any `#nosec` annotations with justification

## Dependencies

We regularly update dependencies to patch security vulnerabilities:

```bash
# Check for vulnerable dependencies
govulncheck ./...

# Update dependencies
go get -u ./...
go mod tidy
```

## Audit Log

The optimizer records Kubernetes events for all changes:

- Resource modifications are logged with before/after states
- Failed operations are recorded with error details
- Events can be queried with `kubectl get events`

This provides an audit trail for security review and incident response.
