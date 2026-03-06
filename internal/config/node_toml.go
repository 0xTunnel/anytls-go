package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

const (
	defaultTCPTimeoutMinutes = 60
	defaultUDPTimeoutMinutes = 2
	maxTimeoutMinutes        = int64((1<<63 - 1) / int64(time.Minute))
)

type NodeConfig struct {
	PanelURL    string
	SecretKey   string
	ServerID    int64
	TLSCertFile string
	TLSKeyFile  string
	LogLevel    string
	LogFileDir  string
	TimeZone    string
	TCPTimeout  time.Duration
	UDPTimeout  time.Duration
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
		TimeZone   string `toml:"timezone"`
	} `toml:"Config"`
	Network struct {
		TCPTimeout *int `toml:"tcp_timeout"`
		UDPTimeout *int `toml:"udp_timeout"`
	} `toml:"Network"`
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
	tcpTimeout, err := parsePositiveMinutes(raw.Network.TCPTimeout, defaultTCPTimeoutMinutes, "Network.tcp_timeout")
	if err != nil {
		return nil, err
	}
	udpTimeout, err := parsePositiveMinutes(raw.Network.UDPTimeout, defaultUDPTimeoutMinutes, "Network.udp_timeout")
	if err != nil {
		return nil, err
	}

	config := &NodeConfig{
		PanelURL:    strings.TrimSpace(raw.Panel.WebAPIURL),
		SecretKey:   strings.TrimSpace(raw.Panel.WebAPIKey),
		ServerID:    raw.Panel.NodeID,
		TLSCertFile: resolvePath(configDir, raw.TLS.CertFile),
		TLSKeyFile:  resolvePath(configDir, raw.TLS.KeyFile),
		LogLevel:    strings.ToLower(strings.TrimSpace(raw.Config.LogLevel)),
		LogFileDir:  resolveOptionalPath(configDir, raw.Config.LogFileDir),
		TimeZone:    strings.TrimSpace(raw.Config.TimeZone),
		TCPTimeout:  tcpTimeout,
		UDPTimeout:  udpTimeout,
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
	if config.TimeZone != "" {
		if _, err := parseTimeZone(config.TimeZone); err != nil {
			return nil, fmt.Errorf("config parse Config.timezone: %w", err)
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

func parsePositiveMinutes(value *int, defaultMinutes int, field string) (time.Duration, error) {
	if value == nil {
		return time.Duration(defaultMinutes) * time.Minute, nil
	}
	if *value <= 0 {
		return 0, fmt.Errorf("config %s must be greater than zero", field)
	}
	if int64(*value) > maxTimeoutMinutes {
		return 0, fmt.Errorf("config %s is too large", field)
	}
	return time.Duration(*value) * time.Minute, nil
}

func parseTimeZone(value string) (*time.Location, error) {
	timeZone := strings.TrimSpace(value)
	if timeZone == "" {
		return time.FixedZone("UTC+8", 8*60*60), nil
	}
	if location, ok := parseUTCOffsetLocation(timeZone); ok {
		return location, nil
	}
	location, err := time.LoadLocation(timeZone)
	if err != nil {
		return nil, fmt.Errorf("unsupported value %q", value)
	}
	return location, nil
}

func ParseTimeZoneForLogging(value string) (*time.Location, error) {
	return parseTimeZone(value)
}

func parseUTCOffsetLocation(value string) (*time.Location, bool) {
	upper := strings.ToUpper(strings.TrimSpace(value))
	if !strings.HasPrefix(upper, "UTC") || len(upper) < 4 {
		return nil, false
	}
	sign := upper[3]
	if sign != '+' && sign != '-' {
		return nil, false
	}
	offsetText := strings.TrimSpace(upper[4:])
	hours, err := strconv.Atoi(offsetText)
	if err != nil || hours < 0 || hours > 14 {
		return nil, false
	}
	offsetSeconds := hours * 60 * 60
	if sign == '-' {
		offsetSeconds = -offsetSeconds
	}
	return time.FixedZone("UTC"+string(sign)+strconv.Itoa(hours), offsetSeconds), true
}
