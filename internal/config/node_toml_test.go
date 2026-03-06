package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
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
	content := []byte("[Panel]\nwebapi_url = \"https://api.ppanel.dev\"\nwebapi_key = \"secret\"\nnode_id = 12\n\n[TLS]\ncert_file = \"./cert.pem\"\nkey_file = \"./key.pem\"\n\n[Config]\nlog_level = \"warning\"\nlog_file_dir = \"./log\"\nlog_file_retention_days = 7\ntimezone = \"Asia/Shanghai\"\n\n[Network]\ntcp_timeout = 90\nudp_timeout = 5\ntcp_limit = 20\n")
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
	if config.LogFileRetentionDays != 7 {
		t.Fatalf("LogFileRetentionDays = %d", config.LogFileRetentionDays)
	}
	if config.TimeZone != "Asia/Shanghai" {
		t.Fatalf("TimeZone = %q", config.TimeZone)
	}
	if config.TCPTimeout != 90*time.Minute {
		t.Fatalf("TCPTimeout = %s", config.TCPTimeout)
	}
	if config.UDPTimeout != 5*time.Minute {
		t.Fatalf("UDPTimeout = %s", config.UDPTimeout)
	}
	if config.TCPLimit != 20 {
		t.Fatalf("TCPLimit = %d", config.TCPLimit)
	}
}

func TestLoadNodeConfigUsesDefaultNetworkTimeouts(t *testing.T) {
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
	content := []byte("[Panel]\nwebapi_url = \"https://api.ppanel.dev\"\nwebapi_key = \"secret\"\nnode_id = 12\n\n[TLS]\ncert_file = \"./cert.pem\"\nkey_file = \"./key.pem\"\n")
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}

	config, err := LoadNodeConfig(configPath)
	if err != nil {
		t.Fatalf("LoadNodeConfig() error = %v", err)
	}
	if config.TCPTimeout != 60*time.Minute {
		t.Fatalf("TCPTimeout = %s, want %s", config.TCPTimeout, 60*time.Minute)
	}
	if config.UDPTimeout != 2*time.Minute {
		t.Fatalf("UDPTimeout = %s, want %s", config.UDPTimeout, 2*time.Minute)
	}
	if config.LogFileRetentionDays != 0 {
		t.Fatalf("LogFileRetentionDays = %d, want %d", config.LogFileRetentionDays, 0)
	}
	if config.TCPLimit != 0 {
		t.Fatalf("TCPLimit = %d, want %d", config.TCPLimit, 0)
	}
}

func TestLoadNodeConfigUsesDefaultForMissingNetworkField(t *testing.T) {
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
	content := []byte("[Panel]\nwebapi_url = \"https://api.ppanel.dev\"\nwebapi_key = \"secret\"\nnode_id = 12\n\n[TLS]\ncert_file = \"./cert.pem\"\nkey_file = \"./key.pem\"\n\n[Network]\ntcp_timeout = 30\n")
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}

	config, err := LoadNodeConfig(configPath)
	if err != nil {
		t.Fatalf("LoadNodeConfig() error = %v", err)
	}
	if config.TCPTimeout != 30*time.Minute {
		t.Fatalf("TCPTimeout = %s, want %s", config.TCPTimeout, 30*time.Minute)
	}
	if config.UDPTimeout != 2*time.Minute {
		t.Fatalf("UDPTimeout = %s, want %s", config.UDPTimeout, 2*time.Minute)
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

func TestLoadNodeConfigRejectsInvalidTimeZone(t *testing.T) {
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
	content := []byte("[Panel]\nwebapi_url = \"https://api.ppanel.dev\"\nwebapi_key = \"secret\"\nnode_id = 1\n\n[TLS]\ncert_file = \"./cert.pem\"\nkey_file = \"./key.pem\"\n\n[Config]\ntimezone = \"Mars/Base\"\n")
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}

	if _, err := LoadNodeConfig(configPath); err == nil {
		t.Fatal("LoadNodeConfig() expected time zone error")
	}
}

func TestLoadNodeConfigRejectsInvalidTCPTimeout(t *testing.T) {
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
	content := []byte("[Panel]\nwebapi_url = \"https://api.ppanel.dev\"\nwebapi_key = \"secret\"\nnode_id = 1\n\n[TLS]\ncert_file = \"./cert.pem\"\nkey_file = \"./key.pem\"\n\n[Network]\ntcp_timeout = 0\n")
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}

	if _, err := LoadNodeConfig(configPath); err == nil {
		t.Fatal("LoadNodeConfig() expected tcp timeout error")
	}
}

func TestLoadNodeConfigRejectsInvalidUDPTimeout(t *testing.T) {
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
	content := []byte("[Panel]\nwebapi_url = \"https://api.ppanel.dev\"\nwebapi_key = \"secret\"\nnode_id = 1\n\n[TLS]\ncert_file = \"./cert.pem\"\nkey_file = \"./key.pem\"\n\n[Network]\nudp_timeout = -1\n")
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}

	if _, err := LoadNodeConfig(configPath); err == nil {
		t.Fatal("LoadNodeConfig() expected udp timeout error")
	}
}

func TestLoadNodeConfigRejectsNegativeLogFileRetentionDays(t *testing.T) {
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
	content := []byte("[Panel]\nwebapi_url = \"https://api.ppanel.dev\"\nwebapi_key = \"secret\"\nnode_id = 1\n\n[TLS]\ncert_file = \"./cert.pem\"\nkey_file = \"./key.pem\"\n\n[Config]\nlog_file_retention_days = -1\n")
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}

	if _, err := LoadNodeConfig(configPath); err == nil {
		t.Fatal("LoadNodeConfig() expected log file retention days error")
	}
}

func TestLoadNodeConfigRejectsNegativeTCPLimit(t *testing.T) {
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
	content := []byte("[Panel]\nwebapi_url = \"https://api.ppanel.dev\"\nwebapi_key = \"secret\"\nnode_id = 1\n\n[TLS]\ncert_file = \"./cert.pem\"\nkey_file = \"./key.pem\"\n\n[Network]\ntcp_limit = -1\n")
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}

	if _, err := LoadNodeConfig(configPath); err == nil {
		t.Fatal("LoadNodeConfig() expected tcp limit error")
	}
}

func TestLoadNodeConfigRejectsTooLargeTCPTimeout(t *testing.T) {
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
	content := []byte("[Panel]\nwebapi_url = \"https://api.ppanel.dev\"\nwebapi_key = \"secret\"\nnode_id = 1\n\n[TLS]\ncert_file = \"./cert.pem\"\nkey_file = \"./key.pem\"\n\n[Network]\ntcp_timeout = 153722868\n")
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatalf("WriteFile(config) error = %v", err)
	}

	if _, err := LoadNodeConfig(configPath); err == nil {
		t.Fatal("LoadNodeConfig() expected oversized tcp timeout error")
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
