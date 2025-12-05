package storage

import (
	"encoding/json"
	"intelligent-cluster-optimizer/pkg/models"
	"os"
	"sync"
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
