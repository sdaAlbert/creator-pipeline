package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr          string
	InfraMode         string
	MySQLDSN          string
	RabbitMQURL       string
	RedisAddr         string
	RedisPassword     string
	RedisDB           int
	MinIOEndpoint     string
	MinIOAccessKey    string
	MinIOSecretKey    string
	MinIOBucket       string
	MinIOUseSSL       bool
	CDNBaseURL        string
	WorkerConcurrency int
	JobTimeout        time.Duration
	LLMConfigPath     string
	LLMRequired       bool
	MiniMaxStrictMode bool
}

func Load() Config {
	return Config{
		HTTPAddr:          env("HTTP_ADDR", ":8080"),
		InfraMode:         env("INFRA_MODE", "memory"),
		MySQLDSN:          env("MYSQL_DSN", "creator:creator@tcp(127.0.0.1:3306)/creator_pipeline?parseTime=true&charset=utf8mb4&loc=UTC"),
		RabbitMQURL:       env("RABBITMQ_URL", "amqp://guest:guest@localhost:35672/"),
		RedisAddr:         env("REDIS_ADDR", "localhost:6379"),
		RedisPassword:     env("REDIS_PASSWORD", ""),
		RedisDB:           envInt("REDIS_DB", 0),
		MinIOEndpoint:     env("MINIO_ENDPOINT", "localhost:9000"),
		MinIOAccessKey:    env("MINIO_ACCESS_KEY", "minio"),
		MinIOSecretKey:    env("MINIO_SECRET_KEY", "minio123"),
		MinIOBucket:       env("MINIO_BUCKET", "creator-results"),
		MinIOUseSSL:       envBool("MINIO_USE_SSL", false),
		CDNBaseURL:        env("CDN_BASE_URL", "http://localhost:9000/creator-results"),
		WorkerConcurrency: envInt("WORKER_CONCURRENCY", 2),
		JobTimeout:        time.Duration(envInt("JOB_TIMEOUT_SECONDS", 3)) * time.Second,
		LLMConfigPath:     env("LLM_CONFIG_PATH", ""),
		LLMRequired:       envBool("LLM_REQUIRED", false),
		MiniMaxStrictMode: envBool("MINIMAX_STRICT_MODE", false),
	}
}

func env(key string, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
