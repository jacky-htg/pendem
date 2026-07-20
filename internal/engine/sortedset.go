// internal/engine/sortedset.go
package engine

import (
	"sort"
	"sync"
)

type SortedSet struct {
	members map[string]float64
	mu      sync.RWMutex
}

type SortedSetMember struct {
	Member string
	Score  float64
}

func NewSortedSet() *SortedSet {
	return &SortedSet{
		members: make(map[string]float64),
	}
}

func (ss *SortedSet) Add(score float64, member string) bool {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	_, exists := ss.members[member]
	ss.members[member] = score
	return !exists
}

func (ss *SortedSet) Remove(member string) bool {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	if _, exists := ss.members[member]; exists {
		delete(ss.members, member)
		return true
	}
	return false
}

func (ss *SortedSet) Range(start, stop int) []SortedSetMember {
	ss.mu.RLock()
	defer ss.mu.RUnlock()

	// Collect and sort by score
	members := make([]SortedSetMember, 0, len(ss.members))
	for m, s := range ss.members {
		members = append(members, SortedSetMember{Member: m, Score: s})
	}

	sort.Slice(members, func(i, j int) bool {
		return members[i].Score < members[j].Score
	})

	// Handle negative indices
	length := len(members)
	if start < 0 {
		start = length + start
	}
	if stop < 0 {
		stop = length + stop
	}
	if start < 0 {
		start = 0
	}
	if stop >= length {
		stop = length - 1
	}
	if start > stop || start >= length {
		return []SortedSetMember{}
	}

	return members[start : stop+1]
}

func (ss *SortedSet) Len() int {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	return len(ss.members)
}

func (ss *SortedSet) Score(member string) (float64, bool) {
	ss.mu.RLock()
	defer ss.mu.RUnlock()
	score, exists := ss.members[member]
	return score, exists
}
