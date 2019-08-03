package main

import (
	"fmt"
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
	blacklist := NewRegistry([]string{
		"ads.google.com.",
		"ad.google.com.",
		"facebook.com.",
		"microsoft.com.",
	})

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

			switch {
			case msg.MsgHdr.Response == false:
				// Check if in blacklist.
				if len(msg.Question) > 0 {
					q := msg.Question[0]
					if blacklist.exists(q.Name) {
						msg.MsgHdr.Response = true
						msg.MsgHdr.Opcode = dns.RcodeNameError
						msg.MsgHdr.Authoritative = true
						msg.Answer = make([]dns.RR, 0)
						if err := packMsgAndSend(msg, conn, req, logger); err != nil {
							logger.Errorf("could not write to upstream DNS UDP connection: %v", err)
							continue
						}

						logger.Debugf("Blocked (%v): %v", upstreamResolver.String(), msg.Question)
						continue
					}
				}

				// Forward to upstream DNS.
				if err := packMsgAndSend(msg, conn, upstreamResolver, logger); err != nil {
					logger.Errorf("could not write to upstream DNS UDP connection: %v", err)
					continue
				}

				// Keep track of the originalReq
				// so that we can respond back.
				responseRegistry.store(msg.Id, req)
				logger.Debugf("Forwarded to upstream DNS (%v): %v", upstreamResolver.String(), msg.Question)

			case msg.MsgHdr.Response:
				// This is a response.
				// Check if we have a request that needs reconciliation.
				originalReq := responseRegistry.retrieve(msg.Id)
				if originalReq == nil {
					logger.Errorf("found dangling DNS message ID (%v): %v", msg.Id, msg.Question)
					continue
				}

				// Respond to the initial requester.
				if err := packMsgAndSend(msg, conn, originalReq, logger); err != nil {
					logger.Errorf("could not write to original UDP connection (%v): %v", originalReq.String(), err)
					continue
				}

				// If everything was OK, we can assume
				// that the request was fulfilled and thus
				// we can safely delete the ID from our registry.
				responseRegistry.remove(msg.Id)
				logger.Debugf("Responded to original requester (%v): %v", originalReq.String(), msg.Question)
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

func packMsgAndSend(msg dns.Msg, conn *net.UDPConn, req *net.UDPAddr, logger *zap.SugaredLogger) error {
	packed, err := msg.Pack()
	if err != nil {
		return fmt.Errorf("could not pack dns message: %v", err)
	}

	if _, err := conn.WriteToUDP(packed, req); err != nil {
		return fmt.Errorf("could not write to upstream DNS UDP connection: %v", err)
	}

	return nil
}
