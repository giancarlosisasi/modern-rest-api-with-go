package database

import (
	"context"
	"shopping/config"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

func NewDB(config *config.Config) (*pgxpool.Pool, error) {
	dbConfig, err := pgxpool.ParseConfig(
		config.DBUrl,
	)
	if err != nil {
		log.Err(err).Msg("there was an error creating the database configuration")
		return nil, err
	}

	dbConfig.MaxConns = 30
	dbConfig.MaxConnIdleTime = 15 * time.Minute

	dbpool, err := pgxpool.NewWithConfig(context.Background(), dbConfig)
	if err != nil {
		log.Err(err).Msg("there was an error connecting to the database...")
		return nil, err
	}

	err = dbpool.Ping(context.Background())
	if err != nil {
		log.Err(err).Msg("error to ping the database")
		return nil, err
	}

	log.Info().Msg("> success to connect to the database")

	return dbpool, nil
}
