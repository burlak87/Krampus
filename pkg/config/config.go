package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env         string
	HTTPPort    string `env:"HTTP_PORT" default:":8080"`
	GRPCPort    string
	SSEPort     string
	PostgresDSN string `env:"POSTGRES_DSN"`
	Redis       RedisConfig
	Kafka       KafkaConfig
	File        FileConfig
}

type RedisConfig struct {
	Addr     string `env:"REDIS_ADDR" default:"localhost:6379"`
	Password string `env:"REDIS_PASSWORD"`
	DB       int    `env:"REDIS_DB" default:"0"`
}

type KafkaConfig struct {
	Brokers []string `env:"KAFKA_BROKERS" default:"localhost:9092"`
	Topics  KafkaTopics
}

type KafkaTopics struct {
	Incoming  string `env:"KAFKA_TOPIC_INCOMING" default:"messages.incoming"`
	Validated string
	Saved     string
	Broadcast string
}

type FileConfig struct {
	BasePath     string        `env:"FILE_BASE_PATH" default:"./storage"`
	SegmentSize  time.Duration `env:"FILE_SEGMENT_SIZE" default:"1h"`
	BufferSize   int
	FlushTimeout time.Duration
}

func Load() (*Config, error) {
	cfg := &Config{
		Env:         getEnv("ENV", "development"),
		HTTPPort:    getEnv("HTTP_PORT", ":8080"),
		GRPCPort:    getEnv("GRPC_PORT", ":9090"),
		SSEPort:     getEnv("SSE_PORT", ":8081"),
		PostgresDSN: getEnv("POSTGRES_DSN", "postgres://user:pass@localhost/chatdb?sslmode=disable"),
		File: FileConfig{
			BasePath:     getEnv("FILE_BASE_PATH", "./storage"),
			SegmentSize:  parseDuration(getEnv("FILE_SEGMENT_SIZE", "1h")),
			BufferSize:   parseSize(getEnv("FILE_BUFFER_SIZE", "64MB")),
			FlushTimeout: parseDuration(getEnv("FILE_FLUSH_TIMEOUT", "100ms")),
		},
	}

	cfg.Redis.Addr = getEnv("REDIS_ADDR", "localhost:6379")
	cfg.Redis.Password = getEnv("REDIS_PASSWORD", "")
	cfg.Redis.DB, _ = strconv.Atoi(getEnv("REDIS_DB", "0"))

	cfg.Kafka.Brokers = getEnvAsSlice("KAFKA_BROKERS", []string{"localhost:9092"})
	cfg.Kafka.Topics.Incoming = getEnv("KAFKA_TOPICS_INCOMING", "incoming")
	cfg.Kafka.Topics.Validated = getEnv("KAFKA_TOPICS_VALIDATED", "validated")
	cfg.Kafka.Topics.Saved = getEnv("KAFKA_TOPICS_SAVED", "saved")
	cfg.Kafka.Topics.Broadcast = getEnv("KAFKA_TOPICS_BROADCAST", "broadcast")

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvAsSlice(key string, defaultVal []string) []string {
	val := getEnv(key, strings.Join(defaultVal, ","))
	return strings.Split(val, ",")
}

func parseDuration(s string) time.Duration {
	d, _ := time.ParseDuration(s)
	return d
}

func parseSize(s string) int {
	// Stub: 64*1024*1024
	return 67108864
}
