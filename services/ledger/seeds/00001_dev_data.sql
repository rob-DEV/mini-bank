-- +goose Up

-- Opening balances: one transaction per customer account,
-- £1,000 for current accounts and £5,000 for savings.
-- Seed transaction ids are random v4 (gen_random_uuid); the service uses v7.
INSERT INTO transactions (id, kind, initiated_by, idempotency_key, request_hash)
SELECT gen_random_uuid(),
       'opening_balance',
       '00000000-0000-7000-8000-000000000000',
       'opening:' || id,
       sha256(convert_to('opening:' || id, 'UTF8'))
FROM accounts
WHERE kind = 'customer';

-- Two legs each: funding account debited, customer account credited.
INSERT INTO entries (transaction_id, account_id, amount_minor, currency)
SELECT t.id, leg.account_id, leg.amount_minor, 'GBP'
FROM accounts a
         JOIN transactions t
              ON t.initiated_by = '00000000-0000-7000-8000-000000000000'
                  AND t.idempotency_key = 'opening:' || a.id
         CROSS JOIN LATERAL (
    VALUES ('00000000-0000-7000-8003-000000000001'::uuid,
            -(CASE a.account_type WHEN 'savings' THEN 500000 ELSE 100000 END)::bigint),
           (a.id,
            (CASE a.account_type WHEN 'savings' THEN 500000 ELSE 100000 END)::bigint)
        ) AS leg (account_id, amount_minor)
WHERE a.kind = 'customer';

-- +goose Down
TRUNCATE entries, transactions;
