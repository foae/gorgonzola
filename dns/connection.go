package dns

import (
	"fmt"
	"go.uber.org/zap"
	"log"
	"net"
	"sync"
)

type Conn struct {
	udpConn          *net.UDPConn
	logger           *zap.SugaredLogger
	m                *sync.RWMutex
	upstreamResolver *net.UDPAddr
}

type Config struct {
	Logger           *zap.SugaredLogger
	UpstreamResolver *net.UDPAddr
}

func NewUDPConn(localDNSPort int, upstreamResolver *net.UDPAddr) (*Conn, error) {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: localDNSPort})
	if err != nil {
		return nil, fmt.Errorf("dns: could not listen on port (%v) UDP: %v", localDNSPort, err)
	}

	log.Printf("dns: started UDP resolver on port (%v)", localDNSPort)
	return &Conn{
		udpConn:          conn,
		upstreamResolver: upstreamResolver,
		m:                &sync.RWMutex{},
	}, nil
}

func (c *Conn) ReadFromUDP(buf []byte) (int, *net.UDPAddr, error) {
	return c.udpConn.ReadFromUDP(buf)
}

func (c *Conn) Close() error {
	return c.udpConn.Close()
}

func (c *Conn) WithConfig(cfg Config) {
	if cfg.Logger != nil {
		c.logger = cfg.Logger
	}

	if cfg.UpstreamResolver != nil {
		c.m.Lock()
		c.upstreamResolver = cfg.UpstreamResolver
		c.m.Unlock()
	}
}
