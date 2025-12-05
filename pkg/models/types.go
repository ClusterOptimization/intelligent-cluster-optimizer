package models

import "time"

// PodMetric represents a single data point for a pod
type PodMetric struct {
	PodName    string		`json:"pod_name"`
	Namespace  string		`json:"namespace"`
	Timestamp  time.Time	`json:"timestamp"`
	CPUMillis  int64 		`json:"cpu_millis"`		// CPU usage in millicores (m)
	MemoryMB   int64		`json:"memory_mb"`		// Memory usage in Megabytes (Mi)
}