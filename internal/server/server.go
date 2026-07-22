package server

import (
	"fmt"
	"io"
	"log"
	"net"
	"pendem/internal/config"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Server struct {
	addr       string
	listener   net.Listener // Listener untuk menerima koneksi
	logger     *log.Logger
	config     config.ServerConfig
	handlers   map[string]CommandHandler // Handler untuk command (sederhana)
	activeConn int32                     // Counter aktif koneksi (atomic)
	wg         sync.WaitGroup            // WaitGroup untuk graceful shutdown
	quit       chan struct{}             // Channel untuk signal shutdown
	mu         sync.Mutex                // Untuk protect listener close
	closed     bool                      // Flag untuk cek sudah closed

	watchers  map[string][]*Connection // key → list of connections
	watcherMu sync.RWMutex
}

// CommandHandler adalah fungsi untuk menangani perintah
type CommandHandler func(args []string) RESPValue

// Konstruktior: NewServer membuat instance server baru
func NewServer(addr string, log *log.Logger) *Server {
	s := &Server{
		addr:       addr,
		logger:     log,
		config:     config.DefaultConfig().Server,
		handlers:   make(map[string]CommandHandler),
		activeConn: 0,
		quit:       make(chan struct{}),
		closed:     false,
		watchers:   make(map[string][]*Connection),
	}

	return s
}

// NewServerWithConfig membuat server dengan konfigurasi kustom
func NewServerWithConfig(addr string, log *log.Logger, cfg config.ServerConfig) *Server {
	s := NewServer(addr, log)
	s.config = cfg
	return s
}

// RegisterHandler mendaftarkan handler untuk perintah
func (s *Server) RegisterHandler(cmd string, handler CommandHandler) {
	s.handlers[strings.ToUpper(cmd)] = handler
}

func (s *Server) GetHandler(cmd string) (CommandHandler, bool) {
	handler, exists := s.handlers[cmd]
	return handler, exists
}

// Start memulai server
func (s *Server) Start() error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}
	s.mu.Lock()
	s.listener = listener
	s.mu.Unlock()

	s.logger.Printf("Server listening on %s", s.addr)
	s.logger.Printf("Max connections: %d", s.config.MaxConnections)
	s.logger.Printf("Read timeout: %v", s.config.ReadTimeout)
	s.logger.Printf("Write timeout: %v", s.config.WriteTimeout)
	s.logger.Printf("Idle timeout: %v", s.config.IdleTimeout)

	for {
		// Cek apakah ada sinyal shutdown
		select {
		case <-s.quit:
			s.logger.Println("Server stopped")
			return nil
		default:
		}

		// Terima koneksi
		conn, err := s.listener.Accept()
		if err != nil {
			// Cek apakah listener sengaja ditutup
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()

			if closed {
				s.logger.Println("Listener closed, stopping accept loop")
				return nil
			}

			// Cek apakah error karena listener ditutup (shutdown)
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}

			s.logger.Printf("Accept error: %v", err)
			continue
		}

		// Cek shutdown signal sebelum menerima koneksi baru
		select {
		case <-s.quit:
			conn.Write([]byte("ERR server shutting down\n"))
			conn.Close()
			continue
		default:
		}

		// Cek apakah sudah mencapai batas maksimal koneksi
		currentConn := atomic.LoadInt32(&s.activeConn)
		if currentConn >= int32(s.config.MaxConnections) {
			// Tolak koneksi dengan pesan error
			s.logger.Printf("Max connections reached (%d), rejecting connection from %s",
				s.config.MaxConnections, conn.RemoteAddr().String())
			conn.Write([]byte("ERR server full, maximum connections reached\n"))
			conn.Close()
			continue
		}

		// Tambah counter aktif koneksi
		atomic.AddInt32(&s.activeConn, 1)

		go s.handleConnection(conn)
	}
}

// Shutdown menghentikan server dengan graceful
func (s *Server) Shutdown() error {
	return s.ShutdownWithTimeout(30 * time.Second)
}

// ShutdownWithTimeout menghentikan server dengan timeout
func (s *Server) ShutdownWithTimeout(timeout time.Duration) error {
	s.logger.Println("Starting graceful shutdown...")

	// 1. Signal untuk stop menerima koneksi baru
	close(s.quit)

	// 2. Tutup listener (stop accept)
	s.mu.Lock()
	if !s.closed {
		s.closed = true
		if s.listener != nil {
			s.listener.Close()
		}
	}
	s.mu.Unlock()

	// 3. Tunggu semua koneksi selesai dengan timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait() // Tunggu semua goroutine selesai
		close(done)
	}()

	// 4. Tunggu atau timeout
	select {
	case <-done:
		s.logger.Println("All connections finished gracefully")
		return nil
	case <-time.After(timeout):
		active := atomic.LoadInt32(&s.activeConn)
		s.logger.Printf("Shutdown timeout after %v, %d connections still active",
			timeout, active)
		return fmt.Errorf("shutdown timeout after %v, %d connections still active",
			timeout, active)
	}
}

func (s *Server) PrintMemoryUsage() string {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Konversi ke MB dengan 2 desimal
	allocKB := float64(m.Alloc) / 1024
	totalAllocKB := float64(m.TotalAlloc) / 1024
	sysMB := float64(m.Sys) / 1024 / 1024

	return fmt.Sprintf(
		`{"alloc": "%.2f KB", "total_alloc": "%.2f KB", "sys": "%.2f MB", "num_gc": "%d"}`,
		allocKB,
		totalAllocKB,
		sysMB,
		m.NumGC,
	)
}

func (s *Server) parseCmd(value *RESPValue) (string, []string) {
	// Perintah pertama adalah command
	cmd := strings.ToUpper(value.Array[0].Str)
	args := make([]string, 0)

	for i := 1; i < len(value.Array); i++ {
		if value.Array[i].Type == BulkString {
			args = append(args, value.Array[i].Str)
		}
	}
	return cmd, args
}

// processCommand memproses perintah dari client
func (s *Server) processCommand(value *RESPValue, cmd string, args []string) RESPValue {
	if value.Type != Array || len(value.Array) < 1 {
		return RESPValue{
			Type: Error,
			Str:  "ERR invalid command",
		}
	}
	if len(cmd) == 0 {
		cmd, args = s.parseCmd(value)
	}

	handler, exists := s.GetHandler(cmd)
	if !exists {
		return RESPValue{
			Type: Error,
			Str:  fmt.Sprintf("ERR unknown command '%s'", cmd),
		}
	}

	// Check if any watched key was modified
	if s.isWriteCommand(cmd) {
		keys := s.extractKeys(cmd, args)
		if len(keys) > 0 {
			s.markWatchedKeysDirty(keys)
		}
	}

	// Eksekusi handler
	return handler(args)
}

func (s *Server) processCommandWithTx(c *Connection, value *RESPValue) RESPValue {
	result, cmd, args, isTxCmd := s.handleTxCmd(c, value)
	if isTxCmd {
		return result
	}

	return s.processCommand(value, cmd, args)
}

// handleConnection menangani satu koneksi
func (s *Server) handleConnection(conn net.Conn) {
	// Tambahkan ke WaitGroup untuk gracefull shutdown
	s.wg.Add(1)
	defer s.wg.Done()

	c := NewConnection(conn, s.config.IdleTimeout, s.logger)

	// Pastikan connection ditutup saat function selesai
	defer func() {
		c.Close()
		s.cleanupWatchers(c)
		// Kurangi counter aktif koneksi
		atomic.AddInt32(&s.activeConn, -1)
		s.logger.Printf("Active connections: %d", atomic.LoadInt32(&s.activeConn))
	}()

	// Cek apakah shutdown sedang berlangsung
	select {
	case <-s.quit:
		s.logger.Printf("Shutting down, rejecting new connection from %s", c.remoteAddr)
		conn.Write([]byte("ERR server shutting down\n"))
		return
	default:
	}

	// Start idle monitor
	c.UpdateActivity()
	go c.MonitorIdle()

	// Log koneksi baru
	s.logger.Printf("New connection from %s", c.remoteAddr)
	s.logger.Printf("Active connections: %d", atomic.LoadInt32(&s.activeConn))

	// Buat parser
	parser := NewRESPParser(conn)

	// Channel untuk shutdown interrupt
	shutdown := make(chan struct{})
	go func() {
		<-s.quit
		c.Close() // Langsung close connection
		close(shutdown)
	}()

	// Loop membaca perintah dari client
	for {
		if c.IsClosed() {
			return
		}

		// Cek shutdown signal
		select {
		case <-s.quit:
			s.logger.Printf("Shutting down, closing connection from %s", c.remoteAddr)
			return
		default:
		}

		// Set read timeout
		if s.config.ReadTimeout > 0 {
			conn.SetReadDeadline(time.Now().Add(s.config.ReadTimeout))
		}

		// Baca di goroutine terpisah dengan channel
		type readResult struct {
			respVal *RESPValue
			err     error
		}
		readCh := make(chan readResult, 1)
		go func() {
			respVal, err := parser.Read()
			readCh <- readResult{respVal, err}
		}()

		// Wait for read OR shutdown
		select {
		case <-shutdown:
			s.logger.Printf("Shutdown interrupt during read from %s", c.remoteAddr)
			return
		case res := <-readCh:

			if res.err != nil {
				// EOF berarti client disconnect
				if res.err == io.EOF {
					s.logger.Printf("Client %s disconnected", c.remoteAddr)
				} else if ne, ok := res.err.(net.Error); ok && ne.Timeout() {
					s.logger.Printf("Client %s timeout", c.remoteAddr)
				} else {
					s.logger.Printf("Read error from %s: %v", c.remoteAddr, res.err)
				}
				return
			}

			// handleConnection - bagian pipeline
			if res.respVal.Type == Array && len(res.respVal.Array) > 0 {
				// Cek apakah ini pipeline (array of arrays)
				// Pipeline: [ [SET key value], [GET key] ]
				// Single command: [ SET key value ] (bukan array of arrays)
				if res.respVal.Array[0].Type == Array {
					// Ini pipeline!
					responses := s.processPipeline(c, res.respVal.Array)
					if s.config.WriteTimeout > 0 {
						conn.SetWriteDeadline(time.Now().Add(s.config.WriteTimeout))
					}
					for _, resp := range responses {
						conn.Write([]byte(EncodeRESP(resp)))
					}
					c.UpdateActivity()
					continue
				}
			}

			// Proses command
			response := s.processCommandWithTx(c, res.respVal)

			// Set write timeout
			if s.config.WriteTimeout > 0 {
				conn.SetWriteDeadline(time.Now().Add(s.config.WriteTimeout))
			}

			conn.Write([]byte(EncodeRESP(response)))
			c.UpdateActivity()
		}
	}
}

// Pipeline dengan error handling
func (s *Server) processPipeline(c *Connection, commands []RESPValue) []RESPValue {
	results := make([]RESPValue, len(commands))
	for i, cmd := range commands {
		// Process each command independently
		// One error doesn't affect others
		results[i] = s.processCommand(&cmd, "", nil)
	}
	return results
}
