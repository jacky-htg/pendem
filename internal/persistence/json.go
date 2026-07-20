package persistence

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"pendem/internal/config"
	"pendem/internal/engine" // Import cache package kita
	"sort"
	"sync"
	"time"
)

// JSONManager mengelola snapshot JSON
type JSONManager[V any] struct {
	config    config.JSONConfig
	cache     *engine.Cache[string] // Cache engine kita
	mu        sync.RWMutex          // Untuk mengamankan operasi file
	logger    *log.Logger
	stopChan  chan struct{}
	wg        sync.WaitGroup
	isRunning bool
}

// JSONData adalah struktur data yang disimpan ke JSON
type JSONData[V any] struct {
	Version    string              `json:"version"`     // Versi format
	Timestamp  int64               `json:"timestamp"`   // Waktu snapshot
	NumShards  int                 `json:"num_shards"`  // Jumlah shard
	TotalItems int                 `json:"total_items"` // Total item
	Shards     []JSONShard[string] `json:"shards"`      // Data per shard
}

// JSONShard merepresentasikan satu shard dalam snapshot
type JSONShard[V any] struct {
	ID    int                       `json:"id"`
	Items map[string]engine.Item[V] `json:"items"`
}

// NewJSONManager membuat json snapshot manager baru
func NewJSONManager[V any](logger *log.Logger, cache *engine.Cache[string], cfg config.JSONConfig) *JSONManager[V] {
	return &JSONManager[V]{
		config:   cfg,
		cache:    cache,
		logger:   logger,
		stopChan: make(chan struct{}),
	}
}

// Save membuat snapshot ke file JSON
func (sm *JSONManager[V]) Save() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.logger.Println("Starting JSON snapshot save...")
	startTime := time.Now()

	// 1. Kumpulkan data dari semua shard
	snapshotData, err := sm.collectData()
	if err != nil {
		return fmt.Errorf("failed to collect data: %w", err)
	}

	// 2. Marshal ke JSON dengan indentasi
	jsonData, err := json.MarshalIndent(snapshotData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// 3. Tulis ke file temporary dulu (atomic write)
	tempFile := sm.config.Path + ".tmp"
	if err := os.WriteFile(tempFile, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// 4. Rename file (atomic operation)
	if err := os.Rename(tempFile, sm.config.Path); err != nil {
		return fmt.Errorf("failed to rename file: %w", err)
	}

	// 5. Rotasi file snapshot (keep N versions)
	if sm.config.MaxSnapshots > 0 {
		sm.rotateSnapshots()
	}

	elapsed := time.Since(startTime)
	sm.logger.Printf("Snapshot saved successfully in %v (%d items)",
		elapsed, snapshotData.TotalItems)

	return nil
}

// collectData mengumpulkan data dari semua shard
func (sm *JSONManager[V]) collectData() (*JSONData[V], error) {
	// Dapatkan stats untuk mengetahui jumlah shard
	numShards := sm.cache.NumShards()

	data := &JSONData[V]{
		Version:   "1.0",
		Timestamp: time.Now().Unix(),
		NumShards: numShards,
		Shards:    make([]JSONShard[string], 0, numShards),
	}

	// Iterasi setiap shard
	for shardID := 0; shardID < numShards; shardID++ {
		// Ambil shard
		shard := sm.cache.GetShard(shardID)
		if shard == nil {
			continue
		}

		items := shard.GetItems()

		shardData := JSONShard[string]{
			ID:    shardID,
			Items: items,
		}
		data.Shards = append(data.Shards, shardData)
		data.TotalItems += len(items)
	}

	return data, nil
}

// ============================================
// 4. LOAD SNAPSHOT
// ============================================

// Load memuat snapshot dari file JSON
func (sm *JSONManager[V]) Load() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Cek apakah file snapshot ada
	if _, err := os.Stat(sm.config.Path); os.IsNotExist(err) {
		sm.logger.Println("No snapshot file found, starting with empty cache")
		return nil
	}

	sm.logger.Printf("Loading snapshot from %s...", sm.config.Path)
	startTime := time.Now()

	// 1. Baca file
	jsonData, err := os.ReadFile(sm.config.Path)
	if err != nil {
		return fmt.Errorf("failed to read snapshot file: %w", err)
	}

	// 2. Parse JSON
	var snapshotData JSONData[V]
	if err := json.Unmarshal(jsonData, &snapshotData); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	// 3. Restore data ke cache
	if err := sm.restoreData(&snapshotData); err != nil {
		return fmt.Errorf("failed to restore data: %w", err)
	}

	elapsed := time.Since(startTime)
	sm.logger.Printf("Snapshot loaded successfully in %v (%d items)",
		elapsed, snapshotData.TotalItems)

	return nil
}

// restoreData mengembalikan data ke cache
func (sm *JSONManager[V]) restoreData(data *JSONData[V]) error {
	// Hanya restore jika versi kompatibel
	if data.Version != "1.0" {
		return fmt.Errorf("unsupported snapshot version: %s", data.Version)
	}

	// Iterasi setiap shard
	for _, shardData := range data.Shards {
		// Ambil shard berdasarkan ID
		shard := sm.cache.GetShard(shardData.ID)
		if shard == nil {
			sm.logger.Printf("Warning: Shard %d not found, skipping...", shardData.ID)
			continue
		}

		shard.Restore(shardData.Items)
		sm.logger.Printf("Restored shard %d: %d items", shardData.ID, len(shardData.Items))
	}

	return nil
}

// ============================================
// 5. SNAPSHOT ROTATION
// ============================================

// rotateSnapshots menjaga jumlah file snapshot tidak lebih dari MaxSnapshots
func (sm *JSONManager[V]) rotateSnapshots() {
	// Cari semua file snapshot yang ada
	pattern := sm.config.Path + ".*"
	files, err := filepath.Glob(pattern)
	if err != nil {
		sm.logger.Printf("Error finding snapshot files: %v", err)
		return
	}

	// Tambahkan file utama
	files = append(files, sm.config.Path)

	// Sort berdasarkan waktu modifikasi (terlama dulu)
	sort.Slice(files, func(i, j int) bool {
		fi, _ := os.Stat(files[i])
		fj, _ := os.Stat(files[j])
		if fi == nil || fj == nil {
			return false
		}
		return fi.ModTime().Before(fj.ModTime())
	})

	// Hapus yang terlama jika melebihi limit
	if len(files) > sm.config.MaxSnapshots {
		for i := 0; i < len(files)-sm.config.MaxSnapshots; i++ {
			if err := os.Remove(files[i]); err != nil {
				sm.logger.Printf("Error removing old snapshot %s: %v", files[i], err)
			} else {
				sm.logger.Printf("Removed old snapshot: %s", files[i])
			}
		}
	}
}

// ============================================
// 6. AUTO SNAPSHOT
// ============================================

// StartAutoSnapshot memulai auto-snapshot di background
func (sm *JSONManager[V]) Start() {
	if sm.isRunning {
		return
	}

	if sm.config.Interval <= 0 {
		sm.logger.Println("Auto-snapshot disabled (interval <= 0)")
		return
	}

	sm.isRunning = true
	sm.wg.Add(1)

	go func() {
		defer sm.wg.Done()

		ticker := time.NewTicker(sm.config.Interval)
		defer ticker.Stop()

		sm.logger.Printf("Auto-snapshot started (interval: %v)", sm.config.Interval)

		for {
			select {
			case <-ticker.C:
				// Lakukan snapshot di goroutine terpisah agar tidak block
				go func() {
					if err := sm.Save(); err != nil {
						sm.logger.Printf("Auto-snapshot error: %v", err)
					}
				}()

			case <-sm.stopChan:
				sm.logger.Println("Auto-snapshot stopped")
				return
			}
		}
	}()
}

// StopAutoSnapshot menghentikan auto-snapshot
func (sm *JSONManager[V]) Stop() {
	if !sm.isRunning {
		return
	}

	sm.isRunning = false
	close(sm.stopChan)
	sm.wg.Wait()
	sm.logger.Println("Auto-snapshot stopped")
}

// ============================================
// 7. UTILITY FUNCTIONS
// ============================================

// Info mengembalikan informasi tentang snapshot terakhir
func (sm *JSONManager[V]) Info() (map[string]any, error) {
	info, err := os.Stat(sm.config.Path)
	if os.IsNotExist(err) {
		return map[string]any{
			"exists":  false,
			"message": "No snapshot file found",
		}, nil
	}
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"exists":     true,
		"path":       sm.config.Path,
		"size_bytes": info.Size(),
		"size_human": formatSize(info.Size()),
		"modified":   info.ModTime().Format(time.RFC3339),
		"age":        time.Since(info.ModTime()).String(),
	}, nil
}

// formatSize mengubah bytes ke human-readable format
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
