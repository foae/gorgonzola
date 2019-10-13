package dns

import (
	"context"
	"fmt"
	"github.com/foae/gorgonzola/repository"
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
				c.logger.Errorf("dns: could not read from UDP connection: %v", err)
				continue
			}

			// Unpack and validate the received message.
			var msg dns.Msg
			if err := msg.Unpack(buf); err != nil {
				c.logger.Errorf("dns: could not read message from (%v)", req.String())
				continue
			}

			switch msg.MsgHdr.Response {
			case false:
				c.logger.Infof("Query for (%v) from (%v)", msg.Question[0].Name, req.IP.String())

				// Check if in domainBlocklist.
				if len(msg.Question) > 0 {
					if domainBlocklist.exists(msg.Question[0].Name) {
						msg = block(msg)
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

				q := repository.NewQuery(*req, msg)
				if err := c.db.Create(q); err != nil {
					c.logger.Errorf("could not create query entry: %v", err)
				}

				c.logger.Debugf("Forwarded msg (%v) to upstream DNS (%v): %v", msg.Id, c.upstreamResolver.String(), msg.Question)

			case true:
				c.logger.Infof("Query response for (%v) from (%v)", msg.Question[0].Name, req.IP.String())

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

				_, err := c.db.Read(msg.Id)
				if err != nil {
					c.logger.Errorf("could not read query entry (%v): %v", msg.Id, err)
				}
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

func block(m dns.Msg) dns.Msg {
	msg := m.Copy()
	msg.MsgHdr.Response = true
	msg.MsgHdr.Opcode = dns.RcodeNameError
	msg.MsgHdr.Authoritative = true
	msg.Answer = make([]dns.RR, 0)

	return *msg
}
