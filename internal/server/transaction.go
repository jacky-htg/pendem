package server

func (s *Server) handleTxCmd(conn *Connection, value *RESPValue) (RESPValue, string, []string, bool) {
	if len(value.Array) < 1 {
		return *value, "", nil, false
	}

	cmd, args := s.parseCmd(value)

	switch cmd {
	case "MULTI":
		return s.handleMulti(conn, args), cmd, args, true
	case "WATCH":
		return s.handleWatch(conn, args), cmd, args, true
	case "UNWATCH":
		return s.handleUnwatch(conn, args), cmd, args, true
	case "EXEC":
		keys := s.extractKeys(cmd, args)
		s.removeWatcherByKeys(keys, conn)

		return s.handleExec(conn, args), cmd, args, true
	case "DISCARD":
		keys := s.extractKeys(cmd, args)
		s.removeWatcherByKeys(keys, conn)

		return s.handleDiscard(conn, args), cmd, args, true
	}

	if conn.inTransaction {
		// Queue command
		conn.mu.Lock()
		conn.queuedCommands = append(conn.queuedCommands, value)
		conn.mu.Unlock()

		return RESPValue{
			Type: SimpleString,
			Str:  "QUEUED",
		}, cmd, args, true
	}

	return *value, "", nil, false
}

func (s *Server) handleMulti(conn *Connection, args []string) RESPValue {
	if len(args) > 0 {
		return RESPValue{
			Type: Error,
			Str:  "ERR wrong number of arguments for 'multi' command",
		}
	}

	// If already in transaction, ignore (Redis behavior)
	if conn.inTransaction {
		return RESPValue{
			Type: SimpleString,
			Str:  "OK",
		}
	}

	conn.mu.Lock()
	conn.inTransaction = true
	conn.queuedCommands = make([]*RESPValue, 0)
	conn.mu.Unlock()

	return RESPValue{
		Type: SimpleString,
		Str:  "OK",
	}
}

func (s *Server) handleExec(conn *Connection, args []string) RESPValue {
	if !conn.inTransaction {
		return RESPValue{
			Type: Error,
			Str:  "ERR EXEC without MULTI",
		}
	}

	// Check if transaction is dirty (watched keys changed)
	if conn.IsDirty() {
		conn.ResetTransaction()
		return RESPValue{
			Type:   BulkString,
			Str:    "",
			IsNull: true,
		}
	}

	// Get queued commands
	conn.mu.Lock()
	commands := conn.queuedCommands
	conn.queuedCommands = make([]*RESPValue, 0)
	conn.inTransaction = false
	conn.mu.Unlock()

	// Execute all commands
	results := make([]RESPValue, 0, len(commands))
	for _, cmd := range commands {
		// Execute command (without recursion)
		result := s.processCommand(cmd, "", nil)
		results = append(results, result)

		// If error occurs, continue (no rollback in Redis)
		// This is the Redis way
	}

	return RESPValue{
		Type:  Array,
		Array: results,
	}
}

func (s *Server) handleDiscard(conn *Connection, args []string) RESPValue {
	if !conn.inTransaction {
		return RESPValue{
			Type: Error,
			Str:  "ERR DISCARD without MULTI",
		}
	}

	conn.ResetTransaction()

	return RESPValue{
		Type: SimpleString,
		Str:  "OK",
	}
}

// WATCH key [key ...]
func (s *Server) handleWatch(conn *Connection, args []string) RESPValue {
	if len(args) < 1 {
		return RESPValue{
			Type: Error,
			Str:  "ERR wrong number of arguments for 'watch' command",
		}
	}
	// If in transaction, WATCH is ignored (Redis behavior)
	if conn.inTransaction {
		s.logger.Printf("🔍 WATCH ignored (in transaction)")
		return RESPValue{
			Type: SimpleString,
			Str:  "OK",
		}
	}

	// Add keys to watch list
	for _, key := range args {
		conn.WatchKey(key)
		s.addWatcher(key, conn)
		s.logger.Printf("🔍 Added watcher for key: %s", key)
	}

	return RESPValue{
		Type: SimpleString,
		Str:  "OK",
	}
}

// UNWATCH - clear all watched keys
func (s *Server) handleUnwatch(conn *Connection, args []string) RESPValue {
	conn.mu.Lock()
	conn.watchedKeys = make(map[string]bool)
	conn.mu.Unlock()

	return RESPValue{
		Type: SimpleString,
		Str:  "OK",
	}
}

// isWriteCommand checks if a command modifies data
func (s *Server) isWriteCommand(cmd string) bool {
	writeCommands := map[string]bool{
		"SET": true, "DEL": true, "HSET": true, "HDEL": true,
		"LPUSH": true, "RPUSH": true, "LPOP": true, "RPOP": true,
		"SADD": true, "SREM": true, "ZADD": true, "ZREM": true,
		"MSET": true, "MSETNX": true, "APPEND": true,
	}
	return writeCommands[cmd]
}

// extractKeys mengambil keys dari command
func (s *Server) extractKeys(cmd string, args []string) []string {
	switch cmd {
	case "SET", "DEL", "GET", "TTL", "EXISTS", "APPEND", "STRLEN":
		if len(args) > 0 {
			return []string{args[0]}
		}
	case "HSET", "HGET", "HDEL", "HLEN", "HEXISTS", "HKEYS", "HVALS":
		if len(args) > 0 {
			return []string{args[0]}
		}
	case "LPUSH", "RPUSH", "LPOP", "RPOP", "LLEN":
		if len(args) > 0 {
			return []string{args[0]}
		}
	case "SADD", "SREM", "SMEMBERS", "SISMEMBER", "SCARD":
		if len(args) > 0 {
			return []string{args[0]}
		}
	case "ZADD", "ZRANGE", "ZREM", "ZCARD", "ZSCORE":
		if len(args) > 0 {
			return []string{args[0]}
		}
	case "MGET", "MSET", "MSETNX":
		// Multiple keys
		keys := make([]string, 0)
		for i := 0; i < len(args); i += 2 {
			keys = append(keys, args[i])
		}
		return keys
	}
	return []string{}
}

// markWatchedKeysDirty menandai semua koneksi yang menonton key sebagai dirty
func (s *Server) markWatchedKeysDirty(keys []string) {
	if len(keys) == 0 {
		return
	}

	s.watcherMu.Lock()
	defer s.watcherMu.Unlock()

	for _, key := range keys {
		if watchers, exists := s.watchers[key]; exists {
			for _, conn := range watchers {
				conn.MarkDirty()
			}
			// Hapus watchers setelah ditandai (Redis behavior)
			delete(s.watchers, key)
		}
	}
}

// removes a connection from watchers untuk sejumlah keys
func (s *Server) removeWatcherByKeys(keys []string, conn *Connection) {
	for _, key := range keys {
		s.removeWatcher(key, conn)
	}
}

// removes a connection from watchers for a key
func (s *Server) removeWatcher(key string, conn *Connection) {
	s.watcherMu.Lock()
	defer s.watcherMu.Unlock()

	s.removeWatcherUnsafe(key, conn)
}

// removes a connection from watchers for a key tanpa lock
func (s *Server) removeWatcherUnsafe(key string, conn *Connection) {
	if watchers, exists := s.watchers[key]; exists {
		newWatchers := make([]*Connection, 0, len(watchers))
		for _, w := range watchers {
			if w != conn {
				newWatchers = append(newWatchers, w)
			}
		}
		if len(newWatchers) == 0 {
			delete(s.watchers, key)
		} else {
			s.watchers[key] = newWatchers
		}
	}
}

// adds a connection to watchers for a key
func (s *Server) addWatcher(key string, conn *Connection) {
	s.watcherMu.Lock()
	defer s.watcherMu.Unlock()

	// Remove existing watcher for this connection
	// Menggunakan unsafe karena sudah dilock di sini untuk mencegah deadlock
	s.removeWatcherUnsafe(key, conn)

	s.watchers[key] = append(s.watchers[key], conn)
}

// cleanupWatchers removes all watchers for a connection
func (s *Server) cleanupWatchers(conn *Connection) {
	conn.mu.Lock()
	keys := make([]string, 0, len(conn.watchedKeys))
	for key := range conn.watchedKeys {
		keys = append(keys, key)
	}
	conn.mu.Unlock()

	for _, key := range keys {
		s.removeWatcher(key, conn)
	}
}
