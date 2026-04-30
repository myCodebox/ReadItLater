package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"math/rand"

	"go.uber.org/zap"
)

var httpClient = &http.Client{}
var sqlCache *SQLCache
var logger *zap.SugaredLogger

func main() {
	// parse debug flag and env
	debugFlag := flag.Bool("debug", false, "enable debug logging to console")
	flag.Parse()
	envDebug := strings.ToLower(os.Getenv("READITLATER_DEBUG"))
	enabled := *debugFlag || envDebug == "1" || envDebug == "true"

	// initialize zap
	var zapLogger *zap.Logger
	var err error
	if enabled {
		// development config with stacktraces, console-friendly
		cfg := zap.NewDevelopmentConfig()
		zapLogger, err = cfg.Build()
		if err != nil {
			panic(err)
		}
	} else {
		cfg := zap.NewProductionConfig()
		// keep human-readable console output for production? leave defaults
		zapLogger, err = cfg.Build()
		if err != nil {
			panic(err)
		}
	}
	defer zapLogger.Sync()
	logger = zapLogger.Sugar()
	// seed rand for jitter
	rand.Seed(time.Now().UnixNano())

	addr := getServerAddr()
	mux := http.NewServeMux()
	mux.HandleFunc("/", handler)
	// Serve static assets (CSS/JS)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	fmt.Printf("Server läuft auf http://%s\n", addr)
	// Default http client timeout
	httpClient.Timeout = 15 * time.Second
	// Cookie jar so prefetches can set cookies for origin
	if httpClient.Jar == nil {
		if jar, err := cookiejar.New(nil); err == nil {
			httpClient.Jar = jar
		}
	}
	// Configure transport for connection pooling and timeouts
	httpClient.Transport = &http.Transport{
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
	}

	// initialize sqlite cache (persisted)
	var cacheErr error
	sqlCache, cacheErr = NewSQLCache("cache/readitlater.db", 10*time.Minute, 1000)
	if cacheErr != nil {
		logger.Warnw("Cache init failed, continuing without cache", "err", cacheErr)
	} else {
		defer func() {
			if cerr := sqlCache.Close(); cerr != nil {
				logger.Warnw("Error closing cache", "err", cerr)
			}
		}()
	}

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Start server in background
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("ListenAndServe: %v", err)
		}
	}()

	// Graceful shutdown on SIGINT/SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Infow("Shutting down server")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Fatalf("Server Shutdown: %v", err)
	}
	logger.Infow("Server stopped")
}
