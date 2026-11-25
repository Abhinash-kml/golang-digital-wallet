package accounts

import (
	"context"
	"errors"
	"fmt"

	queries "github.com/abhinash-kml/digital-wallet/gen"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrAlreadyExists = errors.New("already exists")
	ErrInvalidInput  = errors.New("invalid input")
	ErrNotFound      = errors.New("not founnd")
)

type Service struct {
	q *queries.Queries
}

func NewService(pgxConn *pgx.Conn) *Service {
	return &Service{
		q: queries.New(pgxConn),
	}
}

func (s *Service) Create(ctx context.Context, name, acctype string) (queries.Account, error) {
	acc, err := s.q.CreateAccount(ctx, queries.CreateAccountParams{
		Name: name,
		Type: queries.NullAccountType{
			AccountType: queries.AccountTypeSAVINGS,
			Valid:       true,
		},
	})
	if err != nil {
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
			switch pgErr.Code {
			case pgerrcode.UniqueViolation:
				return queries.Account{}, ErrAlreadyExists
			case pgerrcode.NotNullViolation:
				return queries.Account{}, ErrInvalidInput
			}
		}
		return queries.Account{}, fmt.Errorf("creating account: %w", err)
	}
	return acc, nil
}

func (s *Service) Delete(ctx context.Context, accID pgtype.UUID) (pgtype.UUID, error) {
	deletedID, err := s.q.DeleteAccount(ctx, accID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return pgtype.UUID{}, ErrNotFound
		}
		return pgtype.UUID{}, fmt.Errorf("deleting account: %w", err)
	}
	return deletedID, nil
}

func (s *Service) Freeze(ctx context.Context, accID pgtype.UUID) (queries.Account, error) {
	acc, err := s.q.FreezeAcount(ctx, accID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return queries.Account{}, ErrNotFound
		}
		return queries.Account{}, fmt.Errorf("deleting account: %w", err)
	}
	return acc, nil
}
