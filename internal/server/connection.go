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
