package domain

import "sync"

type RecentCalls struct {
	mu    sync.Mutex
	cap   int
	calls []*Call
}

func NewRecentCalls(cap int) *RecentCalls {
	return &RecentCalls{cap: cap}
}

func (r *RecentCalls) Add(c *Call) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append([]*Call{c}, r.calls...)
	if len(r.calls) > r.cap {
		r.calls = r.calls[:r.cap]
	}
}

func (r *RecentCalls) Snapshot() []*Call {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*Call, len(r.calls))
	copy(out, r.calls)
	return out
}
