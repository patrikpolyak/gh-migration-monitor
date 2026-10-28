package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// DefaultPageSize is the default and maximum GraphQL/REST pagination size.
const DefaultPageSize = 100

// DefaultTimeout is the default maximum duration of a single refresh.
const DefaultTimeout = 10 * time.Minute

// Config represents the application configuration
type Config struct {
	GitHub struct {
		Token        string `mapstructure:"token"`
		Organization string `mapstructure:"organization"`
	} `mapstructure:"github"`

	Migration struct {
		IsLegacy bool          `mapstructure:"is_legacy"`
		PageSize int           `mapstructure:"page_size"`
		Timeout  time.Duration `mapstructure:"timeout"`
		Since    string        `mapstructure:"since"`
	} `mapstructure:"migration"`

	Output struct {
		Format string `mapstructure:"format"`
		Quiet  bool   `mapstructure:"quiet"`
	} `mapstructure:"output"`
}

// LoadConfig loads configuration from environment variables and config files
func LoadConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("$HOME/.gh-migration-monitor")
	viper.AddConfigPath(".")

	viper.SetDefault("migration.page_size", DefaultPageSize)
	viper.SetDefault("migration.timeout", DefaultTimeout)

	// Environment variables
	viper.SetEnvPrefix("GHMM")
	viper.AutomaticEnv()

	// Bind specific environment variables
	viper.BindEnv("github.token", "GHMM_GITHUB_TOKEN")
	viper.BindEnv("github.organization", "GHMM_GITHUB_ORGANIZATION")
	viper.BindEnv("migration.is_legacy", "GHMM_ISLEGACY")
	viper.BindEnv("migration.page_size", "GHMM_PAGE_SIZE")
	viper.BindEnv("migration.timeout", "GHMM_TIMEOUT")
	viper.BindEnv("migration.since", "GHMM_SINCE")

	// Read configuration file if it exists
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.GitHub.Organization == "" {
		return fmt.Errorf("github organization is required")
	}

	if c.GitHub.Token == "" {
		return fmt.Errorf("github token is required")
	}

	if c.Migration.PageSize < 1 || c.Migration.PageSize > DefaultPageSize {
		return fmt.Errorf("page size must be between 1 and %d", DefaultPageSize)
	}

	if c.Migration.Timeout <= 0 {
		return fmt.Errorf("timeout must be greater than zero")
	}

	if _, err := ParseSince(c.Migration.Since); err != nil {
		return err
	}

	return nil
}

// ParseSince parses a cutoff for migration creation time. It accepts an RFC3339
// timestamp, a date (2006-01-02), or a relative age such as 36h or 7d. Relative
// ages are evaluated on every call so the window rolls forward. An empty value
// returns a nil function, meaning no cutoff.
func ParseSince(value string) (func() time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}

	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if t, err := time.Parse(layout, value); err == nil {
			return func() time.Time { return t }, nil
		}
	}

	var age time.Duration
	if days, ok := strings.CutSuffix(value, "d"); ok {
		n, err := strconv.Atoi(days)
		if err != nil {
			return nil, fmt.Errorf("invalid since value %q: use a timestamp, a date (2006-01-02), or an age like 36h or 7d", value)
		}
		age = time.Duration(n) * 24 * time.Hour
	} else {
		d, err := time.ParseDuration(value)
		if err != nil {
			return nil, fmt.Errorf("invalid since value %q: use a timestamp, a date (2006-01-02), or an age like 36h or 7d", value)
		}
		age = d
	}
	if age <= 0 {
		return nil, fmt.Errorf("invalid since value %q: age must be greater than zero", value)
	}

	return func() time.Time { return time.Now().Add(-age) }, nil
}
