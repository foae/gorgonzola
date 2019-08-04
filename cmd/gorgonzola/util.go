package main

import (
	"fmt"
	"github.com/blendle/zapdriver"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"log"
	"os"
	"runtime"
	"strconv"
	"time"
)

func mustGetEnv(value string) string {
	v := os.Getenv(value)
	if v == "" {
		log.Fatalf("could not retrieve needed value (%v) from the environment", value)
	}

	return v
}

func mustGetEnvInt(value string) int {
	v := os.Getenv(value)
	if v == "" {
		log.Fatalf("could not retrieve needed value (%v) from the environment", value)
	}

	i, err := strconv.Atoi(v)
	if err != nil {
		log.Fatalf("could not convert needed value (%v) from string to int: %v", value, err)
	}

	return i
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

func printMemUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	fmt.Printf("Alloc: %v MB", bToMb(m.Alloc))
	fmt.Printf("\tTotalAlloc: %v MB", bToMb(m.TotalAlloc))
	fmt.Printf("\tSys: %v MB", bToMb(m.Sys))
	fmt.Printf("\tNumGC: %v", m.NumGC)
	fmt.Printf("\tHeap: alloc (%v) | in use (%v) | object (%v) | released (%v)\n",
		bToMb(m.HeapAlloc),
		bToMb(m.HeapInuse),
		m.HeapObjects,
		bToMb(m.HeapReleased),
	)
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
