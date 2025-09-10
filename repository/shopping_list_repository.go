package repository

import (
	"context"
	"errors"
	db_queries "shopping/database/queries"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
)

type ShoppingListRepository interface {
	PartialUpdate(id string, name *string, items *[]string) (*db_queries.ShoppingList, error)
}

type ShoppingListPostgresRepository struct {
	dbQueries *db_queries.Queries
}

func NewShoppingListRepository(dbQueries *db_queries.Queries) ShoppingListRepository {
	return &ShoppingListPostgresRepository{
		dbQueries: dbQueries,
	}
}

func (r *ShoppingListPostgresRepository) PartialUpdate(id string, name *string, items *[]string) (
	*db_queries.ShoppingList, error,
) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, errors.New("invalid id value")
	}

	params := db_queries.ShoppingListPartialUpdateParams{
		ID: pgtype.UUID{
			Bytes: uid,
			Valid: true,
		},
	}

	if name != nil && *name != "" {
		params.Name = pgtype.Text{
			String: *name,
			Valid:  true,
		}
	}

	if items != nil {
		params.Items = *items
	}

	row, err := r.dbQueries.ShoppingListPartialUpdate(
		ctx,
		params,
	)

	if err != nil {
		log.Debug().Msgf("shopping list partial update error: %s", err.Error())
		return nil, err
	}

	return &row, nil
}
