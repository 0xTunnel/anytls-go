package main

import (
	"anytls/internal/config"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestResolveLogLevel(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "default", input: "", want: "info"},
		{name: "warning alias", input: "warning", want: "warn"},
		{name: "debug passthrough", input: "debug", want: "debug"},
		{name: "error passthrough", input: "error", want: "error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveLogLevel(tt.input); got != tt.want {
				t.Fatalf("resolveLogLevel() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConfigureLoggingCreatesFileAndFormatter(t *testing.T) {
	tempDir := t.TempDir()
	oldOutput := logrus.StandardLogger().Out
	oldFormatter := logrus.StandardLogger().Formatter
	defer logrus.SetOutput(oldOutput)
	defer logrus.SetFormatter(oldFormatter)

	logFile, err := configureLogging(&config.NodeConfig{LogFileDir: tempDir})
	if err != nil {
		t.Fatalf("configureLogging() error = %v", err)
	}
	defer logFile.Close()

	logPath := filepath.Join(tempDir, "anytls-server.log")
	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("configureLogging() did not create log file: %v", err)
	}

	formatter, ok := logrus.StandardLogger().Formatter.(*logrus.TextFormatter)
	if !ok {
		t.Fatalf("configureLogging() formatter type = %T, want *logrus.TextFormatter", logrus.StandardLogger().Formatter)
	}
	if !formatter.FullTimestamp {
		t.Fatal("configureLogging() did not enable full timestamp")
	}
	if !formatter.DisableColors {
		t.Fatal("configureLogging() did not disable colors")
	}
}

func TestConfigureLoggingNoopWhenConfigMissing(t *testing.T) {
	oldOutput := logrus.StandardLogger().Out
	defer logrus.SetOutput(oldOutput)
	logrus.SetOutput(io.Discard)

	logFile, err := configureLogging(nil)
	if err != nil {
		t.Fatalf("configureLogging(nil) error = %v", err)
	}
	if logFile != nil {
		t.Fatal("configureLogging(nil) returned unexpected file")
	}

	logFile, err = configureLogging(&config.NodeConfig{})
	if err != nil {
		t.Fatalf("configureLogging(empty) error = %v", err)
	}
	if logFile != nil {
		t.Fatal("configureLogging(empty) returned unexpected file")
	}
}

func TestEventLoggerMergesFields(t *testing.T) {
	entry := eventLogger("node", logrus.Fields{"user_count": 3, "event": "override"}, "sync_snapshot")

	if got := entry.Data["component"]; got != "node" {
		t.Fatalf("eventLogger() component = %v, want %q", got, "node")
	}
	if got := entry.Data["event"]; got != "override" {
		t.Fatalf("eventLogger() event = %v, want %q", got, "override")
	}
	if got := entry.Data["user_count"]; got != 3 {
		t.Fatalf("eventLogger() user_count = %v, want %d", got, 3)
	}
	if _, ok := entry.Data["missing"]; ok {
		t.Fatal("eventLogger() returned unexpected field")
	}
}
