package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"
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
	fmt.Printf("\tHeap: alloc (%v) | idle (%v) | in use (%v) | obj (%v) | released (%v)\n", bToMb(m.HeapAlloc), bToMb(m.HeapIdle), bToMb(m.HeapInuse), m.HeapObjects, bToMb(m.HeapReleased))
}
