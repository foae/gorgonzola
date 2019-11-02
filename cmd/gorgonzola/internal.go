package main

import (
	"fmt"
	"github.com/blendle/zapdriver"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
	"log"
	"net"
	"os"
	"strconv"
	"time"
)

var (
	toClose []io.Closer
)

func registerOnClose(closer io.Closer) {
	toClose = append(toClose, closer)
}

func closeAll() {
	for _, c := range toClose {
		if err := c.Close(); err != nil {
			log.Printf("error closing registered Closer: %v", err)
		}
	}
}

func fromEnv(envParam string, defaultValue string) string {
	v := os.Getenv(envParam)
	if v != "" {
		return v
	}

	if defaultValue == "" {
		log.Fatalf("envParam (%v) needs a value from ENV or a default. Both were empty, will stop here.", envParam)
	}

	return defaultValue
}

func fromEnvInt(envParam string, defaultValue int) int {
	v := fromEnv(envParam, "def")
	switch {
	case v == "def" && defaultValue == 0:
		log.Fatalf("envParam (%v) needs a value from ENV or a default. Both were empty, will stop here.", envParam)
	case v != "def":
		i, err := strconv.Atoi(v)
		if err != nil {
			log.Fatalf("envParam (%v): could not convert needed value (%v) from string to int: %v", envParam, v, err)
		}

		return i
	}

	return defaultValue
}

func newProductionLogger() (*zap.SugaredLogger, error) {
	zapConfig := zapdriver.NewProductionConfig()
	zapConfig.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	logger, err := zapConfig.Build(zapdriver.WrapCore())
	if err != nil {
		return nil, err
	}

	return logger.Sugar(), nil
}

func newDevelopmentLogger() (*zap.SugaredLogger, error) {
	encConfig := zap.NewDevelopmentEncoderConfig()
	encConfig.LineEnding = "\n"
	encConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	encConfig.EncodeTime = devLogTimeEncoder
	zapConfig := zap.Config{
		Level:            zap.NewAtomicLevelAt(zap.DebugLevel),
		Development:      true,
		Encoding:         "console",
		EncoderConfig:    encConfig,
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}

	logger, err := zapConfig.Build()
	if err != nil {
		return nil, err
	}

	return logger.Sugar(), nil
}

func devLogTimeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString("\x1b[90m" + t.Format(time.RFC3339) + "\x1b[0m")
}

func getIPAddr() ([]string, error) {
	found := make([]string, 0)

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip != nil && !ip.IsLoopback() && ip.To4() != nil {
				found = append(found, ip.String())
			}
		}
	}

	if len(found) == 0 {
		return nil, fmt.Errorf("no valid IPv4 addresses found")
	}

	return found, nil
}
