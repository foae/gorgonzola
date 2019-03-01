package main

import (
	"golang.org/x/net/dns/dnsmessage"
	"log"
)

func block(m dnsmessage.Message) dnsmessage.Message {
	c := m
	c.Answers = make([]dnsmessage.Resource, 1)

	for _, q := range m.Questions {

		c.Answers[0].Header = dnsmessage.ResourceHeader{
			Name:   q.Name,
			Class:  q.Class,
			Type:   q.Type,
			Length: 4,
		}

		switch q.Type {
		case dnsmessage.TypeA:
			c.Answers[0].Body = &dnsmessage.AResource{
				A: [4]byte{127, 0, 0, 1},
			}
		case dnsmessage.TypeAAAA:
			c.Answers[0].Body = &dnsmessage.AAAAResource{
				AAAA: [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
			}
		case dnsmessage.TypeMX:
			c.Answers[0].Body = &dnsmessage.MXResource{
				MX: dnsmessage.Name{
					Length: uint8(len("m.com.")),
					Data:   [255]byte{'m', '.', 'c', 'o', 'm', '.'},
				},
				Pref: uint16(0),
			}
		case dnsmessage.TypeCNAME:
			c.Answers[0].Body = &dnsmessage.CNAMEResource{
				CNAME: dnsmessage.Name{
					Length: uint8(len("m.com.")),
					Data:   [255]byte{'m', '.', 'c', 'o', 'm', '.'},
				},
			}
		case dnsmessage.TypePTR:
			c.Answers[0].Body = &dnsmessage.PTRResource{
				PTR: dnsmessage.Name{
					Length: uint8(len("m.com.")),
					Data:   [255]byte{'m', '.', 'c', 'o', 'm', '.'},
				},
			}
		default:
			log.Printf("Cannot handle dns message type: %v", q.GoString())
			break
		}

		break
	}

	return c
}
