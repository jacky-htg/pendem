// internal/engine/list.go
package engine

import (
	"container/list"
	"sync"
)

type List struct {
	items *list.List
	mu    sync.RWMutex
}

func NewList() *List {
	return &List{
		items: list.New(),
	}
}

func (l *List) LPush(values ...string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, v := range values {
		l.items.PushFront(v)
	}
	return l.items.Len()
}

func (l *List) RPush(values ...string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, v := range values {
		l.items.PushBack(v)
	}
	return l.items.Len()
}

func (l *List) LPop() (string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	elem := l.items.Front()
	if elem == nil {
		return "", false
	}
	l.items.Remove(elem)
	return elem.Value.(string), true
}

func (l *List) RPop() (string, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	elem := l.items.Back()
	if elem == nil {
		return "", false
	}
	l.items.Remove(elem)
	return elem.Value.(string), true
}

func (l *List) LRange(start, stop int) []string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	length := l.items.Len()
	if length == 0 {
		return []string{}
	}

	// Handle negative indices
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
	if start > stop {
		return []string{}
	}

	result := make([]string, 0, stop-start+1)
	i := 0
	for elem := l.items.Front(); elem != nil; elem = elem.Next() {
		if i >= start && i <= stop {
			result = append(result, elem.Value.(string))
		}
		if i > stop {
			break
		}
		i++
	}
	return result
}

func (l *List) LLen() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.items.Len()
}
