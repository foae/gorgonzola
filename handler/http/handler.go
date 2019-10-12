package http

import (
	"github.com/dgraph-io/badger"
	"go.uber.org/zap"
)

// Handler defines the structure of a handler.
type Handler struct {
	logger *zap.SugaredLogger
	db     *badger.DB
}

// Config defines the structure of a handler config.
type Config struct {
	Logger *zap.SugaredLogger
	DB     *badger.DB
}

// New returns a new instance of a handler
// based on a given configuration.
func New(cfg Config) *Handler {
	return &Handler{
		logger: cfg.Logger,
		db:     cfg.DB,
	}
}
