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

-- name: GetTransactionByIdempotency :one
SELECT *
FROM transactions
WHERE idempotency = $1;

-- name: GetAccountBalance :one
SELECT COALESCE(SUM(
    CASE direction
        WHEN 'CREDIT' THEN amount_minor
        WHEN 'DEBIT' THEN -amount_minor
    END
), 0)::BIGINT AS balance_minor
FROM ledger
WHERE account_id = $1;

-- name: GetAccount :one
SELECT *
FROM accounts
WHERE id = $1;