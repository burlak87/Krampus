package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env         string
	NodeID      string `env:"NODE_ID"`
	JWTSecret   string `env:"JWT_SECRET"`
	HTTPPort    string `env:"HTTP_PORT" default:":8080"`
	GRPCPort    string
	SSEPort     string
	PostgresDSN string `env:"POSTGRES_DSN"`
	Redis       RedisConfig
	Kafka       KafkaConfig
	File        FileConfig
	S3          S3Config
	Call        CallConfig
}

// CallConfig holds WebRTC signaling / presence settings.
type CallConfig struct {
	PresenceTTL time.Duration
	MaxPeers    int
	StunServers []string
	TurnURL     string
	TurnUser    string
	TurnPass    string
}

type S3Config struct {
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	Endpoint        string
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
		NodeID:      getEnv("NODE_ID", "node-1"),
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

	cfg.S3.Region = getEnv("AWS_REGION", "us-east-1")
	cfg.S3.Bucket = getEnv("AWS_BUCKET", "krampus-media")
	cfg.S3.AccessKeyID = getEnv("AWS_ACCESS_KEY_ID", "")
	cfg.S3.SecretAccessKey = getEnv("AWS_SECRET_ACCESS_KEY", "")
	cfg.S3.Endpoint = getEnv("AWS_ENDPOINT", "")

	cfg.Kafka.Brokers = getEnvAsSlice("KAFKA_BROKERS", []string{"localhost:9092"})
	cfg.Kafka.Topics.Incoming = getEnv("KAFKA_TOPICS_INCOMING", "incoming")
	cfg.Kafka.Topics.Validated = getEnv("KAFKA_TOPICS_VALIDATED", "validated")
	cfg.Kafka.Topics.Saved = getEnv("KAFKA_TOPICS_SAVED", "saved")
	cfg.Kafka.Topics.Broadcast = getEnv("KAFKA_TOPICS_BROADCAST", "broadcast")

	cfg.Call.PresenceTTL = parseDuration(getEnv("CALL_PRESENCE_TTL", "30s"))
	cfg.Call.MaxPeers, _ = strconv.Atoi(getEnv("CALL_MAX_PEERS", "8"))
	cfg.Call.StunServers = getEnvAsSlice("STUN_SERVERS", []string{"stun:stun.l.google.com:19302"})
	cfg.Call.TurnURL = getEnv("TURN_URL", "")
	cfg.Call.TurnUser = getEnv("TURN_USER", "")
	cfg.Call.TurnPass = getEnv("TURN_PASS", "")

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
