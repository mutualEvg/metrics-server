// storage/storage.go
package storage

import "sync"

type Storage interface {
	UpdateGauge(name string, value float64)
	UpdateCounter(name string, value int64)
	GetGauge(name string) (float64, bool)
	GetCounter(name string) (int64, bool)
	GetAll() (map[string]float64, map[string]int64)
}

type MemStorage struct {
	gauges   map[string]float64
	counters map[string]int64
	mu       sync.RWMutex
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauges:   make(map[string]float64),
		counters: make(map[string]int64),
	}
}

func (ms *MemStorage) UpdateGauge(name string, value float64) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.gauges[name] = value
}

func (ms *MemStorage) UpdateCounter(name string, value int64) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.counters[name] += value
}

func (ms *MemStorage) GetGauge(name string) (float64, bool) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	val, ok := ms.gauges[name]
	return val, ok
}

func (ms *MemStorage) GetCounter(name string) (int64, bool) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	val, ok := ms.counters[name]
	return val, ok
}

func (ms *MemStorage) GetAll() (map[string]float64, map[string]int64) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	gCopy := make(map[string]float64)
	cCopy := make(map[string]int64)
	for k, v := range ms.gauges {
		gCopy[k] = v
	}
	for k, v := range ms.counters {
		cCopy[k] = v
	}
	return gCopy, cCopy
}
