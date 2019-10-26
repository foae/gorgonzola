package dns

import (
	"fmt"
	"github.com/foae/gorgonzola/adblock"
	"github.com/foae/gorgonzola/internal"
	"github.com/foae/gorgonzola/repository"
	"github.com/miekg/dns"
	uuid "github.com/satori/go.uuid"
	"log"
	"net"
	"strconv"
	"strings"
	"time"
)

type Service struct {
	repository repository.Interactor
	cache      Cacher
	logger     Logger
	adblocker  adblock.Servicer
}

func NewService(
	repo repository.Interactor,
	cache Cacher,
	logger Logger,
	adblocker adblock.Servicer,
) *Service {
	return &Service{
		repository: repo,
		cache:      cache,
		logger:     logger,
		adblocker:  adblocker,
	}
}

func (svc *Service) HandleInitialRequest(conn *Conn, msg dns.Msg, addr *net.UDPAddr) error {
	svc.logger.Debugf("Initial query for (%v) from (%v)...", msg.Question[0].Name, addr.IP.String())

	if len(msg.Question) == 0 {
		svc.logger.Infow("Received empty query.", "msg", msg)
		return nil
	}

	// Check if this query can be blocked.
	shouldBlock, err := svc.adblocker.ShouldBlock(msg.Question[0].Name)
	switch {
	case err != nil:
		svc.logger.Errorf("could not run the adblocker service: %v", err)
	case shouldBlock:
		msg = svc.block(msg)
		if err := svc.packMsgAndSend(conn, msg, addr); err != nil {
			return fmt.Errorf("dns: could not pack and send blocked msg: %v", err)
		}

		q := newQueryFrom(*addr, msg)
		q.Responded = true
		q.Blocked = true
		if err := svc.repository.Create(q); err != nil {
			return fmt.Errorf("dns: could not persist blocked query in repo: %v", err)
		}

		svc.logger.Infof("Blocked (%v) in msg (%v)", msg.Question, msg.Id)
		return nil
	}

	// Forward to upstream DNS.
	if err := svc.packMsgAndSend(conn, msg, conn.upstreamResolver); err != nil {
		return fmt.Errorf("dns: could not write to upstream DNS conn: %v", err)
	}

	// Keep track of the originalReq so that we can respond back.
	svc.cache.Set(strconv.Itoa(int(msg.Id)), addr, time.Minute*30)

	// Store in the repository.
	svc.createResource(msg, *addr)

	svc.logger.Debugf("Forwarded msg (%v) to upstream DNS (%v): %v", msg.Id, conn.upstreamResolver.String(), msg.Question)
	return nil
}

func (svc *Service) HandleResponseRequest(conn *Conn, msg dns.Msg) error {
	msgID := strconv.Itoa(int(msg.Id))

	// This is a response.
	// Check if we have a request that needs reconciliation.
	addrOrig, ok := svc.cache.Get(msgID)
	if addrOrig == nil || !ok {
		return fmt.Errorf("dns: found dangling DNS msg (%v): %v", msg.Id, msg.Question)
	}
	originalAddr, ok := addrOrig.(*net.UDPAddr)
	if !ok {
		svc.cache.Delete(msgID)
		return fmt.Errorf("dns: invalid DNS msg (%v) - not stored as UDPAddr: %v", msg.Id, msg.Question)
	}

	// Respond to the initial requester.
	if err := svc.packMsgAndSend(conn, msg, originalAddr); err != nil {
		return fmt.Errorf("dns: could not write to original conn (%v) for msg (%v): %v", originalAddr.String(), msg.Id, err)
	}

	// If everything was OK, we can assume
	// that the request was fulfilled and thus
	// we can safely delete the ID from our registry.
	svc.cache.Delete(msgID)
	svc.logger.Debugf("Responded to original requester (%v) for msg (%v): %v", originalAddr.String(), msg.Id, msg.Question)

	// Also update in repository.
	svc.updateResource(msg)

	return nil
}

func (svc *Service) CanHandle(addr *net.UDPAddr) bool {
	// We can handle only IPv4 for now.
	return addr.IP.To4() != nil
}

func (svc *Service) createResource(msg dns.Msg, addr net.UDPAddr) {
	q := newQueryFrom(addr, msg)
	if err := svc.repository.Create(q); err != nil {
		svc.logger.Errorf("could not create query entry: %v", err)
	}
}

func (svc *Service) updateResource(msg dns.Msg) {
	q, err := svc.repository.Find(msg.Id)
	if err != nil {
		svc.logger.Errorf("could not read query entry (%v): %v", msg.Id, err)
	}

	q.Responded = true
	ts := time.Now()
	q.UpdatedAt = &ts
	if len(msg.Answer) > 0 {
		q.Response = strings.TrimSuffix(msg.Answer[0].Header().Name, ".")
	}

	if err := svc.repository.Update(q); err != nil {
		svc.logger.Errorf("could not update query entry (%v): %v", msg.Id, err)
	}
}

func (svc *Service) packMsgAndSend(conn *Conn, msg dns.Msg, req *net.UDPAddr) error {
	packed, err := msg.Pack()
	if err != nil {
		return fmt.Errorf("dns: could not pack dns message: %v", err)
	}

	if _, err := conn.udpConn.WriteToUDP(packed, req); err != nil {
		return fmt.Errorf("dns: could not write to UDP conn (%v): %v", req.String(), err)
	}

	svc.logger.Debugf("Packed msg (%v) and sent to (%v)", msg.Id, req.IP)
	return nil
}

func (svc *Service) block(m dns.Msg) dns.Msg {
	msg := m.Copy()
	msg.MsgHdr.Response = true
	msg.MsgHdr.Opcode = dns.RcodeNameError
	msg.MsgHdr.Authoritative = true
	msg.Answer = make([]dns.RR, 0)

	return *msg
}

func newQueryFrom(req net.UDPAddr, msg dns.Msg) *repository.Query {
	q := &repository.Query{
		ID:         int64(msg.Id),
		UUID:       uuid.NewV4().String(),
		Originator: req.IP.String(),
		OriginatorType: func() int {
			if req.IP.To4() != nil {
				return 4
			}
			return 6
		}(),
		Response:  "",
		Responded: false,
		Blocked:   false,
		Valid:     true,
		CreatedAt: time.Now(),
		UpdatedAt: nil,
	}

	if len(msg.Question) == 0 {
		q.Valid = false
		log.Printf("repository: msg question not valid: 0 length: %v", msg.Question)
		return q
	}

	qt, ok := repository.QueryTypeMap[msg.Question[0].Qtype]
	if !ok {
		q.Valid = false
		log.Printf("repository: query type not mapped: got (%#v)", msg.Question)
		return q
	}
	q.Type = qt

	if msg.MsgHdr.Response {
		if len(msg.Answer) > 0 {
			// TODO: handle multiple answers
			q.Response = strings.TrimSuffix(msg.Answer[0].Header().Name, ".")
		}
	}

	domain := strings.TrimSuffix(msg.Question[0].Name, ".")
	rootDomain := internal.ExtractRootDomainFrom(domain)

	q.Domain = domain
	q.RootDomain = rootDomain

	return q
}
