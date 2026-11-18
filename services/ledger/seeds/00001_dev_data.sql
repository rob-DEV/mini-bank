-- Dev-only seed data. Same fixed UUIDs as the accounts service seed:
--   customer accounts 00000000-0000-7000-8002-0000000000NN   (01-10 current, 11-13 savings)
--   system accounts   00000000-0000-7000-8003-0000000000NN   (01 = funding)
--   system principal  00000000-0000-7000-8000-000000000000

-- +goose Up
INSERT INTO ledger_accounts (id, kind, currency, allow_negative) VALUES
    ('00000000-0000-7000-8003-000000000001', 'system', 'GBP', true);

INSERT INTO ledger_accounts (id, kind, currency)
SELECT ('00000000-0000-7000-8002-' || lpad(i::text, 12, '0'))::uuid, 'customer', 'GBP'
FROM generate_series(1, 13) AS i;

-- Opening balances: £1,000 for accounts 01-10 (current), £5,000 for 11-13 (savings).
INSERT INTO transactions (id, kind, initiated_by, idempotency_key, request_hash)
SELECT gen_random_uuid(),
       'opening_balance',
       '00000000-0000-7000-8000-000000000000',
       'opening:' || id,
       sha256(convert_to('opening:' || id, 'UTF8'))
FROM ledger_accounts
WHERE kind = 'customer';

INSERT INTO entries (transaction_id, account_id, amount_minor, currency)
SELECT t.id, leg.account_id, leg.amount_minor, 'GBP'
FROM ledger_accounts a
         JOIN transactions t
              ON t.initiated_by = '00000000-0000-7000-8000-000000000000'
                  AND t.idempotency_key = 'opening:' || a.id
         CROSS JOIN LATERAL (
    SELECT CASE WHEN right(a.id::text, 2)::int >= 11 THEN 500000 ELSE 100000 END::bigint AS amt
    ) AS o
    CROSS JOIN LATERAL (
VALUES ('00000000-0000-7000-8003-000000000001'::uuid, -o.amt),
    (a.id,                                          o.amt)
    ) AS leg (account_id, amount_minor)
WHERE a.kind = 'customer';

-- +goose Down
TRUNCATE entries, transactions, ledger_accounts;