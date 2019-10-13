package repository

import (
	"github.com/miekg/dns"
	uuid "github.com/satori/go.uuid"
	"log"
	"net"
	"strings"
	"time"
)

type Queryable interface {
	Create(q *Query) error
	Read(id uint16) (*Query, error)
	Update(q *Query) error
	Delete(q *Query) error
}

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
)

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
	ID             uint16     `json:"id" db:"id"`
	UUID           string     `json:"uuid" db:"uuid"`
	Type           QueryType  `json:"type" db:"type"`
	Originator     string     `json:"originator" db:"originator"`
	OriginatorType uint8      `json:"originatorType" db:"originator_type"`
	Domain         string     `json:"domain" db:"domain"`
	RootDomain     string     `json:"rootDomain" db:"root_domain"`
	Responded      bool       `json:"responded" db:"responded"`
	Blocked        bool       `json:"blocked" db:"blocked"`
	Valid          bool       `json:"valid" db:"valid"`
	CreatedAt      time.Time  `json:"createdAt" db:"created_at"`
	UpdatedAt      *time.Time `json:"updatedAt,omitempty" db:"updated_at,omitempty"`
}

func NewQuery(req net.UDPAddr, msg dns.Msg) *Query {
	q := &Query{
		ID:         msg.Id,
		UUID:       uuid.NewV4().String(),
		Originator: req.IP.String(),
		OriginatorType: func() uint8 {
			if req.IP.To4() != nil {
				return 4
			}
			return 6
		}(),
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
	return nil
}

func (r *Repo) Read(id uint16) (*Query, error) {
	return &Query{}, nil
}

func (r *Repo) Delete(q *Query) error {
	return nil
}

func (r *Repo) Update(q *Query) error {
	return nil
}
