package main

import (
	"strings"
	"sync"
)

// Registry defines the structure of an response registry.
type Registry struct {
	m map[string]int64
	l *sync.RWMutex
}

// NewWhitelist returns a new Registry reference.
func NewWhitelist() *Registry {
	return &Registry{
		l: &sync.RWMutex{},
		m: make(map[string]int64),
	}
}

// NewFrom returns a new registry reference with
// all the items loaded from a given list.
func NewFrom(list []string) *Registry {
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
	domain = strings.TrimSuffix(domain, ".")

	wr.l.RLock()
	defer wr.l.RUnlock()
	_, found := wr.m[domain]
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
	domain = strings.TrimSuffix(domain, ".")

	wr.l.Lock()
	wr.m[domain]++
	wr.l.Unlock()
}

func (wr Registry) retrieve(domain string) int64 {
	domain = strings.TrimSuffix(domain, ".")

	wr.l.RLock()
	defer wr.l.RUnlock()
	return wr.m[domain]
}

func (wr Registry) remove(domain string) {
	domain = strings.TrimSuffix(domain, ".")

	wr.l.Lock()
	delete(wr.m, domain)
	wr.l.Unlock()
}
