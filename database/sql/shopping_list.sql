-- name: ShoppingListPartialUpdate :one
UPDATE shopping_lists
SET name = COALESCE(sqlc.narg('name'), name),
    items = COALESCE(sqlc.narg('items'), items)
WHERE id = $1
RETURNING id, name, items, created_at, updated_at;