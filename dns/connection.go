package dns

import (
	"fmt"
	"github.com/foae/gorgonzola/internal"
	"log"
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
func NewUDPConn(localDNSPort int, upstreamResolver string) (*Conn, error) {
	if localDNSPort <= 0 {
		return nil, fmt.Errorf("dns: port must be greater than 0; passed (%v)", localDNSPort)
	}

	ip, err := toIP(upstreamResolver)
	if err != nil {
		return nil, err
	}
	up := &net.UDPAddr{Port: localDNSPort, IP: ip}

	conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: localDNSPort})
	if err != nil {
		return nil, fmt.Errorf("dns: could not listen on port (%v) UDP: %v", localDNSPort, err)
	}

	log.Printf("dns: started UDP resolver on port (%v)", localDNSPort)
	return &Conn{
		udpConn:          conn,
		upstreamResolver: up,
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
		ip, err := toIP(cfg.UpstreamResolver)
		if err != nil {
			c.logger.Errorf("dns: failed to parse upstream DNS address in new config: %v", err)
			return
		}

		c.m.Lock()
		c.upstreamResolver = &net.UDPAddr{IP: ip, Port: 53, Zone: ""}
		c.m.Unlock()
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

// toIP validates and parses a string-based IP address.
func toIP(in string) (net.IP, error) {
	ip := net.ParseIP(in)
	switch {
	case ip == nil:
		return nil, fmt.Errorf("dns: upstream dns (%v) is not a valid IP", in)
	case ip.To4() == nil:
		return nil, fmt.Errorf("dns: only IPv4 is supported at this time; passed (%v)", in)
	}

	return ip, nil
}
