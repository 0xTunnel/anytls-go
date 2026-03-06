package ppanel

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func WriteUserList(outputPath string, users []ServerUser) error {
	path := strings.TrimSpace(outputPath)
	if path == "" {
		return fmt.Errorf("output path is required")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create user snapshot directory: %w", err)
	}
	payload, err := json.MarshalIndent(UserListResponse{Users: users}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal user snapshot: %w", err)
	}
	payload = append(payload, '\n')
	tempFile, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create user snapshot temp file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)
	if _, err := tempFile.Write(payload); err != nil {
		tempFile.Close()
		return fmt.Errorf("write user snapshot temp file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close user snapshot temp file: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace user snapshot file: %w", err)
	}
	return nil
}
