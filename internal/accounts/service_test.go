package accounts_test

import (
	"context"
	"testing"

	queries "github.com/abhinash-kml/digital-wallet/gen"
	"github.com/abhinash-kml/digital-wallet/internal/accounts"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newService(t *testing.T) *accounts.Service {
	t.Helper()
	return accounts.NewService(conn)
}

func truncateTable(t *testing.T) {
	_, err := conn.Exec(context.Background(), "TRUNCATE TABLE accounts RESTART IDENTITY CASCADE")
	require.NoError(t, err)
}

func TestCreateAccount(t *testing.T) {
	//t.Parallel()
	t.Cleanup(func() {
		truncateTable(t)
	})

	srv := newService(t)
	name := "neo"
	acc, err := srv.Create(t.Context(), name, "savings")
	require.NoError(t, err)
	assert.Equal(t, acc.Name, name)
	assert.Equal(t, queries.AccountTypeSAVINGS, acc.Type.AccountType)
}

func TestCreateCurrentAccount(t *testing.T) {
	t.Cleanup(func() {
		truncateTable(t)
	})

	srv := newService(t)
	acc, err := srv.Create(t.Context(), "neo", "CURRENT")
	require.NoError(t, err)
	assert.Equal(t, queries.AccountTypeCURRENT, acc.Type.AccountType)
}

func TestCreateAccountInvalidInput(t *testing.T) {
	srv := newService(t)

	_, err := srv.Create(t.Context(), "  ", "savings")
	assert.ErrorIs(t, err, accounts.ErrInvalidInput)

	_, err = srv.Create(t.Context(), "neo", "checking")
	assert.ErrorIs(t, err, accounts.ErrInvalidInput)
}

func TestDeleteAccount(t *testing.T) {
	//t.Parallel()
	t.Cleanup(func() {
		truncateTable(t)
	})

	// Prepare
	srv := newService(t)
	name := "neo"

	// Act
	acc, err := srv.Create(t.Context(), name, "savings")
	require.NoError(t, err)

	// Assert
	assert.Equal(t, acc.Name, name)

	// Act main
	deletedID, err := srv.Delete(t.Context(), acc.ID)
	require.NoError(t, err)

	// Assert
	assert.Equal(t, deletedID, acc.ID)
}

func TestDeleteAccountNonExistent(t *testing.T) {
	t.Cleanup(func() {
		truncateTable(t)
	})

	// Prepare
	srv := newService(t)
	accID, _ := uuid.NewRandom()

	// Act
	_, err := srv.Delete(t.Context(), pgtype.UUID{Bytes: accID})

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, accounts.ErrNotFound)
}

func TestFreezeAccount(t *testing.T) {
	t.Cleanup(func() {
		truncateTable(t)
	})

	// Prepare
	srv := newService(t)
	acc, err := srv.Create(t.Context(), "neo", "savings")
	require.NoError(t, err)
	require.Equal(t, acc.Isfrozen, pgtype.Bool{Bool: false, Valid: true})

	// Act
	accFrozen, err := srv.Freeze(t.Context(), acc.ID)
	require.NoError(t, err)
	require.Equal(t, acc.ID, accFrozen.ID)

	// Assert
	assert.Equal(t, accFrozen.Isfrozen, pgtype.Bool{Bool: true, Valid: true})
}

func TestFreezeAccountNotExist(t *testing.T) {
	t.Cleanup(func() {
		truncateTable(t)
	})

	// Prepare
	srv := newService(t)
	randomID, _ := uuid.NewRandom()

	// Act
	_, err := srv.Freeze(t.Context(), pgtype.UUID{Bytes: randomID})
	require.NotNil(t, err)

	// Assert
	assert.ErrorIs(t, err, accounts.ErrNotFound)
}
