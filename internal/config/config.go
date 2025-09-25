package config

import (
	"os"
	"strings"
	"time"
)

type Config struct {
	SQLitePath     string
	RedisAddr      string
	KafkaBrokers   []string
	KafkaTopicUser string
	CacheTTL       time.Duration
	OutboxPeriod   time.Duration
	OutboxLimit    int
	HTTPPort       string
}

func LoadConfig() *Config {
	getEnv := func(key, fallback string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return fallback
	}

	kafkaBrokers := strings.Split(getEnv("KAFKA_BROKERS", "localhost:9092"), ",")

	return &Config{
		SQLitePath:     getEnv("SQLITE_PATH", "./hexagolab_users.db"),
		RedisAddr:      getEnv("REDIS_ADDR", "localhost:6379"),
		KafkaBrokers:   kafkaBrokers,
		KafkaTopicUser: getEnv("KAFKA_TOPIC", "user-events"),
		CacheTTL:       5 * time.Minute,
		OutboxPeriod:   1 * time.Second,
		OutboxLimit:    10,
		HTTPPort:       getEnv("HTTP_PORT", "8080"),
	}
}
