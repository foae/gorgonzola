package dns

import (
	"fmt"
	"github.com/dgraph-io/badger"
	"go.uber.org/zap"
	"log"
	"net"
	"sync"
)

type Conn struct {
	udpConn          *net.UDPConn
	db               *badger.DB
	logger           *zap.SugaredLogger
	m                *sync.RWMutex
	upstreamResolver *net.UDPAddr
}

type Config struct {
	DB               *badger.DB
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

func (c *Conn) Close() error {
	var errs []error
	if err := c.udpConn.Close(); err != nil {
		errs = append(errs, fmt.Errorf("dns: err closing UDP conn: %v", err))
	}

	if err := c.logger.Sync(); err != nil {
		errs = append(errs, fmt.Errorf("dns: err closing logger: %v", err))
	}

	if err := c.db.Close(); err != nil {
		errs = append(errs, fmt.Errorf("dns: err closing db: %v", err))
	}

	var e error
	for _, err := range errs {
		e = fmt.Errorf("%v; ", err)
	}

	return e
}

func (c *Conn) WithConfig(cfg Config) {
	if cfg.DB != nil {
		c.db = cfg.DB
	}

	if cfg.Logger != nil {
		c.logger = cfg.Logger
	}

	if cfg.UpstreamResolver != nil {
		c.m.Lock()
		c.upstreamResolver = cfg.UpstreamResolver
		c.m.Unlock()
	}
}
