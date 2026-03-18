package config

import (
	"strconv"
	"sync"
	"time"
	"os"

	"krampus/pkg/logging"

	"github.com/ilyakaznacheev/cleanenv"
)

// type Config struct {
// 	Env string
// 	HTTPPort string
// 	GRPCPort string
// 	SSEPort string
// 	PostgresDSN string
// 	Redis RedisConfig
// 	Kafka KafkaConfig
// 	File  FileConfig
// }

// type RedisConfig struct {
// 	Addr string
// 	Password string
// 	DB int
// }

// type KafkaConfig struct {
// 	Brokers []string
// 	Topics KafkaTopics
// }

// type KafkaTopics struct {
// 	Incoming string
// 	Validated string
// 	Saved string
// 	Broadcast string
// }

// type FileConfig struct {
// 	BasePath string
// 	SegmentSize time.Duration
// 	BufferSize int
// 	FlushTimeout time.Duration
// }

// func Load() (*Config, error) {
// 	cfg := &Config{
// 		Env: getEnv("ENV", "development"),
// 		HTTPPort: getEnv("HTTP_PORT", ":8080"),
// 		GRPCPort: getEnv("GRPC_PORT", ":9090"), 
// 		SSEPort: getEnv("SSE_PORT", ":8081"),
// 		PostgresDSN: getEnv("POSTGRES_DSN", "postgres://user:pass@localhost/chatdb?sslmode=disable"), 
// 		File: FileConfig{
// 			BasePath: getEnv("FILE_BASE_PATH", "./storage"),
// 			SegmentSize: parseDuration(getEnv("FILE_SEGMENT_SIZE", "1h")),
// 			BufferSize: parseSize(getEnv("FILE_BUFFER_SIZE", "64MB")),
// 			FlushTimeout: parseDuration(getEnv("FILE_FLUSH_TIMEOUT", "100ms")),
// 		},
// 	}
	
// 	cfg.Redis.Addr = getEnv("REDIS_ADDR", "localhost:6379")
// 	cfg.Redis.Password = getEnv("REDIS_PASSWORD", "")
// 	cfg.Redis.DB, _ = strconv.Atoi(getEnv("REDIS_DB", "0"))
	
// 	cfg.Kafka.Brokers = getEnvAsSlice("KAFKA_BROKERS", []string{"localhost:9092"})
// 	cfg.Kafka.Topics.Incoming = getEnv("KAFKA_TOPICS_INCOMING", "incoming")
// 	cfg.Kafka.Topics.Validated = getEnv("KAFKA_TOPICS_VALIDATED", "validated")
// 	cfg.Kafka.Topics.Saved = getEnv("KAFKA_TOPICS_SAVED", "saved")
// 	cfg.Kafka.Topics.Broadcast = getEnv("KAFKA_TOPICS_BROADCAST", "broadcast")
	
// 	return cfg, nil
// }

// func getEnv(key, defaultVal string) string {
// 	if val := os.Getenv(key); val != "" {
// 		return val
// 	}
// 	return defaultVal
// }

// func getEnvAsSlice(key string, defaultVal []string) []string {
// 	val := getEnv(key, strings.Join(defaultVal, ","))
// 	return strings.Split(val, ",")
// }

// func parseDuration(s string) time.Duration {
// 	d, _ := time.ParseDuration(s)
// 	return d
// }

// func parseSize(s string) int {
// 	// Stub: 64*1024*1024
// 	return 67108864
// }

type Config struct {
	Env string `yml:"env" env-default:"development"`
	StorageConfig
}

type TransportConfig struct {
	Method string
	Port   string
}

type StorageConfig struct {
	Host     string `yaml:"host" env:"DB_HOST" env-default:"db"`
	Port     string `yaml:"port" env:"DB_PORT" env-default:"5432"`
	Database string `yaml:"database" env:"DB_NAME" env-default:"postgres"`
	Username string `yaml:"username" env:"DB_USER" env-default:"postgres"`
	Password string `yaml:"password" env:"DB_PASSWORD" env-default:"postgres"`
}

var instance *Config
var once sync.Once

func GetConfig() *Config {
	once.Do(func() {
		logger := logging.GetLogger()
		logger.Info("read application configuration")
		instance = &Config{}

		if err := cleanenv.ReadEnv(instance); err != nil {
			logger.Errorf("Error reading env vars: %v", err)
		}

		if err := cleanenv.ReadConfig("/config.yml", instance); err != nil {
			logger.Warnf("Config file not found, using env vars: %v", err)
		}

		logger.Infof("Database config: %s:%s", instance.Host, instance.Port)
	})
	return instance
}
