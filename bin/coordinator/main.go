package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"path/filepath"

	"gitlab.com/justnurik/distbuild/bin"
	"gitlab.com/justnurik/distbuild/pkg/dist"
	"gitlab.com/justnurik/distbuild/pkg/filecache"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type args struct {
	port string

	root string

	logPath       string
	fileCachePath string

	logLevelStr string
	logLevel    zapcore.Level
}

func (a *args) parse() {
	flag.StringVar(&a.port, "port", "8080", "port for coordinator")
	flag.StringVar(&a.root, "root", ".", "prefix all path")
	flag.StringVar(&a.logPath, "log-file", "logs", "log file path")
	flag.StringVar(&a.fileCachePath, "file-cache", "filecache", "file cache path")
	flag.StringVar(&a.logLevelStr, "log-level", "error", "log level")

	flag.Parse()

	switch a.logLevelStr {
	case "debug":
		a.logLevel = zap.DebugLevel
	case "info":
		a.logLevel = zap.InfoLevel
	case "warn":
		a.logLevel = zap.WarnLevel
	case "error":
		a.logLevel = zap.ErrorLevel
	case "dpanic":
		a.logLevel = zap.DPanicLevel
	case "panic":
		a.logLevel = zap.PanicLevel
	case "fatal":
		a.logLevel = zap.FatalLevel
	default:
		log.Fatal("log-level in (debug, info, warn, error, dpanic, panic, fatal)")
	}

	a.fileCachePath = filepath.Join(a.root, a.fileCachePath)
	a.logPath = filepath.Join(a.root, a.logPath)
}

func main() {
	var a args
	a.parse()

	fmt.Println("----------start----------")
	fmt.Println(a)

	addr := "127.0.0.1:" + a.port
	logger, err := bin.NewLogger(a.logPath, a.logLevel)
	if err != nil {
		log.Fatal("new logger create fail")
	}

	cache, err := filecache.New(a.fileCachePath)
	if err != nil {
		logger.Fatal("new file cache create fail")
	}

	coordinator := dist.NewCoordinator(logger, cache)
	defer coordinator.Stop()

	router := http.NewServeMux()
	router.Handle("/coordinator/", http.StripPrefix("/coordinator", coordinator))

	server := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	lsn, err := net.Listen("tcp", server.Addr)
	if err != nil {
		logger.Fatal("new listner create fail",
			zap.Error(err), zap.Any("config", a))
	}

	if err := server.Serve(lsn); err != http.ErrServerClosed {
		logger.Fatal("http server stopped",
			zap.Error(err), zap.Any("config", a))
	}

}
