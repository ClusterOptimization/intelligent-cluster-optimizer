package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// PrometheusExporter exposes optimizer metrics to Prometheus
type PrometheusExporter struct {
	// Reconciliation metrics
	ReconciliationTotal    *prometheus.CounterVec
	ReconciliationDuration *prometheus.HistogramVec
	ReconciliationErrors   *prometheus.CounterVec

	// Resource recommendation metrics
	CPURecommendation     *prometheus.GaugeVec
	MemoryRecommendation  *prometheus.GaugeVec
	ReplicaRecommendation *prometheus.GaugeVec

	// SLA metrics
	SLAViolations       *prometheus.CounterVec
	HealthScore         *prometheus.GaugeVec
	OptimizationBlocked *prometheus.CounterVec
	RollbacksTriggered  *prometheus.CounterVec

	// Policy metrics
	PolicyEvaluations    *prometheus.CounterVec
	PolicyBlockedChanges *prometheus.CounterVec

	// GitOps metrics
	GitOpsExports      *prometheus.CounterVec
	GitOpsExportErrors *prometheus.CounterVec

	// Prediction metrics
	PredictionsMade    *prometheus.CounterVec
	PeakLoadsPredicted *prometheus.CounterVec

	// Pareto optimization metrics
	ParetoFrontSize *prometheus.GaugeVec
	ParetoSolutions *prometheus.CounterVec

	// Pod startup time metrics
	PodStartupDuration    *prometheus.HistogramVec
	PodStartupDurationAvg *prometheus.GaugeVec
	PodStartupDurationP95 *prometheus.GaugeVec
	SlowStartupsTotal     *prometheus.CounterVec
}

// NewPrometheusExporter creates a new Prometheus metrics exporter
func NewPrometheusExporter(namespace string) *PrometheusExporter {
	return &PrometheusExporter{
		// Reconciliation metrics
		ReconciliationTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "reconciliation_total",
				Help:      "Total number of reconciliation attempts by result (success/failure)",
			},
			[]string{"config", "namespace", "result"},
		),
		ReconciliationDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "reconciliation_duration_seconds",
				Help:      "Duration of reconciliation in seconds",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"config", "namespace"},
		),
		ReconciliationErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "reconciliation_errors_total",
				Help:      "Total number of reconciliation errors by type",
			},
			[]string{"config", "namespace", "error_type"},
		),

		// Resource recommendation metrics
		CPURecommendation: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "cpu_recommendation_millicores",
				Help:      "Recommended CPU in millicores for workloads",
			},
			[]string{"workload", "namespace", "container"},
		),
		MemoryRecommendation: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "memory_recommendation_mib",
				Help:      "Recommended memory in MiB for workloads",
			},
			[]string{"workload", "namespace", "container"},
		),
		ReplicaRecommendation: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "replica_recommendation",
				Help:      "Recommended replica count for workloads",
			},
			[]string{"workload", "namespace"},
		),

		// SLA metrics
		SLAViolations: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "sla_violations_total",
				Help:      "Total number of SLA violations detected by type",
			},
			[]string{"config", "namespace", "sla_type"},
		),
		HealthScore: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "health_score",
				Help:      "System health score (0-100)",
			},
			[]string{"config", "namespace"},
		),
		OptimizationBlocked: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "optimization_blocked_total",
				Help:      "Total number of optimizations blocked due to health checks",
			},
			[]string{"config", "namespace", "reason"},
		),
		RollbacksTriggered: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "rollbacks_triggered_total",
				Help:      "Total number of rollbacks triggered due to health degradation",
			},
			[]string{"config", "namespace", "reason"},
		),

		// Policy metrics
		PolicyEvaluations: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "policy_evaluations_total",
				Help:      "Total number of policy evaluations by result",
			},
			[]string{"config", "namespace", "policy_name", "result"},
		),
		PolicyBlockedChanges: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "policy_blocked_changes_total",
				Help:      "Total number of changes blocked by policies",
			},
			[]string{"config", "namespace", "policy_name"},
		),

		// GitOps metrics
		GitOpsExports: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "gitops_exports_total",
				Help:      "Total number of GitOps exports by format",
			},
			[]string{"config", "namespace", "format"},
		),
		GitOpsExportErrors: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "gitops_export_errors_total",
				Help:      "Total number of GitOps export errors",
			},
			[]string{"config", "namespace", "format"},
		),

		// Prediction metrics
		PredictionsMade: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "predictions_made_total",
				Help:      "Total number of time series predictions made",
			},
			[]string{"config", "namespace", "metric_type"},
		),
		PeakLoadsPredicted: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "peak_loads_predicted_total",
				Help:      "Total number of peak loads predicted",
			},
			[]string{"config", "namespace"},
		),

		// Pareto optimization metrics
		ParetoFrontSize: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "pareto_front_size",
				Help:      "Number of solutions in the Pareto front",
			},
			[]string{"config", "namespace"},
		),
		ParetoSolutions: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "pareto_solutions_total",
				Help:      "Total number of Pareto optimal solutions generated",
			},
			[]string{"config", "namespace"},
		),

		// Pod startup time metrics
		PodStartupDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "pod_startup_duration_seconds",
				Help:      "Time from pod creation to ready state in seconds",
				Buckets:   []float64{1, 2, 5, 10, 15, 20, 30, 45, 60, 90, 120},
			},
			[]string{"workload", "namespace", "container"},
		),
		PodStartupDurationAvg: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "pod_startup_duration_avg_seconds",
				Help:      "Average pod startup time in seconds",
			},
			[]string{"workload", "namespace"},
		),
		PodStartupDurationP95: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "pod_startup_duration_p95_seconds",
				Help:      "P95 pod startup time in seconds",
			},
			[]string{"workload", "namespace"},
		),
		SlowStartupsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "slow_startups_total",
				Help:      "Total number of pods that exceeded startup time threshold",
			},
			[]string{"workload", "namespace", "threshold"},
		),
	}
}

// RecordReconciliation records a reconciliation attempt
func (e *PrometheusExporter) RecordReconciliation(config, namespace, result string, duration float64) {
	e.ReconciliationTotal.WithLabelValues(config, namespace, result).Inc()
	e.ReconciliationDuration.WithLabelValues(config, namespace).Observe(duration)
}

// RecordReconciliationError records a reconciliation error
func (e *PrometheusExporter) RecordReconciliationError(config, namespace, errorType string) {
	e.ReconciliationErrors.WithLabelValues(config, namespace, errorType).Inc()
}

// RecordCPURecommendation records a CPU recommendation
func (e *PrometheusExporter) RecordCPURecommendation(workload, namespace, container string, millicores float64) {
	e.CPURecommendation.WithLabelValues(workload, namespace, container).Set(millicores)
}

// RecordMemoryRecommendation records a memory recommendation
func (e *PrometheusExporter) RecordMemoryRecommendation(workload, namespace, container string, mib float64) {
	e.MemoryRecommendation.WithLabelValues(workload, namespace, container).Set(mib)
}

// RecordReplicaRecommendation records a replica recommendation
func (e *PrometheusExporter) RecordReplicaRecommendation(workload, namespace string, replicas float64) {
	e.ReplicaRecommendation.WithLabelValues(workload, namespace).Set(replicas)
}

// RecordSLAViolation records an SLA violation
func (e *PrometheusExporter) RecordSLAViolation(config, namespace, slaType string) {
	e.SLAViolations.WithLabelValues(config, namespace, slaType).Inc()
}

// RecordHealthScore records a health score
func (e *PrometheusExporter) RecordHealthScore(config, namespace string, score float64) {
	e.HealthScore.WithLabelValues(config, namespace).Set(score)
}

// RecordOptimizationBlocked records an optimization being blocked
func (e *PrometheusExporter) RecordOptimizationBlocked(config, namespace, reason string) {
	e.OptimizationBlocked.WithLabelValues(config, namespace, reason).Inc()
}

// RecordRollback records a rollback being triggered
func (e *PrometheusExporter) RecordRollback(config, namespace, reason string) {
	e.RollbacksTriggered.WithLabelValues(config, namespace, reason).Inc()
}

// RecordPolicyEvaluation records a policy evaluation
func (e *PrometheusExporter) RecordPolicyEvaluation(config, namespace, policyName, result string) {
	e.PolicyEvaluations.WithLabelValues(config, namespace, policyName, result).Inc()
}

// RecordPolicyBlockedChange records a policy blocking a change
func (e *PrometheusExporter) RecordPolicyBlockedChange(config, namespace, policyName string) {
	e.PolicyBlockedChanges.WithLabelValues(config, namespace, policyName).Inc()
}

// RecordGitOpsExport records a GitOps export
func (e *PrometheusExporter) RecordGitOpsExport(config, namespace, format string) {
	e.GitOpsExports.WithLabelValues(config, namespace, format).Inc()
}

// RecordGitOpsExportError records a GitOps export error
func (e *PrometheusExporter) RecordGitOpsExportError(config, namespace, format string) {
	e.GitOpsExportErrors.WithLabelValues(config, namespace, format).Inc()
}

// RecordPrediction records a time series prediction
func (e *PrometheusExporter) RecordPrediction(config, namespace, metricType string) {
	e.PredictionsMade.WithLabelValues(config, namespace, metricType).Inc()
}

// RecordPeakLoadPredicted records a peak load prediction
func (e *PrometheusExporter) RecordPeakLoadPredicted(config, namespace string) {
	e.PeakLoadsPredicted.WithLabelValues(config, namespace).Inc()
}

// RecordParetoFrontSize records the size of a Pareto front
func (e *PrometheusExporter) RecordParetoFrontSize(config, namespace string, size int) {
	e.ParetoFrontSize.WithLabelValues(config, namespace).Set(float64(size))
}

// RecordParetoSolution records a Pareto optimal solution
func (e *PrometheusExporter) RecordParetoSolution(config, namespace string) {
	e.ParetoSolutions.WithLabelValues(config, namespace).Inc()
}

// RecordPodStartupDuration records a single pod startup time
func (e *PrometheusExporter) RecordPodStartupDuration(workload, namespace, container string, seconds float64) {
	e.PodStartupDuration.WithLabelValues(workload, namespace, container).Observe(seconds)
}

// RecordPodStartupDurationAvg records the average pod startup time for a workload
func (e *PrometheusExporter) RecordPodStartupDurationAvg(workload, namespace string, seconds float64) {
	e.PodStartupDurationAvg.WithLabelValues(workload, namespace).Set(seconds)
}

// RecordPodStartupDurationP95 records the P95 pod startup time for a workload
func (e *PrometheusExporter) RecordPodStartupDurationP95(workload, namespace string, seconds float64) {
	e.PodStartupDurationP95.WithLabelValues(workload, namespace).Set(seconds)
}

// RecordSlowStartup records a pod that exceeded the startup time threshold
func (e *PrometheusExporter) RecordSlowStartup(workload, namespace string, thresholdSeconds float64) {
	threshold := fmt.Sprintf("%.0fs", thresholdSeconds)
	e.SlowStartupsTotal.WithLabelValues(workload, namespace, threshold).Inc()
}
