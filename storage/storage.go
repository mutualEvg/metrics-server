// storage/storage.go
package storage

import "sync"

// Storage defines the interface for metrics storage operations.
// It supports both gauge (floating-point) and counter (integer) metrics.
type Storage interface {
	// UpdateGauge sets the value of a gauge metric
	UpdateGauge(name string, value float64)

	// UpdateCounter adds the delta value to a counter metric
	UpdateCounter(name string, value int64)

	// GetGauge retrieves a gauge metric value. Returns value and true if found, false otherwise.
	GetGauge(name string) (float64, bool)

	// GetCounter retrieves a counter metric value. Returns value and true if found, false otherwise.
	GetCounter(name string) (int64, bool)

	// GetAll returns all gauge and counter metrics as separate maps
	GetAll() (map[string]float64, map[string]int64)
}

// MemStorage is an in-memory implementation of the Storage interface.
// It stores metrics in memory with optional file persistence support.
// All operations are thread-safe using read-write mutexes.
type MemStorage struct {
	gauges      map[string]float64
	counters    map[string]int64
	mu          sync.RWMutex
	fileManager *FileManager
	syncSave    bool
}

// NewMemStorage creates a new in-memory storage instance.
// Maps are pre-allocated with capacity of 50 for better performance.
func NewMemStorage() *MemStorage {
	return &MemStorage{
		gauges:   make(map[string]float64, 50), // Pre-allocate capacity for better performance
		counters: make(map[string]int64, 50),   // Pre-allocate capacity for better performance
	}
}

// SetFileManager sets the file manager for this storage
func (ms *MemStorage) SetFileManager(fm *FileManager, syncSave bool) {
	ms.fileManager = fm
	ms.syncSave = syncSave
}

func (ms *MemStorage) UpdateGauge(name string, value float64) {
	ms.mu.Lock()
	ms.gauges[name] = value

	// Save synchronously if configured
	if ms.syncSave && ms.fileManager != nil {
		// Use internal method to avoid deadlock
		ms.saveToFileInternal()
	}
	ms.mu.Unlock()
}

func (ms *MemStorage) UpdateCounter(name string, value int64) {
	ms.mu.Lock()
	ms.counters[name] += value

	// Save synchronously if configured
	if ms.syncSave && ms.fileManager != nil {
		// Use internal method to avoid deadlock
		ms.saveToFileInternal()
	}
	ms.mu.Unlock()
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

	// Pre-allocate maps with known capacity to avoid map growth
	gCopy := make(map[string]float64, len(ms.gauges))
	cCopy := make(map[string]int64, len(ms.counters))

	for k, v := range ms.gauges {
		gCopy[k] = v
	}
	for k, v := range ms.counters {
		cCopy[k] = v
	}
	return gCopy, cCopy
}

// getAllInternal returns copies of all metrics without acquiring locks
// This method assumes the caller already holds the appropriate locks
func (ms *MemStorage) getAllInternal() (map[string]float64, map[string]int64) {
	// Pre-allocate maps with known capacity to avoid map growth
	gCopy := make(map[string]float64, len(ms.gauges))
	cCopy := make(map[string]int64, len(ms.counters))

	for k, v := range ms.gauges {
		gCopy[k] = v
	}
	for k, v := range ms.counters {
		cCopy[k] = v
	}
	return gCopy, cCopy
}

// saveToFileInternal saves to file without acquiring locks
// This method assumes the caller already holds the appropriate locks
func (ms *MemStorage) saveToFileInternal() {
	if ms.fileManager != nil {
		gauges, counters := ms.getAllInternal()
		ms.fileManager.SaveToFileWithData(gauges, counters)
	}
}

// tempStorageForSaving is a temporary implementation of Storage interface for saving
type tempStorageForSaving struct {
	gauges   map[string]float64
	counters map[string]int64
}

func (t *tempStorageForSaving) UpdateGauge(name string, value float64) {
	// Not used for saving
}

func (t *tempStorageForSaving) UpdateCounter(name string, value int64) {
	// Not used for saving
}

func (t *tempStorageForSaving) GetGauge(name string) (float64, bool) {
	val, ok := t.gauges[name]
	return val, ok
}

func (t *tempStorageForSaving) GetCounter(name string) (int64, bool) {
	val, ok := t.counters[name]
	return val, ok
}

func (t *tempStorageForSaving) GetAll() (map[string]float64, map[string]int64) {
	return t.gauges, t.counters
}
