package repository

import (
	"encoding/base64"
	"fmt"
	"github.com/foae/gorgonzola/internal"
	"github.com/jmoiron/sqlx"
	_ "github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
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
	sqliteFilePath      = "./repository/_db/db.sqlite3"
	sqliteDirectoryPath = "./repository/_db"
	fileDirectoryPath   = "./repository/file_storage"
	fileSeparator       = "/"
)

// Interactor defines the needed functionality to interact with this package.
type Interactor interface {
	Close() error
	Queryable
	StoredFilesList(withPath bool) (storedFiles []string, err error)
	DownloadFromURL(someURL string) (storedFile string, err error)
}

// Config defines the configurable dependencies.
type Config struct {
	Logger          internal.Logger
	FileStoragePath string
	SqliteFilePath  string
}

// Repo defines the structure of the data layer.
type Repo struct {
	db              *sqlx.DB
	logger          internal.Logger
	fileStoragePath string
	sqliteFilePath  string
}

// NewRepo returns a new instance of the data layer handler.
func NewRepo(cfg Config) (*Repo, error) {
	if cfg.SqliteFilePath == "" {
		cfg.SqliteFilePath = sqliteFilePath
	}
	if cfg.FileStoragePath == "" {
		cfg.FileStoragePath = fileDirectoryPath
	}

	/*
		Create the folders for file storage:
		* SQLite
		* file storage (local cache)
	*/
	if err := os.MkdirAll(sqliteDirectoryPath, os.ModePerm); err != nil {
		return nil, fmt.Errorf("repo: could not create directories in path (%v): %v", sqliteDirectoryPath, err)
	}
	if err := os.MkdirAll(fileDirectoryPath, os.ModePerm); err != nil {
		return nil, fmt.Errorf("repo: could not create directories in path (%v): %v", fileDirectoryPath, err)
	}

	/*
		Setup the SQLite DB.
	*/
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

// DownloadFromURL parses and validates an HTTP URL and downloads the remote contents.
// It is used to download and create a local cache of AdBlock Plus / Domains lists and other formats.
func (r *Repo) DownloadFromURL(someURL string) (string, error) {
	uurl, err := url.Parse(someURL)
	if err != nil {
		return "", err
	}

	r.logger.Debugf("Downloading: %v", someURL)
	resp, err := http.Get(uurl.String())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	fileName := base64.StdEncoding.EncodeToString([]byte(someURL))
	fileWithPath := r.fileStoragePath + fileSeparator + fileName
	fileHandler, err := os.Create(fileWithPath)
	if err != nil {
		return "", fmt.Errorf("repo: could not create file name (%v): %v", fileName, err)
	}
	defer fileHandler.Close() // nolint

	if _, err := io.Copy(fileHandler, resp.Body); err != nil {
		return "", err
	}

	if err := fileHandler.Sync(); err != nil {
		return "", err
	}

	r.logger.Debugf("Wrote contents of (%v) to file (%v/%v)", someURL, r.fileStoragePath, fileName)
	return fileWithPath, nil
}

// StoredFilesList reads all files from the local file storage directory,
// returning the full path of each file found.
// Only the file names are base64 encoded while the paths are raw.
func (r *Repo) StoredFilesList(withPath bool) ([]string, error) {
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
		switch withPath {
		case true:
			fileList = append(fileList, r.fileStoragePath+fileSeparator+f)
		default:
			fileList = append(fileList, f)
		}
	}

	return fileList, nil
}

// Close implements the Closer interface.
func (r *Repo) Close() error {
	return r.db.Close()
}
