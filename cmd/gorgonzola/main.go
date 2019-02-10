package main

import (
	"golang.org/x/net/dns/dnsmessage"
	"log"
	"net"
	"os"
)

func main() {
	// Fire up the UDP listener.
	conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: 5300})
	if err != nil {
		log.Fatalf("could not listen on port 53 UDP: %v", err)
	}
	defer conn.Close()

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
		log.Println("----------------------------------------------")
		log.Printf("received UDP message from: %v", udpAddr.String())

		// Unpack and validate the received message.
		var m dnsmessage.Message
		if err := m.Unpack(buf); err != nil {
			log.Fatalf("could not unpack `buf` into a dnsmessage.Message: %v", err)
		}
		log.Println("----------------------------------------------")
		log.Printf("unpacked dnsmessage.Message: %v", m.GoString())

		switch m.Header.Response {
		case false:

			// This is an incoming DNS request that hasn't been "resolved".
			// Pack back to bytes.
			packed, err := m.Pack()
			if err != nil {
				log.Fatalf("could not pack dnsmessage.Message: %v", err)
			}

			// Forward to Cloudflare.
			resolver := &net.UDPAddr{Port: 53, IP: net.ParseIP("1.1.1.1")}
			if _, err := conn.WriteToUDP(packed, resolver); err != nil {
				log.Fatalf("could not write to Cloudflare UDP connection: %v", err)
			}
			log.Println("----------------------------------------------")
			log.Printf("forwarded to Cloudflare: %v", resolver.String())

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
