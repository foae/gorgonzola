package repository

import (
	"fmt"
	"github.com/miekg/dns"
	uuid "github.com/satori/go.uuid"
	"log"
	"net"
	"strings"
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

func NewQuery(req net.UDPAddr, msg dns.Msg) *Query {
	q := &Query{
		ID:         int64(msg.Id),
		UUID:       uuid.NewV4().String(),
		Originator: req.IP.String(),
		OriginatorType: func() int {
			if req.IP.To4() != nil {
				return 4
			}
			return 6
		}(),
		Response:  "",
		Responded: false,
		Blocked:   false,
		Valid:     true,
		CreatedAt: time.Now(),
		UpdatedAt: nil,
	}

	if len(msg.Question) == 0 {
		q.Valid = false
		log.Printf("repository: msg question not valid: 0 length: %v", msg.Question)
		return q
	}

	qt, ok := QueryTypeMap[msg.Question[0].Qtype]
	if !ok {
		q.Valid = false
		log.Printf("repository: query type not mapped: got (%#v)", msg.Question)
		return q
	}
	q.Type = qt

	if msg.MsgHdr.Response {
		if len(msg.Answer) > 0 {
			// TODO: handle multiple answers
			q.Response = strings.TrimSuffix(msg.Answer[0].Header().Name, ".")
		}
	}

	domain := strings.TrimSuffix(msg.Question[0].Name, ".")
	rootDomain := func() string {
		s := strings.Split(domain, ".")
		if len(s) <= 2 {
			// No subdomain requested.
			return domain
		}

		return s[len(s)-2] + "." + s[len(s)-1]
	}()

	q.Domain = domain
	q.RootDomain = rootDomain

	return q
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
