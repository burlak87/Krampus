package storage

import (
	"context"
	"krampus/internal/domain"
	"krampus/pkg/client-database/redis"
	"krampus/pkg/config"
)

type Storage struct {
	MessageStorage     domain.MessageStorage
	RoomStorage        domain.RoomStorage
	UserClientStorage  domain.UserClientStorage
	RoomCache          domain.RoomCache
	MessageDistributor domain.MessageDistributor
	// Redis, File, Kafka fields
	redisCli  *redis.Client
	fileStor  *filestorage.FileStorage
	kafkaProd *KafkaProducer
}

func NewStorages(cfg *config.Config) (*Storages, error) {
	// Postgres
	pg, err := psql.NewPostgres(cfg.PostgresDSN)
	if err != nil {
		return nil, err
	}

	// redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}

	// FileStorage
	fs := filestorage.New(cfg.File.BasePath, cfg.File.SegmentSize)

	return &Storages{
		MessageStorage:    pg.MessageStorage(),
		RoomStorage:       pg.RoomStorage(),
		UserClientStorage: pg.UserClientStorage(),
		RoomCache:         redis.NewRoomCache(rdb),
		redisCli:          redis.NewRedisStorage(rdb),
		fileStor:          fs,
	}, nil
}

func (st *Storages) Close() {

}
