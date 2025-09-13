package repository

import (
	"context"
	"errors"
	"fmt"
	db_queries "shopping/database/queries"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/rs/zerolog/log"
)

type ShoppingListRepository interface {
	GetShoppingListByID(id string) (*db_queries.ShoppingList, error)
	CreateShoppingList(name string, items []string) (*db_queries.ShoppingList, error)
	DeleteShoppingListByID(id string) error
	GetAllShoppingLists() (*[]db_queries.ShoppingList, error)
	PartialUpdate(id string, name *string, items *[]string) (*db_queries.ShoppingList, error)
	UpdateShoppingListByID(id string, name string, items []string) (*db_queries.ShoppingList, error)
	PushItemToShoppingList(id string, item string) (*db_queries.ShoppingList, error)
}

type ShoppingListPostgresRepository struct {
	dbQueries *db_queries.Queries
}

func NewShoppingListRepository(dbQueries *db_queries.Queries) ShoppingListRepository {
	return &ShoppingListPostgresRepository{
		dbQueries: dbQueries,
	}
}

func (r *ShoppingListPostgresRepository) GetAllShoppingLists() (*[]db_queries.ShoppingList, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := r.dbQueries.GetAllShoppingLists(ctx)
	if err != nil {
		log.Err(err).Msg("repository: error to get all shopping lists")
		return nil, errors.New("repository: error to get all the shopping lists")
	}

	return &rows, err
}

func (r *ShoppingListPostgresRepository) CreateShoppingList(name string, items []string) (*db_queries.ShoppingList, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	row, err := r.dbQueries.CreateShoppingList(ctx, db_queries.CreateShoppingListParams{
		Name:  name,
		Items: items,
	})

	if err != nil {
		return nil, errors.New(fmt.Sprintf("error to create the new shopping list with name '%s% and items '%v'", name, items))
	}

	return &row, err
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

func (r *ShoppingListPostgresRepository) GetShoppingListByID(id string) (*db_queries.ShoppingList, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_uid, err := uuid.Parse(id)
	if err != nil {
		return nil, errors.New("invalid uuid")
	}

	uid := pgtype.UUID{
		Bytes: _uid,
		Valid: true,
	}

	shoppingList, err := r.dbQueries.GetShoppingListByID(ctx, uid)
	if err != nil {
		return nil, err
	}

	return &shoppingList, err
}

func (r *ShoppingListPostgresRepository) DeleteShoppingListByID(id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_uid, err := uuid.Parse(id)
	if err != nil {
		log.Err(err).Msg("invalid uuid when deleting a shopping list")
		return errors.New("invalid uuid id")
	}

	uid := pgtype.UUID{
		Bytes: _uid,
		Valid: true,
	}

	err = r.dbQueries.DeleteShoppingListByID(ctx, uid)
	if err != nil {
		log.Err(err).Msgf("Error to delete the shopping list with uuid: '%s'", uid.String())
		return errors.New(fmt.Sprintf("Error to delete the shopping list with the uuid: '%s'", uid.String()))
	}

	return nil
}

func (r *ShoppingListPostgresRepository) UpdateShoppingListByID(id string, name string, items []string) (*db_queries.ShoppingList, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	uid, err := convertStringToUUID(id)
	if err != nil {
		return nil, err
	}

	updated, err := r.dbQueries.UpdateShoppingListByID(ctx, db_queries.UpdateShoppingListByIDParams{
		ID:    uid,
		Name:  name,
		Items: items,
	})
	if err != nil {
		msg := fmt.Sprintf("repository: error to update the shopping list wiht id: %s", id)
		log.Err(err).Msg(msg)
		return nil, errors.New(msg)
	}

	return &updated, nil
}

func (r *ShoppingListPostgresRepository) PushItemToShoppingList(id string, item string) (*db_queries.ShoppingList, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	uid, err := convertStringToUUID(id)
	if err != nil {
		return nil, err
	}

	updated, err := r.dbQueries.PushItemToShoppingList(ctx, db_queries.PushItemToShoppingListParams{
		ID:    uid,
		Items: []string{item},
	})
	if err != nil {
		return nil, errors.New("error to push item")
	}

	return &updated, nil
}

func convertStringToUUID(value string) (pgtype.UUID, error) {
	v, err := uuid.Parse(value)
	if err != nil {
		return pgtype.UUID{Valid: false}, errors.New("invalid uuid")
	}

	return pgtype.UUID{
		Bytes: v,
		Valid: true,
	}, nil
}
