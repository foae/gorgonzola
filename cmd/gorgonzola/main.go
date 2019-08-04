package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/miekg/dns"
	"go.uber.org/zap"
)

func main() {
	/*
		ENV vars
	*/
	upstreamDNS := mustGetEnv("UPSTREAM_DNS_SERVER_IP")
	localDNSPort := mustGetEnvInt("DNS_LISTEN_PORT")
	localHTTPPort := mustGetEnv("HTTP_LISTEN_ADDR")
	env := mustGetEnv("ENV")

	/*
		Setup
	*/
	hostname, _ := os.Hostname()
	// Keep track of all UDP messages
	// that need to be reconciled.
	responseRegistry := NewResponseRegistry()
	upstreamResolver := &net.UDPAddr{Port: 53, IP: net.ParseIP(upstreamDNS)}
	domainBlocklist := NewBlocklist([]string{
		"ads.google.com.",
		"ad.google.com.",
		"facebook.com.",
		"microsoft.com.",
	})

	/*
		Logging
	*/
	var logger *zap.SugaredLogger
	var err error
	switch env {
	case "dev":
		logger, err = newDevelopmentLogger()
	default:
		logger, err = newProductionLogger()
	}
	if err != nil {
		log.Fatal(err)
	}
	defer logger.Sync() // nolint

	/*
		Fire up the UDP listener.
	*/
	conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: localDNSPort})
	if err != nil {
		log.Fatalf("could not listen on port (%v) UDP: %v", localDNSPort, err)
	}
	logger.Infof("Started UDP resolver on port (%v)", localDNSPort)
	defer conn.Close()

	/*
		Process UDP messages.
	*/
	cctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func(ctx context.Context) {
		logger.Infof("Waiting for messages via UDP on port (%v)...", localDNSPort)
		for {
			select {
			case <-ctx.Done():
				logger.Info("Closed background UDP listener.")
				_ = conn.Close()
				return
			default:
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
					// Check if in domainBlocklist.
					if len(msg.Question) > 0 {
						if domainBlocklist.exists(msg.Question[0].Name) {
							msg.MsgHdr.Response = true
							msg.MsgHdr.Opcode = dns.RcodeNameError
							msg.MsgHdr.Authoritative = true
							msg.Answer = make([]dns.RR, 0)
							if err := packMsgAndSend(msg, conn, req, logger); err != nil {
								logger.Errorf("could not pack and send: %v", err)
								continue
							}

							logger.Debugf("Blocked (%v): %v", upstreamResolver.String(), msg.Question)
							continue
						}
					}

					// Forward to upstream DNS.
					if err := packMsgAndSend(msg, conn, upstreamResolver, logger); err != nil {
						logger.Errorf("could not write to upstream DNS connection: %v", err)
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
						logger.Errorf("could not write to original connection (%v): %v", originalReq.String(), err)
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
		}
	}(cctx)

	srv := http.Server{
		Addr:              localHTTPPort,
		ReadTimeout:       time.Second * 5,
		ReadHeaderTimeout: time.Second * 5,
		WriteTimeout:      time.Second * 5,
		IdleTimeout:       time.Second * 5,
		MaxHeaderBytes:    1024,
	}

	go func() {
		logger.Infof("Started HTTP server on port (%v)", localHTTPPort)
		err := srv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			logger.Fatalf("the http server encountered an error: %v", err)
		}
	}()

	logger.Infof("Running pod. Hostname: %v | Go: %v", hostname, runtime.Version())
	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	logger.Infof("Stopping servers, received shutdown signal: %v", <-sig)
	if err := srv.Shutdown(cctx); err != nil {
		log.Fatalf("missed shutting down the http server gracefully: %v", err)
	}
	time.Sleep(time.Second)
}

func packMsgAndSend(msg dns.Msg, conn *net.UDPConn, req *net.UDPAddr, logger *zap.SugaredLogger) error {
	packed, err := msg.Pack()
	if err != nil {
		return fmt.Errorf("packMsgAndSend: could not pack dns message: %v", err)
	}

	if _, err := conn.WriteToUDP(packed, req); err != nil {
		return fmt.Errorf("packMsgAndSend: could not write to UDP connection (%v): %v", req.String(), err)
	}

	return nil
}
