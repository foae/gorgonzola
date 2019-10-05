package dns

import (
	"sync"
)

// Blocklist defines the structure of an response registry.
type Blocklist struct {
	m map[string]int64
	l *sync.RWMutex
}

// NewBlocklist returns a new registry reference with
// all the items loaded from a given list.
func NewBlocklist(list []string) *Blocklist {
	w := &Blocklist{
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

func (b Blocklist) exists(domain string) bool {
	b.l.RLock()
	_, found := b.m[domain]
	b.l.RUnlock()

	return found
}

func (b Blocklist) load(list []string) {
	b.l.Lock()
	for _, d := range list {
		b.m[d]++
	}
	b.l.Unlock()
}

func (b Blocklist) store(domain string) {
	b.l.Lock()
	b.m[domain]++
	b.l.Unlock()
}

func (b Blocklist) retrieve(domain string) int64 {
	b.l.RLock()
	f := b.m[domain]
	b.l.RUnlock()

	return f
}

func (b Blocklist) remove(domain string) {
	b.l.Lock()
	delete(b.m, domain)
	b.l.Unlock()
}
