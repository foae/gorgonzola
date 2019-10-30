package dns

import (
	miek "github.com/miekg/dns"
	"time"
)

var (
	QueryTypeMap = map[uint16]QueryType{
		miek.TypeNone:   TypeNone,
		miek.TypeA:      TypeA,
		miek.TypeNS:     TypeNS,
		miek.TypeCNAME:  TypeCNAME,
		miek.TypeSOA:    TypeSOA,
		miek.TypePTR:    TypePTR,
		miek.TypeMX:     TypeMX,
		miek.TypeTXT:    TypeTXT,
		miek.TypeAAAA:   TypeAAAA,
		miek.TypeSRV:    TypeSRV,
		miek.TypeOPT:    TypeOPT,
		miek.TypeDNSKEY: TypeDNSKEY,
		miek.TypeSPF:    TypeSPF,
	}
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
)

type QueryType string

type Query struct {
	ID             int64      `json:"id" db:"id"`
	OriginatorType int        `json:"originatorType" db:"originator_type"`
	UUID           string     `json:"uuid" db:"uuid"`
	Type           QueryType  `json:"type" db:"type"`
	Originator     string     `json:"originator" db:"originator"`
	Domain         string     `json:"domain" db:"domain"`
	RootDomain     string     `json:"rootDomain" db:"root_domain"`
	Response       string     `json:"response,omitempty" db:"response,omitempty"`
	Responded      bool       `json:"responded" db:"responded"`
	Blocked        bool       `json:"blocked" db:"blocked"`
	Valid          bool       `json:"valid" db:"valid"`
	CreatedAt      time.Time  `json:"createdAt" db:"created_at"`
	UpdatedAt      *time.Time `json:"updatedAt,omitempty" db:"updated_at,omitempty"`
}
