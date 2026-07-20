package persistence

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"
	"pendem/internal/config"
	"pendem/internal/engine" // Import cache package kita
	"sync"
	"time"
)

// RDB format:
// +------------------+
// | MAGIC: "PENDEM"  | 6 bytes
// +------------------+
// | VERSION: 1       | 1 byte
// +------------------+
// | NUM_SHARDS: 16   | 2 bytes
// +------------------+
// | For each shard:  |
// |   SHARD_ID: 0    | 2 bytes
// |   NUM_ITEMS: 10  | 4 bytes
// |   For each item: |
// |     KEY_LEN: 5   | 2 bytes
// |     KEY: "user"  | KEY_LEN bytes
// |     VAL_LEN: 10  | 4 bytes
// |     VAL: "john"  | VAL_LEN bytes
// |     TTL: 3600    | 8 bytes (0 = no TTL)
// |     EXPIRE: ...  | 8 bytes (timestamp)
// +------------------+
// | CHECKSUM: ...    | 4 bytes (optional)
// +------------------+

const RDBMagic = "PENDEM"
const RDBVersion = 1

// RDB mengelola snapshot RDB
type RDB struct {
	config    config.RDBConfig
	cache     *engine.Cache[string]
	mu        sync.RWMutex
	logger    *log.Logger
	stopChan  chan struct{}
	wg        sync.WaitGroup
	isRunning bool
}

// NewRDB membuat rdb snapshot manager baru
func NewRDB(logger *log.Logger, cache *engine.Cache[string], cfg config.RDBConfig) *RDB {
	return &RDB{
		config:   cfg,
		cache:    cache,
		logger:   logger,
		stopChan: make(chan struct{}),
	}
}

// Save membuat rdb snapshot
func (r *RDB) Save() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.logger.Println("Starting RDB save...")
	startTime := time.Now()

	// Create temp file
	tempFile := r.config.Path + ".tmp"
	f, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)

	// 1. Write magic + version
	if _, err := w.Write([]byte(RDBMagic)); err != nil {
		return err
	}
	if err := w.WriteByte(RDBVersion); err != nil {
		return err
	}

	// 2. Write number of shards
	numShards := r.cache.NumShards()
	if err := binary.Write(w, binary.LittleEndian, uint16(numShards)); err != nil {
		return err
	}

	totalItems := 0

	// 3. Write each shard
	for shardID := 0; shardID < numShards; shardID++ {
		shard := r.cache.GetShard(shardID)
		if shard == nil {
			continue
		}

		items := shard.GetItems()
		if len(items) == 0 {
			continue
		}

		// Write shard ID
		if err := binary.Write(w, binary.LittleEndian, uint16(shardID)); err != nil {
			return err
		}

		// Write number of items in this shard
		if err := binary.Write(w, binary.LittleEndian, uint32(len(items))); err != nil {
			return err
		}

		for key, item := range items {
			// Write key
			keyBytes := []byte(key)
			if err := binary.Write(w, binary.LittleEndian, uint16(len(keyBytes))); err != nil {
				return err
			}
			if _, err := w.Write(keyBytes); err != nil {
				return err
			}

			// Write value (as string)
			valBytes := []byte(item.Value)
			if err := binary.Write(w, binary.LittleEndian, uint32(len(valBytes))); err != nil {
				return err
			}
			if _, err := w.Write(valBytes); err != nil {
				return err
			}

			// Write TTL (0 = no TTL)
			ttl := item.TTL()
			if ttl < 0 {
				ttl = 0
			}
			if err := binary.Write(w, binary.LittleEndian, int64(ttl.Seconds())); err != nil {
				return err
			}

			// Write expiration timestamp
			if err := binary.Write(w, binary.LittleEndian, item.Expiration); err != nil {
				return err
			}

			totalItems++
		}
	}

	// 4. Flush and rename
	if err := w.Flush(); err != nil {
		return err
	}
	f.Close()

	if err := os.Rename(tempFile, r.config.Path); err != nil {
		return err
	}

	elapsed := time.Since(startTime)
	r.logger.Printf("RDB saved successfully in %v (%d items, %d shards)",
		elapsed, totalItems, numShards)

	return nil
}

func (r *RDB) Load() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if file exists
	if _, err := os.Stat(r.config.Path); os.IsNotExist(err) {
		r.logger.Println("No RDB file found, starting with empty cache")
		return nil
	}

	r.logger.Printf("Loading RDB from %s...", r.config.Path)
	startTime := time.Now()

	f, err := os.Open(r.config.Path)
	if err != nil {
		return fmt.Errorf("failed to open RDB file: %w", err)
	}
	defer f.Close()

	reader := bufio.NewReader(f)

	// 1. Read magic
	magic := make([]byte, len(RDBMagic))
	if _, err := io.ReadFull(reader, magic); err != nil {
		return fmt.Errorf("failed to read magic: %w", err)
	}
	if string(magic) != RDBMagic {
		return fmt.Errorf("invalid RDB magic: %s", string(magic))
	}

	// 2. Read version
	version, err := reader.ReadByte()
	if err != nil {
		return fmt.Errorf("failed to read version: %w", err)
	}
	if version != RDBVersion {
		return fmt.Errorf("unsupported RDB version: %d", version)
	}

	// 3. Read number of shards
	var numShards uint16
	if err := binary.Read(reader, binary.LittleEndian, &numShards); err != nil {
		return fmt.Errorf("failed to read shard count: %w", err)
	}

	totalItems := 0

	// 4. Read each shard
	for {
		// Check if we're at EOF
		var shardID uint16
		err := binary.Read(reader, binary.LittleEndian, &shardID)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read shard ID: %w", err)
		}

		// Read number of items
		var numItems uint32
		if err := binary.Read(reader, binary.LittleEndian, &numItems); err != nil {
			return fmt.Errorf("failed to read item count: %w", err)
		}

		// Get shard
		shard := r.cache.GetShard(int(shardID))
		if shard == nil {
			r.logger.Printf("Warning: Shard %d not found, skipping %d items", shardID, numItems)
			// Skip items
			for i := 0; i < int(numItems); i++ {
				// Skip key
				var keyLen uint16
				binary.Read(reader, binary.LittleEndian, &keyLen)
				reader.Discard(int(keyLen))

				// Skip value
				var valLen uint32
				binary.Read(reader, binary.LittleEndian, &valLen)
				reader.Discard(int(valLen))

				// Skip TTL
				var ttl int64
				binary.Read(reader, binary.LittleEndian, &ttl)

				// Skip expiration
				var exp int64
				binary.Read(reader, binary.LittleEndian, &exp)
			}
			continue
		}

		items := make(map[string]engine.Item[string])
		for i := 0; i < int(numItems); i++ {
			// Read key
			var keyLen uint16
			if err := binary.Read(reader, binary.LittleEndian, &keyLen); err != nil {
				return fmt.Errorf("failed to read key length: %w", err)
			}
			keyBytes := make([]byte, keyLen)
			if _, err := io.ReadFull(reader, keyBytes); err != nil {
				return fmt.Errorf("failed to read key: %w", err)
			}
			key := string(keyBytes)

			// Read value
			var valLen uint32
			if err := binary.Read(reader, binary.LittleEndian, &valLen); err != nil {
				return fmt.Errorf("failed to read value length: %w", err)
			}
			valBytes := make([]byte, valLen)
			if _, err := io.ReadFull(reader, valBytes); err != nil {
				return fmt.Errorf("failed to read value: %w", err)
			}
			value := string(valBytes)

			// Read TTL
			var ttlSec int64
			if err := binary.Read(reader, binary.LittleEndian, &ttlSec); err != nil {
				return fmt.Errorf("failed to read TTL: %w", err)
			}

			// Read expiration
			var exp int64
			if err := binary.Read(reader, binary.LittleEndian, &exp); err != nil {
				return fmt.Errorf("failed to read expiration: %w", err)
			}

			// Skip expired items
			if exp > 0 && time.Now().UnixNano() > exp {
				continue
			}

			// Create item
			item := engine.Item[string]{
				Value:      value,
				Expiration: exp,
			}
			items[key] = item
			totalItems++
		}

		shard.Restore(items)
		r.logger.Printf("Restored shard %d: %d items", shardID, len(items))
	}

	elapsed := time.Since(startTime)
	r.logger.Printf("RDB loaded successfully in %v (%d items)", elapsed, totalItems)

	return nil
}

func (sm *RDB) Start() {
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

func (sm *RDB) Stop() {
	if !sm.isRunning {
		return
	}

	sm.isRunning = false
	close(sm.stopChan)
	sm.wg.Wait()
	sm.logger.Println("Auto-snapshot stopped")
}
