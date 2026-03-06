package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type NodeConfig struct {
	PanelURL    string
	SecretKey   string
	ServerID    int64
	TLSCertFile string
	TLSKeyFile  string
	LogLevel    string
	LogFileDir  string
	Path        string
}

type nodeConfigFile struct {
	Panel struct {
		WebAPIURL string `toml:"webapi_url"`
		WebAPIKey string `toml:"webapi_key"`
		NodeID    int64  `toml:"node_id"`
	} `toml:"Panel"`
	TLS struct {
		CertFile string `toml:"cert_file"`
		KeyFile  string `toml:"key_file"`
	} `toml:"TLS"`
	Config struct {
		LogLevel   string `toml:"log_level"`
		LogFileDir string `toml:"log_file_dir"`
	} `toml:"Config"`
}

func LoadNodeConfig(path string) (*NodeConfig, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("config path is required")
	}
	if ext := strings.ToLower(filepath.Ext(path)); ext != ".toml" {
		return nil, fmt.Errorf("config file must use .toml extension")
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}

	var raw nodeConfigFile
	if _, err := toml.DecodeFile(absPath, &raw); err != nil {
		return nil, fmt.Errorf("decode toml config: %w", err)
	}

	configDir := filepath.Dir(absPath)
	config := &NodeConfig{
		PanelURL:    strings.TrimSpace(raw.Panel.WebAPIURL),
		SecretKey:   strings.TrimSpace(raw.Panel.WebAPIKey),
		ServerID:    raw.Panel.NodeID,
		TLSCertFile: resolvePath(configDir, raw.TLS.CertFile),
		TLSKeyFile:  resolvePath(configDir, raw.TLS.KeyFile),
		LogLevel:    strings.ToLower(strings.TrimSpace(raw.Config.LogLevel)),
		LogFileDir:  resolveOptionalPath(configDir, raw.Config.LogFileDir),
		Path:        absPath,
	}

	if config.PanelURL == "" {
		return nil, fmt.Errorf("config missing Panel.webapi_url")
	}
	if config.SecretKey == "" {
		return nil, fmt.Errorf("config missing Panel.webapi_key")
	}
	if config.ServerID <= 0 {
		return nil, fmt.Errorf("config Panel.node_id must be greater than zero")
	}
	if config.TLSCertFile == "" {
		return nil, fmt.Errorf("config missing TLS.cert_file")
	}
	if config.TLSKeyFile == "" {
		return nil, fmt.Errorf("config missing TLS.key_file")
	}
	if err := validateReadableFile(config.TLSCertFile, "TLS.cert_file"); err != nil {
		return nil, err
	}
	if err := validateReadableFile(config.TLSKeyFile, "TLS.key_file"); err != nil {
		return nil, err
	}
	if config.LogLevel != "" {
		config.LogLevel, err = parseLogLevel(config.LogLevel)
		if err != nil {
			return nil, fmt.Errorf("config parse Config.log_level: %w", err)
		}
	}
	return config, nil
}

func resolvePath(baseDir string, value string) string {
	path := strings.TrimSpace(value)
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(baseDir, path))
}

func resolveOptionalPath(baseDir string, value string) string {
	path := strings.TrimSpace(value)
	if path == "" {
		return ""
	}
	return resolvePath(baseDir, path)
}

func validateReadableFile(path string, field string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("config %s is not accessible: %w", field, err)
	}
	if info.IsDir() {
		return fmt.Errorf("config %s must point to a file", field)
	}
	return nil
}

func parseLogLevel(value string) (string, error) {
	level := strings.ToLower(strings.TrimSpace(value))
	switch level {
	case "", "debug", "info", "warn", "warning", "error":
		return level, nil
	default:
		return "", fmt.Errorf("unsupported value %q", value)
	}
}
