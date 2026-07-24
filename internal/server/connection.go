package server

import (
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type Connection struct {
	conn         net.Conn
	lastActivity int64
	idleTimeout  time.Duration
	logger       *log.Logger
	remoteAddr   string
	done         chan struct{}
	mu           sync.Mutex
	closed       bool

	// authentication
	authenticated bool

	// Transaction state
	inTransaction  bool
	queuedCommands []*RESPValue
	watchedKeys    map[string]bool
	dirty          bool
}

func NewConnection(conn net.Conn, idleTimeout time.Duration, logger *log.Logger) *Connection {
	return &Connection{
		conn:           conn,
		remoteAddr:     conn.RemoteAddr().String(),
		idleTimeout:    idleTimeout,
		logger:         logger,
		done:           make(chan struct{}),
		inTransaction:  false,
		queuedCommands: make([]*RESPValue, 0),
		watchedKeys:    make(map[string]bool),
		dirty:          false,
	}
}

func (c *Connection) MonitorIdle() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			if c.IsClosed() {
				return
			}
			lastActive := atomic.LoadInt64(&c.lastActivity)
			elapsed := time.Now().Unix() - lastActive

			if elapsed > int64(c.idleTimeout.Seconds()) {
				c.logger.Printf("Idle timeout for %s (idle for %d seconds)",
					c.remoteAddr, elapsed)
				c.Close()
				return
			}
		}
	}
}

func (c *Connection) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.closed {
		c.closed = true
		close(c.done)
		c.conn.Close()
		c.logger.Printf("Connection closed: %s", c.remoteAddr)
	}
}

func (c *Connection) IsClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func (c *Connection) UpdateActivity() {
	atomic.StoreInt64(&c.lastActivity, time.Now().Unix())
}

// set authentication
func (c *Connection) SetAuthenticated(auth bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.authenticated = auth
}

func (c *Connection) IsAuthenticated() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.authenticated
}

// ResetTransaction clears transaction state
func (c *Connection) ResetTransaction() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.inTransaction = false
	c.queuedCommands = make([]*RESPValue, 0)
	c.watchedKeys = make(map[string]bool)
	c.dirty = false
}

// WatchKey adds a key to watch list
func (c *Connection) WatchKey(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.watchedKeys[key] = true
}

// IsWatched checks if a key is being watched
func (c *Connection) IsWatched(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.watchedKeys[key]
}

// MarkDirty marks transaction as dirty (watched key changed)
func (c *Connection) MarkDirty() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dirty = true
}

// IsDirty returns true if transaction is dirty
func (c *Connection) IsDirty() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.dirty
}
