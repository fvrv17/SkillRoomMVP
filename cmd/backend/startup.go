package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/fvrv17/mvp/internal/platform/config"
)

type startupConfig struct {
	Addr               string
	Secret             string
	Issuer             string
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	IdleTimeout        time.Duration
	ShutdownTimeout    time.Duration
	DatabaseURL        string
	RedisAddr          string
	RedisPassword      string
	RedisDB            int
	OpenAIAPIKey       string
	OpenAIModel        string
	OpenAIBaseURL      string
	OpenAIOrganization string
	OpenAIProject      string
	RunnerBaseURL      string
	RunnerTimeout      time.Duration
	AllowInsecureBoot  bool
	TrustedProxyCIDRs  []string
	ProxySecret        string
}

func loadStartupConfig() (startupConfig, error) {
	cfg := startupConfig{
		Addr:               config.String("BACKEND_ADDR", ":8080"),
		Secret:             config.String("AUTH_TOKEN_SECRET", "dev-secret"),
		Issuer:             config.String("AUTH_TOKEN_ISSUER", "mvp-platform"),
		ReadTimeout:        config.Duration("BACKEND_READ_TIMEOUT", 10*time.Second),
		WriteTimeout:       config.Duration("BACKEND_WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:        config.Duration("BACKEND_IDLE_TIMEOUT", 60*time.Second),
		ShutdownTimeout:    config.Duration("BACKEND_SHUTDOWN_TIMEOUT", 10*time.Second),
		DatabaseURL:        config.String("DATABASE_URL", ""),
		RedisAddr:          config.String("REDIS_ADDR", ""),
		RedisPassword:      config.String("REDIS_PASSWORD", ""),
		RedisDB:            config.Int("REDIS_DB", 0),
		OpenAIAPIKey:       config.String("OPENAI_API_KEY", ""),
		OpenAIModel:        config.String("OPENAI_MODEL", "gpt-4.1-mini"),
		OpenAIBaseURL:      config.String("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		OpenAIOrganization: config.String("OPENAI_ORGANIZATION", ""),
		OpenAIProject:      config.String("OPENAI_PROJECT", ""),
		RunnerBaseURL:      config.String("RUNNER_BASE_URL", ""),
		RunnerTimeout:      config.Duration("RUNNER_TIMEOUT", 5*time.Second),
		AllowInsecureBoot:  config.Bool("ALLOW_INSECURE_BOOT", false),
		TrustedProxyCIDRs:  splitCSV(config.String("TRUSTED_PROXY_CIDRS", "")),
		ProxySecret:        config.String("BACKEND_PROXY_SECRET", ""),
	}
	return cfg, cfg.Validate()
}

func (c startupConfig) Validate() error {
	missing := make([]string, 0, 4)

	if strings.TrimSpace(c.RunnerBaseURL) == "" {
		missing = append(missing, "RUNNER_BASE_URL")
	}
	if c.AllowInsecureBoot {
		if len(missing) > 0 {
			return fmt.Errorf("startup configuration is incomplete: missing %s", strings.Join(missing, ", "))
		}
		return nil
	}
	if strings.TrimSpace(c.DatabaseURL) == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if strings.TrimSpace(c.RedisAddr) == "" {
		missing = append(missing, "REDIS_ADDR")
	}
	secret := strings.TrimSpace(c.Secret)
	if secret == "" || secret == "dev-secret" {
		missing = append(missing, "AUTH_TOKEN_SECRET (must not be empty or dev-secret)")
	}
	if len(missing) > 0 {
		return fmt.Errorf(
			"refusing insecure startup; configure %s or set ALLOW_INSECURE_BOOT=true for local-only mode",
			strings.Join(missing, ", "),
		)
	}
	return nil
}

func (c startupConfig) InsecureWarnings() []string {
	if !c.AllowInsecureBoot {
		return nil
	}
	warnings := []string{}
	if strings.TrimSpace(c.DatabaseURL) == "" {
		warnings = append(warnings, "DATABASE_URL is empty: backend will run without PostgreSQL persistence")
	}
	if strings.TrimSpace(c.RedisAddr) == "" {
		warnings = append(warnings, "REDIS_ADDR is empty: backend will run with in-memory ops state")
	}
	if secret := strings.TrimSpace(c.Secret); secret == "" || secret == "dev-secret" {
		warnings = append(warnings, "AUTH_TOKEN_SECRET is using an insecure development value")
	}
	return warnings
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	raw := strings.Split(value, ",")
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		trimmed := strings.TrimSpace(item)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
