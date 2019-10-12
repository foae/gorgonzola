package dns

import (
	"context"
	"fmt"
	"github.com/miekg/dns"
	"net"
)

// ListenAndServe holds the logic for handling an incoming DNS request.
func (c *Conn) ListenAndServe(
	ctx context.Context,
	domainBlocklist *Blocklist,
	responseRegistry *ResponseRegistry,
) error {
	for {
		select {
		case <-ctx.Done():
			if err := c.Close(); err != nil {
				c.logger.Errorf("could not close conn: %v", err)
			}
			
			c.logger.Info("dns: closed background UDP listener.")
			return nil
		default:
			if ctx.Err() != nil {
				return errRec("dns: closed worker w/ ctx: %v", ctx.Err())
			}

			buf := make([]byte, 576, 1024)
			_, req, err := c.udpConn.ReadFromUDP(buf)
			if err != nil {
				return errFatal("dns: could not read from UDP connection: %v", err)
			}

			// Unpack and validate the received message.
			var msg dns.Msg
			if err := msg.Unpack(buf); err != nil {
				return errRec("dns: could not read message from (%v)", req.String())
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
						if err := c.packMsgAndSend(msg, req); err != nil {
							return errRec("dns: could not pack and send: %v", err)
						}

						c.logger.Debugf("Blocked (%v) in msg (%v)", msg.Question, msg.Id)
						continue
					}
				}

				// Forward to upstream DNS.
				if err := c.packMsgAndSend(msg, c.upstreamResolver); err != nil {
					c.logger.Errorf("dns: could not write to upstream DNS connection: %v", err)
					continue
				}

				// Keep track of the originalReq
				// so that we can respond back.
				responseRegistry.store(msg.Id, req)
				c.logger.Debugf("Forwarded msg (%v) to upstream DNS (%v): %v", msg.Id, c.upstreamResolver.String(), msg.Question)

			case msg.MsgHdr.Response:
				// This is a response.
				// Check if we have a request that needs reconciliation.
				originalReq := responseRegistry.retrieve(msg.Id)
				if originalReq == nil {
					c.logger.Errorf("dns: found dangling DNS msg (%v): %v", msg.Id, msg.Question)
					continue
				}

				// Respond to the initial requester.
				if err := c.packMsgAndSend(msg, originalReq); err != nil {
					c.logger.Errorf("dns: could not write to original connection (%v) for msg (%v): %v", originalReq.String(), msg.Id, err)
					continue
				}

				// If everything was OK, we can assume
				// that the request was fulfilled and thus
				// we can safely delete the ID from our registry.
				responseRegistry.remove(msg.Id)
				c.logger.Debugf("Responded to original requester (%v) for msg (%v): %v", originalReq.String(), msg.Id, msg.Question)
			default:
				c.logger.Warnw("dns: received alien message",
					"from", req.String(),
					"fullMsg", msg,
				)
			}
		}
	}
}

func (c *Conn) packMsgAndSend(msg dns.Msg, req *net.UDPAddr) error {
	packed, err := msg.Pack()
	if err != nil {
		return fmt.Errorf("dns: could not pack dns message: %v", err)
	}

	if _, err := c.udpConn.WriteToUDP(packed, req); err != nil {
		return fmt.Errorf("dns: could not write to UDP connection (%v): %v", req.String(), err)
	}

	c.logger.Debugf("Sent msg (%v) to (%v)", msg.Id, req.IP.String())
	return nil
}
