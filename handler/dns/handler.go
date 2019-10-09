package dns

import (
	"context"
	"fmt"
	"github.com/miekg/dns"
	"go.uber.org/zap"
	"net"
)

// Handle holds the logic for handling an incoming DNS request.
func Handle(
	ctx context.Context,
	conn *net.UDPConn,
	domainBlocklist *Blocklist,
	responseRegistry *ResponseRegistry,
	upstreamResolver *net.UDPAddr,
	logger *zap.SugaredLogger,
) error {
	for {
		select {
		case <-ctx.Done():
			if err := conn.Close(); err != nil {
				logger.Errorf("could not close conn: %v", err)
			}
			logger.Info("Closed background UDP listener.")
			return nil
		default:
			if ctx.Err() != nil {
				return errRec("closed worker w/ ctx: %v", ctx.Err())
			}

			buf := make([]byte, 576, 1024)
			_, req, err := conn.ReadFromUDP(buf)
			if err != nil {
				return errFatal("could not read from UDP connection: %v", err)
			}

			// Unpack and validate the received message.
			var msg dns.Msg
			if err := msg.Unpack(buf); err != nil {
				return errRec("could not read message from (%v)", req.String())
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
							return errRec("could not pack and send: %v", err)
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
					"fullMsg", msg,
				)
			}
		}
	}
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
