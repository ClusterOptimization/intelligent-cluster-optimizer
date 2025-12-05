package models

import "time"

// ContainerMetric represents resource usage for a single container
type ContainerMetric struct {
	ContainerName string `json:"container_name"`
	CPUMillis     int64  `json:"cpu_millis"`
	MemoryMB      int64  `json:"memory_mb"`
}

// PodMetric represents a single data point for a pod
type PodMetric struct {
	PodName    string            `json:"pod_name"`
	Namespace  string            `json:"namespace"`
	Timestamp  time.Time         `json:"timestamp"`
	Containers []ContainerMetric `json:"containers"`
	CPUMillis  int64             `json:"cpu_millis"` // CPU usage in millicores (m)
	MemoryMB   int64             `json:"memory_mb"`  // Memory usage in Megabytes (Mi)
}
