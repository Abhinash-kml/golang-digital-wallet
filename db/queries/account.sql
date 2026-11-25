
-- name: CreateAccount :one
INSERT INTO accounts(name, type)
VALUES ($1, $2)
RETURNING *;

-- name: DeleteAccount :one
DELETE FROM accounts
WHERE id = $1
RETURNING id;

-- name: FreezeAcount :one
UPDATE accounts
SET isFrozen = TRUE
WHERE id = $1
RETURNING *;