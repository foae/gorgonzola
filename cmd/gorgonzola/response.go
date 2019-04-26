package main

import (
	"net"
	"sync"
)

// ResponseRegistry defines the structure of an response registry.
type ResponseRegistry struct {
	m map[uint16]*net.UDPAddr
	l *sync.RWMutex
}

// NewResponseRegistry returns a new ResponseRegistry reference.
func NewResponseRegistry() *ResponseRegistry {
	return &ResponseRegistry{
		l: &sync.RWMutex{},
		m: make(map[uint16]*net.UDPAddr),
	}
}

func (rr ResponseRegistry) store(messageID uint16, addr *net.UDPAddr) {
	rr.l.Lock()
	rr.m[messageID] = addr
	rr.l.Unlock()
}

func (rr ResponseRegistry) retrieve(messageID uint16) *net.UDPAddr {
	rr.l.RLock()
	defer rr.l.RUnlock()
	f, found := rr.m[messageID]
	if !found {
		return nil
	}

	return f
}

func (rr ResponseRegistry) remove(messageID uint16) {
	rr.l.Lock()
	delete(rr.m, messageID)
	rr.l.Unlock()
}
