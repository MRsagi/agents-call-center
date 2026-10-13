package domain

import "sync"

type Queue struct {
	mu    sync.Mutex
	order []string
	byID  map[string]*Call
}

func NewQueue() *Queue {
	return &Queue{byID: make(map[string]*Call)}
}

func (q *Queue) Enqueue(c *Call) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if _, ok := q.byID[c.ID]; ok {
		return nil
	}
	q.byID[c.ID] = c
	q.order = append(q.order, c.ID)
	return nil
}

func (q *Queue) Dequeue() (*Call, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.order) == 0 {
		return nil, false
	}
	id := q.order[0]
	q.order = q.order[1:]
	c := q.byID[id]
	return c, true
}

func (q *Queue) Get(id string) (*Call, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	c, ok := q.byID[id]
	return c, ok
}

func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.order)
}

func (q *Queue) Snapshot() []*Call {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]*Call, 0, len(q.order))
	for _, id := range q.order {
		out = append(out, q.byID[id])
	}
	return out
}
