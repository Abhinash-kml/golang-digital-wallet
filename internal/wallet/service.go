package wallet

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
	// ErrInvalidInput represents malformed IDs or invalid transfer amounts.
	ErrInvalidInput = errors.New("invalid input")
	// ErrNotFound represents an account referenced by a transfer that does not exist.
	ErrNotFound = errors.New("not found")
	// ErrAlreadyExists represents a unique-key conflict, including reused idempotency keys.
	ErrAlreadyExists = errors.New("already exists")
	// ErrInsufficientFunds is returned when a debit would overdraw the source account.
	ErrInsufficientFunds = errors.New("insufficient funds")
	// ErrAccountFrozen prevents transfers involving frozen accounts.
	ErrAccountFrozen = errors.New("account frozen")
)

// Service coordinates wallet operations and their database queries.
type Service struct {
	conn *pgx.Conn
	q    *queries.Queries
}

// NewService creates a wallet service using the provided PostgreSQL connection.
func NewService(pgxConn *pgx.Conn) *Service {
	return &Service{
		conn: pgxConn,
		q:    queries.New(pgxConn),
	}
}

// Credit adds funds to accountID and debits counterpartyID.
func (s *Service) Credit(ctx context.Context, idempotency, accountID, counterpartyID pgtype.UUID, amountMinor int64) (queries.Transaction, error) {
	return s.transfer(ctx, idempotency, accountID, counterpartyID, amountMinor, false)
}

// Debit removes funds from accountID and credits counterpartyID.
func (s *Service) Debit(ctx context.Context, idempotency, accountID, counterpartyID pgtype.UUID, amountMinor int64) (queries.Transaction, error) {
	return s.transfer(ctx, idempotency, accountID, counterpartyID, amountMinor, true)
}

// Deposit is a credit into accountID from counterpartyID.
func (s *Service) Deposit(ctx context.Context, idempotency, accountID, counterpartyID pgtype.UUID, amountMinor int64) (queries.Transaction, error) {
	return s.Credit(ctx, idempotency, accountID, counterpartyID, amountMinor)
}

// Withdrawal is a debit from accountID into counterpartyID.
func (s *Service) Withdrawal(ctx context.Context, idempotency, accountID, counterpartyID pgtype.UUID, amountMinor int64) (queries.Transaction, error) {
	return s.Debit(ctx, idempotency, accountID, counterpartyID, amountMinor)
}

func (s *Service) transfer(
	ctx context.Context,
	idempotency, accountID, counterpartyID pgtype.UUID,
	amountMinor int64,
	debitAccount bool,
) (queries.Transaction, error) {
	// Reject missing UUIDs before issuing any database queries.
	if !idempotency.Valid || !accountID.Valid || !counterpartyID.Valid {
		return queries.Transaction{}, ErrInvalidInput
	}

	// Ledger entries represent positive minor-unit amounts; direction records the sign.
	if amountMinor <= 0 {
		return queries.Transaction{}, ErrInvalidInput
	}

	// A self-transfer produces confusing ledger activity and should not be recorded.
	if accountID == counterpartyID {
		return queries.Transaction{}, ErrInvalidInput
	}

	// Confirm the target account exists and is eligible for transactions.
	account, err := s.q.GetAccount(ctx, accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return queries.Transaction{}, ErrNotFound
		}
		return queries.Transaction{}, fmt.Errorf("loading account: %w", mapPostgresError(err))
	}
	if account.Isfrozen.Valid && account.Isfrozen.Bool {
		return queries.Transaction{}, ErrAccountFrozen
	}

	// The counterparty is also required because both sides of the ledger reference accounts.
	counterparty, err := s.q.GetAccount(ctx, counterpartyID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return queries.Transaction{}, ErrNotFound
		}
		return queries.Transaction{}, fmt.Errorf("loading counterparty: %w", mapPostgresError(err))
	}
	if counterparty.Isfrozen.Valid && counterparty.Isfrozen.Bool {
		return queries.Transaction{}, ErrAccountFrozen
	}

	// Only operations that debit the target account require available funds.
	if debitAccount {
		balance, err := s.q.GetAccountBalance(ctx, accountID)
		if err != nil {
			return queries.Transaction{}, fmt.Errorf("getting account balance: %w", mapPostgresError(err))
		}
		if balance < amountMinor {
			return queries.Transaction{}, ErrInsufficientFunds
		}
	}

	// Return the previously committed transaction when a caller retries an idempotency key.
	existing, err := s.q.GetTransactionByIdempotency(ctx, idempotency)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return queries.Transaction{}, fmt.Errorf("checking transaction idempotency: %w", mapPostgresError(err))
	}

	// Start a database transaction
	tx, err := s.conn.Begin(ctx)
	if err != nil {
		return queries.Transaction{}, fmt.Errorf("beginning wallet transaction: %w", mapPostgresError(err))
	}
	defer tx.Rollback(ctx)

	// Bind generated queries to this transaction so all writes share the same commit boundary.
	qtx := s.q.WithTx(tx)
	transaction, err := qtx.CreateTransaction(ctx, idempotency)
	if err != nil {
		if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok && pgErr.Code == pgerrcode.UniqueViolation {
			// A concurrent request may insert the same key after the earlier lookup.
			// PostgreSQL aborts this transaction after the uniqueness error, so roll it
			// back before fetching and returning the competing request's committed result.
			tx.Rollback(ctx)
			existing, lookupErr := s.q.GetTransactionByIdempotency(ctx, idempotency)
			if lookupErr == nil {
				return existing, nil
			}
		}
		return queries.Transaction{}, fmt.Errorf("creating transaction: %w", mapPostgresError(err))
	}

	// Write a debit and credit for equal amounts to maintain double-entry accounting.
	// The database's deferred balance constraint verifies the entries when the transaction commits.
	var entriesErr error
	if debitAccount {
		// In a debit operation, the target pays and the counterparty receives.
		entriesErr = qtx.DebitFromAccount(ctx, queries.DebitFromAccountParams{
			TransactionID: transaction.ID,
			AccountID:     accountID,
			AmountMinor:   amountMinor,
		})
		if entriesErr == nil {
			entriesErr = qtx.CreditToAccount(ctx, queries.CreditToAccountParams{
				TransactionID: transaction.ID,
				AccountID:     counterpartyID,
				AmountMinor:   amountMinor,
			})
		}
	} else {
		// In a credit operation, the counterparty pays and the target receives.
		entriesErr = qtx.DebitFromAccount(ctx, queries.DebitFromAccountParams{
			TransactionID: transaction.ID,
			AccountID:     counterpartyID,
			AmountMinor:   amountMinor,
		})
		if entriesErr == nil {
			entriesErr = qtx.CreditToAccount(ctx, queries.CreditToAccountParams{
				TransactionID: transaction.ID,
				AccountID:     accountID,
				AmountMinor:   amountMinor,
			})
		}
	}
	if entriesErr != nil {
		return queries.Transaction{}, fmt.Errorf("writing ledger entries: %w", mapPostgresError(entriesErr))
	}

	// Commit makes the transaction and both balanced ledger entries visible together.
	if err := tx.Commit(ctx); err != nil {
		return queries.Transaction{}, fmt.Errorf("committing wallet transaction: %w", mapPostgresError(err))
	}
	return transaction, nil
}

// mapPostgresError translates known database constraint errors into domain errors.
func mapPostgresError(err error) error {
	// Preserve non-PostgreSQL errors for callers to inspect or wrap.
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	if !ok {
		return err
	}

	// Translate SQLSTATE codes while retaining the PostgreSQL detail for diagnosis.
	switch pgErr.Code {
	case pgerrcode.UniqueViolation:
		return fmt.Errorf("%w: %s", ErrAlreadyExists, pgErr.Detail)
	case pgerrcode.ForeignKeyViolation:
		return fmt.Errorf("%w: %s", ErrNotFound, pgErr.Detail)
	case pgerrcode.NotNullViolation, pgerrcode.CheckViolation:
		return fmt.Errorf("%w: %s", ErrInvalidInput, pgErr.Message)
	default:
		return err
	}
}
