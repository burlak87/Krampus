package bootstrap

import (
	"context"
	"fmt"
	"krampus/internal/storage"
	"krampus/internal/storage/filestorage"
	"krampus/internal/storage/psql"
	"krampus/internal/storage/redisstorage"
	"krampus/pkg/client-database/redis"
	"krampus/pkg/config"
	"log"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
)

type Storages struct {
	MessageStorage    storage.MessageStorage
	RoomStorage       storage.RoomStorage
	UserClientStorage storage.UserClientStorage
	RoomCache         storage.RoomCache
	UserClientCache   storage.UserClientCache
	// Redis, File, Kafka fields
	redisCli    *redis.Client
	fileStor    *filestorage.FileStorage
	kafkaProd   *kafka.Producer
	distributor storage.MessageDistributor
}

func NewStorages(cfg *config.Config) (*Storages, error) {
	ctx := context.Background()

	// Postgres
	pg, err := psql.NewPostgres(cfg.PostgresDSN)
	if err != nil {
		return nil, fmt.Errorf("postgres: %w", err)
	}

	// redis
	// rdb := redis.NewClient(&redis.Options{
	// 	Addr:         cfg.Redis.Addr,
	// 	Password:     cfg.Redis.Password,
	// 	DB:           cfg.Redis.DB,
	// 	PoolSize:     100,
	// 	MinIdleConns: 10,
	// 	DialTimeout:  5 * time.Second,
	// })
	redisStorage, err := redisstorage.NewRedisStorage(
		cfg.Redis.Addr,
		cfg.Redis.Password,
		cfg.Redis.DB,
	)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to Redis: %w", err)
	}
	// if err := rdb.Ping(ctx).Err(); err != nil {
	// 	return nil, err
	// }

	// FileStorage
	fs := filestorage.New(cfg.File.BasePath, cfg.File.SegmentSize)

	// Kafka Producer
	kafkaProd, err := kafka.NewProducer(&kafka.Config{
		Brokers: cfg.Kafka.Brokers,
		Topic:   cfg.Kafka.Topics.Incoming,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to create Kafka producer: %w", err)
	}

	// Distributor
	distributor := NewMessageDistributor(redisStorage, kafkaProd)

	return &Storages{
		MessageStorage:    pg.MessageStorage(),
		RoomStorage:       pg.RoomStorage(),
		UserClientStorage: pg.UserClientStorage(),
		RoomCache:         redisStorage.RoomCache(),
		UserClientCache:   redisStorage.UserClientCache(),
		redisCli:          redisStorage.Client(),
		fileStor:          fs,
		kafkaProd:         kafkaProd,
		distributor:       distributor,
	}, nil
}

func (st *Storages) Close() {
	if st.kafkaProd != nil {
		st.kafkaProd.Close()
	}
	if st.redisCli != nil {
		st.redisCli.Close()
	}
	log.Println("All storage connections closed")
}

// type Storages struct {
//   // 🗄️ ОСНОВНЫЕ ХРАНИЛИЩА (Postgres ACID)
//   MessageStorage storage.MessageStorage      // сообщения (Postgres+Files)
//   RoomStorage    storage.RoomStorage         // комнаты (Postgres)
//   UserStorage    storage.UserStorage         // пользователи (Postgres)

//   // ⚡ КЭШИ И ВРЕМЕННЫЕ ДАННЫЕ
//   RoomCache      storage.RoomCache           // Redis TTL 10min
//   UserCache      storage.UserCache           // Redis TTL 10min
//   redisCli       *redis.Client               // сырой Redis

//   // 💾 ДОЛГОВРЕМЕННОЕ ХРАНЕНИЕ
//   fileStor       *filestorage.FileStorage    // архивы сообщений

//   // 📨 ОЧЕРЕДИ И MESSAGING
//   kafkaProd      *kafka.Producer             // исходящие очереди
//   // kafkaCons      *kafka.Consumer          // входящие (опционально)

//   // 🌐 РАСПРЕДЕЛИТЕЛЬ СООБЩЕНИЙ
//   distributor    *storage.MessageDistributor // WS Broadcast + Kafka
// }

// func NewStorages(cfg *config.Config) (*Storages, error) {
// 	ctx := context.Background()

// 	// 1️⃣ POSTGRES (основное хранилище ACID)
// 	pg, err := psql.NewPostgres(cfg.PostgresDSN)
// 	if err != nil {
// 		return nil, fmt.Errorf("Failed to connect to PostgreSQL: %w", err)
// 	}

// 	// 2️⃣ REDIS (кэш + сессии)
// 	redisStorage, err := redisstorage.NewRedisStorage(
// 		cfg.Redis.Addr,
// 		cfg.Redis.Password,
// 		cfg.Redis.DB,
// 	)
// 	if err != nil {
// 		return nil, fmt.Errorf("Failed to connect to Redis: %w", err)
// 	}

// 	// 3️⃣ FILE STORAGE (долговременный архив)
// 	fileStor := filestorage.New(cfg.File.BasePath, cfg.File.SegmentSize)

// 	// 4️⃣ KAFKA PRODUCER (асинхронная очередь)
// 	kafkaProd, err := kafka.NewProducer(&kafka.Config{
// 		Brokers: cfg.Kafka.Brokers,
// 		Topic:   cfg.Kafka.Topics.Incoming,
// 	})
// 	if err != nil {
// 		return nil, fmt.Errorf("Failed to create Kafka producer: %w", err)
// 	}

// 	// 5️⃣ DISTRIBUTOR (рассылка WS + Kafka)
// 	distributor := NewMessageDistributor(redisStorage, kafkaProd)

// 	// 🎯 СОБИРАЕМ ВСЕ В Storages
// 	return &Storages{
// 		MessageStorage: pg.MessageStorage(), // Postgres+Files
// 		RoomStorage:    pg.RoomStorage(),
// 		UserStorage:    pg.UserStorage(),
// 		RoomCache:      redisStorage.RoomCache(),
// 		UserCache:      redisStorage.UserCache(),
// 		redisCli:       redisStorage.Client(),
// 		fileStor:       fileStor,
// 		kafkaProd:      kafkaProd,
// 		distributor:    distributor,
// 	}, nil
// }

// func (s *Storages) Close() {
// 	if s.kafkaProd != nil {
// 		s.kafkaProd.Close() // Kafka producer
// 	}
// 	if s.redisCli != nil {
// 		s.redisCli.Close() // Redis pool
// 	}
// 	// Postgres закрывается автоматически (database/sql)
// 	log.Println("All storage connections closed")
// }
