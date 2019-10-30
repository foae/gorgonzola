package dns

import (
	"errors"
	"fmt"
	"github.com/foae/gorgonzola/adblock"
	"github.com/foae/gorgonzola/internal"
	uuid "github.com/satori/go.uuid"
	"net"
	"strconv"
	"strings"
	"time"
)

// Repository describes the functionality necessary to interact with the data layer.
type Repository interface {
	Create(q *Query) error
	Find(id uint16) (*Query, error)
	FindAll() ([]*Query, error)
	Update(q *Query) error
	Delete(q *Query) error
}

// Service describes the structure needed to run and handle the service.
type Service struct {
	repository Repository
	cache      Cacher
	logger     internal.Logger
	adblocker  adblock.Servicer
}

// NewService describes the dependencies needed to build and return a *Service.
func NewService(
	repo Repository,
	cache Cacher,
	logger internal.Logger,
	adblocker adblock.Servicer,
) *Service {
	return &Service{
		repository: repo,
		cache:      cache,
		logger:     logger,
		adblocker:  adblocker,
	}
}

// HandleInitialRequest will handle the initial dns request (query), taking decision whether it should
// be blocked or forwarded to a preconfigured upstream DNS resolver.
func (svc *Service) HandleInitialRequest(conn *Conn, msg Msg, addr *net.UDPAddr) error {
	if len(msg.Question) == 0 {
		svc.logger.Infow("Received empty query.", "msg", msg, "addr", addr)
		return nil
	}

	svc.logger.Debugf("Initial query for (%v) from (%v)...", msg.Question[0].Name, addr.IP.String())

	// Check if this query can be blocked.
	shouldBlock, err := svc.adblocker.ShouldBlock(msg.Question[0].Name)
	switch {
	case err != nil:
		svc.logger.Errorf("dns: could not run the adblocker service: %v", err)
	case shouldBlock:
		msg.block()
		if err := svc.packMsgAndSend(conn, msg, addr); err != nil {
			return err
		}

		q, err := newBlockedQueryFrom(*addr, msg)
		if err != nil {
			return err
		}
		if err := svc.repository.Create(q); err != nil {
			return err
		}

		svc.logger.Infof("Blocked (%v) in msg (%v)", msg.Question[0].Name, msg.Id)
		return nil
	}

	// Forward to upstream DNS.
	if err := svc.packMsgAndSend(conn, msg, conn.upstreamResolver); err != nil {
		return fmt.Errorf("dns: could not write to upstream DNS conn: %v", err)
	}

	// Keep track of the originalReq so that we can respond back.
	svc.cache.Set(strconv.Itoa(int(msg.Id)), addr, time.Minute*30)

	// Store in the repository.
	if err := svc.createResource(msg, *addr); err != nil {
		return err
	}

	svc.logger.Debugf("Forwarded msg (%v) to upstream DNS (%v): %v", msg.Id, conn.upstreamResolver.String(), msg.Question)
	return nil
}

// HandleResponseRequest will handle any incoming responses from forwarded requests
// to the upstream dns resolver(s). It will respond back to the original requester.
func (svc *Service) HandleResponseRequest(conn *Conn, msg Msg) error {
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
	if err := svc.updateResource(msg); err != nil {
		return err
	}

	return nil
}

// CanHandle decides whether our Service can handle the incoming request.
// It will also validate if the requester provided a valid IP address.
// Currently only IPv4 originators can be <handled>. IPv6 is in the makings.
func (svc *Service) CanHandle(addr *net.UDPAddr) bool {
	if addr.IP == nil || len(addr.IP) == 0 {
		return false
	}

	ip := net.ParseIP(addr.IP.String())
	if ip == nil || !ip.Equal(addr.IP) {
		return false
	}

	// We can handle only IPv4 for now.
	return ip.To4() != nil
}

// createResource persists in the repository the newly created query
// using the dns message and the requester's address.
func (svc *Service) createResource(msg Msg, addr net.UDPAddr) error {
	q, err := newQueryFrom(msg, addr)
	if err != nil {
		return fmt.Errorf("dns: could not create query entry: %v", err)
	}

	if err := svc.repository.Create(q); err != nil {
		return fmt.Errorf("dns: could not persist query entry: %v", err)
	}

	return nil
}

// updateResource updates an existing entry in the repository.
func (svc *Service) updateResource(msg Msg) error {
	q, err := svc.repository.Find(msg.Id)
	if err != nil {
		e := fmt.Errorf("dns: could not read query entry (%v): %v", msg.Id, err)
		svc.logger.Error(e)
		return e
	}

	q.Responded = true
	ts := time.Now()
	q.UpdatedAt = &ts
	if len(msg.Answer) > 0 {
		q.Response = strings.TrimSuffix(msg.Answer[0].Header().Name, ".")
	}

	if err := svc.repository.Update(q); err != nil {
		e := fmt.Errorf("dns: could not update query entry (%v): %v", msg.Id, err)
		svc.logger.Error(e)
		return e
	}

	return nil
}

// packMsgAndSend handles packing a dns message and sending it over a provided connection.
func (svc *Service) packMsgAndSend(conn *Conn, msg Msg, req *net.UDPAddr) error {
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

// newQueryFrom builds a new Query based on the provided dns message and the requester's address.
// It will perform minimum validation on the dns message.
// By default, all new Queries are valid. Check for returned error for an invalid query.
func newQueryFrom(msg Msg, req net.UDPAddr) (*Query, error) {
	q := &Query{
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
		return nil, errors.New("dns: msg question not valid: length must not be 0")
	}

	qt, ok := QueryTypeMap[msg.Question[0].Qtype]
	if !ok {
		return nil, fmt.Errorf("dns: query type not mapped: got (%#v)", msg.Question)
	}
	q.Type = qt

	if msg.MsgHdr.Response && len(msg.Answer) > 0 {
		// TODO: handle multiple answers
		q.Response = strings.TrimSuffix(msg.Answer[0].Header().Name, ".")

	}

	domain := strings.TrimSuffix(msg.Question[0].Name, ".")
	rootDomain := internal.ExtractRootDomainFrom(domain)

	q.Domain = domain
	q.RootDomain = rootDomain

	return q, nil
}

func newBlockedQueryFrom(req net.UDPAddr, msg Msg) (*Query, error) {
	q, err := newQueryFrom(msg, req)
	if err != nil {
		return nil, err
	}
	q.Responded = true
	q.Blocked = true

	return q, nil
}
