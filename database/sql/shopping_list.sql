-- name: ShoppingListPartialUpdate :one
UPDATE shopping_lists
SET name = COALESCE(sqlc.narg('name'), name),
    items = COALESCE(sqlc.narg('items'), items),
    updated_at = NOW()
WHERE id = $1
RETURNING id, name, items, created_at, updated_at;

-- name: UpdateShoppingListByID :one
-- its a full update
UPDATE shopping_lists
SET name = $2,
    items = $3,
    updated_at = NOW()
WHERE id = $1
RETURNING id, name, items, created_at, updated_at;

-- name: GetShoppingListByID :one
SELECT id, name, items, created_at, updated_at
FROM shopping_lists
WHERE id = $1;

-- name: CreateShoppingList :one
INSERT INTO shopping_lists (name, items)
VALUES ($1, $2)
RETURNING id, name, items, created_at, updated_at;

-- name: DeleteShoppingListByID :exec
DELETE FROM shopping_lists
WHERE id = $1;

-- name: GetAllShoppingLists :many
SELECT id, name, items, created_at, updated_at
FROM shopping_lists;

-- name: PushItemToShoppingList :one
UPDATE shopping_lists
SET items = items || $2, updated_at = NOW()
WHERE id = $1
RETURNING id, name, items, created_at, updated_at;