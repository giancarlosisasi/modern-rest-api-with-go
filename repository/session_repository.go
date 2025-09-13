package repository

import (
	"context"
	"math/rand"
	db_queries "shopping/database/queries"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
)

type SessionRepository interface {
	AddSession(username string) (*db_queries.AddSessionRow, error)
	GetSessionByToken(token string) (*db_queries.GetSessionByTokenRow, error)
}

type SessionPostgresRepository struct {
	DBQueries *db_queries.Queries
}

func NewSessionRepository(dbQueries *db_queries.Queries) SessionRepository {
	return &SessionPostgresRepository{
		DBQueries: dbQueries,
	}
}

func (r *SessionPostgresRepository) AddSession(username string) (*db_queries.AddSessionRow, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	token := strconv.Itoa(rand.Intn(100000000000))

	row, err := r.DBQueries.AddSession(ctx, db_queries.AddSessionParams{
		Token: token,
		ExpiresAt: pgtype.Timestamptz{
			Time:  time.Now().Add(7 * 24 * time.Hour),
			Valid: true,
		},
		Username: username,
	})

	if err != nil {
		log.Debug().Msgf("> add session error: %s", err.Error())
		return nil, err
	}

	return &row, nil
}

func (r *SessionPostgresRepository) GetSessionByToken(token string) (*db_queries.GetSessionByTokenRow, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	row, err := r.DBQueries.GetSessionByToken(ctx, token)
	if err != nil {
		log.Debug().Msgf("> get session by token error: %s", err.Error())
		return nil, err
	}

	return &row, nil
}
