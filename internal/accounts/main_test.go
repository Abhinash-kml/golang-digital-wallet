package accounts_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

var conn *pgx.Conn

func TestMain(m *testing.M) {
	postgresDsn := fmt.Sprintf("postgresql://%s:%s@%s/%s?sslmode=disable", "postgres", "Abx305", "localhost:5432", "digitalwallet")

	c, err := pgx.Connect(context.Background(), postgresDsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to connect to postgres: %v\n", err)
		os.Exit(1)
	}
	conn = c
	code := m.Run()
	_ = conn.Close(context.Background())
	os.Exit(code)
}
