package models

import "time"

// ContainerMetric represents resource usage for a single container
type ContainerMetric struct {
	ContainerName string `json:"container_name"`
	//CPUMillis     int64  `json:"cpu_millis"`
	//MemoryMB      int64  `json:"memory_mb"`

	// usage (Current actual load)
	UsageCPU    int64 `json:"usage_cpu"`
	UsageMemory int64 `json:"usage_memory"`

	// Spec (What is defined in the YAML)
	RequestCPU    int64 `json:"request_cpu"`
	RequestMemory int64 `json:"request_memory"`
	LimitCPU      int64 `json:"limit_cpu"`
	LimitMemory   int64 `json:"limit_memory"`
}

// PodMetric represents a single data point for a pod
type PodMetric struct {
	PodName    string            `json:"pod_name"`
	Namespace  string            `json:"namespace"`
	Timestamp  time.Time         `json:"timestamp"`
	Containers []ContainerMetric `json:"containers"`
	//CPUMillis  int64             `json:"cpu_millis"` // CPU usage in millicores (m)
	//MemoryMB   int64             `json:"memory_mb"`  // Memory usage in Megabytes (Mi)
}
