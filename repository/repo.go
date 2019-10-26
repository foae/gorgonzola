package repository

import (
	"encoding/base64"
	"fmt"
	"github.com/jmoiron/sqlx"
	_ "github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
	"io"
	"net/http"
	"net/url"
	"os"
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

const (
	sqliteFilePath  = "./repository/_db/db.sqlite3"
	fileStoragePath = "./repository/file_storage"
	fileSeparator   = "/"
)

type Interactor interface {
	Close() error
	Queryable
	StoredFilesList() ([]string, error)
	DownloadFromURL(someURL string) error
}

type Config struct {
	Logger          *zap.SugaredLogger
	FileStoragePath string
	SqliteFilePath  string
}

type Repo struct {
	db              *sqlx.DB
	logger          *zap.SugaredLogger
	fileStoragePath string
	sqliteFilePath  string
}

func NewRepo(cfg Config) (*Repo, error) {
	if cfg.SqliteFilePath == "" {
		cfg.SqliteFilePath = sqliteFilePath
	}
	if cfg.FileStoragePath == "" {
		cfg.FileStoragePath = fileStoragePath
	}

	db, err := sqlx.Open("sqlite3", cfg.SqliteFilePath)
	if err != nil {
		return nil, fmt.Errorf("repo: could not open Repository file (%v): %v", cfg.SqliteFilePath, err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("repo: could not ping Repository (%v): %v", cfg.SqliteFilePath, err)
	}

	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("repo: could not create schema in Repository (%v): %v", cfg.SqliteFilePath, err)
	}

	return &Repo{
		db:              db,
		logger:          cfg.Logger,
		fileStoragePath: cfg.FileStoragePath,
		sqliteFilePath:  cfg.SqliteFilePath,
	}, nil
}

func (r *Repo) DownloadFromURL(someURL string) error {
	uurl, err := url.Parse(someURL)
	if err != nil {
		return err
	}

	r.logger.Debugf("Downloading: %v", someURL)
	resp, err := http.Get(uurl.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	fileName := base64.StdEncoding.EncodeToString([]byte(someURL))
	fileHandler, err := os.Create(r.fileStoragePath + fileSeparator + fileName)
	if err != nil {
		return fmt.Errorf("repo: could not create file name (%v): %v", fileName, err)
	}
	defer fileHandler.Close()

	if _, err := io.Copy(fileHandler, resp.Body); err != nil {
		return err
	}

	if err := fileHandler.Sync(); err != nil {
		return err
	}

	r.logger.Debugf("Wrote contents of (%v) to file (%v/%v)", someURL, r.fileStoragePath, fileName)
	return nil
}

// StoredFilesList reads all files from the local file storage directory,
// returning the full path of each file found.
func (r *Repo) StoredFilesList() ([]string, error) {
	fileHandler, err := os.Open(r.fileStoragePath)
	if err != nil {
		return nil, fmt.Errorf("repo: could not open (%v): %v", r.fileStoragePath, err)
	}

	rawFileList, err := fileHandler.Readdirnames(0)
	if err != nil {
		return nil, fmt.Errorf("repo: could not read in (%v): %v", r.fileStoragePath, err)
	}
	_ = fileHandler.Close()

	fileList := make([]string, 0, len(rawFileList))
	for _, f := range rawFileList {
		fileList = append(fileList, r.fileStoragePath+fileSeparator+f)
	}

	return fileList, nil
}

func (r *Repo) Close() error {
	return r.db.Close()
}
