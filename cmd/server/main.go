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
		eventLogger("server", nil, "missing_config_path").Fatal("please set -c node toml file")
	}

	nodeConfig, err := config.LoadNodeConfig(*configPath)
	if err != nil {
		eventLogger("server", nil, "load_config_failed").WithError(err).Fatal("load node config failed")
	}
	eventLogger("server", logrus.Fields{
		"config_path": nodeConfig.Path,
		"node_id":     nodeConfig.ServerID,
	}, "load_config").Info("node config loaded")

	logFile, err := configureLogging(nodeConfig)
	if err != nil {
		eventLogger("server", logrus.Fields{"node_id": nodeConfig.ServerID}, "configure_logging_failed").WithError(err).Fatal("configure logging failed")
	}
	if logFile != nil {
		defer logFile.Close()
	}

	logLevel, err := logrus.ParseLevel(resolveLogLevel(nodeConfig.LogLevel))
	if err != nil {
		eventLogger("server", logrus.Fields{"node_id": nodeConfig.ServerID, "log_level": nodeConfig.LogLevel}, "parse_log_level_failed").WithError(err).Fatal("parse log level failed")
	}
	logrus.SetLevel(logLevel)

	tlsCert, err := tls.LoadX509KeyPair(nodeConfig.TLSCertFile, nodeConfig.TLSKeyFile)
	if err != nil {
		eventLogger("server", logrus.Fields{"node_id": nodeConfig.ServerID}, "load_tls_cert_failed").WithError(err).Fatal("load tls certificate failed")
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
		eventLogger("server", logrus.Fields{"node_id": nodeConfig.ServerID}, "create_panel_client_failed").WithError(err).Fatal("create panel client failed")
	}
	snapshot, err := fetchNodeSnapshot(ctx, client)
	if err != nil {
		eventLogger("node", logrus.Fields{"node_id": nodeConfig.ServerID}, "initial_snapshot_failed").WithError(err).Fatal("fetch initial node snapshot failed")
	}
	eventLogger("node", logrus.Fields{
		"node_id":       nodeConfig.ServerID,
		"user_count":    len(snapshot.UsersByID),
		"port":          snapshot.Port,
		"pull_interval": snapshot.PullInterval.String(),
		"push_interval": snapshot.PushInterval.String(),
	}, "initial_snapshot").Info("initial node snapshot loaded")
	listenAddr := resolveListenAddr("", snapshot.Port)
	server := NewNodeServer(tlsConfig, state.NewStore(snapshot), state.NewDeviceTracker(), state.NewTrafficAggregator(), client)

	eventLogger("server", logrus.Fields{
		"node_id":     nodeConfig.ServerID,
		"listen_addr": listenAddr,
		"log_level":   logLevel.String(),
		"transport":   "tcp+tls",
		"version":     util.ProgramVersionName,
	}, "startup").Info("server initialized")

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		eventLogger("server", logrus.Fields{"listen_addr": listenAddr, "node_id": nodeConfig.ServerID}, "listen_failed").WithError(err).Fatal("listen server tcp failed")
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
		eventLogger("server", logrus.Fields{"node_id": nodeConfig.ServerID}, "shutdown_signal").Info("shutdown signal received")
	case serveErr = <-serveErrCh:
		serveReturned = true
		if serveErr != nil {
			eventLogger("runtime", logrus.Fields{"node_id": nodeConfig.ServerID}, "serve_failed").WithError(serveErr).Error("serve returned with error")
		}
		stop()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := runtime.Shutdown(shutdownCtx); err != nil {
		eventLogger("runtime", logrus.Fields{"node_id": nodeConfig.ServerID}, "shutdown_failed").WithError(err).Error("shutdown failed")
	}

	if !serveReturned {
		serveErr = <-serveErrCh
	}
	if serveErr != nil && !errors.Is(serveErr, context.Canceled) {
		eventLogger("server", logrus.Fields{"node_id": nodeConfig.ServerID}, "stopped_with_error").WithError(serveErr).Fatal("server stopped with error")
	}

	eventLogger("server", logrus.Fields{"node_id": nodeConfig.ServerID}, "shutdown_complete").Info("server stopped cleanly")
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
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		DisableColors: true,
	})
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
