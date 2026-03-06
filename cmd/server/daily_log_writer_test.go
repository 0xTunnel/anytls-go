package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDailyLogWriterWritesToDateFile(t *testing.T) {
	t.Parallel()

	location := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, 3, 7, 10, 0, 0, 0, location)
	writer, err := newDailyLogWriter(t.TempDir(), 0, location, func() time.Time { return now })
	if err != nil {
		t.Fatalf("newDailyLogWriter() error = %v", err)
	}
	defer writer.Close()

	if _, err := writer.Write([]byte("hello\n")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	logPath := filepath.Join(writer.dir, "anytls-server-2026-03-07.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "hello\n" {
		t.Fatalf("log file content = %q", string(data))
	}
}

func TestDailyLogWriterRemovesExpiredFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	location := time.UTC
	if err := os.WriteFile(filepath.Join(dir, "anytls-server-2026-03-04.log"), []byte("old\n"), 0644); err != nil {
		t.Fatalf("WriteFile(old) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "anytls-server-2026-03-06.log"), []byte("keep\n"), 0644); err != nil {
		t.Fatalf("WriteFile(keep) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignore\n"), 0644); err != nil {
		t.Fatalf("WriteFile(notes) error = %v", err)
	}

	now := time.Date(2026, 3, 7, 1, 0, 0, 0, location)
	writer, err := newDailyLogWriter(dir, 2, location, func() time.Time { return now })
	if err != nil {
		t.Fatalf("newDailyLogWriter() error = %v", err)
	}
	defer writer.Close()

	if _, err := os.Stat(filepath.Join(dir, "anytls-server-2026-03-04.log")); !os.IsNotExist(err) {
		t.Fatalf("expired log file still exists, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "anytls-server-2026-03-06.log")); err != nil {
		t.Fatalf("recent log file removed unexpectedly: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "notes.txt")); err != nil {
		t.Fatalf("non-log file removed unexpectedly: %v", err)
	}
}
