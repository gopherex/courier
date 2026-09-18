// Package app assembles Courier's process configuration and lifecycle.
package app

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/gopherex/courier/internal/service"
)

// Config contains the service and HTTP/database process settings.
type Config struct {
	Service        service.Config
	DatabaseURL    string
	Listen         string
	AdminDirectory string
}

// Load reads explicit secrets and bounded operational defaults from the environment.
func Load() (*Config, error) {
	result := &Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Listen: env("COURIER_LISTEN",
			":8080"),
		AdminDirectory: env("COURIER_ADMIN_DIR",
			"web/dist"),
	}
	config := &result.Service
	config.AdminKey = os.Getenv("COURIER_ADMIN_KEY")
	config.PublicURL = env("COURIER_PUBLIC_URL", "http://localhost:8080")
	config.Development = os.Getenv("COURIER_DEVELOPMENT") == "true"

	key, err := base64.StdEncoding.DecodeString(os.Getenv("COURIER_ENCRYPTION_KEY"))
	if err != nil || len(key) != 32 || len(config.AdminKey) < 32 || result.DatabaseURL == "" {
		return nil, errRequiredSecrets
	}

	config.EncryptionKey = key

	parsed, err := url.Parse(config.PublicURL)
	if err != nil || parsed.Host == "" || parsed.Path != "" || parsed.RawQuery != "" ||
		parsed.Fragment != "" || parsed.User != nil {
		return nil, errPublicOrigin
	}

	if parsed.Scheme != "https" && (!config.Development || parsed.Scheme != "http") {
		return nil, errHTTPSRequired
	}

	if limitErr := loadLimits(config); limitErr != nil {
		return nil, limitErr
	}

	return result, nil
}

func loadLimits(config *service.Config) error {
	var err error

	config.Workers, err = strconv.Atoi(env("COURIER_WORKERS", "4"))
	if err != nil || config.Workers < 1 || config.Workers > 64 {
		return errWorkerLimit
	}

	config.MaxAttempts, err = strconv.Atoi(env("COURIER_MAX_ATTEMPTS", "12"))
	if err != nil || config.MaxAttempts < 1 || config.MaxAttempts > 100 {
		return errAttemptLimit
	}

	options := []struct {
		name     string
		fallback string
		value    *time.Duration
	}{
		{"COURIER_ATTEMPT_TIMEOUT", "30s", &config.AttemptTimeout},
		{"COURIER_LEASE_DURATION", "90s", &config.LeaseDuration},
		{"COURIER_DEDUP_RETENTION", "168h", &config.DedupRetention},
		{"COURIER_DEAD_RETENTION", "168h", &config.DeadRetention},
	}
	for _, option := range options {
		value, parseErr := time.ParseDuration(env(option.name, option.fallback))
		if parseErr != nil || value < time.Second {
			return fmt.Errorf("%s: %w", option.name, errInvalidDuration)
		}

		*option.value = value
	}

	if config.LeaseDuration <= config.AttemptTimeout {
		return errLeaseTooShort
	}

	return nil
}

func env(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}

	return fallback
}

var (
	errRequiredSecrets = errors.New("database URL, admin key (32+ chars), encryption key (base64 of 32 bytes) required")
	errPublicOrigin    = errors.New("COURIER_PUBLIC_URL must be an origin without a path")
	errHTTPSRequired   = errors.New("HTTPS is required outside development mode")
	errWorkerLimit     = errors.New("COURIER_WORKERS must be between 1 and 64")
	errAttemptLimit    = errors.New("COURIER_MAX_ATTEMPTS must be between 1 and 100")
	errLeaseTooShort   = errors.New("lease duration must exceed attempt timeout")
)

var errInvalidDuration = errors.New("must be a positive duration of at least one second")
