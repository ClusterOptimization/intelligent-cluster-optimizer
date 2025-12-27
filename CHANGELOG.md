# Changelog

All notable changes to the Intelligent Cluster Optimizer will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.2.0] - 2025-12-28

### Added

#### CLI Enhancements (optctl)
- Cost calculator command (`optctl cost`)
  - Calculate resource costs for namespaces and workloads
  - Support for multiple cloud pricing models (AWS, GCP, Azure)
  - Spot instance pricing support
  - Cross-namespace cost aggregation (`--all-namespaces`)
  - Detailed cost breakdown (hourly, daily, monthly, yearly)
  - Workloads sorted by cost (highest first)
- History tracking command (`optctl history`)
  - View all optimization history
  - Filter by specific workload
  - Shows CPU, memory, timestamp, and age
  - Sorted by most recent first
- Enhanced rollback command
  - Persistent history file support
  - Automatic history loading/saving

#### Rollback Manager Improvements
- `GetAllHistory()` - Retrieve all stored history entries
- `GetWorkloadHistory()` - Get history for specific workload
- `GetHistoryCount()` - Count total history entries
- Thread-safe history access with copy-on-read

## [1.1.0] - 2025-12-27

### Added

#### Policy & Governance
- Policy engine with expression-based evaluation (expr-lang/expr)
  - YAML-based policy configuration
  - Priority-based policy ordering
  - Actions: allow, deny, skip, skip-scaledown, skip-scaleup, modify, require-approval
  - Resource limit enforcement (min/max CPU/memory)
- SLA monitoring and health checking
  - Latency tracking (P95, P99 percentile-based)
  - Error rate monitoring
  - Availability and throughput SLAs
  - Control chart-based health assessment
  - Blocks scaling during SLA violations

#### GitOps Integration
- Kustomize export support
  - Strategic merge patches
  - JSON 6902 patches
  - Auto-generated kustomization.yaml
- Helm values.yaml generation
- Git repository integration for PR-based workflows

#### Observability
- Prometheus metrics exporter
  - Recommendation metrics (count, confidence, savings)
  - Workload metrics (CPU/memory usage, requests, limits)
  - Safety metrics (circuit breaker state, PDB violations)
  - Operation metrics (apply success/failure, rollback count)
- Structured logging with zap logger
  - JSON and console output formats
  - Configurable log levels
  - Request tracing support

#### Deployment & Operations
- Kubernetes deployment manifests with kustomize
  - Base configuration
  - Production overlay with resource limits
  - Development overlay for testing
- Webhook validator for OptimizerConfig resources
  - Validates configuration before admission
  - Prevents invalid CRD configurations
- Example OptimizerConfig YAMLs
  - Basic configuration
  - Production-ready setup
  - Development/testing configuration
  - GitOps-enabled configuration

#### CI/CD Pipeline
- Enhanced GitHub Actions workflow
  - Lint stage (gofmt, golangci-lint)
  - Test stage with coverage reporting (Codecov)
  - Security scanning (gosec, govulncheck)
  - Multi-platform builds (Linux/macOS, amd64/arm64)
  - Automated GitHub Releases on version tags

#### Documentation
- Comprehensive README with architecture diagrams
- SECURITY.md with vulnerability reporting policy
- ARCHITECTURE.md with system design details
- USAGE.md with operational guides
- Contributing guidelines with conventional commits

### Fixed

#### Security
- Path traversal vulnerabilities (added filepath.Clean sanitization)
- File permission issues (0644 -> 0600 for sensitive files)
- Directory permissions (0755 -> 0750 for output directories)
- Integer overflow annotations for gosec compliance

#### Code Quality
- Resolved all golangci-lint issues
  - Fixed unchecked error returns (errcheck)
  - Removed unused functions (unused)
  - Simplified nil checks (gosimple)
  - Fixed ineffective assignments (ineffassign)
  - Replaced deprecated wait.PollImmediate (staticcheck)
- Code formatting compliance (gofmt)

## [1.0.0] - 2025-12-21

### Added

#### Core Features
- Recommendation engine with P95/P99 percentile calculations
- Cost estimation for AWS, GCP, and Azure pricing models
- Confidence scoring system with 5-weighted-factor evaluation
- Recommendation expiry (TTL-based staleness prevention)

#### Safety & Control
- HPA (Horizontal Pod Autoscaler) conflict detection
- PDB (Pod Disruption Budget) awareness
- Circuit breaker pattern for failure protection
- OOM detection with adaptive memory boost (1.2x-2.0x)
- MaxChangePercent enforcement to limit resource changes
- Emergency rollback system

#### Analysis & Intelligence
- Memory leak detection using linear regression with R² scoring
- Time pattern analysis (business hours, night batch, weekday patterns)
- Statistical anomaly detection with multi-method consensus:
  - Z-Score detector
  - IQR (Interquartile Range) detector
  - Moving Average deviation detector
- Time series prediction using Holt-Winters (Triple Exponential Smoothing):
  - Level, trend, and seasonality component modeling
  - Seasonal decomposition for pattern extraction
  - Peak load prediction with proactive warnings
  - Scaling recommendations based on predicted usage
- Pareto multi-objective optimization:
  - Simultaneous optimization of cost, performance, reliability, efficiency, and stability
  - Pareto frontier calculation for trade-off analysis
  - Profile-specific strategy selection (production, development, performance)
  - Crowding distance for diversity preservation

#### Configuration
- Environment-based optimization profiles (production, staging, development, test)
- Profile overrides for per-resource customization
- Maintenance window scheduling with cron support
- Workload exclusion patterns

#### Kubernetes Integration
- Custom Resource Definition (OptimizerConfig CRD)
- Kubernetes controller with reconciliation loop
- Event recording for audit trails
- Vertical scaling for Deployments, StatefulSets, DaemonSets
- Dry-run mode for safe testing

#### Infrastructure
- In-memory metrics storage with garbage collection
- Dead pod pruning to prevent memory leaks
- Container-level metrics granularity
- Persistent storage support (JSON export)

### Changed
- Moved safety checks before optimizations in reconciler
- Added thread safety to circuit breaker

### Fixed
- Controller cache mutation bugs
- Safety check ordering issues

## [0.1.0] - 2025-12-01

### Added
- Initial project structure
- Basic metrics collection from Kubernetes Metrics API
- GitHub Actions CI/CD pipeline
- Core package organization

---

## Release Notes

### v1.1.0 Highlights

**Policy-Driven Optimization**: New policy engine enables fine-grained control over which recommendations are applied. Define rules using expression-based conditions to allow, deny, modify, or require approval for changes.

**SLA-Aware Scaling**: The optimizer now monitors SLA metrics (latency, error rate, availability) and blocks scaling operations during violations to protect service health.

**GitOps Ready**: Export recommendations as Kustomize patches or Helm values for seamless integration with ArgoCD, Flux, or other GitOps workflows.

**Production Observability**: Prometheus metrics exporter and structured logging provide full visibility into optimizer operations, recommendations, and safety events.

**Enterprise CI/CD**: Comprehensive pipeline with linting, testing, security scanning, and automated multi-platform releases ensures code quality and security.

### v1.0.0 Highlights

**Intelligent Resource Optimization**: Automatically right-size Kubernetes workloads based on historical usage patterns using P95/P99 percentile analysis.

**Safety-First Design**: Multiple layers of protection including HPA/PDB awareness, circuit breakers, anomaly detection, and OOM prevention ensure safe operations in production.

**Enterprise Features**: Environment profiles, maintenance windows, cost estimation, and comprehensive audit trails make this suitable for enterprise deployments.

**Classical Algorithms**: Uses proven statistical methods (no ML dependencies) including linear regression, control charts, time series analysis, Holt-Winters exponential smoothing for predictive scaling, and Pareto optimization for multi-objective decision making.
