package persistence

import (
	"log"
	"pendem/internal/config"
	"pendem/internal/engine"
)

type Manager[V any] struct {
	json   *JSONManager[V]
	rdb    *RDB
	aof    *AOF
	logger *log.Logger
	cache  *engine.Cache[string]
	config config.PersistenceConfig
}

func NewManager[V any](logger *log.Logger, cache *engine.Cache[string], cfg config.PersistenceConfig) *Manager[V] {
	mgr := &Manager[V]{
		logger: logger,
		cache:  cache,
		config: cfg,
	}

	if cfg.JSON.Enabled {
		mgr.json = NewJSONManager[V](logger, cache, cfg.JSON)
	}

	if cfg.RDB.Enabled {
		mgr.rdb = NewRDB(logger, cache, cfg.RDB)
	}

	if cfg.AOF.Enabled {
		mgr.aof = NewAOF(logger, cache, cfg.AOF.FlushInterval)
	}

	// Log persistence status
	logger.Printf("Persistence: JSON=%v, RDB=%v, AOF=%v",
		cfg.JSON.Enabled, cfg.RDB.Enabled, cfg.AOF.Enabled)

	return mgr
}

func (pm *Manager[V]) Start() error {
	// 1. Load data (Smart recovery)
	if err := pm.loadData(); err != nil {
		return err
	}

	// 2. Start background processes
	if pm.config.JSON.Enabled && pm.json != nil {
		pm.json.Start()
	}
	if pm.config.RDB.Enabled && pm.rdb != nil {
		pm.rdb.Start()
	}
	if pm.config.AOF.Enabled && pm.aof != nil {
		if err := pm.aof.Start(); err != nil {
			pm.logger.Printf("Warning: Failed to start AOF: %v", err)
		}
	}

	return nil
}

func (pm *Manager[V]) Stop() error {
	if pm.aof != nil {
		pm.aof.Stop()
	}
	if pm.rdb != nil {
		pm.rdb.Stop()
	}
	if pm.json != nil {
		pm.json.Stop()
	}
	return nil
}

func (pm *Manager[V]) Save() error {
	// 1. Flush AOF (if enabled)
	if pm.aof != nil {
		pm.aof.Flush()
	}

	// 2. RDB Snapshot (if enabled)
	if pm.rdb != nil {
		if err := pm.rdb.Save(); err != nil {
			pm.logger.Printf("Warning: RDB save failed: %v", err)
		}
	}

	// 3. JSON Snapshot (if enabled)
	if pm.json != nil {
		if err := pm.json.Save(); err != nil {
			pm.logger.Printf("Warning: JSON snapshot save failed: %v", err)
		}
	}

	return nil
}

func (m *Manager[V]) LogCommand(cmd string, args ...string) error {
	if m.aof != nil {
		return m.aof.LogCommand(cmd, args...)
	}
	return nil
}

func (pm *Manager[V]) loadData() error {
	// Priority 1: AOF (most up-to-date)
	if pm.config.AOF.Enabled && pm.aof != nil {
		if err := pm.aof.Load(); err == nil {
			pm.logger.Println("✅ Data restored from AOF")
			return nil
		} else {
			pm.logger.Printf("⚠️ AOF load failed: %v, trying next...", err)
		}
	}

	// Priority 2: RDB (fast binary)
	if pm.config.RDB.Enabled && pm.rdb != nil {
		if err := pm.rdb.Load(); err == nil {
			pm.logger.Println("✅ Data restored from RDB")
			return nil
		} else {
			pm.logger.Printf("⚠️ RDB load failed: %v, trying next...", err)
		}
	}

	// Priority 3: JSON Snapshot (human readable)
	if pm.config.JSON.Enabled && pm.json != nil {
		if err := pm.json.Load(); err == nil {
			pm.logger.Println("✅ Data restored from JSON Snapshot")
			return nil
		} else {
			pm.logger.Printf("⚠️ JSON snapshot load failed: %v", err)
		}
	}

	pm.logger.Println("ℹ️ No valid persistence found, starting with empty cache")
	return nil
}
