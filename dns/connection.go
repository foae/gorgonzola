package dns

import (
	"errors"
	"fmt"
	"github.com/foae/gorgonzola/internal"
	"go.uber.org/zap"
	"net"
	"sync"
)

// Conn defines the structure of a
// connection along its dependencies.
type Conn struct {
	udpConn          *net.UDPConn
	logger           internal.Logger
	m                *sync.RWMutex
	upstreamResolver *net.UDPAddr
}

// Config describes the configurable dependencies
// of a connection through this Config.
type Config struct {
	Logger           internal.Logger
	UpstreamResolver string
}

// NewUDPConn returns an instantiated connection based on the provided dependencies.
func NewUDPConn(localDnsServerAddr string, upstreamResolver string, logger *zap.SugaredLogger) (*Conn, error) {
	if localDnsServerAddr == "" {
		return nil, errors.New("dns: passed an empty local DNS server address")
	}
	if upstreamResolver == "" {
		return nil, errors.New("dns: passed an empty upstream DNS server address")
	}

	localAddr, err := net.ResolveUDPAddr("udp", localDnsServerAddr)
	switch {
	case err != nil:
		return nil, fmt.Errorf("dns: local dns address (%v) is not valid: %v", localAddr, err)
	case localAddr.Port == 0:
		localAddr.Port = 53
	}

	upRes, err := net.ResolveUDPAddr("udp", upstreamResolver)
	switch {
	case err != nil:
		return nil, fmt.Errorf("dns: upstream dns address (%v) is not valid: %v", upRes, err)
	case upRes.Port == 0:
		upRes.Port = 53
	}

	conn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		return nil, fmt.Errorf("dns: could not listen on address (%v) UDP: %v", localDnsServerAddr, err)
	}

	logger.Infof("Started local DNS forwarder on address (%v)", localAddr)
	logger.Infof("Registered upstream DNS resolver on address (%v)", upRes)
	return &Conn{
		udpConn:          conn,
		upstreamResolver: upRes,
		logger:           logger,
		m:                &sync.RWMutex{},
	}, nil
}

// WithConfig amends the instantiated connection
// and replaces specific or all dependencies.
func (c *Conn) WithConfig(cfg Config) {
	if cfg.Logger != nil {
		c.logger = cfg.Logger
	}

	if cfg.UpstreamResolver != "" {
		upRes, err := net.ResolveUDPAddr("udp", cfg.UpstreamResolver)
		if err != nil {
			c.logger.Errorf("dns: failed to parse upstream DNS address in new config: %v", err)
			return
		}

		c.m.Lock()
		c.upstreamResolver = upRes
		c.m.Unlock()
		c.logger.Infof("Registered upstream DNS resolver on address (%v)", upRes)
	}
}

// ReadFromUDP is a wrapper for the underlying udp
// connection to read incoming bytes via udp.
func (c *Conn) ReadFromUDP(buf []byte) (int, *net.UDPAddr, error) {
	return c.udpConn.ReadFromUDP(buf)
}

// Close closes the underlying udp connection.
func (c *Conn) Close() error {
	return c.udpConn.Close()
}
