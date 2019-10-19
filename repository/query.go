package repository

import (
	"fmt"
	"github.com/miekg/dns"
	"time"
)

const (
	TypeNone   QueryType = "None"
	TypeA      QueryType = "A"
	TypeNS     QueryType = "NS"
	TypeCNAME  QueryType = "CNAME"
	TypeSOA    QueryType = "SOA"
	TypePTR    QueryType = "PTR"
	TypeMX     QueryType = "MX"
	TypeTXT    QueryType = "TXT"
	TypeAAAA   QueryType = "AAAA"
	TypeSRV    QueryType = "SRV"
	TypeOPT    QueryType = "OPT"
	TypeDNSKEY QueryType = "DNSKEY"
	TypeSPF    QueryType = "SPF"

	queriesTableName = "queries"
)

type Queryable interface {
	Create(q *Query) error
	Find(id uint16) (*Query, error)
	FindAll() ([]*Query, error)
	Update(q *Query) error
	Delete(q *Query) error
}

var (
	QueryTypeMap = map[uint16]QueryType{
		dns.TypeNone:   TypeNone,
		dns.TypeA:      TypeA,
		dns.TypeNS:     TypeNS,
		dns.TypeCNAME:  TypeCNAME,
		dns.TypeSOA:    TypeSOA,
		dns.TypePTR:    TypePTR,
		dns.TypeMX:     TypeMX,
		dns.TypeTXT:    TypeTXT,
		dns.TypeAAAA:   TypeAAAA,
		dns.TypeSRV:    TypeSRV,
		dns.TypeOPT:    TypeOPT,
		dns.TypeDNSKEY: TypeDNSKEY,
		dns.TypeSPF:    TypeSPF,
	}
)

type QueryType string

type Query struct {
	ID             int64      `json:"id" db:"id"`
	UUID           string     `json:"uuid" db:"uuid"`
	Type           QueryType  `json:"type" db:"type"`
	Originator     string     `json:"originator" db:"originator"`
	OriginatorType int        `json:"originator_type" db:"originator_type"`
	Domain         string     `json:"domain" db:"domain"`
	RootDomain     string     `json:"rootDomain" db:"root_domain"`
	Responded      bool       `json:"responded" db:"responded"`
	Response       string     `json:"response,omitempty" db:"response,omitempty"`
	Blocked        bool       `json:"blocked" db:"blocked"`
	Valid          bool       `json:"valid" db:"valid"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      *time.Time `json:"updated_at,omitempty" db:"updated_at,omitempty"`
}

func (r *Repo) Create(q *Query) error {
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

func (r *Repo) Find(id uint16) (*Query, error) {
	qr := `
		SELECT * FROM queries 
		WHERE id = ?
		AND created_at >= ? 
		LIMIT 1
	`
	q := Query{}
	if err := r.db.Get(&q, qr, id, time.Now().Add(time.Minute*time.Duration(-30))); err != nil {
		return nil, fmt.Errorf("repo: find (%v): %v", id, err)
	}

	return &q, nil
}

func (r *Repo) FindAll() ([]*Query, error) {
	qs := make([]*Query, 0)
	if err := r.db.Select(&qs, "SELECT * FROM queries ORDER BY created_at DESC"); err != nil {
		return nil, fmt.Errorf("repo: could not read all: %v", err)
	}

	return qs, nil
}

func (r *Repo) Delete(q *Query) error {
	return nil
}

func (r *Repo) Update(q *Query) error {
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
