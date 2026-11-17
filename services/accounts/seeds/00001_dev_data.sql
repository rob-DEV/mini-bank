-- Dev-only seed data. Run with: goose -dir services/accounts/seeds -no-versioning up
--
-- Fixed, memorable UUIDs so curl commands survive every reset:
--   customers         00000000-0000-7000-8001-0000000000NN
--   customer accounts 00000000-0000-7000-8002-0000000000NN   (01-10 current, 11-13 savings)
--   system accounts   00000000-0000-7000-8003-0000000000NN   (01 = funding)
--   system principal  00000000-0000-7000-8000-000000000000   (initiated_by for system work)

-- +goose Up
INSERT INTO customers (id, first_name, last_name, email, date_of_birth)
SELECT ('00000000-0000-7000-8001-' || lpad(c.n::text, 12, '0'))::uuid,
    c.first_name, c.last_name, c.email, c.dob::date
FROM (VALUES
          ( 1, 'Aoife',   'Murphy',    'aoife.murphy@example.com',  '1994-03-12'),
          ( 2, 'Ciaran',  'Kelly',     'ciaran.kelly@example.com',  '1988-11-02'),
          ( 3, 'Priya',   'Shah',      'priya.shah@example.com',    '1991-07-23'),
          ( 4, 'Tom',     'Henderson', 'tom.henderson@example.com', '1979-01-30'),
          ( 5, 'Niamh',   'O''Neill',  'niamh.oneill@example.com',  '2000-05-17'),
          ( 6, 'James',   'Okafor',    'james.okafor@example.com',  '1985-09-08'),
          ( 7, 'Sophie',  'Clarke',    'sophie.clarke@example.com', '1997-12-01'),
          ( 8, 'Mateusz', 'Nowak',     'mateusz.nowak@example.com', '1990-04-14'),
          ( 9, 'Hannah',  'Brown',     'hannah.brown@example.com',  '1983-06-26'),
          (10, 'Dev',     'Patel',     'dev.patel@example.com',     '1995-10-19')
     ) AS c (n, first_name, last_name, email, dob);

-- The bank's funding account: where opening balances come from. It goes
-- negative by design — every penny a customer holds is matched here.
INSERT INTO accounts (id, kind, account_type) VALUES
    ('00000000-0000-7000-8003-000000000001', 'system', 'internal');

-- One current account per customer (accounts 01-10).
INSERT INTO accounts (id, kind, customer_id, account_type, sort_code, account_number)
SELECT ('00000000-0000-7000-8002-' || lpad(i::text, 12, '0'))::uuid,
    'customer',
       ('00000000-0000-7000-8001-' || lpad(i::text, 12, '0'))::uuid,
    'current',
       '990001',
       '1000' || lpad(i::text, 4, '0')
FROM generate_series(1, 10) AS i;

-- Savings accounts for the first three customers (accounts 11-13).
INSERT INTO accounts (id, kind, customer_id, account_type, sort_code, account_number)
SELECT ('00000000-0000-7000-8002-' || lpad((10 + i)::text, 12, '0'))::uuid,
    'customer',
       ('00000000-0000-7000-8001-' || lpad(i::text, 12, '0'))::uuid,
    'savings',
       '990001',
       '2000' || lpad(i::text, 4, '0')
FROM generate_series(1, 3) AS i;

-- +goose Down
TRUNCATE accounts, customers;
