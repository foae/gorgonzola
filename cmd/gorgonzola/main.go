package main

import (
	"github.com/miekg/dns"
	"go.uber.org/zap"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	/*
		ENV vars
	*/
	upstreamDNS := mustGetEnv("UPSTREAM_DNS_SERVER_IP")
	localDNSPort := mustGetEnvInt("DNS_LISTEN_PORT")
	env := mustGetEnv("ENV")

	/*
		Setup
	*/
	// Keep track of all UDP messages
	// that need to be reconciled.
	responseRegistry := NewResponseRegistry()
	upstreamResolver := &net.UDPAddr{Port: 53, IP: net.ParseIP(upstreamDNS)}

	/*
		Logging
	*/
	var zapLog *zap.Logger
	zapLog, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("could not init zap: %v", err)
	}
	if env == "dev" {
		zapLog, err = zap.NewDevelopment()
		if err != nil {
			log.Fatalf("could not init zap: %v", err)
		}
	}
	defer zapLog.Sync()
	logger := zapLog.Sugar()

	/*
		Fire up the UDP listener.
	*/
	conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: localDNSPort})
	if err != nil {
		log.Fatalf("could not listen on port (%v) UDP: %v", localDNSPort, err)
	}
	defer conn.Close()

	/*
		Process UDP messages.
	*/
	go func() {
		logger.Debugf("Waiting for messages via UDP on port (%v)...", localDNSPort)
		for {
			buf := make([]byte, 576, 1024)
			_, req, err := conn.ReadFromUDP(buf)
			if err != nil {
				logger.Errorf("could not read from UDP connection: %v", err)
				continue
			}

			// Unpack and validate the received message.
			var msg dns.Msg
			if err := msg.Unpack(buf); err != nil {
				logger.Errorf("could not read message from (%v)", req.String())
				continue
			}
			packedMsg, err := msg.Pack()
			if err != nil {
				logger.Errorf("could not pack dns message: %v", err)
				continue
			}

			switch {
			case msg.MsgHdr.Response == false:
				// TODO: check if in blacklist

				// Forward to upstream DNS.
				if _, err := conn.WriteToUDP(packedMsg, upstreamResolver); err != nil {
					logger.Errorf("could not write to upstream DNS UDP connection: %v", err)
					continue
				}

				// Keep track of the originator
				// so that we can respond back.
				responseRegistry.store(msg.Id, req)
				logger.Debugf("Forwarded to upstream DNS (%v): %v", upstreamResolver.String(), msg.Question)

			case msg.MsgHdr.Response:
				// This is a response.
				// Check if we have a request that needs reconciliation.
				originator := responseRegistry.retrieve(msg.Id)
				if originator == nil {
					logger.Errorf("found dangling DNS message ID (%v): %v", msg.Id, msg.Question)
					continue
				}

				// Respond to the initial requester.
				if _, err := conn.WriteToUDP(packedMsg, originator); err != nil {
					logger.Errorf("could not write to original UDP connection (%v): %v", originator.String(), err)
					continue
				}

				// If everything was OK, we can assume
				// that the request was fulfilled and thus
				// we can safely delete the ID from our registry.
				responseRegistry.remove(msg.Id)
				logger.Debugf("Responded to original requester (%v): %v", originator.String(), msg.Question)
			default:
				logger.Warnw("received alien message",
					"from", req.String(),
					"for", msg.Question,
					"fullMsg", msg,
				)
			}
		}
	}()

	logger.Debugf("Started UDP resolver on port (%v)", localDNSPort)
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	log.Fatalf("Stopped server, received signal (%v)", <-sig)
}
