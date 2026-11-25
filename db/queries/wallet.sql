-- name: CreateTransaction :one
INSERT INTO transactions(idempotency)
VALUES 
($1)
RETURNING *;

-- name: DebitFromAccount :exec
INSERT INTO ledger(transaction_id, account_id, amount_minor, direction)
VALUES
($1, $2, $3, 'DEBIT');

-- name: CreditToAccount :exec
INSERT INTO ledger(transaction_id, account_id, amount_minor, direction)
VALUES
($1, $2, $3, 'CREDIT');