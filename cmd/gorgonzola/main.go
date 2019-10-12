package main

import (
	"context"
	"fmt"
	"github.com/dgraph-io/badger"
	"github.com/foae/gorgonzola/handler/dns"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	httpHandler "github.com/foae/gorgonzola/handler/http"
	"github.com/gin-gonic/gin"

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
		Setup data layer
	*/
	dbFile := "./repository/tmp/badger"
	db, err := badger.Open(badger.LSMOnlyOptions(dbFile))
	if err != nil {
		log.Fatalf("could not open db file in (%v): %v", dbFile, err)
	}
	defer db.Close()

	ctx := context.Background()
	hostname, err := os.Hostname()
	if err != nil {
		hostname = fmt.Sprintf("pid-%v-ts-%v", os.Getpid(), time.Now().UnixNano())
	}
	upstreamResolver := &net.UDPAddr{Port: 53, IP: net.ParseIP(upstreamDNS)}
	domainBlocklist := dns.NewBlocklist([]string{
		"ads.google.com.",
		"ad.google.com.",
		"facebook.com.",
		"microsoft.com.",
	})

	// Keep track of all UDP messages
	// that need to be reconciled.
	responseRegistry := dns.NewResponseRegistry()

	/*
		HTTP Router
	*/
	switch env {
	case "dev":
		gin.SetMode(gin.DebugMode)
	default:
		gin.SetMode(gin.ReleaseMode)
	}
	router := gin.Default()

	/*
		Logging
	*/
	var logger *zap.SugaredLogger
	switch env {
	case "dev":
		logger, err = newDevelopmentLogger()
	default:
		logger, err = newProductionLogger()
	}
	if err != nil {
		log.Fatalf("could not init logger: %v", err)
	}
	defer logger.Sync() // nolint

	/*
		Fire up the UDP listener.
	*/
	conn, err := dns.NewUDPConn(localDNSPort, upstreamResolver)
	if err != nil {
		log.Fatalf("could not listen on port (%v) UDP: %v", localDNSPort, err)
	}
	conn.WithConfig(dns.Config{
		DB:     db,
		Logger: logger,
	})
	defer conn.Close()

	/*
		Process UDP messages.
	*/
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		logger.Infof("Waiting for messages via UDP on port (%v)...", localDNSPort)
		if err := conn.ListenAndServe(cctx, domainBlocklist, responseRegistry); err != nil {
			logger.Errorf("error in worker: %v", err)
			return
		}
	}()

	/*
		HTTP routes attachment.
	*/
	handler := httpHandler.New(httpHandler.Config{
		Logger: logger,
	})
	router.POST("/blocklist", handler.AddToBlocklist)
	router.GET("/health", handler.Health)

	srv := http.Server{
		Addr:              localHTTPPort,
		Handler:           router,
		ReadTimeout:       8,
		ReadHeaderTimeout: 8,
		WriteTimeout:      8,
		IdleTimeout:       32,
		MaxHeaderBytes:    1024,
	}
	go func() {
		logger.Infof("Started HTTP server on port (%v)", localHTTPPort)
		err := srv.ListenAndServe()
		switch {
		case err == http.ErrServerClosed:
			logger.Infof("http listener closed: %v", err)
		case err != nil:
			logger.Fatalf("http listener error: %v", err)
		}
	}()

	ips, err := getIPAddr()
	if err != nil {
		logger.Fatalf("could not read the IPv4 address: %v", err)
	}
	logger.Infof("Running instance. Hostname: %v | Go: %v | IPv4: %v", hostname, runtime.Version(), ips[0])

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	logger.Infof("Stopping servers, received shutdown signal: %v", <-sig)

	if err := srv.Shutdown(cctx); err != nil {
		log.Fatalf("error in http server: %v", err)
	}

	cancel()
	time.Sleep(time.Second * 3)
}
