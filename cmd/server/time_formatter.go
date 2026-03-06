package main

import (
	"anytls/internal/config"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

const logTimestampFormat = "2006/01/02 15:04:05"

var logTimeZone = time.FixedZone("UTC+8", 8*60*60)
var logUseColor bool
var logFormatStateMu sync.RWMutex
var stdoutIsTerminal = func() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

type consoleFormatter struct{}

func newLogFormatter() logrus.Formatter {
	return &consoleFormatter{}
}

func (f *consoleFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	timeZone, useColor := currentLogFormatState()
	var builder strings.Builder
	builder.Grow(160)
	builder.WriteString(entry.Time.In(timeZone).Format(logTimestampFormat))
	builder.WriteByte(' ')
	builder.WriteString(formatLevel(entry.Level, useColor))
	builder.WriteString(" - ")
	builder.WriteString(entry.Message)
	if errValue, ok := entry.Data[logrus.ErrorKey]; ok && errValue != nil {
		builder.WriteString(" error: ")
		builder.WriteString(fmt.Sprint(errValue))
	}
	builder.WriteByte('\n')
	return []byte(builder.String()), nil
}

func currentServerTimeZone() string {
	return time.Now().Location().String()
}

func configuredLogTimeZone() string {
	logFormatStateMu.RLock()
	defer logFormatStateMu.RUnlock()
	return logTimeZone.String()
}

func setLogTimeZone(nodeConfig *config.NodeConfig) error {
	timeZone := ""
	if nodeConfig != nil {
		timeZone = nodeConfig.TimeZone
	}
	location, err := config.ParseTimeZoneForLogging(timeZone)
	if err != nil {
		return err
	}
	logFormatStateMu.Lock()
	logTimeZone = location
	logFormatStateMu.Unlock()
	return nil
}

func setLogColorEnabled(nodeConfig *config.NodeConfig) {
	logFormatStateMu.Lock()
	logUseColor = shouldUseLogColor(nodeConfig)
	logFormatStateMu.Unlock()
}

func configuredLogColorEnabled() bool {
	logFormatStateMu.RLock()
	defer logFormatStateMu.RUnlock()
	return logUseColor
}

func shouldUseLogColor(nodeConfig *config.NodeConfig) bool {
	if nodeConfig != nil && nodeConfig.LogFileDir != "" {
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(os.Getenv("TERM")), "dumb") {
		return false
	}
	if !stdoutIsTerminal() {
		return false
	}
	return true
}

func currentLogFormatState() (*time.Location, bool) {
	logFormatStateMu.RLock()
	defer logFormatStateMu.RUnlock()
	return logTimeZone, logUseColor
}

func setLogFormatState(timeZone *time.Location, useColor bool) {
	logFormatStateMu.Lock()
	defer logFormatStateMu.Unlock()
	if timeZone != nil {
		logTimeZone = timeZone
	}
	logUseColor = useColor
}

func formatLevel(level logrus.Level, useColor bool) string {
	label := levelLabel(level)
	if !useColor {
		return label
	}
	return colorForLevel(level) + label + "\x1b[0m"
}

func levelLabel(level logrus.Level) string {
	switch level {
	case logrus.DebugLevel:
		return "DEBUG"
	case logrus.InfoLevel:
		return "INFO"
	case logrus.WarnLevel:
		return "WARN"
	case logrus.ErrorLevel:
		return "ERROR"
	case logrus.FatalLevel:
		return "FATAL"
	case logrus.PanicLevel:
		return "PANIC"
	default:
		return strings.ToUpper(level.String())
	}
}

func colorForLevel(level logrus.Level) string {
	switch level {
	case logrus.DebugLevel:
		return "\x1b[36m"
	case logrus.InfoLevel:
		return "\x1b[32m"
	case logrus.WarnLevel:
		return "\x1b[33m"
	case logrus.ErrorLevel:
		return "\x1b[31m"
	case logrus.FatalLevel, logrus.PanicLevel:
		return "\x1b[1;31m"
	default:
		return "\x1b[37m"
	}
}
