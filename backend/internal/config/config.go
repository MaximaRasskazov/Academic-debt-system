// Package config загружает конфигурацию приложения из переменных окружения.
//
// При запуске Load() сначала пытается прочитать .env (если есть), затем
// собирает Config из ENV с дефолтами на dev-значения. Все обязательные
// поля валидируются перед возвратом — старт сервера должен падать рано,
// если конфигурация неполная, а не на первом запросе.
package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// Config — собранная конфигурация приложения. Хранится в памяти на время
// жизни процесса. Все поля read-only после Load().
type Config struct {
	// Application
	AppEnv   string
	AppPort  string
	LogLevel string

	// PostgreSQL
	DBHost     string
	DBPort     string
	DBName     string
	DBUser     string
	DBPassword string
}

// Load читает .env (если есть) и собирает Config из ENV.
//
// Возвращает ошибку, если обязательные поля не заданы. В этом случае main
// должен сделать log.Fatal — работа без корректной конфигурации невозможна.
func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		AppEnv:   getEnv("APP_ENV", "development"),
		AppPort:  getEnv("APP_PORT", "8080"),
		LogLevel: getEnv("LOG_LEVEL", "info"),

		DBHost:     getEnv("DB_HOST", "127.0.0.1"),
		DBPort:     getEnv("DB_PORT", "5432"),
		DBName:     getEnv("DB_NAME", "academic_debts"),
		DBUser:     getEnv("DB_USER", "academic"),
		DBPassword: getEnv("DB_PASSWORD", ""),
	}

	if cfg.DBName == "" || cfg.DBUser == "" {
		return nil, fmt.Errorf("DB_NAME and DB_USER are required")
	}

	return cfg, nil
}

// DatabaseURL собирает DSN для подключения к Postgres.
// sslmode=disable достаточен для dev/локального docker-compose; для prod
// этот метод нужно расширить параметром или брать готовый URL из env.
func (c *Config) DatabaseURL() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		c.DBUser, c.DBPassword, c.DBHost, c.DBPort, c.DBName)
}

// IsProduction возвращает true, если приложение работает в production-режиме.
// Используется для feature-флагов (например, отключение debug-эндпоинтов).
func (c *Config) IsProduction() bool {
	return c.AppEnv == "production"
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
