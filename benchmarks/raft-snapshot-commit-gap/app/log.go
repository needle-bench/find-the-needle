package main

import "fmt"

// LogEntry represents a single entry in the Raft log.
type LogEntry struct {
	Index   int
	Term    int
	Command string
}

// RaftLog manages the in-memory log of entries.
// Log indexing follows the Raft paper convention: indices start at 1.
// Index 0 is reserved as a sentinel (empty/initial state).
type RaftLog struct {
	entries []LogEntry
	// offset tracks the first real index in entries[].
	// After compaction, entries before offset are discarded.
	offset int
}

func NewRaftLog() *RaftLog {
	return &RaftLog{
		entries: []LogEntry{{Index: 0, Term: 0, Command: ""}}, // sentinel
		offset:  0,
	}
}

// Append adds an entry to the log and returns its index.
func (l *RaftLog) Append(term int, command string) int {
	idx := l.LastIndex() + 1
	l.entries = append(l.entries, LogEntry{
		Index:   idx,
		Term:    term,
		Command: command,
	})
	return idx
}

// Get returns the entry at the given index, or an error if out of range.
func (l *RaftLog) Get(index int) (LogEntry, error) {
	pos := index - l.offset
	if pos < 0 || pos >= len(l.entries) {
		return LogEntry{}, fmt.Errorf("log entry %d not found (offset=%d, len=%d)", index, l.offset, len(l.entries))
	}
	return l.entries[pos], nil
}

// LastIndex returns the index of the last log entry.
func (l *RaftLog) LastIndex() int {
	if len(l.entries) == 0 {
		return l.offset - 1
	}
	return l.entries[len(l.entries)-1].Index
}

// LastTerm returns the term of the last log entry.
func (l *RaftLog) LastTerm() int {
	if len(l.entries) == 0 {
		return 0
	}
	return l.entries[len(l.entries)-1].Term
}

// Slice returns entries from startIndex to endIndex (inclusive).
func (l *RaftLog) Slice(startIndex, endIndex int) []LogEntry {
	startPos := startIndex - l.offset
	endPos := endIndex - l.offset + 1
	if startPos < 0 {
		startPos = 0
	}
	if endPos > len(l.entries) {
		endPos = len(l.entries)
	}
	if startPos >= endPos {
		return nil
	}
	result := make([]LogEntry, endPos-startPos)
	copy(result, l.entries[startPos:endPos])
	return result
}

// TruncateAfter removes all entries after the given index.
func (l *RaftLog) TruncateAfter(index int) {
	pos := index - l.offset + 1
	if pos < 0 {
		pos = 0
	}
	if pos < len(l.entries) {
		l.entries = l.entries[:pos]
	}
}

// CompactUpTo discards log entries that are strictly before the given index.
// The entry at compactIndex is retained as the new base (it becomes the
// snapshot boundary marker). This uses strict less-than (<) because the
// entry at compactIndex itself serves as the snapshot's last-included-entry
// and must remain accessible for AppendEntries consistency checks per
// section 7 of the Raft paper (log matching property at boundary).
func (l *RaftLog) CompactUpTo(compactIndex int) {
	pos := compactIndex - l.offset
	if pos <= 0 || pos >= len(l.entries) {
		return
	}
	// Keep entries[pos:] — the entry AT compactIndex is preserved.
	// This looks like an off-by-one (shouldn't we use pos+1?) but it's
	// correct: the Raft paper requires the snapshot boundary entry to
	// remain for consistency checking in AppendEntries RPCs.
	l.entries = l.entries[pos:]
	l.offset = compactIndex
}

// Len returns the number of entries (including sentinel if present).
func (l *RaftLog) Len() int {
	return len(l.entries)
}

// ReplaceFrom replaces the log starting at the given index with new entries.
// Used when a follower receives entries from a leader that conflict with its log.
func (l *RaftLog) ReplaceFrom(index int, entries []LogEntry) {
	l.TruncateAfter(index - 1)
	for _, e := range entries {
		l.entries = append(l.entries, e)
	}
}
