package repository

import (
	"fmt"
	"github.com/jmoiron/sqlx"
	_ "github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

type Interactor interface {
	Close() error
	Queryable
}

type Config struct {
	Logger *zap.SugaredLogger
}

type Repo struct {
	db     *sqlx.DB
	logger *zap.SugaredLogger
}

func NewRepo(cfg Config) (*Repo, error) {
	dbFile := "./repository/_db/db"
	db, err := sqlx.Open("sqlite3", dbFile)
	if err != nil {
		return nil, fmt.Errorf("could not open DB file (%v): %v", dbFile, err)
	}

	return &Repo{db: db, logger: cfg.Logger}, nil
}

func (r *Repo) Close() error {
	return r.db.Close()
}
