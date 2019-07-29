package main

import (
	"fmt"
	"golang.org/x/net/dns/dnsmessage"
	"log"
	"net"
	"os"
	"runtime"
	"strconv"
	"time"
)

func main() {
	upstreamDNS := mustGetEnv("UPSTREAM_DNS_SERVER_IP")
	localDNSPort := mustGetEnvInt("DNS_LISTEN_PORT")
	env := mustGetEnv("ENV")

	ballast := make([]byte, 1<<20)
	_ = ballast
	if env == "dev" {
		go func() {
			for range time.Tick(time.Second) {
				printMemUsage()
			}
		}()
	}

	// Fire up the UDP listener.
	conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: localDNSPort})
	if err != nil {
		log.Fatalf("could not listen on port 53 UDP: %v", err)
	}
	defer conn.Close()

	// Load the blacklist
	blacklist := NewRegistry([]string{
		"ads.google.com",
		"ad.google.com",
		"facebook.com",
		"microsoft.com",
	})

	// Keep track of all UDP messages
	// that need to be reconciled.
	respMap := NewResponseRegistry()

	log.Println("Waiting for UDP connections...")
	for {
		buf := make([]byte, 576)
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
			log.Printf("Forwarded to upstream DNS (%v): %v", resolver.String(), m.GoString())

			// Keep track of the originator so we can respond back.
			respMap.store(m.ID, udpAddr)

		case true:
			// Check if we have a request that needs reconciliation.
			originator := respMap.retrieve(m.ID)
			if originator == nil {
				log.Printf("found dangling DNS message ID (%v): %v", m.ID, m.GoString())
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

			log.Printf("Responded to original requester (%v): %v", originator.String(), m.GoString())
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

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

func printMemUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	fmt.Printf("Alloc: %v MB", bToMb(m.Alloc))
	fmt.Printf("\tTotalAlloc: %v MB", bToMb(m.TotalAlloc))
	fmt.Printf("\tSys: %v MB", bToMb(m.Sys))
	fmt.Printf("\tNumGC: %v", m.NumGC)
	fmt.Printf("\tHeap: alloc (%v) | idle (%v) | in use (%v) | obj (%v) | released (%v)\n", bToMb(m.HeapAlloc), bToMb(m.HeapIdle), bToMb(m.HeapInuse), m.HeapObjects, bToMb(m.HeapReleased))
}
