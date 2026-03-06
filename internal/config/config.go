package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server           ServerConfig    `yaml:"server"`
	Database         DatabaseConfig  `yaml:"database"`
	Whatsmeow        WhatsmeowConfig `yaml:"whatsmeow"`
	Auth             AuthConfig      `yaml:"auth"`
	Logging          LoggingConfig   `yaml:"logging"`
	InstanceMode     string          `yaml:"instanceMode"`     // "single" or "multi" (default: "single")
	EventsPublishVia string          `yaml:"eventsPublishVia"` // "webhook", "redis", or "none"
	InstanceDefaults InstanceConfig  `yaml:"instanceDefaults"`
	HTTPProxy        string          `yaml:"httpProxy"`
	Redis            *RedisConfig    `yaml:"redis,omitempty"`
}

type ServerConfig struct {
	Port            int    `yaml:"port"`
	ReadTimeout     string `yaml:"readTimeout"`
	WriteTimeout    string `yaml:"writeTimeout"`
	ShutdownTimeout string `yaml:"shutdownTimeout"`
}

func (s ServerConfig) ReadTimeoutDuration() time.Duration {
	d, _ := time.ParseDuration(s.ReadTimeout)
	if d == 0 {
		return 30 * time.Second
	}
	return d
}

func (s ServerConfig) WriteTimeoutDuration() time.Duration {
	d, _ := time.ParseDuration(s.WriteTimeout)
	if d == 0 {
		return 60 * time.Second
	}
	return d
}

func (s ServerConfig) ShutdownTimeoutDuration() time.Duration {
	d, _ := time.ParseDuration(s.ShutdownTimeout)
	if d == 0 {
		return 10 * time.Second
	}
	return d
}

type DatabaseConfig struct {
	Driver string `yaml:"driver"`
	DSN    string `yaml:"dsn"`
}

type WhatsmeowConfig struct {
	LogLevel       string `yaml:"logLevel"`
	PairClientType string `yaml:"pairClientType"`
	PairClientOS   string `yaml:"pairClientOs"`
}

type AuthConfig struct {
	AdminAPIKey string `yaml:"adminApiKey"`
}

type LoggingConfig struct {
	Level     string `yaml:"level"`
	Format    string `yaml:"format"`
	RedactPII bool   `yaml:"redactPii"`
}

// InstanceConfig holds the user-configurable fields for an instance.
type InstanceConfig struct {
	APIKey        string   `json:"apiKey,omitempty" yaml:"apiKey"`
	WebhookURL    string   `json:"webhookUrl" yaml:"webhookUrl" validate:"omitempty,url"`
	SigningSecret string   `json:"signingSecret,omitempty" yaml:"signingSecret"`
	EventFilters  []string `json:"eventFilters" yaml:"eventFilters"`
	HistorySync   *bool    `json:"historySync,omitempty" yaml:"historySync"`
}

type RedisConfig struct {
	Mode             string `yaml:"mode"`             // "standalone" (default) or "sentinel"
	URL              string `yaml:"url"`              // host:port for standalone; comma-separated sentinel addresses for sentinel
	Password         string `yaml:"password"`         // Redis auth password
	DB               int    `yaml:"db"`               // Redis database number
	StreamName       string `yaml:"streamName"`       // Fixed stream name; default: "stream:<instanceId>"
	TLS              bool   `yaml:"tls"`              // Enable TLS
	TLSInsecure      bool   `yaml:"tlsInsecure"`      // Skip TLS certificate verification
	MasterName       string `yaml:"masterName"`       // Sentinel master name (default: "mymaster")
	SentinelPassword string `yaml:"sentinelPassword"` // Sentinel auth password (if different from Redis password)
}

func defaults() *Config {
	return &Config{
		InstanceMode:     "single",
		EventsPublishVia: "webhook",
		Server: ServerConfig{
			Port:            8080,
			ReadTimeout:     "30s",
			WriteTimeout:    "60s",
			ShutdownTimeout: "10s",
		},
		Database: DatabaseConfig{
			Driver: "sqlite",
			DSN:    "./data/wsapi.db",
		},
		Whatsmeow: WhatsmeowConfig{
			LogLevel:       "warn",
			PairClientType: "chrome",
			PairClientOS:   "Windows",
		},
		Logging: LoggingConfig{
			Level:     "info",
			Format:    "text",
			RedactPII: true,
		},
	}
}

// Load reads configuration from a YAML file (if it exists) and then applies
// environment variable overrides with the WSAPI_ prefix.
func Load(path string) (*Config, error) {
	cfg := defaults()

	// Try to read YAML config file
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("reading config file: %w", err)
		}
		if err == nil {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("parsing config file: %w", err)
			}
		}
	}

	// Environment variable overrides
	applyEnv(cfg)

	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("WSAPI_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Server.Port = port
		}
	}

	setIfEnv(&cfg.Database.Driver, "WSAPI_DB_DRIVER")
	setIfEnv(&cfg.Database.DSN, "WSAPI_DB_DSN")
	setIfEnv(&cfg.Whatsmeow.LogLevel, "WSAPI_WHATSMEOW_LOG_LEVEL")
	setIfEnv(&cfg.Whatsmeow.PairClientType, "WSAPI_WHATSMEOW_PAIR_CLIENT_TYPE")
	setIfEnv(&cfg.Whatsmeow.PairClientOS, "WSAPI_WHATSMEOW_PAIR_CLIENT_OS")
	setIfEnv(&cfg.Auth.AdminAPIKey, "WSAPI_ADMIN_API_KEY")
	setIfEnv(&cfg.Logging.Level, "WSAPI_LOG_LEVEL")
	setIfEnv(&cfg.Logging.Format, "WSAPI_LOG_FORMAT")
	if v := os.Getenv("WSAPI_LOG_REDACT"); v != "" {
		cfg.Logging.RedactPII = strings.EqualFold(v, "true") || v == "1"
	}

	// Instance mode
	setIfEnv(&cfg.InstanceMode, "WSAPI_INSTANCE_MODE")

	// HTTP proxy
	setIfEnv(&cfg.HTTPProxy, "WSAPI_HTTP_PROXY")

	// Event publishing
	setIfEnv(&cfg.EventsPublishVia, "WSAPI_PUBLISH_VIA")

	// Instance defaults
	setIfEnv(&cfg.InstanceDefaults.APIKey, "WSAPI_DEFAULT_API_KEY")
	setIfEnv(&cfg.InstanceDefaults.WebhookURL, "WSAPI_DEFAULT_WEBHOOK_URL")
	setIfEnv(&cfg.InstanceDefaults.SigningSecret, "WSAPI_DEFAULT_SIGNING_SECRET")

	if v := os.Getenv("WSAPI_DEFAULT_EVENT_FILTERS"); v != "" {
		cfg.InstanceDefaults.EventFilters = strings.Split(v, ",")
	}
	if v := os.Getenv("WSAPI_DEFAULT_HISTORY_SYNC"); v != "" {
		b := strings.EqualFold(v, "true") || v == "1"
		cfg.InstanceDefaults.HistorySync = &b
	}

	// Redis
	if v := os.Getenv("WSAPI_REDIS_MODE"); v != "" {
		if cfg.Redis == nil {
			cfg.Redis = &RedisConfig{}
		}
		cfg.Redis.Mode = v
	}
	if v := os.Getenv("WSAPI_REDIS_URL"); v != "" {
		if cfg.Redis == nil {
			cfg.Redis = &RedisConfig{}
		}
		cfg.Redis.URL = v
	}
	if v := os.Getenv("WSAPI_REDIS_PASSWORD"); v != "" {
		if cfg.Redis == nil {
			cfg.Redis = &RedisConfig{}
		}
		cfg.Redis.Password = v
	}
	if v := os.Getenv("WSAPI_REDIS_DB"); v != "" {
		if cfg.Redis == nil {
			cfg.Redis = &RedisConfig{}
		}
		if db, err := strconv.Atoi(v); err == nil {
			cfg.Redis.DB = db
		}
	}
	if v := os.Getenv("WSAPI_REDIS_STREAM_NAME"); v != "" {
		if cfg.Redis == nil {
			cfg.Redis = &RedisConfig{}
		}
		cfg.Redis.StreamName = v
	}
	if v := os.Getenv("WSAPI_REDIS_TLS"); v != "" {
		if cfg.Redis == nil {
			cfg.Redis = &RedisConfig{}
		}
		cfg.Redis.TLS = strings.EqualFold(v, "true") || v == "1"
	}
	if v := os.Getenv("WSAPI_REDIS_TLS_INSECURE"); v != "" {
		if cfg.Redis == nil {
			cfg.Redis = &RedisConfig{}
		}
		cfg.Redis.TLSInsecure = strings.EqualFold(v, "true") || v == "1"
	}
	if v := os.Getenv("WSAPI_REDIS_MASTER_NAME"); v != "" {
		if cfg.Redis == nil {
			cfg.Redis = &RedisConfig{}
		}
		cfg.Redis.MasterName = v
	}
	if v := os.Getenv("WSAPI_REDIS_SENTINEL_PASSWORD"); v != "" {
		if cfg.Redis == nil {
			cfg.Redis = &RedisConfig{}
		}
		cfg.Redis.SentinelPassword = v
	}
}

func validate(cfg *Config) error {
	checks := []struct {
		field string
		value *string
		valid []string
	}{
		{"instanceMode", &cfg.InstanceMode, []string{"single", "multi"}},
		{"eventsPublishVia", &cfg.EventsPublishVia, []string{"none", "webhook", "redis"}},
		{"database.driver", &cfg.Database.Driver, []string{"sqlite", "postgres"}},
		{"logging.level", &cfg.Logging.Level, []string{"debug", "info", "warn", "error"}},
		{"logging.format", &cfg.Logging.Format, []string{"text", "json"}},
		{"whatsmeow.logLevel", &cfg.Whatsmeow.LogLevel, []string{"debug", "info", "warn", "error"}},
		{"whatsmeow.pairClientType", &cfg.Whatsmeow.PairClientType, []string{"chrome", "edge", "firefox", "opera", "safari"}},
		{"whatsmeow.pairClientOs", &cfg.Whatsmeow.PairClientOS, []string{"Windows", "Linux", "macOS"}},
	}

	if cfg.Redis != nil && cfg.Redis.Mode != "" {
		checks = append(checks, struct {
			field string
			value *string
			valid []string
		}{"redis.mode", &cfg.Redis.Mode, []string{"standalone", "sentinel"}})
	}

	for _, c := range checks {
		found := false
		for _, allowed := range c.valid {
			if *c.value == allowed {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("invalid %s %q: must be one of %v", c.field, *c.value, c.valid)
		}
	}

	return nil
}

func setIfEnv(target *string, key string) {
	if v := os.Getenv(key); v != "" {
		*target = v
	}
}
