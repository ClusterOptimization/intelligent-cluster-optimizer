package models

import "time"

// PodMetric represents a single data point for a pod
type PodMetric struct {
	PodName    string
	Namespace  string
	Timestamp  time.Time
	CPUMillis  int64 // CPU usage in millicores (m)
	MemoryMB   int64 // Memory usage in Megabytes (Mi)
}