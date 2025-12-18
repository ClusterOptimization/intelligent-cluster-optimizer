package safety

import (
	"context"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// OOMDetector identifies pods that have experienced Out of Memory kills
type OOMDetector struct {
	kubeClient kubernetes.Interface

	// Track OOM history per workload
	mu         sync.RWMutex
	oomHistory map[string]*OOMHistory // key: namespace/workloadName
}

// OOMHistory tracks OOM events for a workload
type OOMHistory struct {
	Namespace     string
	WorkloadName  string
	WorkloadKind  string
	ContainerOOMs map[string]*ContainerOOMInfo // key: containerName
	TotalOOMs     int
	LastOOMTime   time.Time
	FirstOOMTime  time.Time
}

// ContainerOOMInfo tracks OOM events for a specific container
type ContainerOOMInfo struct {
	ContainerName    string
	OOMCount         int
	LastOOMTime      time.Time
	LastMemoryLimit  int64 // bytes - memory limit when OOM occurred
	LastMemoryUsage  int64 // bytes - memory usage at OOM (if available)
	RestartCount     int32
	RecommendedBoost float64 // suggested memory multiplier based on OOM frequency
}

// OOMCheckResult contains the result of checking a workload for OOM events
type OOMCheckResult struct {
	HasOOMHistory     bool
	WorkloadName      string
	Namespace         string
	AffectedContainers []ContainerOOMInfo
	TotalOOMCount     int
	LastOOMTime       time.Time
	Priority          OOMPriority
	RecommendedAction string
}

// OOMPriority indicates how urgently a workload needs memory optimization
type OOMPriority int

const (
	OOMPriorityNone     OOMPriority = 0 // No OOM history
	OOMPriorityLow      OOMPriority = 1 // OOM happened once, long ago
	OOMPriorityMedium   OOMPriority = 2 // Multiple OOMs or recent OOM
	OOMPriorityHigh     OOMPriority = 3 // Frequent OOMs
	OOMPriorityCritical OOMPriority = 4 // Very frequent or very recent OOMs
)

// NewOOMDetector creates a new OOM detector
func NewOOMDetector(kubeClient kubernetes.Interface) *OOMDetector {
	return &OOMDetector{
		kubeClient: kubeClient,
		oomHistory: make(map[string]*OOMHistory),
	}
}

// CheckNamespaceForOOMs scans all pods in a namespace for OOM events
func (d *OOMDetector) CheckNamespaceForOOMs(ctx context.Context, namespace string) ([]OOMCheckResult, error) {
	pods, err := d.kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var results []OOMCheckResult
	workloadOOMs := make(map[string]*OOMCheckResult)

	for _, pod := range pods.Items {
		oomContainers := d.checkPodForOOM(&pod)
		if len(oomContainers) == 0 {
			continue
		}

		workloadName := extractWorkloadNameFromPod(&pod)
		key := namespace + "/" + workloadName

		// Aggregate OOMs by workload
		if existing, ok := workloadOOMs[key]; ok {
			existing.AffectedContainers = mergeContainerOOMs(existing.AffectedContainers, oomContainers)
			existing.TotalOOMCount += countOOMs(oomContainers)
			if getLatestOOMTime(oomContainers).After(existing.LastOOMTime) {
				existing.LastOOMTime = getLatestOOMTime(oomContainers)
			}
		} else {
			result := &OOMCheckResult{
				HasOOMHistory:      true,
				WorkloadName:       workloadName,
				Namespace:          namespace,
				AffectedContainers: oomContainers,
				TotalOOMCount:      countOOMs(oomContainers),
				LastOOMTime:        getLatestOOMTime(oomContainers),
			}
			workloadOOMs[key] = result
		}

		// Update internal history
		d.updateOOMHistory(namespace, workloadName, "Deployment", oomContainers)
	}

	// Calculate priority and recommendations for each workload
	for _, result := range workloadOOMs {
		result.Priority = d.calculatePriority(result)
		result.RecommendedAction = d.getRecommendedAction(result)
		results = append(results, *result)
	}

	return results, nil
}

// checkPodForOOM checks a single pod for OOM events
func (d *OOMDetector) checkPodForOOM(pod *corev1.Pod) []ContainerOOMInfo {
	var oomContainers []ContainerOOMInfo

	for _, containerStatus := range pod.Status.ContainerStatuses {
		oomInfo := d.checkContainerForOOM(containerStatus, pod)
		if oomInfo != nil {
			oomContainers = append(oomContainers, *oomInfo)
		}
	}

	return oomContainers
}

// checkContainerForOOM checks a container status for OOM events
func (d *OOMDetector) checkContainerForOOM(status corev1.ContainerStatus, pod *corev1.Pod) *ContainerOOMInfo {
	var isOOM bool
	var lastOOMTime time.Time
	var memoryLimit int64

	// Check last termination state
	if status.LastTerminationState.Terminated != nil {
		terminated := status.LastTerminationState.Terminated
		if terminated.Reason == "OOMKilled" {
			isOOM = true
			lastOOMTime = terminated.FinishedAt.Time
		}
	}

	// Also check current state if it's terminated
	if status.State.Terminated != nil {
		terminated := status.State.Terminated
		if terminated.Reason == "OOMKilled" {
			isOOM = true
			if terminated.FinishedAt.Time.After(lastOOMTime) {
				lastOOMTime = terminated.FinishedAt.Time
			}
		}
	}

	if !isOOM {
		return nil
	}

	// Get memory limit from pod spec
	for _, container := range pod.Spec.Containers {
		if container.Name == status.Name {
			if limit, ok := container.Resources.Limits[corev1.ResourceMemory]; ok {
				memoryLimit = limit.Value()
			}
			break
		}
	}

	// Calculate recommended boost based on restart count
	boost := calculateMemoryBoost(status.RestartCount)

	klog.V(4).Infof("OOM detected: pod=%s/%s container=%s restarts=%d lastOOM=%v",
		pod.Namespace, pod.Name, status.Name, status.RestartCount, lastOOMTime)

	return &ContainerOOMInfo{
		ContainerName:    status.Name,
		OOMCount:         int(status.RestartCount), // Approximation
		LastOOMTime:      lastOOMTime,
		LastMemoryLimit:  memoryLimit,
		RestartCount:     status.RestartCount,
		RecommendedBoost: boost,
	}
}

// calculateMemoryBoost determines how much to increase memory based on OOM frequency
func calculateMemoryBoost(restartCount int32) float64 {
	switch {
	case restartCount >= 10:
		return 2.0 // Double memory for very frequent OOMs
	case restartCount >= 5:
		return 1.75 // 75% increase
	case restartCount >= 3:
		return 1.5 // 50% increase
	case restartCount >= 1:
		return 1.3 // 30% increase
	default:
		return 1.2 // Default 20% buffer
	}
}

// updateOOMHistory updates the internal OOM history for a workload
func (d *OOMDetector) updateOOMHistory(namespace, workloadName, workloadKind string, containers []ContainerOOMInfo) {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := namespace + "/" + workloadName

	history, exists := d.oomHistory[key]
	if !exists {
		history = &OOMHistory{
			Namespace:     namespace,
			WorkloadName:  workloadName,
			WorkloadKind:  workloadKind,
			ContainerOOMs: make(map[string]*ContainerOOMInfo),
			FirstOOMTime:  time.Now(),
		}
		d.oomHistory[key] = history
	}

	for _, c := range containers {
		existing, ok := history.ContainerOOMs[c.ContainerName]
		if !ok {
			containerCopy := c
			history.ContainerOOMs[c.ContainerName] = &containerCopy
		} else {
			// Update existing record
			if c.OOMCount > existing.OOMCount {
				existing.OOMCount = c.OOMCount
			}
			if c.LastOOMTime.After(existing.LastOOMTime) {
				existing.LastOOMTime = c.LastOOMTime
			}
			existing.RestartCount = c.RestartCount
			existing.RecommendedBoost = c.RecommendedBoost
		}
		history.TotalOOMs += c.OOMCount
		if c.LastOOMTime.After(history.LastOOMTime) {
			history.LastOOMTime = c.LastOOMTime
		}
	}
}

// calculatePriority determines how urgent the OOM situation is
func (d *OOMDetector) calculatePriority(result *OOMCheckResult) OOMPriority {
	if !result.HasOOMHistory {
		return OOMPriorityNone
	}

	hoursSinceLastOOM := time.Since(result.LastOOMTime).Hours()
	totalOOMs := result.TotalOOMCount

	// Critical: OOM in last hour or >20 OOMs total
	if hoursSinceLastOOM < 1 || totalOOMs > 20 {
		return OOMPriorityCritical
	}

	// High: OOM in last 24 hours or >10 OOMs total
	if hoursSinceLastOOM < 24 || totalOOMs > 10 {
		return OOMPriorityHigh
	}

	// Medium: OOM in last week or >3 OOMs total
	if hoursSinceLastOOM < 168 || totalOOMs > 3 {
		return OOMPriorityMedium
	}

	// Low: OOM happened but long ago
	return OOMPriorityLow
}

// getRecommendedAction returns a human-readable recommendation based on OOM analysis
func (d *OOMDetector) getRecommendedAction(result *OOMCheckResult) string {
	switch result.Priority {
	case OOMPriorityCritical:
		return "URGENT: Immediate memory increase required. Container experiencing frequent OOM kills."
	case OOMPriorityHigh:
		return "HIGH: Significant memory increase recommended. Recent OOM events detected."
	case OOMPriorityMedium:
		return "MEDIUM: Memory optimization recommended. Multiple OOM events in history."
	case OOMPriorityLow:
		return "LOW: Consider slight memory buffer increase. Past OOM event detected."
	default:
		return "No action required. No OOM history detected."
	}
}

// GetOOMHistory returns the OOM history for a specific workload
func (d *OOMDetector) GetOOMHistory(namespace, workloadName string) *OOMHistory {
	d.mu.RLock()
	defer d.mu.RUnlock()

	key := namespace + "/" + workloadName
	if history, ok := d.oomHistory[key]; ok {
		return history
	}
	return nil
}

// GetMemoryBoostFactor returns the recommended memory multiplier for a container
// based on its OOM history. Returns 1.0 if no OOM history.
func (d *OOMDetector) GetMemoryBoostFactor(namespace, workloadName, containerName string) float64 {
	d.mu.RLock()
	defer d.mu.RUnlock()

	key := namespace + "/" + workloadName
	history, ok := d.oomHistory[key]
	if !ok {
		return 1.0
	}

	containerInfo, ok := history.ContainerOOMs[containerName]
	if !ok {
		return 1.0
	}

	return containerInfo.RecommendedBoost
}

// GetAllOOMWorkloads returns all workloads with OOM history
func (d *OOMDetector) GetAllOOMWorkloads() []OOMHistory {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var result []OOMHistory
	for _, h := range d.oomHistory {
		result = append(result, *h)
	}
	return result
}

// ClearOOMHistory removes OOM history older than the specified duration
func (d *OOMDetector) ClearOOMHistory(olderThan time.Duration) int {
	d.mu.Lock()
	defer d.mu.Unlock()

	cutoff := time.Now().Add(-olderThan)
	removed := 0

	for key, history := range d.oomHistory {
		if history.LastOOMTime.Before(cutoff) {
			delete(d.oomHistory, key)
			removed++
		}
	}

	return removed
}

// Helper functions

func extractWorkloadNameFromPod(pod *corev1.Pod) string {
	// Try to get workload name from owner references
	for _, owner := range pod.OwnerReferences {
		if owner.Kind == "ReplicaSet" {
			// For ReplicaSet, extract deployment name (remove hash suffix)
			return extractDeploymentName(owner.Name)
		}
		if owner.Kind == "StatefulSet" || owner.Kind == "DaemonSet" {
			return owner.Name
		}
	}

	// Fallback: extract from pod name
	return extractWorkloadFromPodName(pod.Name)
}

func extractDeploymentName(replicaSetName string) string {
	// ReplicaSet names are typically: <deployment-name>-<hash>
	// We need to remove the hash suffix
	parts := splitByDash(replicaSetName)
	if len(parts) > 1 {
		return joinParts(parts[:len(parts)-1])
	}
	return replicaSetName
}

func extractWorkloadFromPodName(podName string) string {
	// Pod names are typically: <deployment-name>-<replicaset-hash>-<pod-hash>
	parts := splitByDash(podName)
	if len(parts) > 2 {
		return joinParts(parts[:len(parts)-2])
	}
	return podName
}

func splitByDash(s string) []string {
	var parts []string
	current := ""
	for _, c := range s {
		if c == '-' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}

func joinParts(parts []string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += "-"
		}
		result += p
	}
	return result
}

func mergeContainerOOMs(existing, new []ContainerOOMInfo) []ContainerOOMInfo {
	merged := make(map[string]ContainerOOMInfo)

	for _, c := range existing {
		merged[c.ContainerName] = c
	}

	for _, c := range new {
		if ex, ok := merged[c.ContainerName]; ok {
			// Merge: take the max values
			if c.OOMCount > ex.OOMCount {
				ex.OOMCount = c.OOMCount
			}
			if c.LastOOMTime.After(ex.LastOOMTime) {
				ex.LastOOMTime = c.LastOOMTime
			}
			if c.RestartCount > ex.RestartCount {
				ex.RestartCount = c.RestartCount
			}
			if c.RecommendedBoost > ex.RecommendedBoost {
				ex.RecommendedBoost = c.RecommendedBoost
			}
			merged[c.ContainerName] = ex
		} else {
			merged[c.ContainerName] = c
		}
	}

	var result []ContainerOOMInfo
	for _, c := range merged {
		result = append(result, c)
	}
	return result
}

func countOOMs(containers []ContainerOOMInfo) int {
	total := 0
	for _, c := range containers {
		total += c.OOMCount
	}
	return total
}

func getLatestOOMTime(containers []ContainerOOMInfo) time.Time {
	var latest time.Time
	for _, c := range containers {
		if c.LastOOMTime.After(latest) {
			latest = c.LastOOMTime
		}
	}
	return latest
}

// String returns a string representation of OOMPriority
func (p OOMPriority) String() string {
	switch p {
	case OOMPriorityNone:
		return "None"
	case OOMPriorityLow:
		return "Low"
	case OOMPriorityMedium:
		return "Medium"
	case OOMPriorityHigh:
		return "High"
	case OOMPriorityCritical:
		return "Critical"
	default:
		return "Unknown"
	}
}
