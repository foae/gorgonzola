package dns

import (
	"bytes"
	"encoding/gob"
	"net"
)

type Addr struct {
	addr net.UDPAddr
}

func (a Addr) Pack() ([]byte, error) {
	buf := bytes.Buffer{}
	if err := gob.NewEncoder(&buf).Encode(a.addr); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (a *Addr) Unpack(b []byte) error {
	return gob.NewDecoder(bytes.NewBuffer(b)).Decode(&a.addr)
}

func fromUDPAddr(a *net.UDPAddr) Addr {
	return Addr{addr: *a}
}

func (a Addr) String() string {
	return a.addr.String()
}
