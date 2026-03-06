package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadNodeConfigFromTOML(t *testing.T) {
	tempDir := t.TempDir()
	certPath := filepath.Join(tempDir, "cert.pem")
	keyPath := filepath.Join(tempDir, "key.pem")
	if err := os.WriteFile(certPath, []byte("cert"), 0644); err != nil {
		t.Fatalf("WriteFile(cert) error = %v", err)
	}
	if err := os.WriteFile(keyPath, []byte("key"), 0644); err != nil {
		t.Fatalf("WriteFile(key) error = %v", err)
	}
	configPath := filepath.Join(tempDir, "node.toml")
	content := []byte("[Panel]\nwebapi_url = \"https://api.ppanel.dev\"\nwebapi_key = \"secret\"\nnode_id = 12\n\n[TLS]\ncert_file = \"./cert.pem\"\nkey_file = \"./key.pem\"\n\n[Config]\nlog_level = \"warning\"\nlog_file_dir = \"./log\"\n")
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}

	config, err := LoadNodeConfig(configPath)
	if err != nil {
		t.Fatalf("LoadNodeConfig() error = %v", err)
	}
	if config.PanelURL != "https://api.ppanel.dev" {
		t.Fatalf("PanelURL = %q", config.PanelURL)
	}
	if config.SecretKey != "secret" {
		t.Fatalf("SecretKey = %q", config.SecretKey)
	}
	if config.ServerID != 12 {
		t.Fatalf("ServerID = %d", config.ServerID)
	}
	if config.TLSCertFile != certPath {
		t.Fatalf("TLSCertFile = %q", config.TLSCertFile)
	}
	if config.TLSKeyFile != keyPath {
		t.Fatalf("TLSKeyFile = %q", config.TLSKeyFile)
	}
	if config.LogLevel != "warning" {
		t.Fatalf("LogLevel = %q", config.LogLevel)
	}
	if config.LogFileDir != filepath.Join(tempDir, "log") {
		t.Fatalf("LogFileDir = %q", config.LogFileDir)
	}
}

func TestLoadNodeConfigRejectsNonTomlExtension(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "node.conf")
	if err := os.WriteFile(configPath, []byte(""), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := LoadNodeConfig(configPath); err == nil {
		t.Fatal("LoadNodeConfig() expected extension error")
	}
}

func TestLoadNodeConfigRejectsInvalidLogLevel(t *testing.T) {
	tempDir := t.TempDir()
	certPath := filepath.Join(tempDir, "cert.pem")
	keyPath := filepath.Join(tempDir, "key.pem")
	if err := os.WriteFile(certPath, []byte("cert"), 0644); err != nil {
		t.Fatalf("WriteFile(cert) error = %v", err)
	}
	if err := os.WriteFile(keyPath, []byte("key"), 0644); err != nil {
		t.Fatalf("WriteFile(key) error = %v", err)
	}
	configPath := filepath.Join(tempDir, "node.toml")
	content := []byte("[Panel]\nwebapi_url = \"https://api.ppanel.dev\"\nwebapi_key = \"secret\"\nnode_id = 1\n\n[TLS]\ncert_file = \"./cert.pem\"\nkey_file = \"./key.pem\"\n\n[Config]\nlog_level = \"trace\"\n")
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}

	if _, err := LoadNodeConfig(configPath); err == nil {
		t.Fatal("LoadNodeConfig() expected log level error")
	}
}

func TestLoadNodeConfigRejectsMissingTLSFiles(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "node.toml")
	content := []byte("[Panel]\nwebapi_url = \"https://api.ppanel.dev\"\nwebapi_key = \"secret\"\nnode_id = 1\n\n[TLS]\ncert_file = \"./missing-cert.pem\"\nkey_file = \"./missing-key.pem\"\n")
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}

	if _, err := LoadNodeConfig(configPath); err == nil {
		t.Fatal("LoadNodeConfig() expected missing tls file error")
	}
}

func TestLoadNodeConfigRejectsTLSDirectoryPath(t *testing.T) {
	tempDir := t.TempDir()
	keyPath := filepath.Join(tempDir, "key.pem")
	if err := os.WriteFile(keyPath, []byte("key"), 0644); err != nil {
		t.Fatalf("WriteFile(key) error = %v", err)
	}
	configPath := filepath.Join(tempDir, "node.toml")
	content := []byte("[Panel]\nwebapi_url = \"https://api.ppanel.dev\"\nwebapi_key = \"secret\"\nnode_id = 1\n\n[TLS]\ncert_file = \".\"\nkey_file = \"./key.pem\"\n")
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}

	if _, err := LoadNodeConfig(configPath); err == nil {
		t.Fatal("LoadNodeConfig() expected directory path error")
	}
}

func TestLoadNodeConfigRejectsMissingPanelFields(t *testing.T) {
	tempDir := t.TempDir()
	certPath := filepath.Join(tempDir, "cert.pem")
	keyPath := filepath.Join(tempDir, "key.pem")
	if err := os.WriteFile(certPath, []byte("cert"), 0644); err != nil {
		t.Fatalf("WriteFile(cert) error = %v", err)
	}
	if err := os.WriteFile(keyPath, []byte("key"), 0644); err != nil {
		t.Fatalf("WriteFile(key) error = %v", err)
	}
	configPath := filepath.Join(tempDir, "node.toml")
	content := []byte("[Panel]\nwebapi_key = \"secret\"\nnode_id = 1\n\n[TLS]\ncert_file = \"./cert.pem\"\nkey_file = \"./key.pem\"\n")
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}

	if _, err := LoadNodeConfig(configPath); err == nil {
		t.Fatal("LoadNodeConfig() expected missing panel field error")
	}
}
