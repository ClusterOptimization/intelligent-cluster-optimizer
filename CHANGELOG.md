# Changelog

All notable changes to the Intelligent Cluster Optimizer will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

### v1.0.0 Highlights

**Intelligent Resource Optimization**: Automatically right-size Kubernetes workloads based on historical usage patterns using P95/P99 percentile analysis.

**Safety-First Design**: Multiple layers of protection including HPA/PDB awareness, circuit breakers, anomaly detection, and OOM prevention ensure safe operations in production.

**Enterprise Features**: Environment profiles, maintenance windows, cost estimation, and comprehensive audit trails make this suitable for enterprise deployments.

**Classical Algorithms**: Uses proven statistical methods (no ML dependencies) including linear regression, control charts, and time series analysis.
