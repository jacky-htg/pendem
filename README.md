# Pendem

>   🏺 "Menyimpan di dalam tanah" - Cache Server yang Kokoh Seperti Tanah

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)

**Pendem** adalah cache server yang dibangun dari awal menggunakan Go. Nama "Pendem" berasal dari bahasa Jawa yang berarti **"menyimpan di dalam tanah"** - mencerminkan filosofi penyimpanan data yang kokoh dan aman, siap diambil dengan cepat.

## 🎯 Why Pendem?

**Pendem** built with:

- **Simplicity in mind** — minimal configuration, maximum performance
- **Go-native concurrency** — goroutines and channels for efficient request handling
- **Sharded by design** — 16 shards for true parallel operations from day one
- **Redis compatible** - using RESP Protocol, compatible with all redis client.

Perfect for microservices, session management, and high-throughput caching.

## ✨ Fitur

| Fitur | Status | Keterangan |
|-------|--------|------------|
| RESP Protocol | ✅ | Full Redis Serialization Protocol support |
| TCP Server | ✅ | Concurrent connection handling with graceful shutdown |
| In-Memory Storage | ✅ | Fast key-value storage with TTL support |
| String Data Type | ✅ | EXISTS, GET, SET, DEL |
| Hash Data Type | ✅ | HGET, HSET |
| LIST Data Type | ✅ | LPUSH, RPUSH, LPOP, RPOP |
| SET Data Type | ✅ | SADD, SREM, SMEMBERS |
| SORTED SET Data Type | ✅ | ZADD, ZRANGE |
| LRU Eviction | ✅ | Least Recently Used eviction policy |
| Sharding | ✅ | 16 shards for parallel operations |
| TTL Support | ✅ | Time-to-live with EX and PX options |
| Batch Operations | ✅ | MGET, MSET, Pipeline |
| Transaction | ✅ | MULTI, EXEC, WATCH |
| Persistence | ✅ | JSON, RDB & AOF |
| Pub/Sub | ⏳ | Publish/Subscribe messaging (coming soon) |
| Monitoring | ⏳ | Monitor performance, system, cache, connection, etc (coming soon) |
| Clustering | ⏳ | Distributed cache support (coming soon) |

## 🚀 Quick Start

### Prerequisites
- Go 1.21 or higher
- Redis CLI (for testing, optional)

### Installation

```bash

# Clone repository
git clone https://github.com/jacky-htg/pendem.git
cd pendem

# Build
go build -o pendem cmd/main.go

# Run
./pendem
```

### Using with Docker

```bash

# Build image
docker build -t pendem .

# Run container
docker run -p 6379:6379 pendem
```

### Testing with redis-cli

```bash

# Connect to Pendem
redis-cli -h localhost -p 6379

# Basic operations
127.0.0.1:6379> PING
PONG

127.0.0.1:6379> SET user:1 "John Doe"
OK

127.0.0.1:6379> GET user:1
"John Doe"

127.0.0.1:6379> SET session:1 "active" EX 10
OK

127.0.0.1:6379> TTL session:1
(integer) 8

127.0.0.1:6379> DEL user:1
(integer) 1

127.0.0.1:6379> MEMORY
{"alloc": "198.09 KB", "total_alloc": "198.09 KB", "sys": "12.27 MB", "num_gc": "0"}

127.0.0.1:6379> POLICY
"Eviction policy: lru (items: 2, max: 160000)"

127.0.0.1:6379> MEMORY USAGE
"12.00 KB"
```

## 📊 Performance

### Throughput

| Scenario | Ops/sec | Latency (avg) |
|----------|---------|---------------|
| SET (single client) | ~150,000 | 6.7 μs |
| GET (single client) | ~170,000 | 5.9 μs |
| Mixed (100 clients) | ~120,000 | 8.3 μs |

### Key Distribution (16 shards)

```text

Shard 0:  6,248 (6.25%)  ████████░░░░░░░░
Shard 1:  6,231 (6.23%)  ████████░░░░░░░░
Shard 2:  6,259 (6.26%)  ████████░░░░░░░░
Shard 3:  6,242 (6.24%)  ████████░░░░░░░░
...
```

## 🏗️ Architecture

```text

┌─────────────────────────────────────────────────────────────────┐
│                         PENDEM                                  │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│   ┌─────────────────────────────────────────────────────────┐   │
│   │                    TCP Server                           │   │
│   │  ┌───────────────────────────────────────────────────┐  │   │
│   │  │              RESP Protocol Parser                 │  │   │
│   │  └───────────────────────────────────────────────────┘  │   │
│   └─────────────────────────────────────────────────────────┘   │
│                              │                                  │
│                              ▼                                  │
│   ┌─────────────────────────────────────────────────────────┐   │
│   │                   Command Handlers                      │   │
│   │  PING │ SET │ GET │ DEL │ TTL │ MEMORY │ POLICY         │   │
│   └─────────────────────────────────────────────────────────┘   │
│                              │                                  │
│                              ▼                                  │
│   ┌─────────────────────────────────────────────────────────┐   │
│   │                  Sharded Cache                          │   │
│   │  ┌──────┬──────┬──────┬──────┬──────┬──────┬──────┐     │   │
│   │  │ S0   │ S1   │ S2   │ S3   │ S4   │ S5   │ S6   │     │   │
│   │  │ LRU  │ LRU  │ LRU  │ LRU  │ LRU  │ LRU  │ LRU  │     │   │
│   │  └──────┴──────┴──────┴──────┴──────┴──────┴──────┘     │   │
│   └─────────────────────────────────────────────────────────┘   │
│                                                                 │
│   ❗ Sharding enables parallel operations!                      │
└─────────────────────────────────────────────────────────────────┘
```

## 🔧 Configuration

### Default Config

```go
func DefaultConfig() Config {
	return Config{
		Server: ServerConfig{
			Port:           "6379",
			MaxConnections: 50_000,
			ReadTimeout:    2 * time.Minute,
			WriteTimeout:   30 * time.Second,
			IdleTimeout:    10 * time.Minute,
		},
		Engine: EngineConfig{
			MaxMemory:       1024 * 1024 * 1024, // 1GB
			EvictionPolicy:  "lru",
			EvictorCapacity: 10_000,
			ShardCount:      16,
			DefaultTTL:      0, // No default TTL
		},
		Persistence: PersistenceConfig{
			JSON: JSONConfig{
				Enabled:      true,
				Path:         "pendem.snapshot.json",
				Interval:     1 * time.Hour,
				MaxSnapshots: 5,
			},
			RDB: RDBConfig{
				Enabled:  true,
				Path:     "pendem.rdb",
				Interval: 1 * time.Hour,
			},
			AOF: AOFConfig{
				Enabled:       true,
				FilePath:      "pendem.aof",
				FlushInterval: 5 * time.Minute, // 5 menit
				SyncOnWrite:   false,
			},
		},
	}
}
```

### Example Custom Config

```ini
[server]
port = 6378
max_connections = 50000
read_timeout = 2m
write_timeout = 30s
idle_timeout = 5m

[engine]
max_memory = 1GB
eviction_policy = lru
evictor_capacity = 10000
shard_count = 16
default_ttl = 0

[persistence.rdb]
enabled = true
path = pendem.rdb
interval = 1h

[persistence.aof]
enabled = true
path = pendem.aof
flush_interval = 5m
sync_on_write = false

[persistence.json]
enabled = false
path = pendem.snapshot.json
interval = 1h
max_snapshots = 5
```

## 📚 Commands Summary

| Category | Commands |
|----------|----------|
| Generic | PING, ECHO, MEMORY, MEMORY USAGE, POLICY, SCAN |
| String | GET, SET, DEL, TTL, APPEND, STRLEN, EXISTS |
| Hash | HSET, HGET, HGETALL, HDEL, HLEN, HEXISTS, HKEYS, HVALS |
| List | LPUSH, RPUSH, LPOP, RPOP, LRANGE, LLEN |
| Set | SADD, SREM, SMEMBERS, SISMEMBER, SCARD |
| Sorted Set | ZADD, ZRANGE, ZREM, ZCARD, ZSCORE |
| Batch Operation | MGET, MSET, MSETNX, Pipeline |
| Transaction | WATCH, UNWATCH, MULTI, EXEC, DISCARD |

## 🧪 Testing

```bash

# Run all tests
go test ./...

# Run integration tests
go test ./test/integration/ -v

# Run with race detection
go test ./... -race

# Run benchmarks
go test ./internal/engine/ -bench=. -benchmem
```

## 🤝 Contributing

Contributions are welcome!

1. Fork the repository
1. Create your feature branch (git checkout -b feature/amazing-feature)
3. Commit your changes (git commit -m 'Add some amazing feature')
4. Push to the branch (git push origin feature/amazing-feature)
5. Open a Pull Request

## 📄 License

This project is licensed under the GNU GPL License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Inspired by [Redis](https://redis.io/)
- Built with [Go](https://golang.org/)
- Special thanks to all contributors

## 🏆 Roadmap

- ✅ Socket Programming
- ✅ RESP Protocol
- ✅ In-Memory Storage
- ✅ LRU Eviction
- ⏳ LFU Eviction
- ⏳ TTL Eviction
- ✅ Sharding
- ✅ Persistence (RDB & AOF)
- ✅ More Data Type (Data Struct: Hash, List, Set, SortedSet)
- ✅ Batch Operations (MGET, MSET, Pipeline)
- ✅ Transaction Using Multi, Exec, Watch
- ⏳ Pub/Sub
- ⏳ Monitoring
- ⏳ Cluster Support

🏺 Pendem - Cache Server yang Kokoh Seperti Tanah

[Report Bug](https://github.com/jacky-htg/pendem/issues) · [Request Feature](https://github.com/jacky-htg/pendem/issues)