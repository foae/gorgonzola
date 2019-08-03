package main

import (
	"sync"
)

// Registry defines the structure of an response registry.
type Registry struct {
	m map[string]int64
	l *sync.RWMutex
}

// NewRegistry returns a new registry reference with
// all the items loaded from a given list.
func NewRegistry(list []string) *Registry {
	w := &Registry{
		l: &sync.RWMutex{},
		m: make(map[string]int64),
	}

	w.l.Lock()
	for _, d := range list {
		w.m[d]++
	}
	w.l.Unlock()

	return w
}

func (wr Registry) exists(domain string) bool {
	wr.l.RLock()
	_, found := wr.m[domain]
	wr.l.RUnlock()

	return found
}

func (wr Registry) load(list []string) {
	wr.l.Lock()
	for _, d := range list {
		wr.m[d]++
	}
	wr.l.Unlock()
}

func (wr Registry) store(domain string) {
	wr.l.Lock()
	wr.m[domain]++
	wr.l.Unlock()
}

func (wr Registry) retrieve(domain string) int64 {
	wr.l.RLock()
	f := wr.m[domain]
	wr.l.RUnlock()

	return f
}

func (wr Registry) remove(domain string) {
	wr.l.Lock()
	delete(wr.m, domain)
	wr.l.Unlock()
}
