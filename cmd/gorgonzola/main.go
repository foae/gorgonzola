package main

import (
	"context"
	"log"
	"net"
	"os"

	"golang.org/x/net/dns/dnsmessage"
)

func main() {
	ctx := context.Background()
	devEnv := mustGetEnv("ENV")
	_ = devEnv
	_ = ctx

	conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: 5300})
	if err != nil {
		log.Fatalf("could not listen on port 53 UDP: %v", err)
	}
	defer conn.Close()

	log.Println("Waiting for UDP connections...")
	for {
		buf := make([]byte, 512)
		_, udpAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			log.Fatalf("could not read from UDP connection: %v", err)
		}
		log.Println("-----------------------")
		log.Printf("received UDP message from: %v", udpAddr.String())

		// Unpack the received DNS message.
		var m dnsmessage.Message
		if err := m.Unpack(buf); err != nil {
			log.Fatalf("could not unpack `buf` into a dnsmessage.Message: %v", err)
		}
		log.Println("-----------------------")
		log.Printf("unpacked dnsmessage.Message: %v", m.GoString())

		//// Pack back to bytes.
		//packed, err := m.Pack()
		//if err != nil {
		//	log.Fatalf("could not pack dnsmessage.Message: %v", err)
		//}

		// Don't forward an answer back to Cloudflare,
		// but respond to the original requester.
		if m.Header.Response == true {
			if _, err := conn.WriteToUDP(buf, udpAddr); err != nil {
				log.Fatalf("could not write to original UDP connection (%v): %v", udpAddr.String(), err)
			}
			continue
		}

		// Forward to Cloudflare.
		resolver := &net.UDPAddr{Port: 53, IP: net.ParseIP("1.1.1.1")}
		if _, err := conn.WriteToUDP(buf, resolver); err != nil {
			log.Fatalf("could not write to Cloudflare UDP connection: %v", err)
		}
		log.Println("-----------------------")
		log.Printf("forwarded to Cloudflare: %v", resolver.String())
	}
}

func mustGetEnv(value string) string {
	v := os.Getenv(value)
	if v == "" {
		log.Fatalf("could not retrieve needed value (%v) from the environment", value)
	}
	return v
}
