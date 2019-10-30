package repository

import (
	"fmt"
	"github.com/foae/gorgonzola/dns"
	"time"
)

const (
	queriesTableName = "queries"
)

// Queryable defines the functionality needed to interact with this package.
type Queryable interface {
	Create(q *dns.Query) error
	Find(id uint16) (*dns.Query, error)
	FindAll() ([]*dns.Query, error)
	Update(q *dns.Query) error
	Delete(q *dns.Query) error
}

// Create creates (duh) a query resource and persists it in the repository.
func (r *Repo) Create(q *dns.Query) error {
	res, err := r.db.NamedExec(`
		INSERT INTO `+queriesTableName+` 
	(
		id, 
		uuid, 
		type,
		originator,
		originator_type,
		domain,
		root_domain,
		responded,
		response,
		blocked,
		valid,
		created_at
	) 
		VALUES 
	(
		:id, 
		:uuid, 
		:type,
		:originator,
		:originator_type,
		:domain,
		:root_domain,
		:responded,
		:response,
		:blocked,
		:valid,
		:created_at
	)
	`,
		q,
	)

	if err != nil {
		return fmt.Errorf("repo: create: %v", err)
	}

	rw, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("repo: create: rows affected: %v", err)
	}

	if rw != 1 {
		r.logger.Errorf("repo: create: expecting 1 row affected, got (%v)", rw)
	}

	return nil
}

// Find finds a resource by its id in the repository.
func (r *Repo) Find(id uint16) (*dns.Query, error) {
	qr := `
		SELECT * FROM queries 
		WHERE id = ?
		AND created_at >= ? 
		LIMIT 1
	`
	q := dns.Query{}
	if err := r.db.Get(&q, qr, id, time.Now().Add(time.Minute*time.Duration(-30))); err != nil {
		return nil, fmt.Errorf("repo: find (%v): %v", id, err)
	}

	return &q, nil
}

// Find returns all resources of type query persisted in the repository.
func (r *Repo) FindAll() ([]*dns.Query, error) {
	qs := make([]*dns.Query, 0)
	if err := r.db.Select(&qs, "SELECT * FROM queries ORDER BY created_at DESC"); err != nil {
		return nil, fmt.Errorf("repo: could not read all: %v", err)
	}

	return qs, nil
}

// Delete from the repository.
func (r *Repo) Delete(q *dns.Query) error {
	return nil
}

// Update a resource of type query in the repository.
func (r *Repo) Update(q *dns.Query) error {
	res, err := r.db.NamedExec(`
		UPDATE `+queriesTableName+` SET
			responded = :responded,
			response = :response,
			blocked = :blocked,
			valid = :valid,
			updated_at = :updated_at
		WHERE uuid = :uuid
	`,
		q,
	)
	if err != nil {
		return fmt.Errorf("repo: update: %v", err)
	}

	rw, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("repo: update: rows affected: %v", err)
	}

	if rw != 1 {
		r.logger.Errorf("repo: expecting 1 row affected, got (%v) for (%#v)", rw, q)
	}

	return nil
}
