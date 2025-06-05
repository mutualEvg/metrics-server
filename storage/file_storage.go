package storage

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// FileStorage represents the data structure for JSON serialization
type FileStorage struct {
	Gauges   map[string]float64 `json:"gauges"`
	Counters map[string]int64   `json:"counters"`
}

// FileManager handles file operations for metrics storage
type FileManager struct {
	filePath string
	mu       sync.RWMutex
}

// NewFileManager creates a new file manager
func NewFileManager(filePath string) *FileManager {
	return &FileManager{
		filePath: filePath,
	}
}

// SaveToFile saves the current metrics to file
func (fm *FileManager) SaveToFile(storage Storage) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	gauges, counters := storage.GetAll()

	data := FileStorage{
		Gauges:   gauges,
		Counters: counters,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	// Write to temporary file first, then rename for atomic operation
	tempFile := fm.filePath + ".tmp"
	err = os.WriteFile(tempFile, jsonData, 0644)
	if err != nil {
		return err
	}

	return os.Rename(tempFile, fm.filePath)
}

// LoadFromFile loads metrics from file into storage
func (fm *FileManager) LoadFromFile(storage Storage) error {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	data, err := os.ReadFile(fm.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, which is fine for first run
			return nil
		}
		return err
	}

	var fileData FileStorage
	err = json.Unmarshal(data, &fileData)
	if err != nil {
		return err
	}

	// Load gauges
	for name, value := range fileData.Gauges {
		storage.UpdateGauge(name, value)
	}

	// Load counters
	for name, value := range fileData.Counters {
		// For counters, we set the value directly rather than adding
		// since we're restoring the exact state
		if memStorage, ok := storage.(*MemStorage); ok {
			memStorage.mu.Lock()
			memStorage.counters[name] = value
			memStorage.mu.Unlock()
		}
	}

	return nil
}

// FileExists checks if the storage file exists
func (fm *FileManager) FileExists() bool {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	_, err := os.Stat(fm.filePath)
	return !os.IsNotExist(err)
}

// PeriodicSaver handles periodic saving of metrics
type PeriodicSaver struct {
	fileManager *FileManager
	storage     Storage
	interval    time.Duration
	stopChan    chan struct{}
	stoppedChan chan struct{}
	mu          sync.Mutex
	running     bool
}

// NewPeriodicSaver creates a new periodic saver
func NewPeriodicSaver(fileManager *FileManager, storage Storage, interval time.Duration) *PeriodicSaver {
	return &PeriodicSaver{
		fileManager: fileManager,
		storage:     storage,
		interval:    interval,
		stopChan:    make(chan struct{}),
		stoppedChan: make(chan struct{}),
	}
}

// Start begins periodic saving
func (ps *PeriodicSaver) Start() {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.running {
		return
	}
	ps.running = true

	go func() {
		defer close(ps.stoppedChan)

		if ps.interval <= 0 {
			// Synchronous mode - save immediately on every update
			return
		}

		ticker := time.NewTicker(ps.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := ps.fileManager.SaveToFile(ps.storage); err != nil {
					// Log error but continue running
					// In a real application, you might want to use a proper logger
					continue
				}
			case <-ps.stopChan:
				return
			}
		}
	}()
}

// Stop stops periodic saving
func (ps *PeriodicSaver) Stop() {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if !ps.running {
		return
	}
	ps.running = false

	close(ps.stopChan)
	<-ps.stoppedChan
}

// SaveNow saves immediately (for synchronous mode or shutdown)
func (ps *PeriodicSaver) SaveNow() error {
	return ps.fileManager.SaveToFile(ps.storage)
}
