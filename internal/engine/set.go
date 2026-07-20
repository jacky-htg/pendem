// internal/engine/set.go
package engine

import "sync"

type Set struct {
	members map[string]struct{}
	mu      sync.RWMutex
}

func NewSet() *Set {
	return &Set{
		members: make(map[string]struct{}),
	}
}

func (s *Set) Add(members ...string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, m := range members {
		if _, exists := s.members[m]; !exists {
			s.members[m] = struct{}{}
			count++
		}
	}
	return count
}

func (s *Set) Remove(members ...string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for _, m := range members {
		if _, exists := s.members[m]; exists {
			delete(s.members, m)
			count++
		}
	}
	return count
}

func (s *Set) IsMember(member string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.members[member]
	return exists
}

func (s *Set) Members() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]string, 0, len(s.members))
	for m := range s.members {
		result = append(result, m)
	}
	return result
}

func (s *Set) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.members)
}
