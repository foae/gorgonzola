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
	whitelist := NewRegistry([]string{"cloudflare.com"})

	// Load the blacklist
	blacklist := NewRegistry([]string{
		"ads.google.com",
		"ad.google.com",
		"facebook.com",
		"microsoft.com",
	})

	// Keep track of all UDP messages
	// that need to be reconciled.
	respMap := NewResponseMap()

	log.Println("Waiting for UDP connections...")
	for {
		buf := make([]byte, 1024)
		_, udpAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Fatalf("could not read from UDP connection: %v", err)
		}

		// Unpack and validate the received message.
		var m dnsmessage.Message
		if err := m.Unpack(buf); err != nil {
			log.Printf("could not unpack into dns message: %v", err)
			continue
		}

		switch m.Header.Response {
		case false:
			// This a new request that hasn't been resolved.
			// Check if it's in the whitelist.
			var isWhitelisted bool
			for _, q := range m.Questions {
				if whitelist.exists(q.Name.String()) {
					isWhitelisted = true
					break
				}
			}

			// TODO: cleanup. We can do better than this.
			if !isWhitelisted {
				// Check if it's in the blacklist
				var isBlacklisted bool
				for _, q := range m.Questions {
					if blacklist.exists(q.Name.String()) {
						isBlacklisted = true
						break
					}
				}

				if isBlacklisted {
					log.Printf("Found in blacklist: %v", isBlacklisted)
					m.Response = true
					m.OpCode = 0
					m.RCode = dnsmessage.RCodeSuccess
					m.Authoritative = true
					// Replace contents of the DNS message.
					m = block(m)

					packed, err := m.Pack()
					if err != nil {
						log.Fatalf("could not pack dns message: %v", err)
					}
					if _, err := conn.WriteToUDP(packed, udpAddr); err != nil {
						log.Fatalf("could not write to blacklisted UDP connection: %v", err)
					}
					continue
				}
			}

			// This is an incoming DNS request that hasn't been "resolved".
			// Pack back to bytes.
			packed, err := m.Pack()
			if err != nil {
				log.Printf("could not pack dns message: %v", err)
				continue
			}

			// Forward to upstream DNS.
			resolver := &net.UDPAddr{Port: 53, IP: net.ParseIP(upstreamDNS)}
			if _, err := conn.WriteToUDP(packed, resolver); err != nil {
				log.Printf("could not write to upstream DNS UDP connection: %v", err)
				continue
			}
			log.Printf("Forwarded to upstream DNS: %v", resolver.String())

			// Keep track of the originator so we can respond back.
			respMap.store(m.ID, udpAddr)

		case true:
			// Check if we have a request that needs reconciliation.
			originator := respMap.retrieve(m.ID)
			if originator == nil {
				log.Printf("found dangling DNS message ID: %v", m.ID)
				// Improvement: we can call the cleanup service on the registry.
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
