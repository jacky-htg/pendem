package engine

import "time"

type ValueType int

const (
	TypeString ValueType = iota
	TypeHash
	TypeList
	TypeSet
	TypeSortedSet
)

type Value struct {
	Type ValueType
	Data any
	TTL  time.Duration
	Exp  int64
}
