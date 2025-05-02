package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"path/filepath"

	"gitlab.com/justnurik/distbuild/bin"
	"gitlab.com/justnurik/distbuild/pkg/api"
	"gitlab.com/justnurik/distbuild/pkg/artifact"
	"gitlab.com/justnurik/distbuild/pkg/filecache"
	"gitlab.com/justnurik/distbuild/pkg/worker"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type args struct {
	id string

	port string

	root string

	logPath string

	fileCachePath     string
	artifactCachePath string

	coordinatorEndpoint string

	logLevelStr string
	logLevel    zapcore.Level
}

func (a *args) parse() {
	flag.StringVar(&a.id, "id", "0", "worker id")
	flag.StringVar(&a.port, "port", "8080", "port for coordinator")
	flag.StringVar(&a.root, "root", ".", "prefix all path")
	flag.StringVar(&a.logPath, "log-file", "logs", "log file path")
	flag.StringVar(&a.fileCachePath, "file-cache", "filecache", "file cache path")
	flag.StringVar(&a.artifactCachePath, "artifacts", "artifacts", "artifact cache path")
	flag.StringVar(&a.coordinatorEndpoint, "coordinator", "", "coordinator endpoint")
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
	a.artifactCachePath = filepath.Join(a.root, a.artifactCachePath)
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

	fileCache, err := filecache.New(a.fileCachePath)
	if err != nil {
		logger.Fatal("new file cache create fail",
			zap.Error(err), zap.Any("config", a))
	}

	artifacts, err := artifact.NewCache(a.artifactCachePath)
	if err != nil {
		logger.Fatal("new artifact cache create fail",
			zap.Error(err), zap.Any("config", a))
	}

	endpoint := fmt.Sprintf("%s/worker/%s", addr, a.id)
	worker := worker.New(api.WorkerID(endpoint), a.coordinatorEndpoint, logger, fileCache, artifacts)

	router := http.NewServeMux()
	router.Handle(fmt.Sprintf("/worker/%s/", a.id), http.StripPrefix("/worker/"+a.id, worker))

	server := &http.Server{
		Addr:    addr,
		Handler: router,
	}

	go func() {
		ctx := context.Background()
		if err := worker.Run(ctx); err != nil {
			logger.Fatal("worker stopped",
				zap.Error(err), zap.Any("config", a))
		}
	}()

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
