package repository

import (
	"fmt"
	"github.com/jmoiron/sqlx"
	_ "github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

var schema = `
CREATE TABLE IF NOT EXISTS queries
(
	id bigint NOT NULL,
	uuid char(36) NOT NULL,
	type varchar(12) NOT NULL,
	originator char(15) NOT NULL,
	originator_type tinyint NOT NULL,
	domain text NOT NULL,
	root_domain varchar(255) NOT NULL,
	responded tinyint NOT NULL,
	response text NULL,
	blocked tinyint NOT NULL,
	valid tinyint NOT NULL,
	created_at datetime NOT NULL,
	updated_at datetime NULL
);

CREATE INDEX IF NOT EXISTS queries_uuid_idx ON queries (uuid);
CREATE INDEX IF NOT EXISTS queries_created_at_idx ON queries (created_at);
`

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
	dbFile := "./repository/_db/db.sqlite3"
	db, err := sqlx.Open("sqlite3", dbFile)
	if err != nil {
		return nil, fmt.Errorf("could not open Repository file (%v): %v", dbFile, err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("could not ping Repository (%v): %v", dbFile, err)
	}

	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("could not create schema in Repository (%v): %v", dbFile, err)
	}

	return &Repo{db: db, logger: cfg.Logger}, nil
}

func (r *Repo) Close() error {
	return r.db.Close()
}
