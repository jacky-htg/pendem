# Pendem

>   🏺 "Menyimpan di dalam tanah" - Cache Server yang Kokoh Seperti Tanah

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/License-GPLv3-blue.svg)](LICENSE)

**Pendem** adalah Redis-like cache server yang dibangun dari awal menggunakan Go. Nama "Pendem" berasal dari bahasa Jawa yang berarti **"menyimpan di dalam tanah"** - mencerminkan filosofi penyimpanan data yang kokoh dan aman, siap diambil dengan cepat.

## 🎯 Why Pendem?

**Pendem** isn't just another Redis clone. It's built with:

- **Simplicity in mind** — minimal configuration, maximum performance
- **Go-native concurrency** — goroutines and channels for efficient request handling
- **Javanese philosophy** — data stored deep, ready to be unearthed instantly
- **Sharded by design** — 16 shards for true parallel operations from day one

Perfect for microservices, session management, and high-throughput caching.

## ✨ Fitur

| Fitur | Status | Keterangan |
|-------|--------|------------|
| RESP Protocol | ✅ | Full Redis Serialization Protocol support |
| TCP Server | ✅ | Concurrent connection handling with graceful shutdown |
| In-Memory Storage | ✅ | Fast key-value storage with TTL support |
| LRU Eviction | ✅ | Least Recently Used eviction policy |
| Sharding | ✅ | 16 shards for parallel operations |
| TTL Support | ✅ | Time-to-live with EX and PX options |
| Batch Operations | ⏳ | MGET, MSET, Pipeline (coming soon) |
| Persistence | ✅ | RDB & AOF |
| Pub/Sub | ⏳ | Publish/Subscribe messaging (coming soon) |
| Clustering | ⏳ | Distributed cache support (coming soon) |
| Monitoring | ⏳ | Monitor performance, system, cache, connection, etc (coming soon) |

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

### Server Config

```go

type ServerConfig struct {
    MaxConnections int           // Maximum simultaneous connections (default: 10000)
    ReadTimeout    time.Duration // Read timeout (default: 30s)
    WriteTimeout   time.Duration // Write timeout (default: 30s)
    IdleTimeout    time.Duration // Idle connection timeout (default: 60s)
}

type EngineConfig struct {
    MaxMemory       int64  // Max memory in bytes (default: 1GB)
    EvictionPolicy  string // "lru", "lfu", "ttl" (default: "lru")
    EvictorCapacity int    // Max items per shard (default: 10000)
    ShardCount      int    // Number of shards (default: 16)
    DefaultTTL      time.Duration // Default TTL for keys (default: 0 = no TTL)
}
```

### Example Custom Config

```go

config := config.Config{
    Server: config.ServerConfig{
        MaxConnections: 50000,
        ReadTimeout:    30 * time.Second,
        WriteTimeout:   10 * time.Second,
        IdleTimeout:    60 * time.Second,
    },
    Engine: config.EngineConfig{
        MaxMemory:       2 * 1024 * 1024 * 1024, // 2GB
        EvictionPolicy:  "lru",
        EvictorCapacity: 20000,
        ShardCount:      32,
        DefaultTTL:      0,
    },
}
```

## 📚 Commands

| Command | Description | Example |
|---------|-------------|---------|
| PING | Test connection | PING → PONG |
| GET | Get value by key | GET key → "value" |
| SET | Set value with TTL | SET key value EX 10 → OK |
| DEL | Delete key(s) | DEL key1 key2 → 2 |
| TTL | Get remaining TTL | TTL key → 10 |
| MEMORY | Server memory stats | MEMORY → JSON |
| MEMORY USAGE | Cache memory usage | MEMORY USAGE → "12.00 KB" |
| POLICY | Show eviction policy | POLICY → "lru (items: 5, max: 10000)" |

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
- ✅ Sharding
- ✅ Persistence (RDB & AOF)
- ⏳ Batch Operations (MGET, MSET, Pipeline)
- ⏳ Pub/Sub
- ⏳ Cluster Support
- ⏳ Monitoring
- ⏳ LFU Eviction
- ⏳ TTL Eviction

🏺 Pendem - Cache Server yang Kokoh Seperti Tanah

[Report Bug](https://github.com/jacky-htg/pendem/issues) · [Request Feature](https://github.com/jacky-htg/pendem/issues)