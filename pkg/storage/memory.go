package storage

import (
	"encoding/json"
	"intelligent-cluster-optimizer/pkg/models"
	"os"
	"sync"
	"time"
)

type InMemoryStorage struct {
	mu      sync.RWMutex                  // Protects the map from concurrent writes
	history map[string][]models.PodMetric // Key: PodName
}

func NewStorage() *InMemoryStorage {
	return &InMemoryStorage{
		history: make(map[string][]models.PodMetric),
	}
}

// Add stores a pod metric in memory
func (s *InMemoryStorage) Add(metric models.PodMetric) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := metric.PodName
	s.history[key] = append(s.history[key], metric)
}

// Cleanup removes metrics older than maxAge and returns the count of removed entries
func (s *InMemoryStorage) Cleanup(maxAge time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoffTime := time.Now().Add(-maxAge)
	removedCount := 0

	for podName, metrics := range s.history {
		var validMetrics []models.PodMetric

		for _, metric := range metrics {
			if metric.Timestamp.After(cutoffTime) {
				validMetrics = append(validMetrics, metric)
			} else {
				removedCount++
			}
		}

		if len(validMetrics) == 0 {
			delete(s.history, podName)
		} else {
			s.history[podName] = validMetrics
		}
	}

	return removedCount
}

// SyncPods removes metrics for pods not in the activePodNames list
func (s *InMemoryStorage) SyncPods(activePodNames []string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	activeSet := make(map[string]bool, len(activePodNames))
	for _, name := range activePodNames {
		activeSet[name] = true
	}

	removedCount := 0

	for podName := range s.history {
		if !activeSet[podName] {
			delete(s.history, podName)
			removedCount++
		}
	}

	return removedCount
}

// SaveToFile writes the current history map to a JSON file
func (s *InMemoryStorage) SaveToFile(filename string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.MarshalIndent(s.history, "", " ") // map to json
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644) // write to disk
}

// LoadFromFile reads a JSON file and restores the history map
func (s *InMemoryStorage) LoadFromFile(filename string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // file not exists
		}
		return err
	}
	return json.Unmarshal(data, &s.history) // json to map
}

// startGarbageCollector periodically cleans up old metrics from storage
func (s *InMemoryStorage) StartGarbageCollector(interval time.Duration, maxAge time.Duration) {
	ticker := time.NewTicker(interval)

	go func() {
		defer ticker.Stop()

		for range ticker.C {
			removed := s.Cleanup(maxAge)
			if removed > 0 {
				println("[GC] Cleaned up", removed, "old metric entries")
			}
		}
	}()
}
