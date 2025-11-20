package server

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"httpserver/enrichment"
)

// Config конфигурация сервера
type Config struct {
	// Сервер
	Port string

	// Базы данных
	DatabasePath          string
	NormalizedDatabasePath string
	ServiceDatabasePath   string

	// AI конфигурация
	ArliaiAPIKey string
	ArliaiModel  string

	// Connection pooling
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration

	// Логирование
	LogBufferSize int

	// Нормализация
	NormalizerEventsBufferSize int

	// Обогащение контрагентов
	Enrichment *EnrichmentConfig
}

// EnrichmentConfig конфигурация обогащения
type EnrichmentConfig struct {
	Enabled         bool                                  `json:"enabled"`
	AutoEnrich      bool                                  `json:"auto_enrich"`
	MinQualityScore float64                               `json:"min_quality_score"`
	Services        map[string]*enrichment.EnricherConfig `json:"services"`
	Cache           *enrichment.CacheConfig               `json:"cache"`
}

// LoadConfig загружает конфигурацию из переменных окружения
func LoadConfig() (*Config, error) {
	config := &Config{
		// Сервер
		Port: getEnv("SERVER_PORT", "9999"),

		// Базы данных
		DatabasePath:           getEnv("DATABASE_PATH", "data.db"),
		NormalizedDatabasePath: getEnv("NORMALIZED_DATABASE_PATH", "normalized_data.db"),
		ServiceDatabasePath:    getEnv("SERVICE_DATABASE_PATH", "service.db"),

		// AI конфигурация
		ArliaiAPIKey: os.Getenv("ARLIAI_API_KEY"),
		ArliaiModel:  getEnv("ARLIAI_MODEL", "GLM-4.5-Air"),

		// Connection pooling
		MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
		MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 5),
		ConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),

		// Логирование
		LogBufferSize: getEnvInt("LOG_BUFFER_SIZE", 100),

		// Нормализация
		NormalizerEventsBufferSize: getEnvInt("NORMALIZER_EVENTS_BUFFER_SIZE", 100),

		// Обогащение
		Enrichment: LoadEnrichmentConfig(),
	}

	// Валидация
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return config, nil
}

// LoadEnrichmentConfig загружает конфигурацию обогащения
func LoadEnrichmentConfig() *EnrichmentConfig {
	enabled := getEnv("ENRICHMENT_ENABLED", "true") == "true"
	autoEnrich := getEnv("ENRICHMENT_AUTO_ENRICH", "true") == "true"
	minQualityScore := 0.3
	if scoreStr := os.Getenv("ENRICHMENT_MIN_QUALITY_SCORE"); scoreStr != "" {
		if score, err := strconv.ParseFloat(scoreStr, 64); err == nil {
			minQualityScore = score
		}
	}

	services := make(map[string]*enrichment.EnricherConfig)

	// Dadata конфигурация
	dadataEnabled := getEnv("DADATA_ENABLED", "true") == "true"
	if dadataEnabled {
		services["dadata"] = &enrichment.EnricherConfig{
			APIKey:      os.Getenv("DADATA_API_KEY"),
			SecretKey:   os.Getenv("DADATA_SECRET_KEY"),
			BaseURL:     getEnv("DADATA_BASE_URL", "https://suggestions.dadata.ru"),
			Timeout:     getEnvDuration("DADATA_TIMEOUT", 30*time.Second),
			MaxRequests: getEnvInt("DADATA_MAX_REQUESTS", 100),
			Enabled:     dadataEnabled,
			Priority:    getEnvInt("DADATA_PRIORITY", 1),
		}
	}

	// Adata конфигурация
	adataEnabled := getEnv("ADATA_ENABLED", "true") == "true"
	if adataEnabled {
		services["adata"] = &enrichment.EnricherConfig{
			APIKey:      os.Getenv("ADATA_API_KEY"),
			BaseURL:     getEnv("ADATA_BASE_URL", "https://adata.kz"),
			Timeout:     getEnvDuration("ADATA_TIMEOUT", 30*time.Second),
			MaxRequests: getEnvInt("ADATA_MAX_REQUESTS", 50),
			Enabled:     adataEnabled,
			Priority:    getEnvInt("ADATA_PRIORITY", 2),
		}
	}

	// Gisp конфигурация
	gispEnabled := getEnv("GISP_ENABLED", "false") == "true"
	if gispEnabled {
		services["gisp"] = &enrichment.EnricherConfig{
			APIKey:      os.Getenv("GISP_API_KEY"),
			BaseURL:     getEnv("GISP_BASE_URL", "https://gisp.gov.ru"),
			Timeout:     getEnvDuration("GISP_TIMEOUT", 30*time.Second),
			MaxRequests: getEnvInt("GISP_MAX_REQUESTS", 50),
			Enabled:     gispEnabled,
			Priority:    getEnvInt("GISP_PRIORITY", 3),
		}
	}

	return &EnrichmentConfig{
		Enabled:         enabled,
		AutoEnrich:      autoEnrich,
		MinQualityScore: minQualityScore,
		Services:        services,
		Cache: &enrichment.CacheConfig{
			Enabled:         true,
			TTL:             getEnvDuration("ENRICHMENT_CACHE_TTL", 24*time.Hour),
			CleanupInterval: getEnvDuration("ENRICHMENT_CACHE_CLEANUP", 1*time.Hour),
		},
	}
}

// Validate валидирует конфигурацию
func (c *Config) Validate() error {
	if c.Port == "" {
		return fmt.Errorf("port is required")
	}

	if c.DatabasePath == "" {
		return fmt.Errorf("database path is required")
	}

	if c.NormalizedDatabasePath == "" {
		return fmt.Errorf("normalized database path is required")
	}

	if c.ServiceDatabasePath == "" {
		return fmt.Errorf("service database path is required")
	}

	if c.MaxOpenConns <= 0 {
		return fmt.Errorf("max open connections must be greater than 0")
	}

	if c.MaxIdleConns <= 0 {
		return fmt.Errorf("max idle connections must be greater than 0")
	}

	if c.MaxIdleConns > c.MaxOpenConns {
		return fmt.Errorf("max idle connections cannot be greater than max open connections")
	}

	return nil
}

// getEnv получает переменную окружения или возвращает значение по умолчанию
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt получает переменную окружения как int или возвращает значение по умолчанию
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// getEnvDuration получает переменную окружения как Duration или возвращает значение по умолчанию
func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

