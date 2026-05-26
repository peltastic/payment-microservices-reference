package config

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	App       AppConfig
	Logger    LoggerConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	Security  SecurityConfig
	Telemetry TelemetryConfig
}

type AppConfig struct {
	Port string
	Env  string
}

type LoggerConfig struct {
	ServiceName string
	Environment string
	Level       string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

type RedisConfig struct {
	Address  string
	Password string
	DB       int
}

type SecurityConfig struct {
	InternalAuthSecret string
}

type TelemetryConfig struct {
	ServiceName  string
	Environment  string
	OTLPEndpoint string
}

func Load() (*Config, error) {
	if os.Getenv("APP_ENV") != "production" {
		_ = godotenv.Load()
	}
	serviceName := getEnv("OTEL_SERVICE_NAME", "auth-service")
	environment := getEnv("ENVIRONMENT", getEnv("APP_ENV", "development"))

	config := &Config{
		App: AppConfig{
			Port: getEnv("APP_PORT", "4001"),
			Env:  getEnv("APP_ENV", environment),
		},
		Logger: LoggerConfig{
			ServiceName: serviceName,
			Environment: environment,
			Level:       getEnv("LOG_LEVEL", "info"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", ""),
			Port:     getEnv("DB_PORT", ""),
			User:     getEnv("DB_USER", ""),
			Password: getEnv("DB_PASSWORD", ""),
			Name:     getEnv("DB_NAME", ""),
			SSLMode:  getEnv("DB_SSLMODE", ""),
		},
		Redis: RedisConfig{
			Address:  fmt.Sprintf("%s:%s", getEnv("REDIS_HOST", "localhost"), getEnv("REDIS_PORT", "6379")),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		Security: SecurityConfig{
			InternalAuthSecret: strings.TrimSpace(getEnv("INTERNAL_AUTH_SECRET", "")),
		},
		Telemetry: TelemetryConfig{
			ServiceName:  serviceName,
			Environment:  environment,
			OTLPEndpoint: strings.TrimSpace(getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "")),
		},
	}
	return config, config.Validate()
}

func (c DatabaseConfig) DataSourceName() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode)
}

func (c Config) Validate() error {
	missing := requiredFields(map[string]string{
		"DB_HOST":              c.Database.Host,
		"DB_PORT":              c.Database.Port,
		"DB_USER":              c.Database.User,
		"DB_PASSWORD":          c.Database.Password,
		"DB_NAME":              c.Database.Name,
		"DB_SSLMODE":           c.Database.SSLMode,
		"INTERNAL_AUTH_SECRET": c.Security.InternalAuthSecret,
	})
	if len(missing) > 0 {
		return fmt.Errorf("missing required config: %s", strings.Join(missing, ", "))
	}
	if c.isProduction() {
		if err := validateSecretStrength("INTERNAL_AUTH_SECRET", c.Security.InternalAuthSecret); err != nil {
			return err
		}
	}
	return nil
}

func (c Config) isProduction() bool {
	return strings.EqualFold(c.App.Env, "production") || strings.EqualFold(c.Logger.Environment, "production")
}

func validateSecretStrength(name string, value string) error {
	if len(strings.TrimSpace(value)) < 32 {
		return fmt.Errorf("%s must be at least 32 characters in production", name)
	}
	return nil
}

func requiredFields(values map[string]string) []string {
	missing := make([]string, 0)
	for name, value := range values {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, name)
		}
	}
	sort.Strings(missing)
	return missing
}

func getEnv(key, defaultValue string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return defaultValue
}
func getEnvInt(key string, defaultValue int) int {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}
