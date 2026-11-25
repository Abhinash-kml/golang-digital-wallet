package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	queries "github.com/abhinash-kml/digital-wallet/gen"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

func main() {
	fmt.Println("Hello World")

	postgresDsn := fmt.Sprintf("postgresql://%s:%s@%s/%s?sslmode=disable", "postgres", "Abx305", "localhost:5432", "digitalwallet")
	fmt.Println(postgresDsn)
	conn, err := pgx.Connect(context.Background(), postgresDsn)
	if err != nil {
		log.Fatalf("Failed to connect to postgress. Error: %s", err)
	}
	defer conn.Close(context.Background())

	db := stdlib.OpenDB(*conn.Config())
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		log.Fatalf("failed to create postgres driver: %s", err)
	}

	// Create migrate instance with file source and postgres driver
	m, err := migrate.NewWithDatabaseInstance("file://db/migrations", "postgres", driver)
	if err != nil {
		log.Fatalf("failed to create migrate instance: %s", err)
	}

	err = m.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatalf("Failed to migrate up. Error: %s", err)
	}

	_ = conn

	q := queries.New(conn)
	account, err := q.CreateAccount(context.Background(), queries.CreateAccountParams{
		Name: "neo",
		Type: queries.NullAccountType{
			AccountType: queries.AccountTypeSAVINGS,
			Valid:       true,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%v", account)
}
