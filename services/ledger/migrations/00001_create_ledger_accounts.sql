-- +goose Up
-- The ledger's own view of an account. Same id as the accounts service uses,
-- but no foreign key across services: the ledger keeps only what it needs to
-- post money correctly.
CREATE TABLE ledger_accounts (
                                 id              uuid        PRIMARY KEY,
                                 kind            text        NOT NULL CHECK (kind IN ('customer', 'system')),
                                 currency        char(3)     NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    allow_negative  boolean     NOT NULL DEFAULT false,  -- true only for system accounts like funding
    status          text        NOT NULL DEFAULT 'open'
                                CHECK (status IN ('open', 'frozen', 'closed')),
    created_at      timestamptz NOT NULL DEFAULT now(),

    -- Customer accounts can never be allowed to go negative.
    CONSTRAINT customer_never_negative CHECK (kind = 'system' OR allow_negative = false)
);

-- +goose Down
DROP TABLE ledger_accounts;