package app

import (
	"errors"
	"fmt"
	"github.com/webmafia/tumladan/internal/middleware"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/webmafia/tumladan/pkg/postgres"
)

type Config struct {
	AppEnv    string
	HTTPHost  string
	HTTPPort  int
	LogLevel  string
	JWTSecret string
	JWTTTL    time.Duration
	Postgres  postgres.Config
	CORS      middleware.CORSConfig
}

func Load() (*Config, error) {
	jwtSecret, err := getRequiredEnv("JWT_SECRET")
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		AppEnv:    getEnv("APP_ENV", "local"),
		HTTPHost:  getEnv("HTTP_HOST", "0.0.0.0"),
		HTTPPort:  getEnvInt("HTTP_PORT", 8080),
		LogLevel:  getEnv("LOG_LEVEL", "info"),
		JWTSecret: jwtSecret,
		JWTTTL:    getEnvDuration("JWT_TTL", 24*time.Hour),
		Postgres: postgres.Config{
			Host:            getEnv("DB_HOST", "postgres"),
			Port:            getEnvInt("DB_PORT", 5432),
			User:            getEnv("DB_USER", "webmafia"),
			Password:        getEnv("DB_PASSWORD", "webmafia"),
			DBName:          getEnv("DB_NAME", "webmafia"),
			SSLMode:         getEnv("DB_SSLMODE", "disable"),
			MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 10),
			MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 30*time.Minute),
		},
		CORS: middleware.CORSConfig{
			AllowedOrigins: getEnvCSV("CORS_ALLOWED_ORIGINS", []string{
				"http://localhost:3000",
				"http://localhost:5173",
			}),
			AllowedMethods: []string{
				"GET", "POST", "PUT", "DELETE", "OPTIONS",
			},
			AllowedHeaders: []string{
				"Content-Type", "Authorization",
			},
			AllowCredentials: false,
		},
	}

	return cfg, nil
}

func (c *Config) HTTPAddress() string {
	return fmt.Sprintf("%s:%d", c.HTTPHost, c.HTTPPort)
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func getRequiredEnv(key string) (string, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return "", errors.New(key + " is required")
	}

	return value, nil
}

func getEnvCSV(key string, fallback []string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}

	if len(result) == 0 {
		return fallback
	}

	return result
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}

	return parsed
}
