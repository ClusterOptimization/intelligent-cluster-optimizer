package storage

import (
	"sync"
    "intelligent-cluster-optimizer/pkg/models"
)

type InMemoryStorage struct {
	mu      sync.RWMutex // Protects the map from concurrent writes
	history map[string][]models.PodMetric // Key: PodName
}

func NewStorage() *InMemoryStorage {
	return &InMemoryStorage{
		history: make(map[string][]models.PodMetric),
	}
}

func (s *InMemoryStorage) Add(metric models.PodMetric) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
    // Append new metric to the list for this pod
	s.history[metric.PodName] = append(s.history[metric.PodName], metric)
    
    // Optional: Prune old data if list gets too long (e.g., > 100 items)
}

func (s *InMemoryStorage) GetAll() map[string][]models.PodMetric {
    s.mu.RLock()
    defer s.mu.RUnlock()
    // Return a copy or direct reference (careful with concurrency)
    return s.history
}