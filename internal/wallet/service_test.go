package wallet_test

import (
	"context"
	"testing"

	"github.com/abhinash-kml/digital-wallet/internal/wallet"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newService(t *testing.T) *wallet.Service {
	t.Helper()
	return wallet.NewService(conn)
}

func truncateTables(t *testing.T) {
	t.Helper()
	_, err := conn.Exec(context.Background(), "TRUNCATE TABLE ledger, transactions, accounts RESTART IDENTITY CASCADE")
	require.NoError(t, err)
}

func newUUID(t *testing.T) pgtype.UUID {
	t.Helper()
	id, err := uuid.NewRandom()
	require.NoError(t, err)
	return pgtype.UUID{Bytes: id, Valid: true}
}

func createAccount(t *testing.T, name string) pgtype.UUID {
	t.Helper()
	id := newUUID(t)
	_, err := conn.Exec(
		context.Background(),
		"INSERT INTO accounts(id, name, type, isFrozen) VALUES ($1, $2, 'SAVINGS', FALSE)",
		id,
		name,
	)
	require.NoError(t, err)
	return id
}

func freezeAccount(t *testing.T, accountID pgtype.UUID) {
	t.Helper()
	_, err := conn.Exec(context.Background(), "UPDATE accounts SET isFrozen = TRUE WHERE id = $1", accountID)
	require.NoError(t, err)
}

func accountBalance(t *testing.T, accountID pgtype.UUID) int64 {
	t.Helper()
	var balance int64
	err := conn.QueryRow(
		context.Background(),
		`SELECT COALESCE(SUM(
			CASE direction
				WHEN 'CREDIT' THEN amount_minor
				WHEN 'DEBIT' THEN -amount_minor
			END
		), 0)
		FROM ledger
		WHERE account_id = $1`,
		accountID,
	).Scan(&balance)
	require.NoError(t, err)
	return balance
}

func TestDeposit(t *testing.T) {
	t.Cleanup(func() {
		truncateTables(t)
	})

	srv := newService(t)
	accountID := createAccount(t, "neo")
	counterpartyID := createAccount(t, "holding")
	idempotency := newUUID(t)

	transaction, err := srv.Deposit(t.Context(), idempotency, accountID, counterpartyID, 100)
	require.NoError(t, err)
	assert.Equal(t, idempotency, transaction.Idempotency)
	assert.Equal(t, int64(100), accountBalance(t, accountID))
	assert.Equal(t, int64(-100), accountBalance(t, counterpartyID))
}

func TestCredit(t *testing.T) {
	t.Cleanup(func() {
		truncateTables(t)
	})

	srv := newService(t)
	accountID := createAccount(t, "neo")
	counterpartyID := createAccount(t, "holding")

	_, err := srv.Credit(t.Context(), newUUID(t), accountID, counterpartyID, 75)
	require.NoError(t, err)
	assert.Equal(t, int64(75), accountBalance(t, accountID))
}

func TestDebit(t *testing.T) {
	t.Cleanup(func() {
		truncateTables(t)
	})

	srv := newService(t)
	accountID := createAccount(t, "neo")
	counterpartyID := createAccount(t, "holding")

	_, err := srv.Deposit(t.Context(), newUUID(t), accountID, counterpartyID, 100)
	require.NoError(t, err)

	_, err = srv.Debit(t.Context(), newUUID(t), accountID, counterpartyID, 40)
	require.NoError(t, err)
	assert.Equal(t, int64(60), accountBalance(t, accountID))
	assert.Equal(t, int64(-60), accountBalance(t, counterpartyID))
}

func TestWithdrawal(t *testing.T) {
	t.Cleanup(func() {
		truncateTables(t)
	})

	srv := newService(t)
	accountID := createAccount(t, "neo")
	counterpartyID := createAccount(t, "holding")

	_, err := srv.Deposit(t.Context(), newUUID(t), accountID, counterpartyID, 50)
	require.NoError(t, err)

	_, err = srv.Withdrawal(t.Context(), newUUID(t), accountID, counterpartyID, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(30), accountBalance(t, accountID))
}

func TestInsufficientFunds(t *testing.T) {
	t.Cleanup(func() {
		truncateTables(t)
	})

	srv := newService(t)
	accountID := createAccount(t, "neo")
	counterpartyID := createAccount(t, "holding")

	_, err := srv.Withdrawal(t.Context(), newUUID(t), accountID, counterpartyID, 1)
	require.Error(t, err)
	assert.ErrorIs(t, err, wallet.ErrInsufficientFunds)
}

func TestInvalidInput(t *testing.T) {
	t.Cleanup(func() {
		truncateTables(t)
	})

	srv := newService(t)
	accountID := createAccount(t, "neo")

	_, err := srv.Deposit(t.Context(), pgtype.UUID{}, accountID, newUUID(t), 100)
	require.Error(t, err)
	assert.ErrorIs(t, err, wallet.ErrInvalidInput)

	_, err = srv.Deposit(t.Context(), newUUID(t), accountID, accountID, 100)
	require.Error(t, err)
	assert.ErrorIs(t, err, wallet.ErrInvalidInput)

	_, err = srv.Deposit(t.Context(), newUUID(t), accountID, newUUID(t), 0)
	require.Error(t, err)
	assert.ErrorIs(t, err, wallet.ErrInvalidInput)
}

func TestAccountNotFound(t *testing.T) {
	t.Cleanup(func() {
		truncateTables(t)
	})

	srv := newService(t)
	missingID := newUUID(t)
	counterpartyID := createAccount(t, "holding")

	_, err := srv.Deposit(t.Context(), newUUID(t), missingID, counterpartyID, 100)
	require.Error(t, err)
	assert.ErrorIs(t, err, wallet.ErrNotFound)
}

func TestFrozenAccount(t *testing.T) {
	t.Cleanup(func() {
		truncateTables(t)
	})

	srv := newService(t)
	accountID := createAccount(t, "neo")
	counterpartyID := createAccount(t, "holding")
	freezeAccount(t, accountID)

	_, err := srv.Deposit(t.Context(), newUUID(t), accountID, counterpartyID, 100)
	require.Error(t, err)
	assert.ErrorIs(t, err, wallet.ErrAccountFrozen)
}

func TestIdempotentDeposit(t *testing.T) {
	t.Cleanup(func() {
		truncateTables(t)
	})

	srv := newService(t)
	accountID := createAccount(t, "neo")
	counterpartyID := createAccount(t, "holding")
	idempotency := newUUID(t)

	first, err := srv.Deposit(t.Context(), idempotency, accountID, counterpartyID, 100)
	require.NoError(t, err)

	second, err := srv.Deposit(t.Context(), idempotency, accountID, counterpartyID, 100)
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID)
	assert.Equal(t, int64(100), accountBalance(t, accountID))
}
