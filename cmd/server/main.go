package main

import (
	"anytls/internal/config"
	"anytls/internal/node/state"
	"anytls/internal/ppanel"
	"anytls/util"
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

const shutdownTimeout = 10 * time.Second

func main() {
	configPath := flag.String("c", "", "path to node toml file")
	flag.Parse()

	if *configPath == "" {
		logrus.Fatalln("please set -c node toml file")
	}

	nodeConfig, err := config.LoadNodeConfig(*configPath)
	if err != nil {
		logrus.Fatalln("load node config:", err)
	}

	logFile, err := configureLogging(nodeConfig)
	if err != nil {
		logrus.Fatalln("configure logging:", err)
	}
	if logFile != nil {
		defer logFile.Close()
	}

	logLevel, err := logrus.ParseLevel(resolveLogLevel(nodeConfig.LogLevel))
	if err != nil {
		logrus.Fatalln("parse log level:", err)
	}
	logrus.SetLevel(logLevel)

	tlsCert, err := tls.LoadX509KeyPair(nodeConfig.TLSCertFile, nodeConfig.TLSKeyFile)
	if err != nil {
		logrus.Fatalln("load tls certificate:", err)
	}
	tlsConfig := &tls.Config{
		GetCertificate: func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
			return &tlsCert, nil
		},
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	client, err := ppanel.NewClient(nodeConfig.PanelURL, nodeConfig.ServerID, nodeConfig.SecretKey)
	if err != nil {
		logrus.Fatalln("create ppanel client:", err)
	}
	snapshot, err := fetchNodeSnapshot(ctx, client)
	if err != nil {
		logrus.Fatalln("fetch node snapshot:", err)
	}
	listenAddr := resolveListenAddr("", snapshot.Port)
	server := NewNodeServer(tlsConfig, state.NewStore(snapshot), state.NewDeviceTracker(), state.NewTrafficAggregator(), client)

	logrus.Infoln("[Server]", util.ProgramVersionName)
	logrus.Infoln("[Server] Listening TCP", listenAddr)

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		logrus.Fatalln("listen server tcp:", err)
	}

	runtime := newServerRuntime(server, listener)
	runtime.StartBackgroundTasks(ctx)

	serveErrCh := make(chan error, 1)
	go func() {
		serveErrCh <- runtime.Serve(ctx)
	}()

	var serveErr error
	var serveReturned bool

	select {
	case <-ctx.Done():
		logrus.Infoln("[Server] shutdown signal received")
	case serveErr = <-serveErrCh:
		serveReturned = true
		if serveErr != nil {
			logrus.Errorln("serve:", serveErr)
		}
		stop()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := runtime.Shutdown(shutdownCtx); err != nil {
		logrus.Errorln("shutdown:", err)
	}

	if !serveReturned {
		serveErr = <-serveErrCh
	}
	if serveErr != nil && !errors.Is(serveErr, context.Canceled) {
		logrus.Fatalln("server stopped with error:", serveErr)
	}
}

func resolveLogLevel(level string) string {
	if level == "" {
		return "info"
	}
	if level == "warning" {
		return "warn"
	}
	return level
}

func configureLogging(nodeConfig *config.NodeConfig) (*os.File, error) {
	if nodeConfig == nil || nodeConfig.LogFileDir == "" {
		return nil, nil
	}
	if err := os.MkdirAll(nodeConfig.LogFileDir, 0755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	logPath := filepath.Join(nodeConfig.LogFileDir, "anytls-server.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	logrus.SetOutput(io.MultiWriter(os.Stdout, logFile))
	return logFile, nil
}
