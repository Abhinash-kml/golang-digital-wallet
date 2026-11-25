package accounts_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
)

var conn *pgx.Conn

func TestMain(m *testing.M) {
	postgresDsn := fmt.Sprintf("postgresql://%s:%s@%s/%s?sslmode=disable", "postgres", "Abx305", "localhost:5432", "digitalwallet")
	c, err := pgx.Connect(context.Background(), postgresDsn)
	if err != nil {
		fmt.Println("Failed to connect to postgres")
	}
	conn = c
	m.Run()
}
