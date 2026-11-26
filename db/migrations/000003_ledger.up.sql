BEGIN;

CREATE TYPE transaction_direction AS ENUM('CREDIT', 'DEBIT');

CREATE TABLE IF NOT EXISTS ledger(
    entry_id UUID PRIMARY KEY DEFAULT uuidv7(),
    transaction_id UUID NOT NULL,
    account_id UUID NOT NULL,
    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    direction transaction_direction NOT NULL,

    FOREIGN KEY(transaction_id) REFERENCES transactions(id)
    ON DELETE CASCADE,

    FOREIGN KEY(account_id) REFERENCES accounts(id)
    ON DELETE CASCADE
);

-- Function to prevent ledger mutations
CREATE OR REPLACE FUNCTION prevent_ledger_mutation()
RETURNS TRIGGER
LANGUAGE PLPGSQL
AS $$
BEGIN
    RAISE EXCEPTION 'LEDGER TABLE IS IMMUTABLE'
    USING ERRCODE = '55001';
    RETURN NULL;
END
$$;

-- Trigger to use the above function
CREATE TRIGGER trigger_prevent_mutations
BEFORE UPDATE OR DELETE
ON ledger
FOR EACH ROW
EXECUTE FUNCTION prevent_ledger_mutation();

-- Function to check balanced entries
CREATE OR REPLACE FUNCTION check_balanced_ledger_entry()
RETURNS TRIGGER
LANGUAGE PLPGSQL
AS $$
DECLARE
    target_transaction_id UUID;
    net_balance BIGINT;
    line_count BIGINT;
BEGIN
    target_transaction_id := NEW.transaction_id;

    -- Calculate net balance and line count for this transaction.
    -- Debits subtract from the transaction total and credits add to it.
    SELECT COALESCE(SUM(
        CASE direction
            WHEN 'CREDIT' THEN amount_minor
            WHEN 'DEBIT' THEN -amount_minor
        END
    ), 0), COUNT(*)
    INTO net_balance, line_count
    FROM ledger
    WHERE transaction_id = target_transaction_id;

    -- Ensure at least 2 lines and a zero sum
    IF line_count < 2 OR net_balance <> 0 THEN
        RAISE EXCEPTION 'Unbalanced ledger entry %: net balance is % across % lines',
        target_transaction_id, net_balance, line_count
        USING ERRCODE = 'check_violation';
    END IF;

    RETURN NULL;
END
$$;

-- Trigger to use above function
CREATE CONSTRAINT TRIGGER ensure_balanced_ledger
AFTER INSERT ON ledger
DEFERRABLE INITIALLY DEFERRED
FOR EACH ROW
EXECUTE FUNCTION check_balanced_ledger_entry();

COMMIT;