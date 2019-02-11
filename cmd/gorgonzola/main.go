package main

import (
	"golang.org/x/net/dns/dnsmessage"
	"log"
	"net"
	"os"
	"strconv"
)

func main() {
	upstreamDNS := mustGetEnv("UPSTREAM_DNS_SERVER_IP")
	localDNSPort := mustGetEnvInt("DNS_LISTEN_PORT")

	// Fire up the UDP listener.
	conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: localDNSPort})
	if err != nil {
		log.Fatalf("could not listen on port 53 UDP: %v", err)
	}
	defer conn.Close()

	// Load the whitelist
	whitelist := NewFrom([]string{"google.com", "cloudflare.com"})

	// Load the blacklist
	blacklist := NewFrom([]string{"ads.google.com", "ad.google.com"})

	// Keep track of all UDP messages
	// that need to be reconciled.
	respMap := NewResponseMap()

	log.Println("Waiting for UDP connections...")
	for {
		buf := make([]byte, 512)
		_, udpAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Fatalf("could not read from UDP connection: %v", err)
		}
		log.Printf("Received UDP message from: %v", udpAddr.String())

		// Unpack and validate the received message.
		var m dnsmessage.Message
		if err := m.Unpack(buf); err != nil {
			log.Fatalf("could not unpack `buf` into a dnsmessage.Message: %v", err)
		}
		//log.Println("----------------------------------------------")
		//log.Printf("unpacked dnsmessage.Message: %v", m.GoString())

		switch m.Header.Response {
		// This a new request that hasn't been resolved.
		case false:
			// Check if it's in the whitelist
			var isWhitelisted bool
			for _, q := range m.Questions {
				if whitelist.exists(q.Name.String()) {
					isWhitelisted = true
				}
			}
			_ = isWhitelisted

			// Check if it's in the blacklist
			var isBlacklisted bool
			for _, q := range m.Questions {
				if blacklist.exists(q.Name.String()) {
					isBlacklisted = true
				}
			}
			if isBlacklisted {
				log.Printf("Found in blacklist: %v", isBlacklisted)
				m.Response = true
				m.OpCode = 0
				m.RCode = dnsmessage.RCodeSuccess
				m.Authoritative = true
				m.Answers = make([]dnsmessage.Resource, 2)
				m.Answers[0] = dnsmessage.Resource{
					Header: dnsmessage.ResourceHeader{
						TTL:    0,
						Length: 4,
						Type:   m.Questions[0].Type,
						Name:   m.Questions[0].Name,
						Class:  m.Questions[0].Class,
					},
					Body: &dnsmessage.AResource{
						A: [4]byte{127, 0, 0, 1},
					},
				}
				m.Answers[1] = dnsmessage.Resource{
					Header: dnsmessage.ResourceHeader{
						TTL:    0,
						Length: 16,
						Type:   m.Questions[0].Type,
						Name:   m.Questions[0].Name,
						Class:  m.Questions[0].Class,
					},
					Body: &dnsmessage.AAAAResource{
						AAAA: [16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}, // is this correct?
					},
				}
				packed, err := m.Pack()
				if err != nil {
					log.Fatalf("could not pack dnsmessage.Message: %v", err)
				}
				if _, err := conn.WriteToUDP(packed, udpAddr); err != nil {
					log.Fatalf("could not write to blacklisted UDP connection: %v", err)
				}

				continue
			}

			// This is an incoming DNS request that hasn't been "resolved".
			// Pack back to bytes.
			packed, err := m.Pack()
			if err != nil {
				log.Fatalf("could not pack dnsmessage.Message: %v", err)
			}

			// Forward to Cloudflare.
			resolver := &net.UDPAddr{Port: 53, IP: net.ParseIP(upstreamDNS)}
			if _, err := conn.WriteToUDP(packed, resolver); err != nil {
				log.Fatalf("could not write to Cloudflare UDP connection: %v", err)
			}
			log.Printf("Forwarded to Cloudflare: %v", resolver.String())

			// Keep track of the originator so we can respond back.
			respMap.store(m.ID, udpAddr)

		case true:
			// Check if we have a request that needs reconciliation.
			originator := respMap.retrieve(m.ID)
			if originator == nil {
				log.Printf("found dangling DNS message ID: %v", m.ID)
				// Improvement: we can call the cleanup func on the ResponseRegistry
				continue
			}

			// This is a response from an upstream DNS server.
			// Make sure we respond to the initial requester.
			if _, err := conn.WriteToUDP(buf, originator); err != nil {
				log.Fatalf("could not write to original UDP connection (%v): %v", udpAddr.String(), err)
			}

			// If everything was OK, we can assume
			// that the request was fulfilled and thus
			// we can safely delete the ID from our registry.
			respMap.remove(m.ID)

			log.Printf("Responded to original requester: %v", originator.String())
		}
	}
}

func mustGetEnv(value string) string {
	v := os.Getenv(value)
	if v == "" {
		log.Fatalf("could not retrieve needed value (%v) from the environment", value)
	}
	return v
}

func mustGetEnvInt(value string) int {
	v := os.Getenv(value)
	if v == "" {
		log.Fatalf("could not retrieve needed value (%v) from the environment", value)
	}

	i, err := strconv.Atoi(v)
	if err != nil {
		log.Fatalf("could not convert needed value (%v) from string to int: %v", value, err)
	}

	return i
}
