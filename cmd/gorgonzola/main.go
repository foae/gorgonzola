package main

import (
	"context"
	"fmt"
	"github.com/foae/gorgonzola/adblock"
	"go.uber.org/zap"
	"log"
	"net/http"
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
	envDev  = "dev"
	envProd = "prod"
)

func main() {
	/*
		ENV vars
	*/
	upstreamDNS := mustGetEnv("UPSTREAM_DNS_SERVER_IP")
	localDNSPort := mustGetEnvInt("DNS_LISTEN_PORT")
	localHTTPPort := mustGetEnv("HTTP_LISTEN_ADDR")
	env := os.Getenv("ENV")
	if env != envDev {
		env = envProd
	}

	ctx := context.Background()
	hostname, err := os.Hostname()
	if err != nil {
		hostname = fmt.Sprintf("pid-%v-ts-%v", os.Getpid(), time.Now().UnixNano())
	}

	/*
		Logging
	*/
	var logger *zap.SugaredLogger
	switch env {
	case envDev:
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
		Cache files locally from the provided URLs
	*/
	//urls := []string{
	//	"https://austinhuang.me/0131-block-list/list.txt",
	//	"https://280blocker.net/files/280blocker_adblock_nanj_supp.txt",
	//	"https://raw.githubusercontent.com/EnergizedProtection/block/master/porn/formats/filter",
	//	"https://raw.githubusercontent.com/DandelionSprout/adfilt/master/NorwegianExperimentalList%20alternate%20versions/NordicFiltersABP.txt",
	//	"https://raw.githubusercontent.com/Crystal-RainSlide/AdditionalFiltersCN/master/all.txt",
	//	"https://raw.githubusercontent.com/StevenBlack/hosts/master/alternates/fakenews-gambling-porn/hosts", // hosts file
	//}
	//for _, u := range urls {
	//	if err := repo.DownloadFromURL(u); err != nil {
	//		logger.Debugf("error for (%v): %v", u, err)
	//	}
	//}

	/*
		Read files from local storage
	*/
	fileList, err := repo.StoredFilesList()
	if err != nil {
		log.Fatalf("could not list files: %v", err)
	}

	/*
		LOAD AdBlock Plus providers
	*/
	for _, fl := range fileList {
		if err := adBlockService.LoadAdBlockPlusProviders(fl); err != nil {
			logger.Errorf("could not load provider: %v", err)
		}
	}

	/*
		Setup HTTP Router
	*/
	switch env {
	case envDev:
		gin.SetMode(gin.DebugMode)
	case envProd:
		gin.SetMode(gin.ReleaseMode)
	default:
		gin.SetMode(gin.TestMode)
	}
	router := gin.Default()

	/*
		Setup in-memory cache
	*/
	cacher := cache.New(time.Minute*30, time.Minute*5)

	/*
		Fire up the UDP listener.
	*/
	dnsConn, err := dns.NewUDPConn(localDNSPort, upstreamDNS)
	if err != nil {
		logger.Fatalf("could not listen on port (%v) UDP: %v", localDNSPort, err)
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
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go processUdpMessages(cctx, dnsConn, dnsService, logger)
	logger.Infof("Waiting for messages via UDP on port (%v)...", localDNSPort)

	/*
		HTTP routes attachment.
	*/
	handler := httpHandler.New(httpHandler.Config{
		Logger:        logger,
		Repository:    repo,
		ParserService: adBlockService,
	})
	router.POST("/blocklist", handler.AddToBlocklist)
	router.GET("/health", handler.Health)
	router.GET("/data", handler.Data)
	router.GET("/query/:url", handler.ShouldBlock)

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
