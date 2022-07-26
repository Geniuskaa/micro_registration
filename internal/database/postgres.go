package database

import (
	"context"
	"fmt"
	"github.com/Geniuskaa/micro_registration/internal/config"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/log/zapadapter"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.uber.org/zap"
	"time"
)

type Postgres struct {
	Pool *pgxpool.Pool
}

func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{Pool: pool}
}

func PoolCreation(ctx context.Context, logger *zap.Logger, conf *config.Entity) *pgxpool.Pool {
	dbConf, err := pgxpool.ParseConfig(fmt.Sprintf("postgres://%s:%s@%s:%d/%s",
		conf.DB.User, conf.DB.Pass, conf.DB.Hostname, conf.DB.Port, conf.DB.Name))
	if err != nil {
		logger.Panic("Err db config parsing", zap.Error(fmt.Errorf("poolCreation failed: %w", err)))
	}
	dbConf.ConnConfig.Logger = zapadapter.NewLogger(logger)
	dbConf.ConnConfig.LogLevel = pgx.LogLevelError
	dbConf.MaxConnLifetime = time.Minute * time.Duration(conf.DB.ConnLifeTime)
	dbConf.MaxConnIdleTime = time.Second * 25
	dbConf.MaxConns = conf.DB.MaxOpenConns // при настройке объединения микросервисов стоит
	dbConf.MinConns = conf.DB.MinConns     // учесть макс соединения на сервисе и в БД

	pool, err := pgxpool.ConnectConfig(ctx, dbConf)
	if err != nil {
		logger.Panic("Err connection to DB", zap.Error(fmt.Errorf("poolCreation failed: %w", err)))
	}

	return pool
}
