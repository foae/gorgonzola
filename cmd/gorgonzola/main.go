package main

import (
	"github.com/miekg/dns"
	"go.uber.org/zap"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// SOA is a string we will append everywhere in the zones values.
const SOA string = "@ SOA prisoner.iana.org. hostmaster.root-servers.org. 2002040800 1800 900 0604800 604800"

// NewRR is a shortcut to dns.NewRR that ignores the error.
func NewRR(s string) dns.RR { r, _ := dns.NewRR(s); return r }

var zones = map[string]dns.RR{
	"10.in-addr.arpa.":      NewRR("$ORIGIN 10.in-addr.arpa.\n" + SOA),
	"254.169.in-addr.arpa.": NewRR("$ORIGIN 254.169.in-addr.arpa.\n" + SOA),
	"168.192.in-addr.arpa.": NewRR("$ORIGIN 168.192.in-addr.arpa.\n" + SOA),
	"16.172.in-addr.arpa.":  NewRR("$ORIGIN 16.172.in-addr.arpa.\n" + SOA),
	"17.172.in-addr.arpa.":  NewRR("$ORIGIN 17.172.in-addr.arpa.\n" + SOA),
	"18.172.in-addr.arpa.":  NewRR("$ORIGIN 18.172.in-addr.arpa.\n" + SOA),
	"19.172.in-addr.arpa.":  NewRR("$ORIGIN 19.172.in-addr.arpa.\n" + SOA),
	"20.172.in-addr.arpa.":  NewRR("$ORIGIN 20.172.in-addr.arpa.\n" + SOA),
	"21.172.in-addr.arpa.":  NewRR("$ORIGIN 21.172.in-addr.arpa.\n" + SOA),
	"22.172.in-addr.arpa.":  NewRR("$ORIGIN 22.172.in-addr.arpa.\n" + SOA),
	"23.172.in-addr.arpa.":  NewRR("$ORIGIN 23.172.in-addr.arpa.\n" + SOA),
	"24.172.in-addr.arpa.":  NewRR("$ORIGIN 24.172.in-addr.arpa.\n" + SOA),
	"25.172.in-addr.arpa.":  NewRR("$ORIGIN 25.172.in-addr.arpa.\n" + SOA),
	"26.172.in-addr.arpa.":  NewRR("$ORIGIN 26.172.in-addr.arpa.\n" + SOA),
	"27.172.in-addr.arpa.":  NewRR("$ORIGIN 27.172.in-addr.arpa.\n" + SOA),
	"28.172.in-addr.arpa.":  NewRR("$ORIGIN 28.172.in-addr.arpa.\n" + SOA),
	"29.172.in-addr.arpa.":  NewRR("$ORIGIN 29.172.in-addr.arpa.\n" + SOA),
	"30.172.in-addr.arpa.":  NewRR("$ORIGIN 30.172.in-addr.arpa.\n" + SOA),
	"31.172.in-addr.arpa.":  NewRR("$ORIGIN 31.172.in-addr.arpa.\n" + SOA),
}

func main() {
	upstreamDNS := mustGetEnv("UPSTREAM_DNS_SERVER_IP")
	upstreamResolver := &net.UDPAddr{Port: 53, IP: net.ParseIP(upstreamDNS)}
	_ = upstreamResolver
	localDNSPort := mustGetEnv("DNS_LISTEN_PORT")
	env := mustGetEnv("ENV")

	zapLog, _ := zap.NewProduction()
	defer zapLog.Sync()
	logger := zapLog.Sugar()

	ts := time.Now()
	c := new(dns.Client)
	r, rtt, err := c.Exchange(&dns.Msg{
		MsgHdr: dns.MsgHdr{
			Id:                 66666,
			Response:           false,
			Opcode:             0,
			Authoritative:      false,
			Truncated:          false,
			RecursionDesired:   false,
			RecursionAvailable: false,
			Zero:               false,
			AuthenticatedData:  false,
			CheckingDisabled:   false,
			Rcode:              0,
		},
		Compress: false,
		Question: nil,
		Answer:   nil,
		Ns:       nil,
		Extra:    nil,
	}, upstreamResolver.String())
	logger.Debugw("exchange",
		"ts",time.Since(ts),
		"r",r,
		"rtt",rtt,
		"err"
		)

	ballast := make([]byte, 1<<20)
	_ = ballast
	if env == "dev" {
		go func() {
			for range time.Tick(time.Second * 5) {
				printMemUsage()
			}
		}()
	}

	srv := dns.Server{
		Addr:              ":" + localDNSPort,
		Net:               "udp",
		ReadTimeout:       3,
		WriteTimeout:      5,
		NotifyStartedFunc: func() { log.Println("Started DNS resolver.") },
		MaxTCPQueries:     128,
		ReusePort:         false,
		DecorateReader: func(r dns.Reader) dns.Reader {
			logger.Debugw("read", "reader", r)
			return r
		},
		DecorateWriter: func(w dns.Writer) dns.Writer {
			logger.Debugw("write", "writer", w)
			return w
		},
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("could not start the DNS resolver: %v", err)
		}
	}()

	for z, rr := range zones {
		rrx := rr.(*dns.SOA) // Needed to create the actual RR, and not an reference.
		dns.HandleFunc(z, func(w dns.ResponseWriter, r *dns.Msg) {
			logger.Debugw("received msg", "msg", r)
			m := new(dns.Msg)
			m.SetReply(r)
			m.Authoritative = true
			m.Ns = []dns.RR{rrx}
			w.WriteMsg(m)
		})
	}

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	s := <-sig

	if err := srv.Shutdown(); err != nil {
		log.Fatalf("could not close the server in a clean way: %v", err)
	}
	log.Fatalf("Stopped server, received (%v)", s)
}
