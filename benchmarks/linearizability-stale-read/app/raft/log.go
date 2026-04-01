package raft

import "sync"

// Entry is a single log entry in the replicated log.
type Entry struct {
	Index uint64
	Term  uint64
	Key   string
	Value string
}

// Log is an append-only replicated log.
type Log struct {
	mu      sync.RWMutex
	entries []Entry
}

func NewLog() *Log {
	return &Log{}
}

func (l *Log) Append(e Entry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e.Index = uint64(len(l.entries)) + 1
	l.entries = append(l.entries, e)
}

func (l *Log) Get(index uint64) (Entry, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if index < 1 || index > uint64(len(l.entries)) {
		return Entry{}, false
	}
	return l.entries[index-1], true
}

func (l *Log) LastIndex() uint64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return uint64(len(l.entries))
}

// Entries returns all entries from startIndex (inclusive) onward.
func (l *Log) Entries(startIndex uint64) []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if startIndex < 1 || startIndex > uint64(len(l.entries)) {
		return nil
	}
	result := make([]Entry, len(l.entries[startIndex-1:]))
	copy(result, l.entries[startIndex-1:])
	return result
}
