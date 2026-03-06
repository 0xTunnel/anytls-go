package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	dailyLogFilePrefix = "anytls-server-"
	dailyLogFileSuffix = ".log"
	dailyLogDateLayout = "2006-01-02"
)

var logWriterNow = time.Now

type dailyLogWriter struct {
	mu            sync.Mutex
	dir           string
	retentionDays int
	location      *time.Location
	now           func() time.Time
	currentDate   string
	currentFile   *os.File
}

func newDailyLogWriter(dir string, retentionDays int, location *time.Location, now func() time.Time) (*dailyLogWriter, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, fmt.Errorf("log directory is required")
	}
	if location == nil {
		location = time.Local
	}
	if now == nil {
		now = time.Now
	}
	writer := &dailyLogWriter{
		dir:           dir,
		retentionDays: retentionDays,
		location:      location,
		now:           now,
	}
	if err := writer.rotateLocked(now()); err != nil {
		return nil, err
	}
	return writer, nil
}

func (w *dailyLogWriter) Write(p []byte) (int, error) {
	if w == nil {
		return 0, fmt.Errorf("daily log writer is nil")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.rotateLocked(w.now()); err != nil {
		return 0, err
	}
	return w.currentFile.Write(p)
}

func (w *dailyLogWriter) Close() error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.currentFile == nil {
		return nil
	}
	err := w.currentFile.Close()
	w.currentFile = nil
	return err
}

func (w *dailyLogWriter) rotateLocked(now time.Time) error {
	currentDate := now.In(w.location).Format(dailyLogDateLayout)
	if w.currentFile != nil && w.currentDate == currentDate {
		return nil
	}
	if w.currentFile != nil {
		if err := w.currentFile.Close(); err != nil {
			return fmt.Errorf("close previous log file: %w", err)
		}
		w.currentFile = nil
	}
	if err := w.cleanupLocked(now); err != nil {
		return err
	}
	logPath := filepath.Join(w.dir, dailyLogFileName(currentDate))
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open log file %s: %w", logPath, err)
	}
	w.currentDate = currentDate
	w.currentFile = logFile
	return nil
}

func (w *dailyLogWriter) cleanupLocked(now time.Time) error {
	if w.retentionDays <= 0 {
		return nil
	}
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return fmt.Errorf("read log directory: %w", err)
	}
	cutoff := dayStart(now.In(w.location)).AddDate(0, 0, -(w.retentionDays - 1))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		dateText, ok := dailyLogDateFromName(entry.Name())
		if !ok {
			continue
		}
		fileDate, err := time.ParseInLocation(dailyLogDateLayout, dateText, w.location)
		if err != nil {
			continue
		}
		if fileDate.Before(cutoff) {
			logPath := filepath.Join(w.dir, entry.Name())
			if err := os.Remove(logPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("remove expired log file %s: %w", logPath, err)
			}
		}
	}
	return nil
}

func dailyLogFileName(dateText string) string {
	return dailyLogFilePrefix + dateText + dailyLogFileSuffix
}

func dailyLogDateFromName(name string) (string, bool) {
	if !strings.HasPrefix(name, dailyLogFilePrefix) || !strings.HasSuffix(name, dailyLogFileSuffix) {
		return "", false
	}
	dateText := strings.TrimSuffix(strings.TrimPrefix(name, dailyLogFilePrefix), dailyLogFileSuffix)
	if len(dateText) != len(dailyLogDateLayout) {
		return "", false
	}
	return dateText, true
}

func dayStart(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}
