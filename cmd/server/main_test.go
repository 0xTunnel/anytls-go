package main

import (
	"anytls/internal/config"
	"anytls/internal/node/state"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	oldLogWriterNow := logWriterNow
	oldLogTimeZone := logTimeZone
	oldLogUseColor := logUseColor
	oldStdoutIsTerminal := stdoutIsTerminal
	defer logrus.SetOutput(oldOutput)
	defer logrus.SetFormatter(oldFormatter)
	defer func() { logWriterNow = oldLogWriterNow }()
	defer setLogFormatState(oldLogTimeZone, oldLogUseColor)
	defer func() { stdoutIsTerminal = oldStdoutIsTerminal }()
	stdoutIsTerminal = func() bool { return true }
	logWriterNow = func() time.Time { return time.Date(2026, 3, 7, 0, 0, 0, 0, time.UTC) }

	logFile, err := configureLogging(&config.NodeConfig{LogFileDir: tempDir, TimeZone: "Asia/Shanghai"})
	if err != nil {
		t.Fatalf("configureLogging() error = %v", err)
	}
	defer logFile.Close()

	logPath := filepath.Join(tempDir, "anytls-server-2026-03-07.log")
	if _, err := os.Stat(logPath); err != nil {
		t.Fatalf("configureLogging() did not create log file: %v", err)
	}

	formatter, ok := logrus.StandardLogger().Formatter.(*consoleFormatter)
	if !ok {
		t.Fatalf("configureLogging() formatter type = %T, want *consoleFormatter", logrus.StandardLogger().Formatter)
	}
	if formatter == nil {
		t.Fatal("configureLogging() returned nil formatter")
	}
	if configuredLogTimeZone() != "Asia/Shanghai" {
		t.Fatalf("configuredLogTimeZone() = %q, want %q", configuredLogTimeZone(), "Asia/Shanghai")
	}
	if configuredLogColorEnabled() {
		t.Fatal("configuredLogColorEnabled() = true, want false when log file is enabled")
	}
}

func TestConfigureLoggingNoopWhenConfigMissing(t *testing.T) {
	oldOutput := logrus.StandardLogger().Out
	oldLogTimeZone := logTimeZone
	oldLogUseColor := logUseColor
	oldStdoutIsTerminal := stdoutIsTerminal
	defer logrus.SetOutput(oldOutput)
	defer setLogFormatState(oldLogTimeZone, oldLogUseColor)
	defer func() { stdoutIsTerminal = oldStdoutIsTerminal }()
	stdoutIsTerminal = func() bool { return true }
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
	if configuredLogTimeZone() != "UTC+8" {
		t.Fatalf("configuredLogTimeZone() = %q, want default UTC+8", configuredLogTimeZone())
	}
	if !configuredLogColorEnabled() {
		t.Fatal("configuredLogColorEnabled() = false, want true without file logging")
	}
}

func TestConfigLoadedMessageUsesConfiguredFormatter(t *testing.T) {
	oldOutput := logrus.StandardLogger().Out
	oldFormatter := logrus.StandardLogger().Formatter
	oldLogTimeZone := logTimeZone
	oldLogUseColor := logUseColor
	oldStdoutIsTerminal := stdoutIsTerminal
	defer logrus.SetOutput(oldOutput)
	defer logrus.SetFormatter(oldFormatter)
	defer setLogFormatState(oldLogTimeZone, oldLogUseColor)
	defer func() { stdoutIsTerminal = oldStdoutIsTerminal }()
	stdoutIsTerminal = func() bool { return false }

	var buffer bytes.Buffer
	logrus.SetOutput(&buffer)

	if _, err := configureLogging(&config.NodeConfig{TimeZone: "UTC+8"}); err != nil {
		t.Fatalf("configureLogging() error = %v", err)
	}

	entry := eventLogger("server", logrus.Fields{
		"config_path": "/etc/anytls/node.toml",
		"node_id":     int64(1),
	}, "load_config")
	entry.Level = logrus.InfoLevel
	entry.Message = "node config loaded"
	entry.Time = time.Date(2026, 3, 6, 12, 21, 20, 0, time.UTC)
	formatted, err := logrus.StandardLogger().Formatter.Format(entry)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	if _, err := logrus.StandardLogger().Out.Write(formatted); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	output := buffer.String()
	if !strings.Contains(output, "2026/03/06 20:21:20 INFO - node config loaded") {
		t.Fatalf("formatted output = %q, want unified console formatter", output)
	}
	if strings.Contains(output, "level=info") || strings.Contains(output, `time="`) {
		t.Fatalf("formatted output = %q, want no default logrus text formatter fields", output)
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

func TestDiagnosticLoggingEnabled(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "empty", value: "", want: false},
		{name: "numeric true", value: "1", want: true},
		{name: "text true", value: "true", want: true},
		{name: "yes alias", value: "yes", want: true},
		{name: "on alias", value: "on", want: true},
		{name: "numeric false", value: "0", want: false},
		{name: "text false", value: "false", want: false},
		{name: "invalid", value: "verbose", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ANYTLS_DEBUG_VERBOSE", tt.value)
			if got := diagnosticLoggingEnabled(); got != tt.want {
				t.Fatalf("diagnosticLoggingEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSnapshotSignatureStableAndSensitiveToChanges(t *testing.T) {
	base := &state.Snapshot{
		Protocol:               "anytls",
		Port:                   443,
		PullInterval:           time.Minute,
		PushInterval:           2 * time.Minute,
		TrafficReportThreshold: 4096,
		PaddingScheme:          "pad-v1",
		UsersByID: map[int64]state.User{
			1: {ID: 1, UUID: "user-1", SpeedLimit: 10, DeviceLimit: 1},
			2: {ID: 2, UUID: "user-2", SpeedLimit: 20, DeviceLimit: 2},
		},
	}

	clone := &state.Snapshot{
		Protocol:               base.Protocol,
		Port:                   base.Port,
		PullInterval:           base.PullInterval,
		PushInterval:           base.PushInterval,
		TrafficReportThreshold: base.TrafficReportThreshold,
		PaddingScheme:          base.PaddingScheme,
		UsersByID: map[int64]state.User{
			2: {ID: 2, UUID: "user-2", SpeedLimit: 20, DeviceLimit: 2},
			1: {ID: 1, UUID: "user-1", SpeedLimit: 10, DeviceLimit: 1},
		},
	}

	if got, want := snapshotSignature(base), snapshotSignature(clone); got != want {
		t.Fatalf("snapshotSignature() = %q, want stable signature %q", got, want)
	}

	clone.UsersByID[2] = state.User{ID: 2, UUID: "user-2", SpeedLimit: 30, DeviceLimit: 2}
	if snapshotSignature(base) == snapshotSignature(clone) {
		t.Fatal("snapshotSignature() did not change after user data change")
	}
}

func TestLogFormatterConvertsToUTC8(t *testing.T) {
	oldLogTimeZone := logTimeZone
	oldLogUseColor := logUseColor
	defer setLogFormatState(oldLogTimeZone, oldLogUseColor)
	setLogFormatState(time.FixedZone("UTC+8", 8*60*60), false)

	formatter, ok := newLogFormatter().(*consoleFormatter)
	if !ok {
		t.Fatalf("newLogFormatter() type = %T, want *consoleFormatter", newLogFormatter())
	}

	entry := logrus.NewEntry(logrus.New())
	entry.Level = logrus.InfoLevel
	entry.Message = "hello"
	entry.Time = time.Date(2026, 3, 6, 9, 10, 27, 0, time.UTC)

	formatted, err := formatter.Format(entry)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	output := string(formatted)
	if !strings.Contains(output, `2026/03/06 17:10:27 INFO - hello`) {
		t.Fatalf("formatted time = %q, want console timestamp and level prefix", output)
	}
}

func TestLogFormatterOmitsDataFieldsByDefault(t *testing.T) {
	oldLogTimeZone := logTimeZone
	oldLogUseColor := logUseColor
	defer setLogFormatState(oldLogTimeZone, oldLogUseColor)
	setLogFormatState(time.FixedZone("UTC+8", 8*60*60), false)

	formatter := newLogFormatter()
	entry := logrus.NewEntry(logrus.New())
	entry.Level = logrus.DebugLevel
	entry.Message = "node config loaded"
	entry.Time = time.Date(2026, 3, 6, 9, 10, 27, 0, time.UTC)
	entry.Data = logrus.Fields{
		"conn_tag":  "{1:49208}",
		"remote_ip": "39.64.247.198",
		"target":    "www.google.com:443",
		"event":     "load_config",
		"node_id":   1,
		"component": "server",
		"extra":     "z",
	}

	formatted, err := formatter.Format(entry)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	output := string(formatted)
	want := "2026/03/06 17:10:27 DEBUG - node config loaded"
	if !strings.Contains(output, want) {
		t.Fatalf("formatted output = %q, want substring %q", output, want)
	}
	if strings.Contains(output, "conn_tag=") || strings.Contains(output, "remote_ip=") || strings.Contains(output, "target=") {
		t.Fatalf("formatted output = %q, want soga style without structured field dump", output)
	}
	if strings.Contains(output, "\x1b[") {
		t.Fatalf("formatted output = %q, want no color when disabled", output)
	}
}

func TestLogFormatterIncludesErrorField(t *testing.T) {
	oldLogTimeZone := logTimeZone
	oldLogUseColor := logUseColor
	defer setLogFormatState(oldLogTimeZone, oldLogUseColor)
	setLogFormatState(time.FixedZone("UTC+8", 8*60*60), false)

	formatter := newLogFormatter()
	entry := logrus.NewEntry(logrus.New())
	entry.Level = logrus.ErrorLevel
	entry.Message = "dial failed"
	entry.Time = time.Date(2026, 3, 6, 9, 10, 27, 0, time.UTC)
	entry.Data = logrus.Fields{
		logrus.ErrorKey: "i/o timeout",
	}

	formatted, err := formatter.Format(entry)
	if err != nil {
		t.Fatalf("Format() error = %v", err)
	}
	output := string(formatted)
	if !strings.Contains(output, "ERROR - dial failed error: i/o timeout") {
		t.Fatalf("formatted output = %q, want error suffix", output)
	}
}

func TestFormatLevelUsesColorWhenEnabled(t *testing.T) {
	oldLogUseColor := logUseColor
	oldLogTimeZone := logTimeZone
	defer setLogFormatState(oldLogTimeZone, oldLogUseColor)
	setLogFormatState(oldLogTimeZone, true)

	formatted := formatLevel(logrus.ErrorLevel, true)
	if !strings.Contains(formatted, "ERROR") {
		t.Fatalf("formatLevel() = %q, want error label", formatted)
	}
	if !strings.Contains(formatted, "\x1b[") {
		t.Fatalf("formatLevel() = %q, want ansi color code", formatted)
	}
	if !strings.Contains(formatted, "\x1b[0m") {
		t.Fatalf("formatLevel() = %q, want ansi reset code", formatted)
	}
}

func TestShouldUseLogColorRequiresTerminal(t *testing.T) {
	oldStdoutIsTerminal := stdoutIsTerminal
	defer func() { stdoutIsTerminal = oldStdoutIsTerminal }()
	stdoutIsTerminal = func() bool { return false }

	if shouldUseLogColor(&config.NodeConfig{}) {
		t.Fatal("shouldUseLogColor() = true, want false when stdout is not a terminal")
	}
}
