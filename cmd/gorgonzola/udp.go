package main

import (
	"context"
	"github.com/foae/gorgonzola/dns"
	"go.uber.org/zap"
	"strings"
)

func processUdpMessages(ctx context.Context, dnsConn *dns.Conn, dnsService *dns.Service, logger *zap.SugaredLogger) {
	for {
		select {
		case <-ctx.Done():
			logger.Info("Closed DNS service.")
			return
		default:
			if ctx.Err() != nil {
				return
			}

			// Read incoming bytes via UDP.
			buf := make([]byte, 576, 1024)
			_, addr, err := dnsConn.ReadFromUDP(buf)
			switch {
			case err == nil:
				// OK.
			case strings.Contains(err.Error(), "use of closed network connection"):
				return
			default:
				logger.Errorf("could not read from UDP dnsConn: %v", err)
				continue
			}

			if !dnsService.CanHandle(addr) {
				logger.Errorf("cannot handle non-IPv4 request: %v", addr.String())
				continue
			}

			// Unpack and validate the received message.
			var msg dns.Msg
			if err := msg.Unpack(buf); err != nil {
				logger.Errorf("could not read message from (%v)", addr.String())
				continue
			}

			// Handle the DNS request.
			switch msg.MsgHdr.Response {
			case false:
				if err := dnsService.HandleInitialRequest(dnsConn, msg, addr); err != nil {
					logger.Error(err)
				}
			default:
				if err := dnsService.HandleResponseRequest(dnsConn, msg); err != nil {
					logger.Error(err)
				}
			}
		}
	}
}
