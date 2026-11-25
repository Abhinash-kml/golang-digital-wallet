package accounts_test

import (
	"context"
	"testing"

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
