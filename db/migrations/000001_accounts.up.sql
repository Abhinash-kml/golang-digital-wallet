BEGIN;

CREATE TYPE account_type AS ENUM('SAVINGS', 'CURRENT');

CREATE TABLE IF NOT EXISTS accounts(
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    name TEXT NOT NULL,
    type account_type DEFAULT 'SAVINGS',
    isFrozen BOOLEAN DEFAULT FALSE
);

INSERT INTO accounts(name, type)
VALUES 
('systems', 'CURRENT'),
('holding', 'CURRENT');

COMMIT;