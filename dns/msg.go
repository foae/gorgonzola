package dns

import miek "github.com/miekg/dns"

// Msg is a simple wrapper for miek's dns struct.
type Msg struct {
	miek.Msg
}

func (m *Msg) block() {
	m.MsgHdr.Response = true
	m.MsgHdr.Authoritative = true
	m.MsgHdr.RecursionAvailable = true
	m.MsgHdr.Rcode = miek.RcodeNameError
	m.Answer = make([]miek.RR, 0)
}
