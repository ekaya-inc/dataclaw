package config

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultBindAddr      = "127.0.0.1"
	DefaultPort          = 18790
	DefaultAdminBindAddr = DefaultBindAddr
	DefaultAdminPort     = DefaultPort
	DefaultMCPBindAddr   = "127.0.0.1"
	DefaultMCPPort       = 18791

	DefaultAdminPassword = "admin"
)

const (
	DefaultAdminSessionTTL     = 12 * time.Hour
	DefaultAdminSessionLongTTL = 30 * 24 * time.Hour
	MaxAdminSessionLongTTL     = 90 * 24 * time.Hour
)

var DefaultLogLevel = slog.LevelInfo

type Config struct {
	BindAddr   string
	Port       int
	DataDir    string
	SQLitePath string
	SecretPath string
	Version    string
	LogLevel   slog.Level
	Admin      AdminConfig
	MCP        ListenerConfig
	Warnings   []string
}

type ListenerConfig struct {
	BindAddr          string
	Port              int
	AdvertisedHost    string
	AdvertisedBaseURL string
	TLS               bool
	TLSCertFile       string
	TLSKeyFile        string
}

type AdminConfig struct {
	ListenerConfig
	Password          string
	PasswordDefaulted bool
	SessionTTL        time.Duration
	SessionLongTTL    time.Duration
}

type fileConfig struct {
	ConfigVersion *int                `json:"config_version"`
	Admin         *fileAdminConfig    `json:"admin"`
	MCP           *fileListenerConfig `json:"mcp"`
	DataDir       *string             `json:"data_dir"`
	DBPath        *string             `json:"db_path"`
	SecretPath    *string             `json:"secret_path"`
	LogLevel      *string             `json:"log_level"`
}

type fileAdminConfig struct {
	BindAddr          *string `json:"bind_addr"`
	Port              *int    `json:"port"`
	AdvertisedHost    *string `json:"advertised_host"`
	AdvertisedBaseURL *string `json:"advertised_base_url"`
	TLS               *bool   `json:"tls"`
	TLSCertFile       *string `json:"tls_cert_file"`
	TLSKeyFile        *string `json:"tls_key_file"`
	Password          *string `json:"password"`
	SessionTTL        *string `json:"session_ttl"`
	SessionLongTTL    *string `json:"session_long_ttl"`
}

type fileListenerConfig struct {
	BindAddr          *string `json:"bind_addr"`
	Port              *int    `json:"port"`
	AdvertisedHost    *string `json:"advertised_host"`
	AdvertisedBaseURL *string `json:"advertised_base_url"`
	TLS               *bool   `json:"tls"`
	TLSCertFile       *string `json:"tls_cert_file"`
	TLSKeyFile        *string `json:"tls_key_file"`
}

func Load(version string) (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir: %w", err)
	}

	defaultDataDir := filepath.Join(home, ".dataclaw")
	cfg := defaultConfig(version, defaultDataDir)
	configSearchDataDir := envOrDefault("DATACLAW_DATA_DIR", defaultDataDir)
	configPath, explicitConfigPath, err := resolveConfigPath(configSearchDataDir)
	if err != nil {
		return nil, err
	}
	if configPath != "" {
		if err := loadJSONConfig(configPath, explicitConfigPath, cfg); err != nil {
			return nil, err
		}
	}
	if err := applyEnvOverrides(cfg); err != nil {
		return nil, err
	}
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}
	cfg.syncLegacyFields()
	return cfg, nil
}

func defaultConfig(version, dataDir string) *Config {
	cfg := &Config{
		DataDir:    dataDir,
		SQLitePath: filepath.Join(dataDir, "dataclaw.sqlite"),
		SecretPath: filepath.Join(dataDir, "secret.key"),
		Version:    version,
		LogLevel:   DefaultLogLevel,
		Admin: AdminConfig{
			ListenerConfig: ListenerConfig{
				BindAddr: DefaultAdminBindAddr,
				Port:     DefaultAdminPort,
			},
			Password:          DefaultAdminPassword,
			PasswordDefaulted: true,
			SessionTTL:        DefaultAdminSessionTTL,
			SessionLongTTL:    DefaultAdminSessionLongTTL,
		},
		MCP: ListenerConfig{
			BindAddr: DefaultMCPBindAddr,
			Port:     DefaultMCPPort,
		},
	}
	cfg.syncLegacyFields()
	return cfg
}

func (c *Config) syncLegacyFields() {
	c.BindAddr = c.Admin.BindAddr
	c.Port = c.Admin.Port
	if c.SQLitePath == "" {
		c.SQLitePath = filepath.Join(c.DataDir, "dataclaw.sqlite")
	}
	if c.SecretPath == "" {
		c.SecretPath = filepath.Join(c.DataDir, "secret.key")
	}
}

func resolveConfigPath(dataDir string) (string, bool, error) {
	if raw, ok := os.LookupEnv("DATACLAW_CONFIG_PATH"); ok {
		path := strings.TrimSpace(raw)
		if path == "" {
			return "", true, fmt.Errorf("DATACLAW_CONFIG_PATH is empty")
		}
		return path, true, nil
	}
	path := filepath.Join(dataDir, "config.json")
	if _, err := os.Stat(path); err == nil {
		return path, false, nil
	} else if err != nil && !os.IsNotExist(err) {
		return "", false, fmt.Errorf("stat config file %s: %w", path, err)
	}
	return "", false, nil
}

func loadJSONConfig(path string, explicit bool, cfg *Config) error {
	file, err := os.Open(path)
	if err != nil {
		if explicit && os.IsNotExist(err) {
			return fmt.Errorf("DATACLAW_CONFIG_PATH %s does not exist", path)
		}
		return fmt.Errorf("open config file %s: %w", path, err)
	}
	defer file.Close()

	dec := json.NewDecoder(file)
	dec.DisallowUnknownFields()
	var loaded fileConfig
	if err := dec.Decode(&loaded); err != nil {
		return fmt.Errorf("parse config file %s: %w", path, err)
	}
	if loaded.ConfigVersion != nil && *loaded.ConfigVersion != 1 {
		return fmt.Errorf("config_version = %d, want 1", *loaded.ConfigVersion)
	}
	if err := applyFileConfig(&loaded, cfg); err != nil {
		return err
	}
	return nil
}

func applyFileConfig(loaded *fileConfig, cfg *Config) error {
	oldDataDir := cfg.DataDir
	if loaded.DataDir != nil {
		cfg.DataDir = strings.TrimSpace(*loaded.DataDir)
	}
	if loaded.DBPath != nil {
		cfg.SQLitePath = strings.TrimSpace(*loaded.DBPath)
	} else if loaded.DataDir != nil && cfg.SQLitePath == filepath.Join(oldDataDir, "dataclaw.sqlite") {
		cfg.SQLitePath = filepath.Join(cfg.DataDir, "dataclaw.sqlite")
	}
	if loaded.SecretPath != nil {
		cfg.SecretPath = strings.TrimSpace(*loaded.SecretPath)
	} else if loaded.DataDir != nil && cfg.SecretPath == filepath.Join(oldDataDir, "secret.key") {
		cfg.SecretPath = filepath.Join(cfg.DataDir, "secret.key")
	}
	if loaded.LogLevel != nil {
		level, err := parseLogLevel(*loaded.LogLevel)
		if err != nil {
			return err
		}
		cfg.LogLevel = level
	}
	if loaded.Admin != nil {
		if err := applyFileAdminConfig(loaded.Admin, cfg); err != nil {
			return err
		}
	}
	if loaded.MCP != nil {
		applyFileListenerConfig(loaded.MCP, &cfg.MCP)
	}
	cfg.syncLegacyFields()
	return nil
}

func applyFileAdminConfig(loaded *fileAdminConfig, cfg *Config) error {
	applyStringPtr(loaded.BindAddr, &cfg.Admin.BindAddr)
	applyIntPtr(loaded.Port, &cfg.Admin.Port)
	applyStringPtr(loaded.AdvertisedHost, &cfg.Admin.AdvertisedHost)
	applyStringPtr(loaded.AdvertisedBaseURL, &cfg.Admin.AdvertisedBaseURL)
	applyBoolPtr(loaded.TLS, &cfg.Admin.TLS)
	applyStringPtr(loaded.TLSCertFile, &cfg.Admin.TLSCertFile)
	applyStringPtr(loaded.TLSKeyFile, &cfg.Admin.TLSKeyFile)
	if loaded.Password != nil {
		cfg.Admin.Password = *loaded.Password
		cfg.Admin.PasswordDefaulted = false
	}
	if loaded.SessionTTL != nil {
		d, err := time.ParseDuration(strings.TrimSpace(*loaded.SessionTTL))
		if err != nil {
			return fmt.Errorf("parse admin.session_ttl: %w", err)
		}
		cfg.Admin.SessionTTL = d
	}
	if loaded.SessionLongTTL != nil {
		d, err := time.ParseDuration(strings.TrimSpace(*loaded.SessionLongTTL))
		if err != nil {
			return fmt.Errorf("parse admin.session_long_ttl: %w", err)
		}
		cfg.Admin.SessionLongTTL = d
	}
	return nil
}

func applyFileListenerConfig(loaded *fileListenerConfig, cfg *ListenerConfig) {
	applyStringPtr(loaded.BindAddr, &cfg.BindAddr)
	applyIntPtr(loaded.Port, &cfg.Port)
	applyStringPtr(loaded.AdvertisedHost, &cfg.AdvertisedHost)
	applyStringPtr(loaded.AdvertisedBaseURL, &cfg.AdvertisedBaseURL)
	applyBoolPtr(loaded.TLS, &cfg.TLS)
	applyStringPtr(loaded.TLSCertFile, &cfg.TLSCertFile)
	applyStringPtr(loaded.TLSKeyFile, &cfg.TLSKeyFile)
}

func applyStringPtr(src *string, dst *string) {
	if src != nil {
		*dst = strings.TrimSpace(*src)
	}
}

func applyIntPtr(src *int, dst *int) {
	if src != nil {
		*dst = *src
	}
}

func applyBoolPtr(src *bool, dst *bool) {
	if src != nil {
		*dst = *src
	}
}

func applyEnvOverrides(cfg *Config) error {
	if raw, ok := os.LookupEnv("DATACLAW_BIND_ADDR"); ok && strings.TrimSpace(raw) != "" {
		cfg.Admin.BindAddr = strings.TrimSpace(raw)
		cfg.Warnings = append(cfg.Warnings, "DATACLAW_BIND_ADDR is deprecated; use DATACLAW_ADMIN_BIND_ADDR")
	}
	if raw, ok := os.LookupEnv("DATACLAW_PORT"); ok && strings.TrimSpace(raw) != "" {
		port, err := parsePortEnv("DATACLAW_PORT", raw)
		if err != nil {
			return err
		}
		cfg.Admin.Port = port
		cfg.Warnings = append(cfg.Warnings, "DATACLAW_PORT is deprecated; use DATACLAW_ADMIN_PORT")
	}
	oldDataDir := cfg.DataDir
	if raw, ok := os.LookupEnv("DATACLAW_DATA_DIR"); ok && strings.TrimSpace(raw) != "" {
		cfg.DataDir = strings.TrimSpace(raw)
		if _, ok := os.LookupEnv("DATACLAW_DB_PATH"); !ok && cfg.SQLitePath == filepath.Join(oldDataDir, "dataclaw.sqlite") {
			cfg.SQLitePath = filepath.Join(cfg.DataDir, "dataclaw.sqlite")
		}
		if _, ok := os.LookupEnv("DATACLAW_SECRET_PATH"); !ok && cfg.SecretPath == filepath.Join(oldDataDir, "secret.key") {
			cfg.SecretPath = filepath.Join(cfg.DataDir, "secret.key")
		}
	}
	applyStringEnv("DATACLAW_DB_PATH", &cfg.SQLitePath)
	applyStringEnv("DATACLAW_SECRET_PATH", &cfg.SecretPath)
	if raw, ok := os.LookupEnv("DATACLAW_LOG_LEVEL"); ok && strings.TrimSpace(raw) != "" {
		level, err := parseLogLevel(raw)
		if err != nil {
			return err
		}
		cfg.LogLevel = level
	}
	if err := applyListenerEnvOverrides("DATACLAW_ADMIN", &cfg.Admin.ListenerConfig); err != nil {
		return err
	}
	if raw, ok := os.LookupEnv("DATACLAW_ADMIN_PASSWORD"); ok {
		if raw == "" {
			return fmt.Errorf("DATACLAW_ADMIN_PASSWORD must not be empty when set")
		}
		cfg.Admin.Password = raw
		cfg.Admin.PasswordDefaulted = false
	}
	if err := applyDurationEnv("DATACLAW_ADMIN_SESSION_TTL", &cfg.Admin.SessionTTL); err != nil {
		return err
	}
	if err := applyDurationEnv("DATACLAW_ADMIN_SESSION_LONG_TTL", &cfg.Admin.SessionLongTTL); err != nil {
		return err
	}
	if err := applyListenerEnvOverrides("DATACLAW_MCP", &cfg.MCP); err != nil {
		return err
	}
	cfg.syncLegacyFields()
	return nil
}

func applyListenerEnvOverrides(prefix string, cfg *ListenerConfig) error {
	applyStringEnv(prefix+"_BIND_ADDR", &cfg.BindAddr)
	if raw, ok := os.LookupEnv(prefix + "_PORT"); ok && strings.TrimSpace(raw) != "" {
		port, err := parsePortEnv(prefix+"_PORT", raw)
		if err != nil {
			return err
		}
		cfg.Port = port
	}
	applyStringEnv(prefix+"_ADVERTISED_HOST", &cfg.AdvertisedHost)
	applyStringEnv(prefix+"_ADVERTISED_BASE_URL", &cfg.AdvertisedBaseURL)
	if raw, ok := os.LookupEnv(prefix + "_TLS"); ok && strings.TrimSpace(raw) != "" {
		parsed, err := strconv.ParseBool(strings.TrimSpace(raw))
		if err != nil {
			return fmt.Errorf("parse %s: %w", prefix+"_TLS", err)
		}
		cfg.TLS = parsed
	}
	applyStringEnv(prefix+"_TLS_CERT_FILE", &cfg.TLSCertFile)
	applyStringEnv(prefix+"_TLS_KEY_FILE", &cfg.TLSKeyFile)
	return nil
}

func applyStringEnv(key string, dst *string) {
	if raw, ok := os.LookupEnv(key); ok && strings.TrimSpace(raw) != "" {
		*dst = strings.TrimSpace(raw)
	}
}

func applyDurationEnv(key string, dst *time.Duration) error {
	if raw, ok := os.LookupEnv(key); ok && strings.TrimSpace(raw) != "" {
		parsed, err := time.ParseDuration(strings.TrimSpace(raw))
		if err != nil {
			return fmt.Errorf("parse %s: %w", key, err)
		}
		*dst = parsed
	}
	return nil
}

func validateConfig(cfg *Config) error {
	cfg.DataDir = expandUserPath(cfg.DataDir)
	cfg.SQLitePath = expandUserPath(cfg.SQLitePath)
	cfg.SecretPath = expandUserPath(cfg.SecretPath)
	cfg.Admin.TLSCertFile = expandUserPath(cfg.Admin.TLSCertFile)
	cfg.Admin.TLSKeyFile = expandUserPath(cfg.Admin.TLSKeyFile)
	cfg.MCP.TLSCertFile = expandUserPath(cfg.MCP.TLSCertFile)
	cfg.MCP.TLSKeyFile = expandUserPath(cfg.MCP.TLSKeyFile)
	if strings.TrimSpace(cfg.DataDir) == "" {
		return fmt.Errorf("data_dir must not be empty")
	}
	if strings.TrimSpace(cfg.SQLitePath) == "" {
		cfg.SQLitePath = filepath.Join(cfg.DataDir, "dataclaw.sqlite")
	}
	if strings.TrimSpace(cfg.SecretPath) == "" {
		cfg.SecretPath = filepath.Join(cfg.DataDir, "secret.key")
	}
	if err := validateListenerConfig("admin", &cfg.Admin.ListenerConfig); err != nil {
		return err
	}
	if cfg.Admin.Password == "" {
		return fmt.Errorf("admin.password must not be empty when configured")
	}
	if cfg.Admin.SessionTTL <= 0 {
		return fmt.Errorf("admin.session_ttl must be positive")
	}
	if cfg.Admin.SessionLongTTL <= 0 || cfg.Admin.SessionLongTTL > MaxAdminSessionLongTTL {
		return fmt.Errorf("admin.session_long_ttl must be positive and <= %s", MaxAdminSessionLongTTL)
	}
	if err := validateListenerConfig("mcp", &cfg.MCP); err != nil {
		return err
	}
	return nil
}

func expandUserPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~/"))
}

func validateListenerConfig(name string, cfg *ListenerConfig) error {
	if strings.TrimSpace(cfg.BindAddr) == "" {
		return fmt.Errorf("%s.bind_addr must not be empty", name)
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("%s.port must be between 1 and 65535", name)
	}
	if err := validateAdvertisedHost(name, cfg.AdvertisedHost); err != nil {
		return err
	}
	baseURL, err := validateAdvertisedBaseURL(name, cfg.AdvertisedBaseURL)
	if err != nil {
		return err
	}
	cfg.AdvertisedBaseURL = baseURL
	if (cfg.TLSCertFile == "") != (cfg.TLSKeyFile == "") {
		return fmt.Errorf("%s TLS cert/key must be configured together", name)
	}
	return nil
}

func validateAdvertisedHost(name, host string) error {
	host = strings.TrimSpace(host)
	if host == "" {
		return nil
	}
	if strings.Contains(host, "://") || strings.ContainsAny(host, "/?#@") {
		return fmt.Errorf("%s.advertised_host must be a host name or IP address without scheme, path, query, or fragment", name)
	}
	if splitHost, splitPort, err := net.SplitHostPort(host); err == nil && splitHost != "" && splitPort != "" {
		return fmt.Errorf("%s.advertised_host must not include a port", name)
	}
	if strings.Count(host, ":") == 1 {
		parts := strings.Split(host, ":")
		if _, err := strconv.Atoi(parts[1]); err == nil {
			return fmt.Errorf("%s.advertised_host must not include a port", name)
		}
	}
	return nil
}

func validateAdvertisedBaseURL(name, raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("%s.advertised_base_url: %w", name, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("%s.advertised_base_url must use http or https", name)
	}
	if parsed.Host == "" {
		return "", fmt.Errorf("%s.advertised_base_url must include a host", name)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("%s.advertised_base_url must not include query or fragment", name)
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", fmt.Errorf("%s.advertised_base_url must not include a path", name)
	}
	parsed.Path = ""
	parsed.RawPath = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func parseLogLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return DefaultLogLevel, nil
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid DATACLAW_LOG_LEVEL: %q (expected debug, info, warn, or error)", raw)
	}
}

func normalizeBindAddr(bindAddr string) string {
	bindAddr = strings.TrimSpace(bindAddr)
	if bindAddr == "" || bindAddr == "localhost" {
		return DefaultBindAddr
	}
	return bindAddr
}

func (c *Config) BaseURL(port int) string {
	return c.AdminBaseURL(port)
}

func (c *Config) UIBaseURL(port int) string {
	return c.AdminBaseURL(port)
}

func (c *Config) AdminBaseURL(port int) string {
	return c.Admin.BaseURL(port)
}

func (c *Config) MCPBaseURL(port int) string {
	return c.MCP.BaseURL(port)
}

func (c ListenerConfig) BaseURL(port int) string {
	if c.AdvertisedBaseURL != "" {
		return c.AdvertisedBaseURL
	}
	host := c.AdvertisedHost
	if host == "" {
		host = c.BindAddr
	}
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		host = DefaultBindAddr
	}
	scheme := "http"
	if c.AdvertisesHTTPS() {
		scheme = "https"
	}
	return (&url.URL{Scheme: scheme, Host: net.JoinHostPort(host, strconv.Itoa(port))}).String()
}

func (c ListenerConfig) ServesTLS() bool {
	return c.TLSCertFile != "" && c.TLSKeyFile != ""
}

func (c ListenerConfig) AdvertisesHTTPS() bool {
	return c.TLS || c.ServesTLS()
}

func envOrDefault(key, fallback string) string {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}
	return val
}

func parsePortEnv(key, raw string) (int, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return parsed, nil
}

func intEnvOrDefault(key string, fallback int) (int, error) {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	return parsed, nil
}
