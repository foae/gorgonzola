package main

import (
	"context"
	"fmt"
	"github.com/foae/gorgonzola/adblock"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/foae/gorgonzola/dns"
	httpHandler "github.com/foae/gorgonzola/handler/http"
	"github.com/foae/gorgonzola/repository"
	"github.com/gin-gonic/gin"
	"github.com/patrickmn/go-cache"
)

const (
	envModeDev  = "dev"
	envModeProd = "prod"
)

// defaultEnvVars represents the minimum and sensible
// environment variables needed for this program to work.
var defaultEnvVars = envVars{
	HTTP_LISTEN_ADDR:         "127.0.0.1:8000",
	HTTP_PPROF_LISTEN_ADDR:   "127.0.0.1:8001",
	DNS_LISTEN_ADDR:          ":53",
	UPSTREAM_DNS_SERVER_ADDR: "116.203.111.0:53",
	ENV_MODE:                 "dev",
}

// envVars describes the configurable environment variables.
// Check .env.example for a full explanation.
type envVars struct {
	HTTP_LISTEN_ADDR         string `json:"HTTP_LISTEN_ADDR"`
	HTTP_PPROF_LISTEN_ADDR   string `json:"HTTP_PPROF_LISTEN_ADDR"`
	DNS_LISTEN_ADDR          string `json:"DNS_LISTEN_ADDR"`
	UPSTREAM_DNS_SERVER_ADDR string `json:"UPSTREAM_DNS_SERVER_ADDR"`
	ENV_MODE                 string `json:"ENV_MODE"`
}

func main() {
	/*
		ENV vars
	*/
	if err := godotenv.Load(); err != nil {
		log.Printf("Could not load .env file, will use default values.")
	}
	upstreamDnsAddr := fromEnv("UPSTREAM_DNS_SERVER_ADDR", defaultEnvVars.UPSTREAM_DNS_SERVER_ADDR)
	localDNSAddr := fromEnv("DNS_LISTEN_ADDR", defaultEnvVars.DNS_LISTEN_ADDR)
	localHTTPPort := fromEnv("HTTP_LISTEN_ADDR", defaultEnvVars.HTTP_LISTEN_ADDR)
	env := fromEnv("ENV_MODE", defaultEnvVars.ENV_MODE)
	if env != envModeDev {
		env = envModeProd
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = fmt.Sprintf("pid-%v-ts-%v", os.Getpid(), time.Now().UnixNano())
	}

	/*
		Logging
	*/
	var logger *zap.SugaredLogger
	switch env {
	case envModeDev:
		logger, err = newDevelopmentLogger()
	default:
		logger, err = newProductionLogger()
	}
	if err != nil {
		log.Fatalf("could not init logger: %v", err)
	}
	defer logger.Sync() // nolint

	/*
		Setup data layer
	*/
	repo, err := repository.NewRepo(repository.Config{Logger: logger})
	if err != nil {
		logger.Fatalf("could not init Repository repo: %v", err)
	}
	registerOnClose(repo)

	/*
		AdBlock Service
	*/
	adBlockService := adblock.NewService(logger)

	/*
		Read files from local storage
	*/
	fileList, err := repo.StoredFilesList(true)
	if err != nil {
		log.Fatalf("could not list files: %v", err)
	}

	/*
		LOAD AdBlock Plus providers
	*/
	//if err := adBlockService.LoadAdBlockPlusProviders(fileList); err != nil {
	//	logger.Debugf("could not load file provider, skipped: %v", err)
	//}
	_ = fileList

	/*
		Setup HTTP Router
	*/
	switch env {
	case envModeProd:
		gin.SetMode(gin.ReleaseMode)
	default:
		gin.SetMode(gin.DebugMode)
	}
	router := gin.Default()

	/*
		Setup in-memory cache
	*/
	cacher := cache.New(time.Minute*30, time.Minute*5)

	/*
		Fire up the UDP listener.
	*/
	dnsConn, err := dns.NewUDPConn(localDNSAddr, upstreamDnsAddr, logger)
	if err != nil {
		logger.Fatalf("could not listen on port (%v) UDP: %v", localDNSAddr, err)
	}
	dnsConn.WithConfig(dns.Config{Logger: logger})
	registerOnClose(dnsConn)

	/*
		Setup the DNS service
	*/
	dnsService := dns.NewService(repo, cacher, logger, adBlockService)

	/*
		Process UDP messages.
	*/
	cctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go processUdpMessages(cctx, dnsConn, dnsService, logger)
	logger.Infof("Waiting for messages via UDP on port (%v)...", localDNSAddr)

	/*
		HTTP routes attachment.
	*/
	handler := httpHandler.New(httpHandler.Config{
		Logger:        logger,
		Repository:    repo,
		ParserService: adBlockService,
	})
	router.POST("/block", handler.AddToBlocklist)
	router.GET("/health", handler.Health)
	router.GET("/data/db", handler.DataDB)
	router.GET("/data/files", handler.DataFiles)
	router.GET("/query/*url", handler.ShouldBlock)

	srv := http.Server{
		Addr:    localHTTPPort,
		Handler: router,
	}

	go func() {
		logger.Infof("Started HTTP server on port (%v)", localHTTPPort)
		err := srv.ListenAndServe()
		switch {
		case err == http.ErrServerClosed:
			logger.Info(err)
		case err != nil:
			logger.Fatalf("HTTP listener closed with an error: %v", err)
		}
	}()

	go func() {
		pport := fromEnv("HTTP_PPROF_LISTEN_ADDR", defaultEnvVars.HTTP_PPROF_LISTEN_ADDR)

		r := http.NewServeMux()
		r.HandleFunc("/debug/pprof/", pprof.Index)
		r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		r.HandleFunc("/debug/pprof/profile", pprof.Profile)
		r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		r.HandleFunc("/debug/pprof/trace", pprof.Trace)

		logger.Infof("Attached pprof mux on address (%v)", pport)
		err := http.ListenAndServe(pport, r)
		switch {
		case err == http.ErrServerClosed:
			logger.Info(err)
		case err != nil:
			log.Fatalf("pprof mux error: %v", err)
		}
	}()

	ips, err := getIPAddr()
	if err != nil {
		logger.Fatalf("could not read the IPv4 address: %v", err)
	}
	logger.Infof("Running instance. Hostname: %v | Go: %v | IPv4: %v", hostname, runtime.Version(), ips[0])

	sig := make(chan os.Signal)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	logger.Infof("Stopping servers, received signal: %v", <-sig)

	ctxCancel, cCancel := context.WithTimeout(cctx, time.Second*5)
	defer cCancel()

	cancel()
	closeAll()

	if err := srv.Shutdown(ctxCancel); err != nil {
		logger.Fatalf("could not close the HTTP server gracefully: %v", err)
	}
}
