// pkg/client-database/postgresql/client.go
package postgresql

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/your-team/taskmanager-chat/backend/pkg/config"
	repeatable "github.com/your-team/taskmanager-chat/backend/pkg/utils"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Client interface {
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

func NewClient(ctx context.Context, maxAttempts int, sc config.StorageConfig) (pool *pgxpool.Pool, err error) {
	dsn := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s", sc.Username, sc.Password, sc.Host, sc.Port, sc.Database)
	err = repeatable.DoWithTries(func() error {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		
		pool, err = pgxpool.New(ctx, dsn)
		if err != nil {
			return err
		}
		
		return nil
	}, maxAttempts, 5*time.Second)
	
	if err != nil {
		log.Fatal("Error do with tries postgresql")
	}
	
	return pool, nil
}